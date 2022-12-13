// Copyright (c) 2016 Company 0, LLC.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package ratchet

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/ratchet/disk"
	"github.com/companyzero/bisonrelay/sw"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/companyzero/sntrup4591761"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/ed25519"
)

type client struct {
	PrivateKey     zkidentity.FixedSizeSntrupPrivateKey
	PublicKey      zkidentity.FixedSizeSntrupPublicKey
	SigningPrivate zkidentity.FixedSizeEd25519PrivateKey
	SigningPublic  zkidentity.FixedSizeEd25519PublicKey
	Identity       zkidentity.ShortID
}

func newClient() *client {
	ed25519Pub, ed25519Priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	ntruprimePub, ntruprimePriv, err := sntrup4591761.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	identity := sha256.Sum256(ntruprimePub[:])

	c := client{}
	copy(c.SigningPrivate[:], ed25519Priv[:])
	copy(c.SigningPublic[:], ed25519Pub[:])
	copy(c.PrivateKey[:], ntruprimePriv[:])
	copy(c.PublicKey[:], ntruprimePub[:])
	copy(c.Identity[:], identity[:])

	return &c
}

func pairedRatchet(t *testing.T) (a, b *Ratchet) {
	alice := newClient()
	bob := newClient()

	a = New(rand.Reader)
	a.MyPrivateKey = &alice.PrivateKey
	a.TheirPublicKey = &bob.PublicKey

	b = New(rand.Reader)
	b.MyPrivateKey = &bob.PrivateKey
	b.TheirPublicKey = &alice.PublicKey

	kxA, kxB := new(KeyExchange), new(KeyExchange)
	if err := a.FillKeyExchange(kxA); err != nil {
		t.Fatal(err)
	}
	if err := b.FillKeyExchange(kxB); err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteKeyExchange(kxB, false); err != nil {
		t.Fatal(err)
	}
	if err := b.CompleteKeyExchange(kxA, true); err != nil {
		t.Fatal(err)
	}

	return
}

func TestNonce(t *testing.T) {
	a, b := pairedRatchet(t)

	//t.Logf("sendCount %v %v", a.sendCount, b.recvCount)
	msg := []byte(strings.Repeat("test message", 1024*1024))
	encrypted, err := a.Encrypt(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	//t.Logf("sendCount %v %v", a.sendCount, b.recvCount)
	result, err := b.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msg, result) {
		t.Fatalf("result doesn't match: %x vs %x", msg, result)
	}

	encrypted, err = a.Encrypt(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	//t.Logf("sendCount %v %v", a.sendCount, b.recvCount)
	result, err = b.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msg, result) {
		t.Fatalf("result doesn't match: %x vs %x", msg, result)
	}

	// XXX
	encrypted, err = b.Encrypt(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	//t.Logf("sendCount %v %v", a.sendCount, b.recvCount)
	result, err = a.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msg, result) {
		t.Fatalf("result doesn't match: %x vs %x", msg, result)
	}
}

func TestExchange(t *testing.T) {
	a, b := pairedRatchet(t)

	msg := []byte(strings.Repeat("test message", 1024*1024))
	encrypted, err := a.Encrypt(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	result, err := b.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msg, result) {
		t.Fatalf("result doesn't match: %x vs %x", msg, result)
	}
}

func TestDrain(t *testing.T) {
	a, b := pairedRatchet(t)

	msg := []byte("test message")
	for i := 0; i < 5; i++ {
		// alice -> bob
		//printRecvDrain(t, "1a", a)
		//printRecvDrain(t, "1b", b)
		encrypted, err := a.Encrypt(nil, msg)
		if err != nil {
			t.Fatal(err)
		}
		//printRecvDrain(t, "2a", a)
		//printRecvDrain(t, "2b", b)
		result, err := b.Decrypt(encrypted)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(msg, result) {
			t.Fatalf("result doesn't match: %x vs %x", msg, result)
		}
		//printRecvDrain(t, "3a", a)
		//printRecvDrain(t, "3b", b)

		// bob -> alice
		encrypted, err = b.Encrypt(nil, msg)
		if err != nil {
			t.Fatal(err)
		}
		result, err = a.Decrypt(encrypted)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(msg, result) {
			t.Fatalf("result doesn't match: %x vs %x", msg, result)
		}
	}
}

func TestMeh(t *testing.T) {
	a, b := pairedRatchet(t)

	//dumpHeaderKeys(t, "1a ", a)
	//dumpHeaderKeys(t, "1b ", b)
	//printRecvDrain(t, "1a", a)
	//printRecvDrain(t, "1b", b)
	msg := []byte(strings.Repeat("test message", 1024*1024))
	encrypted, err := a.Encrypt(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	//printRecvDrain(t, "2a", a)
	//printRecvDrain(t, "2b", b)
	//dumpHeaderKeys(t, "2a ", a)
	//dumpHeaderKeys(t, "2b ", b)
	result, err := b.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	//printRecvDrain(t, "3a", a)
	//printRecvDrain(t, "3b", b)
	//dumpHeaderKeys(t, "3a ", a)
	//dumpHeaderKeys(t, "3b ", b)
	if !bytes.Equal(msg, result) {
		t.Fatalf("result doesn't match: %x vs %x", msg, result)
	}

	// bob and alice reply, changing alice send rendezvous
	encrypted, err = b.Encrypt(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	//printRecvDrain(t, "4a", a)
	//printRecvDrain(t, "4b", b)
	_, err = a.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	//printRecvDrain(t, "5a", a)
	//printRecvDrain(t, "5b", b)
	_, err = a.Encrypt(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	//printRecvDrain(t, "6a", a)
	//printRecvDrain(t, "6b", b)

	_, err = b.Encrypt(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	//printRecvDrain(t, "7a", a)
	//printRecvDrain(t, "7b", b)
}

func TestBigSkip(t *testing.T) {
	a, b := pairedRatchet(t)
	//dumpHeaderKeys(t, "a", a)
	//dumpHeaderKeys(t, "b", b)
	//t.Logf("==================")

	// XX uncomment to make big skip skip a bunch of blobs and "work" again
	var (
		encrypted []byte
		//e         []byte
		err error
	)
	msg := []byte(strings.Repeat("test message", 1024*1024))
	// Breaks at 82, maxMissingMessages = 80 + currentKey + nextKey
	for i := 0; i < maxMissingMessages+2; i++ {
		//e = encrypted
		encrypted, err = a.Encrypt(nil, msg)
		if err != nil {
			t.Fatal(err)
		}
	}
	//result, err := b.Decrypt(e)
	//if err != nil {
	//	t.Fatal(err)
	//}
	// XXX header keys remain unchanged
	result, err := b.Decrypt(encrypted)
	//dumpHeaderKeys(t, "a", a)
	//dumpHeaderKeys(t, "b", b)
	//t.Logf("==================")
	if err == nil {
		// This is expected in this test
		t.Fatal("xxx")
	}
	_ = result
	//if !bytes.Equal(msg, result) {
	//	t.Fatalf("result doesn't match: %x vs %x", msg, result)
	//}
}

func TestBothWays(t *testing.T) {
	a, b := pairedRatchet(t)
	//dumpHeaderKeys(t, "a", a)
	//dumpHeaderKeys(t, "b", b)
	//t.Logf("==================")

	msg := []byte(strings.Repeat("test message", 1024*1024))
	encrypted, err := a.Encrypt(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	result, err := b.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msg, result) {
		t.Fatalf("result doesn't match: %x vs %x", msg, result)
	}

	encrypted2, err := b.Encrypt(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	result2, err := a.Decrypt(encrypted2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msg, result2) {
		t.Fatalf("result doesn't match: %x vs %x", msg, result)
	}
	//dumpHeaderKeys(t, "a", a)
	//dumpHeaderKeys(t, "b", b)
	//t.Logf("==================")

	// Header keys only ratchet once the next side sends
}

func TestBreak(t *testing.T) {
	a, b := pairedRatchet(t)
	//dumpChainKeys(t, "a", a)
	//dumpChainKeys(t, "b", b)
	//t.Logf("==================")

	msg := []byte(strings.Repeat("test message", 1024*1024))
	encrypted, err := a.Encrypt(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	result, err := b.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msg, result) {
		t.Fatalf("result doesn't match: %x vs %x", msg, result)
	}
	//dumpChainKeys(t, "a", a)
	//dumpChainKeys(t, "b", b)
	//t.Logf("==================")

	_, err = b.Decrypt(encrypted)
	if err == nil {
		t.Fatal("can't go backwards")
	}

	// Encrypt something and skip one decrypt
	_, err = a.Encrypt(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	//dumpChainKeys(t, "a", a)
	//dumpChainKeys(t, "b", b)
	//t.Logf("==================")
	encrypted3, err := a.Encrypt(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.Decrypt(encrypted3)
	if err != nil {
		t.Fatal(err)
	}
	//dumpChainKeys(t, "a", a)
	//dumpChainKeys(t, "b", b)
	//t.Logf("==================")
}

type scriptAction struct {
	// object is one of sendA, sendB or sendDelayed. The first two options
	// cause a message to be sent from one party to the other. The latter
	// causes a previously delayed message, identified by id, to be
	// delivered.
	object int
	// result is one of deliver, drop or delay. If delay, then the message
	// is stored using the value in id. This value can be repeated later
	// with a sendDelayed.
	result int
	id     int
}

const (
	sendA = iota
	sendB
	sendDelayed
	deliver
	drop
	delay
)

func reinitRatchet(t *testing.T, r *Ratchet) *Ratchet {
	state := r.DiskState(1 * time.Hour)
	newR := New(rand.Reader)
	newR.MyPrivateKey = r.MyPrivateKey
	if err := newR.Unmarshal(state); err != nil {
		t.Fatalf("Failed to unmarshal: %s", err)
	}

	return newR
}

func testScript(t *testing.T, script []scriptAction) {
	type delayedMessage struct {
		msg       []byte
		encrypted []byte
		fromA     bool
	}
	delayedMessages := make(map[int]delayedMessage)
	a, b := pairedRatchet(t)

	for i, action := range script {
		switch action.object {
		case sendA, sendB:
			sender, receiver := a, b
			if action.object == sendB {
				sender, receiver = receiver, sender
			}

			var msg [20]byte
			rand.Reader.Read(msg[:])
			encrypted, err := sender.Encrypt(nil, msg[:])
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}
			switch action.result {
			case deliver:
				result, err := receiver.Decrypt(encrypted)
				if err != nil {
					t.Fatalf("#%d: receiver returned error: %s", i, err)
				}
				if !bytes.Equal(result, msg[:]) {
					t.Fatalf("#%d: bad message: got %x, not %x", i, result, msg[:])
				}
			case delay:
				if _, ok := delayedMessages[action.id]; ok {
					t.Fatalf("#%d: already have delayed message with id %d", i, action.id)
				}
				delayedMessages[action.id] = delayedMessage{msg[:], encrypted, sender == a}
			case drop:
			}
		case sendDelayed:
			delayed, ok := delayedMessages[action.id]
			if !ok {
				t.Fatalf("#%d: no such delayed message id: %d", i, action.id)
			}

			receiver := a
			if delayed.fromA {
				receiver = b
			}

			result, err := receiver.Decrypt(delayed.encrypted)
			if err != nil {
				t.Fatalf("#%d: receiver returned error: %s", i, err)
			}
			if !bytes.Equal(result, delayed.msg) {
				t.Fatalf("#%d: bad message: got %x, not %x", i, result, delayed.msg)
			}
		}

		a = reinitRatchet(t, a)
		b = reinitRatchet(t, b)
	}
}

func TestBackAndForth(t *testing.T) {
	testScript(t, []scriptAction{
		{sendA, deliver, -1},
		{sendB, deliver, -1},
		{sendA, deliver, -1},
		{sendB, deliver, -1},
		{sendA, deliver, -1},
		{sendB, deliver, -1},
	})
}

func TestReorder(t *testing.T) {
	testScript(t, []scriptAction{
		{sendA, deliver, -1},
		{sendA, delay, 0},
		{sendA, deliver, -1},
		{sendDelayed, deliver, 0},
	})
}

func TestReorderAfterRatchet(t *testing.T) {
	testScript(t, []scriptAction{
		{sendA, deliver, -1},
		{sendA, delay, 0},
		{sendB, deliver, -1},
		{sendA, deliver, -1},
		{sendB, deliver, -1},
		{sendDelayed, deliver, 0},
	})
}

func TestDrop(t *testing.T) {
	testScript(t, []scriptAction{
		{sendA, drop, -1},
		{sendA, drop, -1},
		{sendA, drop, -1},
		{sendA, drop, -1},
		{sendA, deliver, -1},
		{sendB, deliver, -1},
	})
}

func TestLots(t *testing.T) {
	testScript(t, []scriptAction{
		{sendA, deliver, -1},
		{sendA, deliver, -1},
		{sendA, deliver, -1},
		{sendA, deliver, -1},
		{sendA, deliver, -1},
		{sendA, deliver, -1},
		{sendA, deliver, -1},
		{sendA, deliver, -1},
		{sendA, deliver, -1},
		{sendA, deliver, -1},
		{sendA, deliver, -1},
		{sendA, deliver, -1},
		{sendA, deliver, -1},
		{sendA, deliver, -1},
		{sendA, deliver, -1},
		{sendA, deliver, -1},
		{sendA, deliver, -1},
		{sendA, deliver, -1},
		{sendA, deliver, -1},
		{sendA, deliver, -1},
		{sendB, deliver, -1},
		{sendB, deliver, -1},
		{sendB, deliver, -1},
		{sendB, deliver, -1},
		{sendB, deliver, -1},
		{sendB, deliver, -1},
		{sendB, deliver, -1},
		{sendB, deliver, -1},
		{sendB, deliver, -1},
		{sendB, deliver, -1},
		{sendB, deliver, -1},
		{sendB, deliver, -1},
		{sendB, deliver, -1},
		{sendB, deliver, -1},
		{sendB, deliver, -1},
		{sendB, deliver, -1},
		{sendB, deliver, -1},
		{sendB, deliver, -1},
		{sendB, deliver, -1},
		{sendB, deliver, -1},
	})
}

func TestHalfDiskState(t *testing.T) {
	alice := newClient()
	bob := newClient()

	// half ratchet
	a := New(rand.Reader)
	a.MyPrivateKey = &alice.PrivateKey
	a.TheirPublicKey = &bob.PublicKey

	// full ratchet
	b := New(rand.Reader)
	b.MyPrivateKey = &bob.PrivateKey
	b.TheirPublicKey = &alice.PublicKey

	kxB := new(KeyExchange)
	if err := b.FillKeyExchange(kxB); err != nil {
		panic(err)
	}

	// remainder of alice
	kxA := new(KeyExchange)
	if err := a.FillKeyExchange(kxA); err != nil {
		panic(err)
	}
	if err := a.CompleteKeyExchange(kxB, false); err != nil {
		panic(err)
	}

	// return kx to bob
	if err := b.CompleteKeyExchange(kxA, true); err != nil {
		panic(err)
	}
}

func TestDiskState(t *testing.T) {
	a, b := pairedRatchet(t)

	msg := []byte("test message")
	encrypted, err := a.Encrypt(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	result, err := b.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msg, result) {
		t.Fatalf("result doesn't match: %x vs %x", msg, result)
	}

	encrypted, err = b.Encrypt(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	result, err = a.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msg, result) {
		t.Fatalf("result doesn't match: %x vs %x", msg, result)
	}

	// save alice ratchet state to disk
	as := a.DiskState(time.Hour)
	asJ, err := json.Marshal(as)
	if err != nil {
		t.Fatal(err)
	}
	af, err := os.CreateTemp("", "alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = af.Write(asJ); err != nil {
		t.Fatal(err)
	}
	if err = af.Close(); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(af.Name())

	// save bob ratchet state to disk
	bs := b.DiskState(time.Hour)
	bsJ, err := json.Marshal(bs)
	if err != nil {
		t.Fatal(err)
	}
	bf, err := os.CreateTemp("", "bob")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = bf.Write(bsJ); err != nil {
		t.Fatal(err)
	}
	if err = bf.Close(); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(bf.Name())

	// Read back
	afBytes, err := os.ReadFile(af.Name())
	if err != nil {
		t.Fatal(err)
	}
	var diskAlice disk.RatchetState
	err = json.Unmarshal(afBytes, &diskAlice)
	if err != nil {
		t.Fatal(err)
	}
	newAlice := New(rand.Reader)
	err = newAlice.Unmarshal(&diskAlice)
	if err != nil {
		t.Fatal(err)
	}

	// read back bob
	bfBytes, err := os.ReadFile(bf.Name())
	if err != nil {
		t.Fatal(err)
	}
	var diskBob disk.RatchetState
	err = json.Unmarshal(bfBytes, &diskBob)
	if err != nil {
		t.Fatal(err)
	}
	newBob := New(rand.Reader)
	err = newBob.Unmarshal(&diskBob)
	if err != nil {
		t.Fatal(err)
	}

	// send message to alice
	encrypted, err = newBob.Encrypt(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	result, err = newAlice.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msg, result) {
		t.Fatalf("result doesn't match: %x vs %x", msg, result)
	}

	encrypted, err = newAlice.Encrypt(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	result, err = newBob.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msg, result) {
		t.Fatalf("result doesn't match: %x vs %x", msg, result)
	}
}

func FillKeyExchangeWithPublicPoint(r *Ratchet, kx *KeyExchange, pub *[32]byte) error {
	c, k, err := sntrup4591761.Encapsulate(r.rand, (*sntrup4591761.PublicKey)(r.TheirPublicKey))
	if err != nil {
		return err
	}
	packed, err := sw.Seal(pub[:], k)
	if err != nil {
		return err
	}

	r.myHalf = k
	copy(kx.Cipher[:], c[:])
	kx.Public = packed

	return nil
}

func testECDHpoint(t *testing.T, a *Ratchet, pubDH *[32]byte) error {
	alice := newClient()
	bob := newClient()

	a.MyPrivateKey = &alice.PrivateKey
	a.TheirPublicKey = &bob.PublicKey

	b := New(rand.Reader)
	b.MyPrivateKey = &bob.PrivateKey
	b.TheirPublicKey = &alice.PublicKey

	kxA, kxB := new(KeyExchange), new(KeyExchange)
	if err := FillKeyExchangeWithPublicPoint(a, kxA, pubDH); err != nil {
		t.Fatal(err)
	}
	if err := b.FillKeyExchange(kxB); err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteKeyExchange(kxB, false); err != nil {
		return err
	}
	if err := b.CompleteKeyExchange(kxA, true); err != nil {
		return err
	}

	return nil
}

func TestECDHpoints(t *testing.T) {
	a := New(rand.Reader)
	pubDH := new([32]byte)
	// test 1: dh = 0
	err := testECDHpoint(t, a, pubDH)
	if err == nil {
		panic("invalid ECDH kx succeeded")
	}
	// test 2: dh = 1
	a = New(rand.Reader)
	pubDH[0] = 1
	err = testECDHpoint(t, a, pubDH)
	if err == nil {
		panic("invalid ECDH kx succeeded")
	}
	// test 3: Dh = 2^256 - 1
	a = New(rand.Reader)
	for i := 0; i < 32; i++ {
		pubDH[i] = 0xff
	}
	err = testECDHpoint(t, a, pubDH)
	if err == nil {
		panic("invalid ECDH kx succeeded")
	}
	// test 4: make sure testECDHpoint() works
	a = New(rand.Reader)
	curve25519.ScalarBaseMult(pubDH, a.kxPrivate)
	err = testECDHpoint(t, a, pubDH)
	if err != nil {
		panic("valid ECDH kx failed")
	}
}

func TestImpersonation(t *testing.T) {
	alice := newClient()
	bob := newClient()
	chris := newClient()

	b := New(rand.Reader)
	b.MyPrivateKey = &bob.PrivateKey

	c := New(rand.Reader)
	c.MyPrivateKey = &chris.PrivateKey

	// pair Bob and Chris
	b.TheirPublicKey = &chris.PublicKey
	c.TheirPublicKey = &bob.PublicKey

	// kx from Bob to Chris
	kxBC := new(KeyExchange)
	if err := b.FillKeyExchange(kxBC); err != nil {
		t.Fatal(err)
	}
	// kx from Chris to Bob
	kxCB := new(KeyExchange)
	if err := c.FillKeyExchange(kxCB); err != nil {
		t.Fatal(err)
	}
	if err := c.CompleteKeyExchange(kxBC, false); err != nil {
		t.Fatal(err)
	}
	if err := b.CompleteKeyExchange(kxCB, true); err != nil {
		t.Fatal(err)
	}

	// Chris knows Bob's public key, and will now impersonate Bob to Alice.
	a := New(rand.Reader)
	a.MyPrivateKey = &alice.PrivateKey

	notB := New(rand.Reader)
	notB.MyPrivateKey = &chris.PrivateKey // I am actually Chris...

	// Alice thinks she's talking to Bob
	a.TheirPublicKey = &bob.PublicKey

	// While notBob (Chris) knows it's talking to Alice
	notB.TheirPublicKey = &alice.PublicKey

	kxCA := new(KeyExchange)
	if err := notB.FillKeyExchange(kxCA); err != nil {
		t.Fatal(err)
	}
	kxAC := new(KeyExchange)
	if err := a.FillKeyExchange(kxAC); err != nil {
		t.Fatal(err)
	}
	// Here, Chris (notB) is able to complete a kx with Alice on behalf of
	// Bob. Notice that this also works with bogus Dh, Dh1 values:
	// for i := 0; i < len(kxCA.Dh); i++ {
	// 	kxCA.Dh[i] = 0
	// }
	// for i := 0; i < len(kxCA.Dh1); i++ {
	// 	kxCA.Dh1[i] = 0
	// }
	// ^ These could be set to 1 to leak part of a zkclient's private key.
	if err := a.CompleteKeyExchange(kxCA, false); err != nil {
		t.Fatal(err)
	}
	if err := notB.CompleteKeyExchange(kxAC, true); err == nil {
		t.Fatal("kx should not have completed")
	}
}

func TestEncryptSize(t *testing.T) {
	// Fixed size test.
	gotSize := EncryptedSize(128)
	wantSize := 24 + (64 + 16) + (128 + 16)
	if gotSize != wantSize {
		t.Fatalf("unexpected size -- got %d, want %d", gotSize, wantSize)
	}

	// Double check with an actual Encrypt() call.
	a, _ := pairedRatchet(t)
	msg := []byte(strings.Repeat("test message", 1024*1024))
	encrypted, err := a.Encrypt(nil, msg)
	if err != nil {
		t.Fatal(err)
	}
	wantSize = len(encrypted)
	gotSize = EncryptedSize(len(msg))
	if gotSize != wantSize {
		t.Fatalf("unexpected double check size -- got %d, want %d",
			gotSize, wantSize)
	}
}
