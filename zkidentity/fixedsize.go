package zkidentity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/companyzero/sntrup4591761"
)

// FixedSizeSignature is a 64-byte, fixed size signature. This is used as an
// alternative for 64-byte signatures to ensure compact encoding into json.
type FixedSizeSignature [64]byte

// FixedSizeEd25519PrivateKey is a 64-byte, fixed size private key.
type FixedSizeEd25519PrivateKey = FixedSizeSignature

// FixedSizeEd25519PublicKey is a 32-byte, fixed size ed25519 public key.
type FixedSizeEd25519PublicKey = ShortID

// FixedSizeDigest is a 32-byte, fixed size sha256 digest.
type FixedSizeDigest = ShortID

// FixedSizeSymmetricKey is a 32-byte, fixed size symmetric encryption key.
type FixedSizeSymmetricKey [32]byte

// NewFixedSizeEd25519KeyPair generates a new, random keypair.
func NewFixedSizeEd25519KeyPair() (*FixedSizeEd25519PrivateKey, *FixedSizeEd25519PublicKey) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		// Should not happen with crypto rand reader.
		panic(err)
	}

	var fixedPriv FixedSizeEd25519PrivateKey
	var fixedPub FixedSizeEd25519PublicKey

	copy(fixedPub[:], pub)
	copy(fixedPriv[:], priv)
	return &fixedPriv, &fixedPub
}

// String returns the hex encoding of the FixedSizeSignature.
func (u FixedSizeSignature) String() string {
	return hex.EncodeToString(u[:])
}

// MarshalJSON marshals the id into a json string.
func (u FixedSizeSignature) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

// UnmarshalJSON unmarshals the json representation of an FixedSizeSignature.
func (u *FixedSizeSignature) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	return u.FromString(s)
}

// FromString decodes s into an FixedSizeSignature. s must contain an
// hex-encoded signature of the correct length.
func (u *FixedSizeSignature) FromString(s string) error {
	h, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	if len(h) != len(u) {
		return fmt.Errorf("invalid FixedSizeSignature length: %d", len(h))
	}
	copy(u[:], h)
	return nil
}

// FromBytes copies the signature from the given byte slice. The passed slice
// must have the correct length.
func (u *FixedSizeSignature) FromBytes(b []byte) error {
	if len(b) != len(u) {
		return fmt.Errorf("invalid FixedSizeSignature length: %d", len(b))
	}
	copy(u[:], b)
	return nil
}

// FixedSizeSntrupPublicKey is a fixed size sntrup public key.
type FixedSizeSntrupPublicKey [sntrup4591761.PublicKeySize]byte

// String returns the hex encoding of the FixedSizeSntrupPublicKey.
func (u FixedSizeSntrupPublicKey) String() string {
	return hex.EncodeToString(u[:])
}

// MarshalJSON marshals the id into a json string.
func (u FixedSizeSntrupPublicKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

// UnmarshalJSON unmarshals the json representation of an FixedSizeSntrupPublicKey.
func (u *FixedSizeSntrupPublicKey) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	return u.FromString(s)
}

// FromString decodes s into an FixedSizeSntrupPublicKey. s must contain an hex-encoded FixedSizeSntrupPrivateKey of the
// correct length.
func (u *FixedSizeSntrupPublicKey) FromString(s string) error {
	h, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	if len(h) != len(u) {
		return fmt.Errorf("invalid FixedSizeSntrupPublicKey length: %d", len(h))
	}
	copy(u[:], h)
	return nil
}

// FromBytes copies the key from the given byte slice. The passed slice
// must have the correct length.
func (u *FixedSizeSntrupPublicKey) FromBytes(b []byte) error {
	if len(b) != len(u) {
		return fmt.Errorf("invalid FixedSizeSntrupPublicKey length: %d", len(b))
	}
	copy(u[:], b)
	return nil
}

func (u *FixedSizeSntrupPublicKey) Encapsulate() (*sntrup4591761.Ciphertext, *sntrup4591761.SharedKey) {
	cipher, key, err := sntrup4591761.Encapsulate(rand.Reader, (*sntrup4591761.PublicKey)(u))
	if err != nil {
		// crypto rand.Reader never errors.
		panic(err)
	}

	return cipher, key
}

// FixedSizeSntrupPrivateKey is a fixed size sntrup private key.
type FixedSizeSntrupPrivateKey [sntrup4591761.PrivateKeySize]byte

// String returns the hex encoding of the FixedSizeSntrupPrivateKey.
func (u FixedSizeSntrupPrivateKey) String() string {
	return hex.EncodeToString(u[:])
}

// MarshalJSON marshals the id into a json string.
func (u FixedSizeSntrupPrivateKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

// UnmarshalJSON unmarshals the json representation of an FixedSizeSntrupPrivateKey.
func (u *FixedSizeSntrupPrivateKey) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	return u.FromString(s)
}

// FromString decodes s into an FixedSizeSntrupPrivateKey. s must contain an hex-encoded ID of the
// correct length.
func (u *FixedSizeSntrupPrivateKey) FromString(s string) error {
	h, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	if len(h) != len(u) {
		return fmt.Errorf("invalid FixedSizeSntrupPrivateKey length: %d", len(h))
	}
	copy(u[:], h)
	return nil
}

// FromBytes copies the key from the given byte slice. The passed slice
// must have the correct length.
func (u *FixedSizeSntrupPrivateKey) FromBytes(b []byte) error {
	if len(b) != len(u) {
		return fmt.Errorf("invalid FixedSizeSntrupPrivateKey length: %d", len(b))
	}
	copy(u[:], b)
	return nil
}

func (u *FixedSizeSntrupPrivateKey) Decapsulate(b []byte, sk *[32]byte) bool {
	if len(b) < sntrup4591761.CiphertextSize {
		return false
	}
	var cipher sntrup4591761.Ciphertext
	copy(cipher[:], b)
	shared, ok := sntrup4591761.Decapsulate(&cipher, (*sntrup4591761.PrivateKey)(u))
	if ok != 1 {
		return false
	}
	copy(sk[:], shared[:])
	return true
}

func NewFixedSizeSntrupKeyPair() (*FixedSizeSntrupPrivateKey, *FixedSizeSntrupPublicKey) {
	pub, priv, err := sntrup4591761.GenerateKey(rand.Reader)
	if err != nil {
		panic(err) // Should never happen
	}
	return (*FixedSizeSntrupPrivateKey)(priv), (*FixedSizeSntrupPublicKey)(pub)
}

// FixedSizeSntrupCiphertext is a fixed size byte slice capable of holding a
// sntrup4591761 cipher text that encodes as an hex string in json.
type FixedSizeSntrupCiphertext [sntrup4591761.CiphertextSize]byte

// String returns the hex encoding of the FixedSizeSntrupCiphertext.
func (u FixedSizeSntrupCiphertext) String() string {
	return hex.EncodeToString(u[:])
}

// MarshalJSON marshals the id into a json string.
func (u FixedSizeSntrupCiphertext) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

// UnmarshalJSON unmarshals the json representation of a FixedSizeSntrupCiphertext.
func (u *FixedSizeSntrupCiphertext) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	return u.FromString(s)
}

// FromString decodes s into an FixedSizeSntrupCiphertext. s must contain an hex-encoded ID of the
// correct length.
func (u *FixedSizeSntrupCiphertext) FromString(s string) error {
	h, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	if len(h) != len(u) {
		return fmt.Errorf("invalid FixedSizeSntrupCiphertext length: %d", len(h))
	}
	copy(u[:], h)
	return nil
}

// FromBytes copies the short id from the given byte slice. The passed slice
// must have the correct length.
func (u *FixedSizeSntrupCiphertext) FromBytes(b []byte) error {
	if len(b) != len(u) {
		return fmt.Errorf("invalid FixedSizeSntrupCiphertext length: %d", len(b))
	}
	copy(u[:], b)
	return nil
}

// NewFixedSizeSymmetricKey creates a new, random fixed size symmetric key.
func NewFixedSizeSymmetricKey() *FixedSizeSymmetricKey {
	var res FixedSizeSymmetricKey
	rand.Read(res[:])
	return &res
}

// String returns the hex encoding of the FixedSizeSymmetricKey.
func (u FixedSizeSymmetricKey) String() string {
	return hex.EncodeToString(u[:])
}

// MarshalJSON marshals the key into a json string.
func (u FixedSizeSymmetricKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

// UnmarshalJSON unmarshals the json representation of an FixedSizeSymmetricKey.
func (u *FixedSizeSymmetricKey) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	return u.FromString(s)
}

// FromString decodes s into a FixedSizeSymmetricKey. s must contain an
// hex-encoded signature of the correct length.
func (u *FixedSizeSymmetricKey) FromString(s string) error {
	h, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	if len(h) != len(u) {
		return fmt.Errorf("invalid FixedSizeSymmetricKey length: %d", len(h))
	}
	copy(u[:], h)
	return nil
}
