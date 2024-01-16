package rpc

import (
	"bytes"
	"compress/zlib"
	"errors"
	"reflect"
	"testing"

	"github.com/companyzero/bisonrelay/zkidentity"
)

const testMaxDecompressSize = 1024 * 1024

func TestEncryptDecryptHalfRatchet(t *testing.T) {
	alice, err := zkidentity.New("Alice McMalice", "alice")
	if err != nil {
		t.Fatal(err)
	}

	bob, err := zkidentity.New("Bob Bobberino", "bob")
	if err != nil {
		t.Fatal(err)
	}

	// Alice creates new half ratchet and kx
	r, hkx, err := NewHalfRatchetKX(alice, bob.Public)
	if err != nil {
		t.Fatal(err)
	}
	_ = r

	// Alice creates half KX RPC
	halfKX, err := NewHalfKX(alice, hkx)
	if err != nil {
		t.Fatal(err)
	}

	// Alice encrypts and packs structure
	packed, err := EncryptRMO(*halfKX, bob.Public, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Bob decrypts packed blob
	halfKXAtBob, err := DecryptOOBHalfKXBlob(packed, &bob.PrivateKey, testMaxDecompressSize)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(halfKX, halfKXAtBob) {
		t.Fatal("not equal")
	}
}

func TestEncryptDecryptFullRatchet(t *testing.T) {
	alice, err := zkidentity.New("Alice McMalice", "alice")
	if err != nil {
		t.Fatal(err)
	}

	bob, err := zkidentity.New("Bob Bobberino", "bob")
	if err != nil {
		t.Fatal(err)
	}

	// Alice creates new half ratchet and kx
	_, hkx, err := NewHalfRatchetKX(alice, bob.Public)
	if err != nil {
		t.Fatal(err)
	}

	// Bob creates new full ratchet and kx
	_, fkx, err := NewFullRatchetKX(bob, alice.Public, hkx)
	if err != nil {
		t.Fatal(err)
	}

	// Bob creates full KX RPC
	fullKX, err := NewFullKX(fkx)
	if err != nil {
		t.Fatal(err)
	}

	// Bob encrypts and packs structure
	packed, err := EncryptRMO(*fullKX, alice.Public, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Alice decrypts packed blob
	fullKXAtAlice, err := DecryptOOBFullKXBlob(packed, &alice.PrivateKey, testMaxDecompressSize)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(fullKX, fullKXAtAlice) {
		t.Fatal("not equal")
	}
}

//func TestFullOOB(t *testing.T) {
//	alice, err := zkidentity.New("Alice McMalice", "alice")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	bob, err := zkidentity.New("Bob Bobberino", "bob")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	// Create OOB blob that Alice sends to Bob
//	alicePIBlob, err := CreateOOBPublicIdentityInviteBlob(alice.Public)
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	// Bob receives OOB public identity invite and prepares a half kx
//	var alicePI OOBPublicIdentityInvite
//	err = json.Unmarshal(alicePIBlob, &alicePI)
//	if err != nil {
//		t.Fatal(err)
//	}
//	hkxb, halfRatchetAtBob, err := CreateOOBHalfKXBlob(bob, alicePI.Public)
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	// Alice decrypts blob and sends full ratchet to bob
//	hkxd, err := DecryptOOBHalfKXBlob(hkxb, &alice.PrivateKey)
//	if err != nil {
//		t.Fatal(err)
//	}
//	fkxb, fullRatchetAtAlice, err := CreateOOBFullKXBlob(alice, bob.Public,
//		&hkxd.HalfKX)
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	// Bob decrypts blob and completes ratchet on his end.
//	fkxd, err := DecryptOOBFullKXBlob(fkxb, &bob.PrivateKey)
//	if err != nil {
//		t.Fatal(err)
//	}
//	err = halfRatchetAtBob.CompleteKeyExchange(&fkxd.FullKX, true)
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	// Test messaging alice to bob and bob to alice
//	a := fullRatchetAtAlice
//	b := halfRatchetAtBob
//	msg := []byte(strings.Repeat("test message", 1024))
//	encrypted, err := a.Encrypt(nil, msg)
//	if err != nil {
//		t.Fatal(err)
//	}
//	result, err := b.Decrypt(encrypted)
//	if err != nil {
//		t.Fatal(err)
//	}
//	if !bytes.Equal(msg, result) {
//		t.Fatalf("result doesn't match: %x vs %x", msg, result)
//	}
//
//	encrypted2, err := b.Encrypt(nil, msg)
//	if err != nil {
//		t.Fatal(err)
//	}
//	result2, err := a.Decrypt(encrypted2)
//	if err != nil {
//		t.Fatal(err)
//	}
//	if !bytes.Equal(msg, result2) {
//		t.Fatalf("result doesn't match: %x vs %x", msg, result)
//	}
//}

//func TestOOB(t *testing.T) {
//	alice, err := zkidentity.New("Alice McMalice", "alice")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	bob, err := zkidentity.New("Bob Bobberino", "bob")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	// alice sends OOB invite
//	aliceInvite := RMPublicIdentityInvite{
//		Public: alice.Public,
//	}
//
//	// bob creates a key and send it to alice
//	cipherText, sharedKeyAtBob, err := sntrup4591761.Encapsulate(rand.Reader,
//		&aliceInvite.Public.Key)
//	if err != nil {
//		t.Fatal(err)
//	}
//	bobInvite := RMPublicIdentityInvite{Public: bob.Public}
//	bobInviteBlob, err := json.Marshal(bobInvite)
//	if err != nil {
//		t.Fatal(err)
//	}
//	bobInviteBlobEncrypted, err := sw.Seal(bobInviteBlob, sharedKeyAtBob)
//	if err != nil {
//		t.Fatal(err)
//	}
//	packed := make([]byte, sntrup4591761.CiphertextSize+len(bobInviteBlobEncrypted))
//	copy(packed[0:], cipherText[:])
//	copy(packed[sntrup4591761.CiphertextSize:], bobInviteBlobEncrypted)
//
//	// alice receives packed blob @ rendezvous thus she knows it is a kx
//	var cipherTextAtAlice sntrup4591761.Ciphertext
//	copy(cipherTextAtAlice[:], packed[0:sntrup4591761.CiphertextSize])
//
//	// alice decrypts key and invite blob
//	sharedKeyAtAlice, n := sntrup4591761.Decapsulate(&cipherTextAtAlice,
//		&alice.PrivateKey)
//	if n != 1 {
//		t.Fatal("could not decap")
//	}
//	bobInviteBlobAtAlice, ok := sw.Open(packed[sntrup4591761.CiphertextSize:],
//		sharedKeyAtAlice)
//	if !ok {
//		t.Fatal("could not open")
//	}
//
//	// verify shared key
//	if !bytes.Equal(sharedKeyAtAlice[:], sharedKeyAtBob[:]) {
//		t.Fatal("not equal")
//	}
//
//	// verify decrypted invite blob
//	if !bytes.Equal(bobInviteBlobAtAlice[:], bobInviteBlob[:]) {
//		t.Fatal("blob not equal")
//	}
//
//	// verify unmarshaled invite
//	var bobInviteAtAlice RMPublicIdentityInvite
//	err = json.Unmarshal(bobInviteBlobAtAlice, &bobInviteAtAlice)
//	if err != nil {
//		t.Fatal(err)
//	}
//	if !reflect.DeepEqual(bobInviteAtAlice, bobInvite) {
//		t.Fatal("invite not equal")
//	}
//}

// TestDecomposeRMOLimitsZlibDeflate tests that the decompose function limits the
// max amount that can be decompressed from an encoded RMO message.
func TestDecomposeRMOLimitsZlibDeflate(t *testing.T) {
	// Figure out the max valid size when we prepend the blob with a valid
	// RMO message.
	validPrefix := `{"command":"ohalfkx"}` + "\n{" // Valid header and start of message.
	validSuffix := "}"                             // End of message.
	maxSize := testMaxDecompressSize - len(validPrefix) - 1

	tests := []struct {
		name    string
		size    int
		wantErr error
	}{{
		name:    "max size decompression",
		size:    maxSize,
		wantErr: nil,
	}, {
		name:    "one past max size decompression",
		size:    maxSize + 1,
		wantErr: errLimitedReaderExhausted,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Generate a small compressed stream that decompresses to a large
			// message.
			mb := &bytes.Buffer{}
			w, err := zlib.NewWriterLevel(mb, zlib.BestCompression)
			if err != nil {
				t.Fatal(err)
			}

			// Write [validPrefix][repeat of ' '][validSuffix]
			if _, err := w.Write([]byte(validPrefix)); err != nil {
				t.Fatal(err)
			}
			padding := bytes.Repeat([]byte{' '}, tc.size)
			if _, err := w.Write(padding); err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write([]byte(validSuffix)); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			bts := mb.Bytes()

			// Sanity check the generated message is small.
			if len(bts) > 10*1024 {
				t.Fatalf("Sanity check failed: compressed message is too large: %d",
					len(bts))
			}

			// Attempt to decompress it.
			_, _, err = DecomposeRMO(bts, testMaxDecompressSize)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("unexpected error: got %v, want %v", err, tc.wantErr)
			}
		})
	}
}
