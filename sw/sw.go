package sw

// Package sw (secretbox wrap) wraps nacl/secretbox and hides the very awkward
// append interface.

import (
	"crypto/rand"
	"io"

	"golang.org/x/crypto/nacl/secretbox"
)

// MinPackedEncryptedSize is the minimum size of an encrypted message packed
// with a nonce.
const MinPackedEncryptedSize = 24 + secretbox.Overhead

// Seal encrypts a message with the provided key. Behind the scenes it adds a
// random nonce and returns an encrypted blob that is prefixed by the nonce.
func Seal(message []byte, key *[32]byte) ([]byte, error) {
	// random nonce
	var nonce [24]byte
	_, err := io.ReadFull(rand.Reader, nonce[:])
	if err != nil {
		return nil, err
	}

	// encrypt data
	return secretbox.Seal(nonce[:], message, &nonce, key), nil
}

// Open decrypts a message with the provided key. It uses the prefixed nonce
// and returns the decrypted message and true. If the message is corrupt or
// could not be decrypted it returns false.
func Open(box []byte, key *[32]byte) ([]byte, bool) {
	// Peel nonce out
	var nonce [24]byte
	copy(nonce[:], box[:24])
	return secretbox.Open(nil, box[24:], &nonce, key)
}

// PackedEncryptedSize returns the estimated size for an encrypted sw message
// with a prepended (packed) nonce, given the specified payload msg size.
func PackedEncryptedSize(msgSize int) int {
	// The output slice for an Encrypt() call is modified by appending:
	//
	//   [nonce][seal(msg)]
	//
	// nonce is a 24 byte slice.
	//
	// seal(x) appends len(x) + secretbox.Overhead bytes.
	return 24 + // nonce length
		msgSize + secretbox.Overhead // len(seal(msg))
}
