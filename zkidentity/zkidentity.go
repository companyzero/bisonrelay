// Copyright (c) 2016 Company 0, LLC.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// zkidentity package manages public and private identities.
package zkidentity

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/companyzero/sntrup4591761"
	"golang.org/x/crypto/ed25519"
)

var (
	prng = rand.Reader

	ErrNotEqual = errors.New("not equal")
	ErrVerify   = errors.New("verify error")
)

const (
	IdentitySize = sha256.Size
)

// A zkc public identity consists of a name and nick (e.g "John Doe" and "jd"
// respectively), a ed25519 public signature key, and a NTRU Prime public key
// (used to derive symmetric encryption keys). An extra Identity field, taken
// as the SHA256 of the NTRU public key, is used as a short handle to uniquely
// identify a user in various zkc structures.
type PublicIdentity struct {
	Name      string                    `json:"name"`
	Nick      string                    `json:"nick"`
	SigKey    FixedSizeEd25519PublicKey `json:"sigKey"`
	Key       FixedSizeSntrupPublicKey  `json:"key"`
	Identity  ShortID                   `json:"identity"`
	Digest    FixedSizeDigest           `json:"digest"`    // digest of name, keys and identity
	Signature FixedSizeSignature        `json:"signature"` // signature of Digest
	Avatar    []byte                    `json:"avatar"`
}

type FullIdentity struct {
	Public        PublicIdentity             `json:"publicIdentity"`
	PrivateSigKey FixedSizeEd25519PrivateKey `json:"privateSigKey"`
	PrivateKey    FixedSizeSntrupPrivateKey  `json:"privateKey"`
}

func NewWithRNG(name, nick string, prng io.Reader) (*FullIdentity, error) {
	ed25519Pub, ed25519Priv, err := ed25519.GenerateKey(prng)
	if err != nil {
		return nil, err
	}
	ntruprimePub, ntruprimePriv, err := sntrup4591761.GenerateKey(prng)
	if err != nil {
		return nil, err
	}
	identity := IdentityFromPub(ntruprimePub)

	fi := new(FullIdentity)
	fi.Public.Name = name
	fi.Public.Nick = nick
	copy(fi.Public.SigKey[:], ed25519Pub[:])
	copy(fi.Public.Key[:], ntruprimePub[:])
	copy(fi.Public.Identity[:], identity[:])
	copy(fi.PrivateSigKey[:], ed25519Priv[:])
	copy(fi.PrivateKey[:], ntruprimePriv[:])
	err = fi.RecalculateDigest()
	if err != nil {
		return nil, err
	}

	zero(ed25519Pub[:])
	zero(ed25519Priv[:])
	zero(ntruprimePub[:])
	zero(ntruprimePriv[:])

	return fi, nil
}

func New(name, nick string) (*FullIdentity, error) {
	return NewWithRNG(name, nick, prng)
}

// MustNew generates a new identity or panics.
func MustNew(name, nick string) *FullIdentity {
	id, err := New(name, nick)
	if err != nil {
		panic(err)
	}
	return id
}

func Fingerprint(id [IdentitySize]byte) string {
	return hex.EncodeToString(id[:])
}

func (fi *FullIdentity) RecalculateDigest() error {
	// calculate digest
	d := sha256.New()
	d.Write(fi.Public.SigKey[:])
	d.Write(fi.Public.Key[:])
	d.Write(fi.Public.Identity[:])
	copy(fi.Public.Digest[:], d.Sum(nil))

	// sign and verify
	signature := ed25519.Sign(fi.PrivateSigKey[:], fi.Public.Digest[:])
	copy(fi.Public.Signature[:], signature)
	if !fi.Public.Verify() {
		return fmt.Errorf("could not verify public signature")
	}

	return nil
}

// SignMessage signs a message with an Ed25519 private key.
func SignMessage(message []byte, privKey *FixedSizeEd25519PrivateKey) FixedSizeSignature {
	var sig [ed25519.SignatureSize]byte
	copy(sig[:], ed25519.Sign(privKey[:], message))
	return sig
}

func (fi *FullIdentity) SignMessage(message []byte) FixedSizeSignature {
	return SignMessage(message, &fi.PrivateSigKey)
}

// VerifyMessage verifies a message with an Ed25519 public key.
func VerifyMessage(msg []byte, sig *FixedSizeSignature, pubKey *FixedSizeEd25519PublicKey) bool {
	return ed25519.Verify(pubKey[:], msg, sig[:])
}

func (p PublicIdentity) VerifyMessage(msg []byte, sig *FixedSizeSignature) bool {
	return VerifyMessage(msg, sig, &p.SigKey)
}

func (p PublicIdentity) String() string {
	return hex.EncodeToString(p.Identity[:])
}

func (p PublicIdentity) Fingerprint() string {
	return Fingerprint(p.Identity)
}

func (p PublicIdentity) Verify() bool {
	d := sha256.New()
	d.Write(p.SigKey[:])
	d.Write(p.Key[:])
	d.Write(p.Identity[:])
	if !bytes.Equal(p.Digest[:], d.Sum(nil)) {
		return false
	}
	return ed25519.Verify(p.SigKey[:], p.Digest[:], p.Signature[:])
}

func (p PublicIdentity) VerifyIdentity() bool {
	key := (*sntrup4591761.PublicKey)(&p.Key)
	wantID := *IdentityFromPub(key)
	return p.Identity == wantID
}

// Zero out a byte slice.
func zero(in []byte) {
	if in == nil {
		return
	}
	for i := 0; i < len(in); i++ {
		in[i] ^= in[i]
	}
}

func IdentityFromPub(pub *sntrup4591761.PublicKey) *[32]byte {
	identity := sha256.Sum256(pub[:])
	return &identity
}

func Byte2ID(to []byte) (*[32]byte, error) {
	if len(to) != 32 {
		return nil, fmt.Errorf("invalid length")
	}
	var id32 [32]byte
	copy(id32[:], to)

	return &id32, nil
}

func String2ID(to string) (*[32]byte, error) {
	id, err := hex.DecodeString(to)
	if err != nil {
		return nil, err
	}
	if len(id) != 32 {
		return nil, fmt.Errorf("invalid length")
	}
	return Byte2ID(id)
}
