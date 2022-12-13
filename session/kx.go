// Copyright (c) 2016,2017 Company 0, LLC.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package session

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/companyzero/sntrup4591761"
	"golang.org/x/crypto/nacl/secretbox"
)

var (
	ErrEncrypt        = errors.New("encrypt failure")
	ErrDecrypt        = errors.New("decrypt failure")
	ErrOverflow       = errors.New("message too large")
	ErrInvalidKx      = errors.New("invalid kx")
	ErrMarshal        = errors.New("could not marshal")
	ErrUnmarshal      = errors.New("could not unmarshal")
	ErrNilTheirPubKey = errors.New("nil TheirPublicKey")
)

// KX allows two peers to derive a pair of shared keys. One peer must trigger
// Initiate (the client) while the other (the server) should call Init once
// followed by Respond for each connection.
type KX struct {
	Conn           io.ReadWriter
	MaxMessageSize uint
	OurPrivateKey  *zkidentity.FixedSizeSntrupPrivateKey
	OurPublicKey   *zkidentity.FixedSizeSntrupPublicKey
	TheirPublicKey *zkidentity.FixedSizeSntrupPublicKey
	writeKey       *[32]byte
	readKey        *[32]byte
	writeSeq       *sequence
	readSeq        *sequence
}

// Initiate performs a key exchange on behalf of a connecting client. A key
// exchange involves the following variables:
// k1, k2, k3, k4: NTRU Prime shared keys.
// c1, c2, c3, c4: NTRU Prime ciphertexts corresponding to k1, k2, k3, k4.
// From the perspective of the initiator, the process unfolds as follows:
func (kx *KX) Initiate() error {
	if kx.TheirPublicKey == nil {
		return ErrNilTheirPubKey
	}

	// Step 1: Generate k1, send c1.
	c, k1, err := sntrup4591761.Encapsulate(rand.Reader, (*sntrup4591761.PublicKey)(kx.TheirPublicKey))
	if err != nil {
		return ErrEncrypt
	}
	var lenBytes [4]byte
	binary.LittleEndian.PutUint32(lenBytes[:], uint32(len(c)))

	_, err = kx.Conn.Write(lenBytes[:])
	if err != nil {
		return err
	}
	_, err = kx.Conn.Write(c[:])
	if err != nil {
		return err
	}

	kx.writeKey = k1
	kx.readKey = k1
	cn := newSequence(halfForClient)
	sn := newSequence(halfForServer)
	kx.writeSeq = cn
	kx.readSeq = sn

	return nil
}

// Respond performs a key exchange on behalf of a responding server. A key
// exchange involves the following variables:
// k1, k2, k3, k4: NTRU Prime shared keys.
// c1, c2, c3, c4: NTRU Prime ciphertexts corresponding to k1, k2, k3, k4.
// From the perspective of the responder, the process unfolds as follows:
func (kx *KX) Respond() error {
	// Step 1: Receive c1, obtain k1.
	var lenBytes [4]byte
	_, err := kx.Conn.Read(lenBytes[:])
	if err != nil {
		return err
	}
	l := binary.LittleEndian.Uint32(lenBytes[:])
	if l != sntrup4591761.CiphertextSize {
		return fmt.Errorf("invalid ciphertext size received")
	}
	c := new(sntrup4591761.Ciphertext)
	_, err = kx.Conn.Read(c[:])
	if err != nil {
		return err
	}

	k1, ok := sntrup4591761.Decapsulate(c, (*sntrup4591761.PrivateKey)(kx.OurPrivateKey))
	if ok != 1 {
		return ErrInvalidKx
	}

	kx.writeKey = k1
	kx.readKey = k1
	cn := newSequence(halfForClient)
	sn := newSequence(halfForServer)
	kx.writeSeq = sn
	kx.readSeq = cn

	return nil
}

func (kx *KX) Read() ([]byte, error) {
	var lenBytes [4]byte
	_, err := io.ReadFull(kx.Conn, lenBytes[:])
	if err != nil {
		return nil, err
	}
	l := binary.LittleEndian.Uint32(lenBytes[:])
	if l > uint32(kx.MaxMessageSize) {
		return nil, fmt.Errorf("len > max message size: %d > %d", l, kx.MaxMessageSize)
	}
	payload := make([]byte, l)
	_, err = io.ReadFull(kx.Conn, payload)
	if err != nil {
		return nil, err
	}
	data, ok := secretbox.Open(nil, payload, kx.readSeq.Nonce(), kx.readKey)
	kx.readSeq.Decrease()
	if !ok {
		return payload, ErrDecrypt
	}
	return data, nil
}

// Write encrypts and marshals data to the underlying writer.
func (kx *KX) Write(data []byte) error {
	payload := secretbox.Seal(nil, data, kx.writeSeq.Nonce(), kx.writeKey)
	kx.writeSeq.Decrease()
	if uint(len(payload)) > kx.MaxMessageSize {
		return ErrOverflow
	}
	var lenBytes [4]byte
	binary.LittleEndian.PutUint32(lenBytes[:], uint32(len(payload)))
	_, err := kx.Conn.Write(lenBytes[:])
	if err != nil {
		return err
	}
	_, err = kx.Conn.Write(payload)
	if err != nil {
		return err
	}
	return nil
}
