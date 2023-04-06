package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/client/internal/lowlevel"
	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/slog"
	"golang.org/x/sync/errgroup"
)

type kxList struct {
	q             rmqIntf
	rmgr          rdzvManagerIntf
	id            *zkidentity.FullIdentity
	randReader    io.Reader
	db            *clientdb.DB
	ctx           context.Context
	dbCtx         context.Context
	compressLevel int

	kxCompleted func(*zkidentity.PublicIdentity, *ratchet.Ratchet,
		clientdb.RawRVID, clientdb.RawRVID, clientdb.RawRVID)

	log slog.Logger
}

func newKXList(q rmqIntf, rmgr rdzvManagerIntf, id *zkidentity.FullIdentity,
	db *clientdb.DB, ctx context.Context) *kxList {
	return &kxList{
		q:          q,
		rmgr:       rmgr,
		id:         id,
		db:         db,
		randReader: rand.Reader,
		log:        slog.Disabled,
		ctx:        ctx,
		dbCtx:      ctx,
	}
}

// makePaidForRMCB generates a function to be added to the rawRM type such that
// the function is called once that RM is sent.
func (kx *kxList) makePaidForRMCB(uid UserID, event string) func(int64, int64) {
	return func(amount, fees int64) {
		// Amount is set to negative due to being an outbound payment.
		amount = -amount
		fees = -fees

		err := kx.db.Update(kx.dbCtx, func(tx clientdb.ReadWriteTx) error {
			return kx.db.RecordUserPayEvent(tx, uid, event, amount, fees)
		})
		if err != nil {
			kx.log.Warnf("Unable to store payment %d of event %q: %v", amount,
				event, err)
		}
	}
}

// createInvite creates a new invite that can be used to create a ratchet with
// a remote party.
func (kx *kxList) createInvite(w io.Writer, invitee *zkidentity.PublicIdentity,
	mediator *clientintf.UserID, isForReset bool, funds *rpc.InviteFunds) (rpc.OOBPublicIdentityInvite, error) {

	var rv, resetRV [32]byte
	if _, err := io.ReadFull(kx.randReader, rv[:]); err != nil {
		return rpc.OOBPublicIdentityInvite{}, err
	}
	if _, err := io.ReadFull(kx.randReader, resetRV[:]); err != nil {
		return rpc.OOBPublicIdentityInvite{}, err
	}

	// Write the invite.
	pii := rpc.OOBPublicIdentityInvite{
		Public:            kx.id.Public,
		InitialRendezvous: rv,
		ResetRendezvous:   resetRV,
		Funds:             funds,
	}
	if w != nil {
		jw := json.NewEncoder(w)
		if err := jw.Encode(pii); err != nil {
			return pii, fmt.Errorf("unable to encode kx invite: %w", err)
		}
	}

	// Track the invite in the DB.
	kxd := clientdb.KXData{
		Stage:      clientdb.KXStageStep2IDKX,
		InitialRV:  rv,
		MyResetRV:  resetRV,
		Timestamp:  time.Now(),
		Invitee:    invitee,
		MediatorID: mediator,
		IsForReset: isForReset,
	}
	err := kx.db.Update(kx.dbCtx, func(tx clientdb.ReadWriteTx) error {
		return kx.db.SaveKX(tx, kxd)
	})
	if err != nil {
		return pii, err
	}

	// Subscribe to the invite RV.
	if err := kx.listenInvite(&kxd); err != nil {
		return pii, fmt.Errorf("unable to listen to new invite: %v", err)
	}

	kx.log.Infof("KX %s: Created invite", kxd.InitialRV.ShortLogID())
	return pii, nil
}

// createPrepaidInvite creates an invite that is pushed to the server and
// pre-paid.
func (kx *kxList) createPrepaidInvite(w io.Writer, funds *rpc.InviteFunds) (
	invite rpc.OOBPublicIdentityInvite, key clientintf.PaidInviteKey,
	err error) {

	// Create the invite.
	var b bytes.Buffer
	invite, err = kx.createInvite(&b, nil, nil, false, funds)
	if err != nil {
		return
	}

	// Encrypt the invite.
	plainInvite := b.Bytes()
	key = clientintf.GeneratePaidInviteKey()
	var encrypted []byte
	encrypted, err = key.Encrypt(plainInvite)
	if err != nil {
		return
	}

	// Determine the invite RV.
	inviteRV := key.RVPoint()

	// Prepay the invite.
	err = kx.rmgr.PrepayRVSub(inviteRV, nil)
	if err != nil {
		return
	}

	// Push the invite data.
	rm := rawRM{
		rv:  inviteRV,
		msg: encrypted,
	}
	err = kx.q.SendRM(rm)
	if err != nil {
		return
	}

	// Copy to the external writer.
	if _, err = w.Write(plainInvite); err != nil {
		return
	}

	return
}

// fetchPrepaidInvite attempts to fetch a prepaid invite with the server.
func (kx *kxList) fetchPrepaidInvite(ctx context.Context, key clientintf.PaidInviteKey, w io.Writer) (rpc.OOBPublicIdentityInvite, error) {
	// Fetch the data from the server.
	var invite rpc.OOBPublicIdentityInvite
	blob, err := kx.rmgr.FetchPrepaidRV(ctx, key.RVPoint())
	if err != nil {
		return invite, err
	}

	// Decrypt blob data.
	decrypted, err := key.Decrypt(blob.Decoded)
	if err != nil {
		return invite, fmt.Errorf("unable to decrypt: %v", err)
	}

	// Decode OOBPI.
	err = json.Unmarshal(decrypted, &invite)
	if err != nil {
		return invite, fmt.Errorf("unable to decode OOBPI: %v", err)
	}

	// Copy to writer.
	if _, err := w.Write(decrypted); err != nil {
		return invite, err
	}

	return invite, nil
}

// decodeInvite decodes an invite from an io.Reader.
func (kx *kxList) decodeInvite(r io.Reader) (rpc.OOBPublicIdentityInvite, error) {
	var pii rpc.OOBPublicIdentityInvite
	jr := json.NewDecoder(r)
	err := jr.Decode(&pii)
	return pii, err
}

// acceptInvite accepts the given invite from a remote party. It sends a
// message on the initial RV and waits for a reply.
func (kx *kxList) acceptInvite(pii rpc.OOBPublicIdentityInvite, isForReset bool) error {
	// Make sure we don't add ourselves
	identity := pii.Public.Identity
	if bytes.Equal(kx.id.Public.Identity[:], identity[:]) {
		return fmt.Errorf("can't perform kx with self")
	}
	err := kx.db.View(context.Background(), func(tx clientdb.ReadTx) error {
		if kx.db.IsBlocked(tx, identity) {
			return fmt.Errorf("%s: %w", identity, errUserBlocked)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sendRV := pii.InitialRendezvous

	// Setup a new ratchet
	hr, kxRatchet, err := rpc.NewHalfRatchetKX(kx.id, pii.Public)
	if err != nil {
		return fmt.Errorf("could not setup ratchet key exchange: %v",
			err)
	}

	// Generate the new response rv.
	var rv, resetRV [32]byte
	if _, err := io.ReadFull(kx.randReader, rv[:]); err != nil {
		return fmt.Errorf("could not setup obtain entropy: %v", err)
	}
	if _, err := io.ReadFull(kx.randReader, resetRV[:]); err != nil {
		return fmt.Errorf("could not setup obtain entropy: %v", err)
	}

	// Update the DB with the current stage of the kx process.
	kxd := clientdb.KXData{
		Public:       pii.Public,
		Stage:        clientdb.KXStageStep3IDKX,
		InitialRV:    sendRV,
		Step3RV:      rv,
		HalfRatchet:  hr.DiskState(31 * 24 * time.Hour),
		MyResetRV:    resetRV,
		TheirResetRV: pii.ResetRendezvous,
		Timestamp:    time.Now(),
		IsForReset:   isForReset,
	}
	err = kx.db.Update(kx.dbCtx, func(tx clientdb.ReadWriteTx) error {
		return kx.db.SaveKX(tx, kxd)
	})
	if err != nil {
		return err
	}

	// Start listening to the expected reply.
	if err := kx.listenInvite(&kxd); err != nil {
		return fmt.Errorf("unable to listen to accepted invite: %v", err)
	}

	// Send the RMOHalfKX response to the remote user.
	kx.log.Infof("KX %s: accepting invite from %q id %s",
		sendRV.ShortLogID(), pii.Public.Nick,
		pii.Public.Identity)

	rmohk := rpc.RMOHalfKX{
		Public:            kx.id.Public,
		HalfKX:            *kxRatchet,
		InitialRendezvous: rv,
		ResetRendezvous:   resetRV,
	}
	rm := rawRM{
		rv:       sendRV,
		paidRMCB: kx.makePaidForRMCB(pii.Public.Identity, "kx.acceptinvite"),
	}
	rm.msg, err = rpc.EncryptRMO(rmohk, pii.Public, kx.compressLevel)
	if err != nil {
		return fmt.Errorf("unable to encrypt RMOHalfKX: %v", err)
	}
	err = kx.q.SendRM(rm)
	if err != nil {
		return err
	}

	return nil
}

func (kx *kxList) handleStep2IDKX(kxid clientdb.RawRVID, blob lowlevel.RVBlob) error {
	// Perform step2IDKX.

	// Decode and decrypt the RMOHalfRatchet msg.
	rmohk, err := rpc.DecryptOOBHalfKXBlob(blob.Decoded, &kx.id.PrivateKey)
	if err != nil {
		return fmt.Errorf("step2IDKX DecryptOOBHalfKXBlob: %v", err)
	}

	if bytes.Equal(rmohk.Public.Identity[:], kx.id.Public.Identity[:]) {
		return fmt.Errorf("can't kx with self")
	}

	sendRV := rmohk.InitialRendezvous

	// Create full ratchet from rmohk.
	r, fkx, err := rpc.NewFullRatchetKX(kx.id, rmohk.Public, &rmohk.HalfKX)
	if err != nil {
		return fmt.Errorf("could not create full ratchet: %v", err)
	}

	kx.log.Debugf("KX %s: sending RMOFullKX to RV %s", kxid.ShortLogID(), sendRV)

	// Send RMOFullKX to the other end.
	rmofkx := rpc.RMOFullKX{FullKX: *fkx}
	rm := rawRM{
		rv:       sendRV,
		paidRMCB: kx.makePaidForRMCB(rmohk.Public.Identity, "kx.step2idkx"),
	}
	rm.msg, err = rpc.EncryptRMO(rmofkx, rmohk.Public, kx.compressLevel)
	if err != nil {
		return err
	}
	err = kx.q.SendRM(rm)
	if err != nil {
		return err
	}

	kx.log.Infof("KX %s: completed ratchet setup with guest %q id %s",
		kxid.ShortLogID(), rmohk.Public.Nick, rmohk.Public.Identity)

	// Success! The remote side sent all we needed to complete kx, so we
	// now have a fully setup ratchet to use.

	// Unsub from the now completed kx.
	err = kx.rmgr.Unsub(blob.ID)
	if err != nil {
		kx.log.Warnf("Unable to unsubscribe from step2 kx RV: %v", err)
	}

	// Remove completed kx from DB.
	var kxd clientdb.KXData
	err = kx.db.Update(kx.dbCtx, func(tx clientdb.ReadWriteTx) error {
		var err error
		kxd, err = kx.db.GetKX(tx, kxid)
		if err != nil {
			return err
		}
		return kx.db.DeleteKX(tx, kxid)
	})
	if err != nil {
		return err
	}

	// Alert client of completed kx.
	if kx.kxCompleted != nil {
		kx.kxCompleted(&rmohk.Public, r, kxd.InitialRV, kxd.MyResetRV, rmohk.ResetRendezvous)
	}
	return nil
}

func (kx *kxList) handleStep3IDKX(kxid clientdb.RawRVID, blob lowlevel.RVBlob) error {
	// Decrypt remote msg.
	fullKX, err := rpc.DecryptOOBFullKXBlob(blob.Decoded, &kx.id.PrivateKey)
	if err != nil {
		return fmt.Errorf("step3IDKX DecryptOOBFullKXBlob: %v", err)
	}

	var r *ratchet.Ratchet
	var public zkidentity.PublicIdentity

	// Load half ratchet from db.
	var kxd clientdb.KXData
	err = kx.db.Update(kx.dbCtx, func(tx clientdb.ReadWriteTx) error {
		var err error
		kxd, err = kx.db.GetKX(tx, kxid)
		if err != nil {
			return err
		}

		if kxd.HalfRatchet == nil {
			return fmt.Errorf("nil half ratchet in db")
		}

		r = ratchet.New(rand.Reader)
		err = r.Unmarshal(kxd.HalfRatchet)
		if err != nil {
			return fmt.Errorf("could not unmarshal Ratchet")
		}

		public = kxd.Public
		r.MyPrivateKey = &kx.id.PrivateKey
		r.TheirPublicKey = &public.Key

		// Complete the key exchange.
		err = r.CompleteKeyExchange(&fullKX.FullKX, true)
		if err != nil {
			return fmt.Errorf("could not complete key exchange: %v",
				err)
		}

		// Completed successfully! Delete in-progress kx from DB.
		return kx.db.DeleteKX(tx, kxid)
	})
	if err != nil {
		return err
	}

	// Success! We now have a complete ratchet _and_ the remote user also
	// has a complete ratchet. We're ready to comm!
	kx.log.Infof("KX %s: completed ratchet setup with host %q id %s",
		kxid.ShortLogID(), public.Nick, public.Identity)

	// Unsub from the now completed kx.
	err = kx.rmgr.Unsub(blob.ID)
	if err != nil {
		kx.log.Warnf("Unable to unsubscribe from step2 kx RV: %v", err)
	}

	// Alert client of completed kx.
	if kx.kxCompleted != nil {
		kx.kxCompleted(&public, r, kxid, kxd.MyResetRV, kxd.TheirResetRV)
	}

	return nil
}

// listenInvite listens for a KX step of the given in-progress kx. The action
// taken depends on the current stage of the kx.
func (kx *kxList) listenInvite(kxd *clientdb.KXData) error {
	var rv ratchet.RVPoint
	var kxHandler func(clientdb.RawRVID, lowlevel.RVBlob) error

	var subPaidHandler lowlevel.SubPaidHandler

	switch kxd.Stage {
	case clientdb.KXStageStep2IDKX:
		rv = kxd.InitialRV
		kxHandler = kx.handleStep2IDKX

		// MediatorID will be nil in manually created invites, where
		// we don't know yet the ID of the remote user.
		if kxd.MediatorID != nil {
			payType := "step2IDKX"
			if kxd.IsForReset {
				payType = "step2IDKX_reset"
			}
			payEvent := fmt.Sprintf("sub.%s", payType)
			subPaidHandler = kx.makePaidForRMCB(*kxd.MediatorID, payEvent)
		}
	case clientdb.KXStageStep3IDKX:
		rv = kxd.Step3RV
		kxHandler = kx.handleStep3IDKX
		subPaidHandler = kx.makePaidForRMCB(kxd.Public.Identity, "sub.step3IDKX")
	default:
		return fmt.Errorf("unknown kdx data to listen on: %d", kxd.Stage)
	}

	// Close over the id.
	handler := func(blob lowlevel.RVBlob) error {
		// Called as a goroutine to immediately ack the received message.
		go func() {
			err := kxHandler(kxd.InitialRV, blob)
			if err != nil && !errors.Is(err, clientintf.ErrSubsysExiting) {
				kx.log.Errorf("Error during KX %s stage %s: %v",
					kxd.InitialRV, kxd.Stage, err)
			}
		}()
		return nil
	}

	kx.log.Debugf("KX %s: Listening at stage %s in RV %s",
		rv.ShortLogID(), kxd.Stage, rv)

	return kx.rmgr.Sub(rv, handler, subPaidHandler)
}

// listenAllKXs listens for all outstanding kxs in the db. KXs which are older
// than the passed kxExpiryLimit are dropped.
func (kx *kxList) listenAllKXs(kxExpiryLimit time.Duration) error {
	var kxs []clientdb.KXData
	err := kx.db.Update(kx.dbCtx, func(tx clientdb.ReadWriteTx) error {
		var err error
		kxs, err = kx.db.ListKXs(tx)
		if err != nil {
			return err
		}

		// Remove any KXs that have expired. We remove all KXs which
		// timestamp is older then now()-kxExpiryLimit (i.e. keep all
		// which timestamp is after the limit).
		limit := time.Now().Add(-kxExpiryLimit)
		for i := 0; i < len(kxs); {
			if kxs[i].Timestamp.After(limit) {
				i += 1
				continue
			}

			kx.log.Infof("Removing stale KX %s (created %s)",
				kxs[i].InitialRV, kxs[i].Timestamp.Format(time.RFC3339))
			if err := kx.db.DeleteKX(tx, kxs[i].InitialRV); err != nil {
				return err
			}

			// Modify the list.
			if i < len(kxs)-1 {
				kxs[i] = kxs[len(kxs)-1]
			}
			kxs = kxs[:len(kxs)-1]
		}

		return err
	})
	if err != nil {
		return err
	}

	// Listen on all KXs in parallel, so that only a single sub message is
	// sent to server.
	g := &errgroup.Group{}
	for _, kxd := range kxs {
		kxd := kxd
		g.Go(func() error {
			err := kx.listenInvite(&kxd)
			if errors.Is(err, lowlevel.ErrRVAlreadySubscribed{}) {
				// This error might happen when the invite is
				// created before the client is first connected
				// to the server and can be safely ignored.
				err = nil
			}
			return err
		})
	}

	return g.Wait()
}

// requestReset sends a new invite to the given rv point, which should be a
// reset RV of the specified remote user.
func (kx *kxList) requestReset(rv clientdb.RawRVID, id *zkidentity.PublicIdentity) error {
	invite, err := kx.createInvite(nil, nil, &id.Identity, true, nil)
	if err != nil {
		return err
	}

	packed, err := rpc.EncryptRMO(invite, *id, kx.compressLevel)
	if err != nil {
		return err
	}
	rm := rawRM{
		rv:       rv,
		msg:      packed,
		paidRMCB: kx.makePaidForRMCB(id.Identity, "kx.requestReset"),
	}
	return kx.q.SendRM(rm)
}

// handleReset is called when we receive a msg in a reset RV point meant for
// the given user.
func (kx *kxList) handleReset(id *zkidentity.PublicIdentity, blob lowlevel.RVBlob) error {
	pii, err := rpc.DecryptOOBPublicIdentityInvite(blob.Decoded,
		&kx.id.PrivateKey)
	if err != nil {
		return fmt.Errorf("handleReset DecryptOOBPublicIdentityInvite:"+
			" %v", err)
	}

	// Verify id is the same as the existing identity
	correctID := pii.Public.SigKey == id.SigKey &&
		pii.Public.Identity == id.Identity &&
		pii.Public.Key == id.Key
	if !correctID {
		return fmt.Errorf("handleReset: received unexpected public identity"+
			"(want %s, got %s)", id.Identity, pii.Public.Identity)
	}

	kx.log.Infof("Received reset cmd on RV %s from user %s (%q)",
		blob.ID, id.Identity, id.Nick)

	// Kickstart a new kx process.
	return kx.acceptInvite(*pii, true)
}

// listenReset listens for a reset invite from the given user in the specified
// id.
func (kx *kxList) listenReset(rv lowlevel.RVID, id *zkidentity.PublicIdentity) error {
	handler := func(blob lowlevel.RVBlob) error {
		// Called as a goroutine to immediately ack the received msg.
		go func() {
			err := kx.handleReset(id, blob)
			if err != nil && !errors.Is(err, clientintf.ErrSubsysExiting) {
				kx.log.Errorf("Error handling reset with %s: %v",
					id.Identity, err)
			}
		}()
		return nil
	}

	subPaidHandler := kx.makePaidForRMCB(id.Identity, "sub.resetRV")
	kx.log.Debugf("Listening to reset RV %s for user %s", rv, id.Identity)
	return kx.rmgr.Sub(rv, handler, subPaidHandler)
}

// unlistenReset stops listening to the specified reset rv.
func (kx *kxList) unlistenReset(rv lowlevel.RVID) {
	// Ignore errors since they are irrelevant here.
	_ = kx.rmgr.Unsub(rv)
}
