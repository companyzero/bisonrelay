package clientintf

import (
	"crypto/rand"
	"encoding/json"
	"fmt"

	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/sw"
	"github.com/decred/dcrd/bech32"
	"github.com/decred/dcrd/chaincfg/chainhash"
)

// PaidInviteKey is the encryption key that is used in paid invites.
type PaidInviteKey struct {
	key *[32]byte
}

// RVPoint calculates the RV point that corresponds to this key.
func (pik PaidInviteKey) RVPoint() ratchet.RVPoint {
	return ratchet.RVPoint(chainhash.HashFunc(pik.key[:]))
}

// Encrypt a message with this key.
func (pik PaidInviteKey) Encrypt(message []byte) ([]byte, error) {
	return sw.Seal(message, pik.key)
}

// Decrypt a message with this key.
func (pik PaidInviteKey) Decrypt(box []byte) ([]byte, error) {
	message, ok := sw.Open(box, pik.key)
	if !ok {
		return nil, fmt.Errorf("unable to decrypt message with PaidInviteKey")
	}
	return message, nil
}

// Encode this key as a string.
func (pik PaidInviteKey) Encode() (string, error) {
	return bech32.EncodeFromBase256("brpik", pik.key[:])
}

// MarshalJSON marshals the id into a json string.
func (pik PaidInviteKey) MarshalJSON() ([]byte, error) {
	s, err := pik.Encode()
	if err != nil {
		return nil, err
	}
	return json.Marshal(s)
}

// Decode the key from its string encoding.
func (pik *PaidInviteKey) Decode(s string) error {
	hrp, keyBytes, err := bech32.DecodeToBase256(s)
	if err != nil {
		return fmt.Errorf("unable to decode paid invite key: %v", err)
	}
	if hrp != "brpik" {
		return fmt.Errorf("hrp for string is not brpik")
	}
	if len(keyBytes) != 32 {
		return fmt.Errorf("incorrect length for decoded paid "+
			"invite key: %d != %d", len(keyBytes), 32)
	}
	var key [32]byte
	copy(key[:], keyBytes)
	for i := range keyBytes {
		keyBytes[i] = 0
	}
	pik.key = &key
	return nil
}

// UnmarshalJSON unmarshals the json representation of a ShortID.
func (pik *PaidInviteKey) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	return pik.Decode(s)
}

// String returns the encoded paid invite key or an error string.
func (pik PaidInviteKey) String() string {
	enc, err := pik.Encode()
	if err != nil {
		return fmt.Sprintf("[invalid PaidInviteKey: %v]", err)
	}
	return enc
}

// DecodePaidInviteKey decodes a given string as a PaidInviteKey.
func DecodePaidInviteKey(s string) (PaidInviteKey, error) {
	var pik PaidInviteKey
	err := pik.Decode(s)
	return pik, err
}

// GeneratePaidInviteKey generates a new, cryptographically secure paid invite
// key.
func GeneratePaidInviteKey() PaidInviteKey {
	var key [32]byte
	n, err := rand.Read(key[:])
	if err != nil {
		panic(fmt.Errorf("failed to generate random bytes: %v", err))
	}
	if n != len(key) {
		panic(fmt.Errorf("read too few bytes: %d < %d", n, len(key)))
	}
	return PaidInviteKey{key: &key}
}
