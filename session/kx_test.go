// Copyright (c) 2016,2017 Company 0, LLC.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package session

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/companyzero/sntrup4591761"
	"github.com/davecgh/go-spew/spew"
	"golang.org/x/sync/errgroup"
)

func loadIdentities(t *testing.T) (alice, bob *zkidentity.FullIdentity) {
	blob, err := os.ReadFile("testdata/alice.json")
	if err != nil {
		t.Fatal(err)
	}
	err = json.Unmarshal(blob, &alice)
	if err != nil {
		t.Fatal(err)
	}
	blob, err = os.ReadFile("testdata/bob.json")
	if err != nil {
		t.Fatal(err)
	}
	err = json.Unmarshal(blob, &bob)
	if err != nil {
		t.Fatal(err)
	}
	return alice, bob
}

func newIdentities(t *testing.T) (alice, bob *zkidentity.FullIdentity) {
	alice, err := zkidentity.New("Alice The Malice", "alice")
	if err != nil {
		t.Fatal(err)
	}
	bob, err = zkidentity.New("Bob The Builder", "bob")
	if err != nil {
		t.Fatal(err)
	}
	return alice, bob
}

func TestLessKX(t *testing.T) {
	// Alice sends bob encapsulated shared key sk
	// Alice sends bob ephemeral key ae encrypted with sk

	// Bob receives and decapsulates sk
	// Bob receives alice encrypted ephemeral key ae and decrypts it with sk
	clientNonce := true
	serverNonce := !clientNonce
	_ = clientNonce
	_ = serverNonce
	cn := newSequence(halfForClient)
	sn := newSequence(halfForServer)
	t.Logf("c: %x", cn.Nonce())
	t.Logf("s: %x", sn.Nonce())
	for i := 0; i < 100000000; i++ {
		sn.Decrease()
		cn.Decrease()
	}
	t.Logf("clientNonce: %x", cn.Nonce())
	t.Logf("serverNonce: %x", sn.Nonce())

	// Generate ntrup client session key
	alicePub, alicePriv, err := sntrup4591761.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// Generate ntruo well-known server key
	bobPub, bobPriv, err := sntrup4591761.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// Client sends encrypted random key to well-known server
	sessionEncryptedSharedKey, sessionSharedKey, err := sntrup4591761.Encapsulate(rand.Reader, bobPub)
	if err != nil {
		t.Fatal(err)
	}

	// Server receives encrypted key from client
	decaptext, ok := sntrup4591761.Decapsulate(sessionEncryptedSharedKey, bobPriv)
	if ok != 1 {
		t.Fatal("decap error")
	}
	t.Log(spew.Sdump(decaptext))

	if !bytes.Equal(decaptext[:], sessionSharedKey[:]) {
		t.Fatal("not equal")
	}

	_ = alicePriv
	_ = alicePub
}

func testKX(t *testing.T, alice, bob *zkidentity.FullIdentity) {
	loadIdentities(t)

	msg := []byte("this is a message of sorts")
	eg := errgroup.Group{}
	wait := make(chan bool)
	eg.Go(func() error {
		listener, err := net.Listen("tcp", "127.0.0.1:12346")
		if err != nil {
			wait <- false
			return err
		}
		defer listener.Close()
		wait <- true // start client

		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		defer conn.Close()

		bobKX := new(KX)
		bobKX.Conn = conn
		bobKX.MaxMessageSize = 4096
		bobKX.OurPublicKey = &bob.Public.Key
		bobKX.OurPrivateKey = &bob.PrivateKey
		t.Logf("bob fingerprint: %v", bob.Public.Fingerprint())

		err = bobKX.Respond()
		if err != nil {
			return err
		}

		// read
		received, err := bobKX.Read()
		if err != nil {
			return err
		}
		if !bytes.Equal(received, msg) {
			return fmt.Errorf("message not identical")
		}

		// write
		return bobKX.Write(msg)
	})

	ok := <-wait
	if !ok {
		t.Fatalf("server not started")
	}

	conn, err := net.Dial("tcp", "127.0.0.1:12346")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	aliceKX := new(KX)
	aliceKX.Conn = conn
	aliceKX.MaxMessageSize = 4096
	aliceKX.OurPublicKey = &alice.Public.Key
	aliceKX.OurPrivateKey = &alice.PrivateKey
	aliceKX.TheirPublicKey = &bob.Public.Key
	t.Logf("alice fingerprint: %v", alice.Public.Fingerprint())

	err = aliceKX.Initiate()
	if err != nil {
		t.Fatalf("initiator %v", err)
	}

	err = aliceKX.Write(msg)
	if err != nil {
		t.Error(err)
		// fallthrough
	} else {
		// read
		received, err := aliceKX.Read()
		if err != nil {
			t.Error(err)
			// fallthrough
		} else if !bytes.Equal(received, msg) {
			t.Errorf("message not identical")
			// fallthrough
		}
	}

	if err := eg.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestStaticIdentities(t *testing.T) {
	alice, bob := loadIdentities(t)
	testKX(t, alice, bob)
}

func TestRandomIdentities(t *testing.T) {
	alice, bob := newIdentities(t)
	testKX(t, alice, bob)
}
