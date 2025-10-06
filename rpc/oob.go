package rpc

import (
	"bytes"
	"compress/zlib"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/sw"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/companyzero/sntrup4591761"
)

type InviteFunds struct {
	Tx         TxHash `json:"txid"`
	Index      uint32 `json:"index"`
	Tree       int8   `json:"tree"`
	PrivateKey string `json:"private_key"`
	HeightHint uint32 `json:"height_hint"`
	Address    string `json:"address"`
}

// OOBPublicIdentityInvite is an unencrypted OOB command which contains all
// information to kick of an initial KX. This command is NOT part of the wire
// protocol. This is provided out-of-band. With this the other side can
// commence a KX by issuing a RMOCHalfKX command to the provided
// InitialRendezvous.
type OOBPublicIdentityInvite struct {
	Public            zkidentity.PublicIdentity `json:"public"`
	InitialRendezvous zkidentity.ShortID        `json:"initialrendezvous"`
	ResetRendezvous   zkidentity.ShortID        `json:"resetrendezvous"`
	Funds             *InviteFunds              `json:"funds,omitempty"`
}

const OOBCPublicIdentityInvite = "oobpublicidentityinvite"

// CreateOOBPublicIdentityInvite returns a OOBPublicIdentityInvite structure
// with a random initial and reset rendezvous.
func CreateOOBPublicIdentityInvite(pi zkidentity.PublicIdentity) (*OOBPublicIdentityInvite, error) {
	opii := OOBPublicIdentityInvite{
		Public: pi,
	}
	_, err := io.ReadFull(rand.Reader, opii.InitialRendezvous[:])
	if err != nil {
		return nil, err
	}
	_, err = io.ReadFull(rand.Reader, opii.ResetRendezvous[:])
	if err != nil {
		return nil, err
	}
	return &opii, nil
}

// MarshalOOBPublicIdentityInvite returns a JSON encoded OOBPublicIdentityInvite.
func MarshalOOBPublicIdentityInvite(pii *OOBPublicIdentityInvite) ([]byte, error) {
	return json.Marshal(pii)
}

func DecryptOOBPublicIdentityInvite(packed []byte, key *zkidentity.FixedSizeSntrupPrivateKey, maxDecompressSize uint) (*OOBPublicIdentityInvite, error) {
	pii, err := DecryptOOB(packed, key, maxDecompressSize)
	if err != nil {
		return nil, err
	}

	p, ok := pii.(OOBPublicIdentityInvite)
	if !ok {
		return nil, fmt.Errorf("invalid type: %T", pii)
	}

	return &p, nil
}

// UnmarshalOOBPublicIdentityInviteFile returns an OOBPublicIdentityInvite from
// a file.
func UnmarshalOOBPublicIdentityInviteFile(filename string) (*OOBPublicIdentityInvite, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	jr := json.NewDecoder(f)
	var pii OOBPublicIdentityInvite
	err = jr.Decode(&pii)
	if err != nil {
		return nil, err
	}

	return &pii, nil
}

// NewHalfRatchetKX creates a new half ratchet between two identities. It returns
// the half ratchet and a random key exchange structure.
//
// ourPrivKey should be the local client's private key from their full identity.
func NewHalfRatchetKX(ourPrivKey *zkidentity.FixedSizeSntrupPrivateKey, them zkidentity.PublicIdentity) (*ratchet.Ratchet, *ratchet.KeyExchange, error) {
	// Create new ratchet with remote identity
	r := ratchet.New(rand.Reader)
	r.MyPrivateKey = ourPrivKey
	r.TheirPublicKey = &them.Key

	// Fill out half the kx
	hkx := new(ratchet.KeyExchange)
	err := r.FillKeyExchange(hkx)
	if err != nil {
		return nil, nil, err
	}

	return r, hkx, nil
}

// NewHalfKX creates a RMOHalfKX structure from a half ratchet.
//
// us should be the public identity that is derived from the local client's
// full identity.
func NewHalfKX(us zkidentity.PublicIdentity, hkx *ratchet.KeyExchange) (*RMOHalfKX, error) {
	// Create half KX RPC
	halfKX := RMOHalfKX{
		Public: us,
		HalfKX: *hkx,
	}
	_, err := io.ReadFull(rand.Reader, halfKX.InitialRendezvous[:])
	if err != nil {
		return nil, err
	}
	_, err = io.ReadFull(rand.Reader, halfKX.ResetRendezvous[:])
	if err != nil {
		return nil, err
	}

	return &halfKX, nil
}

// NewFullRatchetKX creates a new full ratchet between two identities. It returns
// the completed full ratchet.
//
// ourPrivKey should be the private key that corresponds to the local client's
// full identity.
func NewFullRatchetKX(ourPrivKey *zkidentity.FixedSizeSntrupPrivateKey, them zkidentity.PublicIdentity, halfKX *ratchet.KeyExchange) (*ratchet.Ratchet, *ratchet.KeyExchange, error) {
	// Fill out missing bits to create full ratchet
	r := ratchet.New(rand.Reader)
	r.MyPrivateKey = ourPrivKey
	r.TheirPublicKey = &them.Key
	fkx := new(ratchet.KeyExchange)
	err := r.FillKeyExchange(fkx)
	if err != nil {
		return nil, nil, err
	}
	// Complete ratchet
	err = r.CompleteKeyExchange(halfKX, false)
	if err != nil {
		return nil, nil, err
	}

	return r, fkx, nil
}

// NewFullKX creates a RMOFullKX structure from a full ratchet.
func NewFullKX(fkx *ratchet.KeyExchange) (*RMOFullKX, error) {
	return &RMOFullKX{FullKX: *fkx}, nil
}

// EncryptRMO returns an encrypted blob from x.
// The returned blob is packed and prefixed with a sntrup ciphertext followed
// by an encrypted JSON objecti; or [sntrup ciphertext][encrypted JSON object].
func EncryptRMO(x interface{}, theirKey *zkidentity.FixedSizeSntrupPublicKey, zlibLevel int) ([]byte, error) {
	// Create shared key that will be discarded as function exits
	cipherText, sharedKey, err := sntrup4591761.Encapsulate(rand.Reader,
		(*sntrup4591761.PublicKey)(theirKey))
	if err != nil {
		return nil, err
	}
	defer func() {
		for i := 0; i < len(sharedKey); i++ {
			sharedKey[i] ^= sharedKey[i]
		}
	}()

	blob, err := ComposeRMO(x, zlibLevel)
	if err != nil {
		return nil, err
	}
	blobEncrypted, err := sw.Seal(blob, sharedKey)
	if err != nil {
		return nil, err
	}
	packed := make([]byte, sntrup4591761.CiphertextSize+len(blobEncrypted))
	copy(packed[0:], cipherText[:])
	copy(packed[sntrup4591761.CiphertextSize:], blobEncrypted)

	return packed, nil
}

func DecryptOOB(packed []byte, key *zkidentity.FixedSizeSntrupPrivateKey, maxDecompressSize uint) (interface{}, error) {
	if len(packed) < sntrup4591761.CiphertextSize {
		return nil, fmt.Errorf("packed blob too small")
	}
	var cipherText sntrup4591761.Ciphertext
	copy(cipherText[:], packed[0:sntrup4591761.CiphertextSize])

	// Decrypt key and invite blob
	sharedKey, n := sntrup4591761.Decapsulate(&cipherText, (*sntrup4591761.PrivateKey)(key))
	if n != 1 {
		return nil, fmt.Errorf("could not decapsulate")
	}
	hkxb, ok := sw.Open(packed[sntrup4591761.CiphertextSize:], sharedKey)
	if !ok {
		return nil, fmt.Errorf("could not open")
	}
	_, hkx, err := DecomposeRMO(hkxb, maxDecompressSize)
	if err != nil {
		return nil, fmt.Errorf("DecomposeRMO %v", err)
	}

	return hkx, nil
}

// DecryptOOBHalfKXBlob decrypts a packed RMOHalfKX blob.
func DecryptOOBHalfKXBlob(packed []byte, key *zkidentity.FixedSizeSntrupPrivateKey, maxDecompressSize uint) (*RMOHalfKX, error) {
	hkx, err := DecryptOOB(packed, key, maxDecompressSize)
	if err != nil {
		return nil, err
	}

	h, ok := hkx.(RMOHalfKX)
	if !ok {
		return nil, fmt.Errorf("invalid type: %T", hkx)
	}

	return &h, nil
}

// DecryptOOBFullKXBlob decrypts a packed RMOFullKX blob.
func DecryptOOBFullKXBlob(packed []byte, key *zkidentity.FixedSizeSntrupPrivateKey, maxDecompressSize uint) (*RMOFullKX, error) {
	fkx, err := DecryptOOB(packed, key, maxDecompressSize)
	if err != nil {
		return nil, err
	}

	f, ok := fkx.(RMOFullKX)
	if !ok {
		return nil, fmt.Errorf("invalid type: %T", fkx)
	}

	return &f, nil
}

// Following is the portion of the oob protocol which does travel over the
// wire.
const (
	RMOHeaderVersion = 1
)

// RMOHeader describes which command follows this structure.
type RMOHeader struct {
	Version   uint64 `json:"version"`
	Timestamp int64  `json:"timestamp"`
	Command   string `json:"command"`
}

// RMOHalfKX is the command that flows after receiving an
// OOBPublicIdentityInvite.
type RMOHalfKX struct {
	Public            zkidentity.PublicIdentity `json:"public"`
	HalfKX            ratchet.KeyExchange       `json:"halfkx"`
	InitialRendezvous ratchet.RVPoint           `json:"initialrendezvous"`
	ResetRendezvous   ratchet.RVPoint           `json:"resetrendezvous"`
}

const RMOCHalfKX = "ohalfkx"

// RMOFullKX
type RMOFullKX struct {
	FullKX ratchet.KeyExchange `json:"fullkx"`
}

const RMOCFullKX = "ofullkx"

// XXX see if we can combine this with the regular code path (ComposeRM)

// ComposeRMO creates a blobified oob message that has a header and a
// payload that can then be encrypted and transmitted to the other side.
func ComposeRMO(rm interface{}, zlibLevel int) ([]byte, error) {
	h := RMOHeader{
		Version:   RMOHeaderVersion,
		Timestamp: time.Now().Unix(),
	}
	switch rm.(type) {
	case OOBPublicIdentityInvite:
		h.Command = OOBCPublicIdentityInvite

	case RMOHalfKX:
		h.Command = RMOCHalfKX

	case RMOFullKX:
		h.Command = RMOCFullKX

	default:
		return nil, fmt.Errorf("unknown oob routed message "+
			"type: %T", rm)
	}

	// Create header, note that the encoder appends a '\n'
	mb := &bytes.Buffer{}
	w, err := zlib.NewWriterLevel(mb, zlibLevel)
	if err != nil {
		return nil, err
	}

	e := json.NewEncoder(w)
	err = e.Encode(h)
	if err != nil {
		return nil, err
	}

	// Append payload
	err = e.Encode(rm)
	if err != nil {
		return nil, err
	}
	err = w.Close()
	if err != nil {
		return nil, err
	}

	return mb.Bytes(), nil
}

func DecomposeRMO(mb []byte, maxDecompressSize uint) (*RMOHeader, interface{}, error) {
	cr, err := zlib.NewReader(bytes.NewReader(mb))
	if err != nil {
		return nil, nil, err
	}
	lr := &limitedReader{R: cr, N: maxDecompressSize}

	// Read header
	var h RMOHeader
	d := json.NewDecoder(lr)
	err = d.Decode(&h)
	if err != nil {
		return nil, nil, fmt.Errorf("decode %v", err)
	}

	// Decode payload
	pmd := d
	var payload interface{}
	switch h.Command {
	case OOBCPublicIdentityInvite:
		var pii OOBPublicIdentityInvite
		err = pmd.Decode(&pii)
		payload = pii

	case RMOCHalfKX:
		var hkx RMOHalfKX
		err = pmd.Decode(&hkx)
		payload = hkx

	case RMOCFullKX:
		var fkx RMOFullKX
		err = pmd.Decode(&fkx)
		payload = fkx

	default:
		return nil, nil, fmt.Errorf("unknown oob "+
			"message command: %v", h.Command)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("decode command %v: %w",
			h.Command, err)
	}

	return &h, payload, nil
}
