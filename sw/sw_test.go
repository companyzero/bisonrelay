package sw

import (
	"bytes"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"golang.org/x/crypto/nacl/secretbox"
)

func TestSecretboxRaw(t *testing.T) {
	// This is a test that verifies how secretbox appends to the provided
	// slice.
	var key [32]byte
	var nonce [24]byte
	for i := 0; i < len(nonce); i++ {
		nonce[i] = 1
	}
	for i := 0; i < len(key); i++ {
		key[i] = 2
	}
	message := []byte("Hello, world!")
	out := make([]byte, 24)
	copy(out[:], nonce[:])
	box := secretbox.Seal(out, message[:], &nonce, &key)
	if !bytes.Equal(box[:24], out[:24]) {
		t.Fatalf("Seal didn't correctly append")
	}
	t.Logf("%v", spew.Sdump(box))

	var opened []byte
	var ok bool
	opened, ok = secretbox.Open(nil, box[24:], &nonce, &key)
	if !ok {
		t.Fatalf("failed to open box")
	}

	if !bytes.Equal(opened, message) {
		t.Fatalf("got %x, expected %x", opened, message)
	}
}

func TestSealOpen(t *testing.T) {
	var key [32]byte
	message := []byte("Hello, world!")
	encrypted, err := Seal(message, &key)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%v", spew.Sdump(encrypted))

	decrypted, ok := Open(encrypted, &key)
	if !ok {
		t.Fatal("not ok")
	}
	t.Logf("%v", spew.Sdump(decrypted))
}

func TestSizedSealOpen(t *testing.T) {
	// Make a stupid key
	var key [32]byte
	for i := byte(0); i < byte(len(key)); i++ {
		key[i] = i
	}

	for i := 128; i < 65536; i += 17 {
		ct := make([]byte, i)
		for x := 0; x < len(ct); x++ {
			ct[x] = 1
		}

		// Encrypt cleartext
		encrypted, err := Seal(ct, &key)
		if err != nil {
			t.Fatal(err)
		}
		//t.Logf("encrypted %v", spew.Sdump(encrypted))

		// Decrypt encrypted
		decrypted, ok := Open(encrypted, &key)
		if !ok {
			t.Fatal("not ok")
		}
		//t.Logf("decrypted %v", spew.Sdump(decrypted))

		// Verify we got the same cleartext
		if !bytes.Equal(ct, decrypted) {
			t.Fatal("not equal")
		}
	}
}
