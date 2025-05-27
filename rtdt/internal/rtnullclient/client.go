package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"math"
	mathrand "math/rand"
	"math/rand/v2"
	"net"
	"runtime"
	"slices"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/rpc"
	rtdtclient "github.com/companyzero/bisonrelay/rtdt/client"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/companyzero/sntrup4591761"
	"github.com/decred/slog"
	"github.com/puzpuzpuz/xsync/v3"
)

// packetWindowInterval is the max timeout to use to track burst windows. If a
// packet fails to arrive within this period, it is considered lost.
const packetWindowInterval = 5 * time.Second

var (
	// epochUnixMilli is the epoch for timestamp values.
	today          = time.Now()
	epochUnixMilli = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.Local)
)

type pregeneratedKeyPair struct {
	ciphertext *sntrup4591761.Ciphertext
	sharedKey  *sntrup4591761.SharedKey
}

type pregeneratedSharedKeys struct {
	mtx          sync.Mutex
	serverPubKey *zkidentity.FixedSizeSntrupPublicKey
	keys         []pregeneratedKeyPair
}

func (psk *pregeneratedSharedKeys) regen(ctx context.Context, n int) error {
	// Setup semaphore for parallel generation.
	sema := make(chan struct{}, max(runtime.NumCPU(), 1))
	for i := 0; i < cap(sema); i++ {
		sema <- struct{}{}
	}

	// Resize slice and keep track of slice of new keys to fill.
	psk.keys = slices.Grow(psk.keys, n)
	newKeys := psk.keys[len(psk.keys) : len(psk.keys)+n]
	psk.keys = psk.keys[:len(psk.keys)+n]

	// Generate in multiple goroutines to speed up generation.
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		<-sema
		wg.Add(1)
		go func(i int) {
			ciphertext, sharedKey := psk.serverPubKey.Encapsulate()
			newKeys[i] = pregeneratedKeyPair{ciphertext: ciphertext, sharedKey: sharedKey}
			sema <- struct{}{}
			wg.Done()
			if ctx.Err() != nil {
				return
			}
		}(i)

		if ctx.Err() != nil {
			break
		}
	}

	wg.Wait()
	return nil
}

func (psk *pregeneratedSharedKeys) NextKey() (*sntrup4591761.Ciphertext, *sntrup4591761.SharedKey) {
	psk.mtx.Lock()
	if len(psk.keys) == 0 {
		psk.regen(context.Background(), 1)
	}
	i := len(psk.keys) - 1
	kp := psk.keys[i]
	cipher, shared := kp.ciphertext, kp.sharedKey
	kp.ciphertext, kp.sharedKey = nil, nil
	psk.keys = psk.keys[:i]
	psk.mtx.Unlock()

	return cipher, shared
}

type client struct {
	basePeerID   rpc.RTDTPeerID
	log          slog.Logger
	serverAddr   *net.UDPAddr
	serverPubKey *zkidentity.FixedSizeSntrupPublicKey
	sigKey       *zkidentity.FixedSizeEd25519PrivateKey
	publisherKey *zkidentity.FixedSizeSymmetricKey
	cookieKey    *zkidentity.FixedSizeSymmetricKey

	readRoutines int

	sessionKeysGen *pregeneratedSharedKeys

	clients *xsync.MapOf[int, *rtdtclient.Client]

	ctx    context.Context
	cancel func()

	stats *stats

	mtx         sync.Mutex
	outRoutines map[time.Duration][]*outboundGoroutine
	inSessions  map[uint64]*inSession
}

// e2eKeysForBasePeerID generates consistent E2E sig and encryption keys given
// a peer id. The first 16 bits of id are used as a "base" peer id to derive
// the same keys without having to exchange data through side channels.
//
// Only to be used on this demo.
func e2eKeysForBasePeerID(id rpc.RTDTPeerID) (*zkidentity.FixedSizeEd25519PrivateKey, *zkidentity.FixedSizeEd25519PublicKey, *zkidentity.FixedSizeSymmetricKey) {
	var seed [32]byte
	copy(seed[:], bytes.Repeat([]byte{0x12}, 32))

	// Use the "base" peer id (lower 16 bits) of the full peer id to
	// generate unique per-peer keys.
	seed[0] = byte(id)
	seed[1] = byte(id >> 8)

	// Signature keys.
	rng := rand.NewChaCha8(seed)
	pub, priv, err := ed25519.GenerateKey(rng)
	if err != nil {
		panic(err)
	}
	var sigPub zkidentity.FixedSizeEd25519PublicKey
	var sigPriv zkidentity.FixedSizeEd25519PrivateKey
	copy(sigPub[:], pub)
	copy(sigPriv[:], priv)

	// Encryption key.
	var encKey zkidentity.FixedSizeSymmetricKey
	rng.Read(encKey[:])

	return &sigPriv, &sigPub, &encKey
}

func (c *client) newPeerKeys(id rpc.RTDTPeerID, sessRV *zkidentity.ShortID) (*zkidentity.FixedSizeEd25519PublicKey, *zkidentity.FixedSizeSymmetricKey) {
	// E2E is enabled.
	_, sigKey, encKey := e2eKeysForBasePeerID(id)

	if c.publisherKey == nil {
		// Disable E2E encryption.
		encKey = nil
	} else {
		c.log.Debugf("Initializing demo encryption key for peer %s as %x", id, encKey[:])
	}

	if c.sigKey == nil {
		// Disable authentication verification.
		sigKey = nil
	}

	return sigKey, encKey
}

func (c *client) rtClientForConn(connID int) (*rtdtclient.Client, error) {
	var err error
	rtClient, loaded := c.clients.LoadOrCompute(connID, func() *rtdtclient.Client {
		opts := []rtdtclient.Option{
			rtdtclient.WithLogger(c.log),
			rtdtclient.WithRandomStreamHandler(c.handlePacket),
			rtdtclient.WithPerConnReadRoutines(c.readRoutines),
			rtdtclient.WithNewPeerCallback(c.newPeerKeys),
			rtdtclient.WithPacketIOCallbacks(c.packetInCb, c.packetOutCb),
		}

		var c *rtdtclient.Client
		c, err = rtdtclient.New(opts...)
		return c
	})
	if !loaded && err != nil {
		c.clients.Delete(connID)
	} else if !loaded {
		c.log.Debugf("Created conn %d", connID)
	}
	return rtClient, err
}

func (c *client) reportBurst(b *inBurst, missed, duplicated int, burstDelay time.Duration, packetDelays []time.Duration) {
	if burstDelay != 0 {
		delta := float64(burstDelay-b.Interval) / float64(time.Millisecond)
		if delta >= 0 {
			c.stats.burstHisto.Observe(delta)
		} else {
			c.stats.burstNegHisto.Observe(-delta)
		}
		factor := float64(burstDelay) / float64(b.Interval)
		c.stats.burstFactorHisto.Observe(factor)
	}
	if missed > 0 {
		c.stats.missedPackets.Add(float64(missed))
		c.stats.discardedPktsAtomic.Add(uint64(missed))
	}
	if duplicated > 0 {
		c.stats.duplicatedPackets.Add(float64(duplicated))
	}
}

func (c *client) reportBurstDiscarded(b *inBurst, seq uint32) {
	c.stats.discardedPackets.Add(1)
	c.stats.discardedPktsAtomic.Add(1)
}

func (c *client) packetInCb(n int) {
	c.stats.bytesRead.Add(float64(n))
	c.stats.inPackets.Inc()
	c.stats.bytesReadAtomic.Add(uint64(n))
	c.stats.pktsReadAtomic.Add(1)
}

func (c *client) packetOutCb(n int) {
	c.stats.bytesWritten.Add(float64(n))
	c.stats.outPackets.Inc()
	c.stats.bytesWrittenAtomic.Add(uint64(n))
	c.stats.pktsWrittenAtomic.Add(1)
}

func (c *client) handlePacket(_ *rtdtclient.Session, enc *rpc.RTDTFramedPacket, plain *rpc.RTDTDataPacket) error {
	if len(plain.Data) < 18 {
		c.log.Warnf("Received packet with too little data (%d bytes) "+
			"from sessiond %s", len(plain.Data), enc.Source)
		return nil
	}

	// absTs := time.UnixMilli(int64(plain.Timestamp) + epochUnixMilli)
	absTs := epochUnixMilli.Add(time.Duration(plain.Timestamp) * time.Millisecond)
	absDelay := time.Since(absTs)
	absDelayMs := absDelay / time.Millisecond
	c.stats.absDelay.Observe(float64(absDelayMs))

	burstIdx := int(binary.BigEndian.Uint16(plain.Data))
	burstSeq := binary.BigEndian.Uint32(plain.Data[12:])
	burstPkt := binary.BigEndian.Uint16(plain.Data[16:])

	// The inbound session id (used to track stats) is the concatenation of
	// the source + target ids.
	inSessionID := uint64(enc.Source)<<32 | uint64(enc.Target)
	c.mtx.Lock()
	sess := c.inSessions[inSessionID]
	if sess == nil {
		sess = &inSession{}
		c.inSessions[inSessionID] = sess
	}
	nbSessions := len(c.inSessions)
	c.mtx.Unlock()

	sess.mtx.Lock()
	if burstIdx > len(sess.Bursts)-1 || sess.Bursts[burstIdx] == nil {
		burstInterval := time.Duration(binary.BigEndian.Uint64(plain.Data[2:]))
		burstPackets := binary.BigEndian.Uint16(plain.Data[10:])

		c.log.Infof("Receiving new burst %d with %d packets at %s interval id %x (total %d)",
			burstIdx, burstPackets, burstInterval, inSessionID, nbSessions)

		wantLen, missingLen := burstIdx+1, burstIdx-len(sess.Bursts)+1
		sess.Bursts = slices.Grow(sess.Bursts, missingLen)[:wantLen]
		sess.Bursts[burstIdx] = newInBurst(burstInterval, packetWindowInterval,
			burstPackets, c.reportBurst, c.reportBurstDiscarded)

		c.stats.inBursts.Add(1)
	}
	burst := sess.Bursts[burstIdx]
	burst.received(time.Now(), burstSeq, burstPkt)
	sess.mtx.Unlock()

	return nil
}

func (c *client) newSession(conn int, id rpc.RTDTPeerID, bursts []outBurst) error {
	rtClient, err := c.rtClientForConn(conn)
	if err != nil {
		return err
	}

	var joinCookie []byte
	if c.cookieKey != nil {
		jc := rpc.RTDTJoinCookie{
			Size:             1 << 16,
			PeerID:           id,
			PublishAllowance: math.MaxInt64, // Large allowance to avoid having to refresh.
			EndTimestamp:     time.Now().AddDate(1, 0, 0).Unix(),
			IsAdmin:          true,
			PaymentTag:       uint64(time.Now().UnixNano()),

			// Use the upper 16 bits of the id as unique identifier
			// of the session by setting server/owner secrets.
			ServerSecret: zkidentity.ShortID{0: byte(id >> 16)},
			OwnerSecret:  zkidentity.ShortID{0: byte(id >> 24)},
		}
		joinCookie = jc.Encrypt(nil, c.cookieKey)
	}

	cfg := rtdtclient.SessionConfig{
		ServerAddr:    c.serverAddr,
		LocalID:       id,
		SessionKeyGen: c.sessionKeysGen.NextKey,
		SigKey:        c.sigKey,
		JoinCookie:    joinCookie,
		PublisherKey:  c.publisherKey,
	}
	rtSess, err := rtClient.NewSession(c.ctx, cfg)
	if err != nil {
		return err
	}

	if len(bursts) == 0 {
		return nil
	}

	c.mtx.Lock()
	for burstID, b := range bursts {
		routines, ok := c.outRoutines[b.Interval]
		if !ok {
			// Init new outbound goroutines.
			c.log.Infof("Initializing %d gorountines for interval %s",
				routinesPerInterval, b.Interval)
			for i := 0; i < routinesPerInterval; i++ {
				g := newOutboundGoroutine(b.Interval, c.log, c.stats)
				go func() {
					err := g.Run(c.ctx)
					if err != nil && !errors.Is(err, context.Canceled) {
						c.log.Errorf("Error running outbound goroutine: %v", err)
					}
				}()
				routines = append(routines, g)
			}
			c.outRoutines[b.Interval] = routines
		}

		i := mathrand.Intn(len(routines))
		ob := outboundBurst{
			BurstID:    uint16(burstID),
			Packets:    b.Packets,
			PacketSize: b.PacketSize,
			RTSess:     rtSess,
		}
		routines[i].c <- ob
	}
	c.mtx.Unlock()

	return nil
}

type clientCfg struct {
	serverAddr   *net.UDPAddr
	serverPubKey *zkidentity.FixedSizeSntrupPublicKey
	cookieKey    *zkidentity.FixedSizeSymmetricKey
	readRoutines int
	log          slog.Logger
	basePeerID   rpc.RTDTPeerID
	sigKey       *zkidentity.FixedSizeEd25519PrivateKey
	publisherKey *zkidentity.FixedSizeSymmetricKey
}

func newClient(ctx context.Context, cfg clientCfg) (*client, error) {

	ctx, cancel := context.WithCancel(ctx)
	c := &client{
		log:            cfg.log,
		basePeerID:     cfg.basePeerID,
		ctx:            ctx,
		readRoutines:   cfg.readRoutines,
		cancel:         cancel,
		serverAddr:     cfg.serverAddr,
		serverPubKey:   cfg.serverPubKey,
		cookieKey:      cfg.cookieKey,
		clients:        xsync.NewMapOf[int, *rtdtclient.Client](),
		sigKey:         cfg.sigKey,
		publisherKey:   cfg.publisherKey,
		sessionKeysGen: &pregeneratedSharedKeys{serverPubKey: cfg.serverPubKey},

		inSessions:  make(map[uint64]*inSession),
		outRoutines: make(map[time.Duration][]*outboundGoroutine),
		stats:       newStats(),
	}

	c.log.Debugf("Pre-generating remote peer session keys...")
	if err := c.sessionKeysGen.regen(ctx, 2000); err != nil {
		return nil, err
	}
	c.log.Debugf("Pre-generated remote peer session keys")

	go func() {
		err := c.runReportStatsLoop(ctx, time.Second)
		if err != nil && !errors.Is(err, context.Canceled) {
			c.log.Warnf("Error running stats loop: %v", err)
		}
	}()

	return c, nil
}
