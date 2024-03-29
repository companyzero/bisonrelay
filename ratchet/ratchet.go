// Copyright (c) 2016 Company 0, LLC.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// Package ratchet implements the axolotl ratchet, by Trevor Perrin. See
// https://github.com/trevp/axolotl/wiki.
package ratchet

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"math/big"
	"time"

	"github.com/companyzero/bisonrelay/ratchet/disk"
	"github.com/companyzero/bisonrelay/sw"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/companyzero/sntrup4591761"
	"github.com/decred/dcrd/crypto/blake256"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/nacl/secretbox"
)

const (
	// headerSize is the size, in bytes, of a header's plaintext contents.
	headerSize = 4 + // uint32 message count
		4 + // uint32 previous message count
		32 + // curve25519 ratchet public
		24 // nonce for message

	// sealedHeader is the size, in bytes, of an encrypted header.
	sealedHeaderSize = 24 + // nonce
		headerSize +
		secretbox.Overhead

	// nonceInHeaderOffset is the offset of the message nonce in the
	// header's plaintext.
	nonceInHeaderOffset = 4 + 4 + 32

	// maxMissingMessages is the maximum number of missing messages that
	// we'll keep track of.
	maxMissingMessages = 80
)

type RVPoint = zkidentity.ShortID

// savedKey contains a message key and timestamp for a message which has not
// been received. The timestamp comes from the message by which we learn of the
// missing message.
type savedKey struct {
	timestamp time.Time
	key       [32]byte
}

// Ratchet contains the per-contact, crypto state.
type Ratchet struct {
	MyPrivateKey   *zkidentity.FixedSizeSntrupPrivateKey
	TheirPublicKey *zkidentity.FixedSizeSntrupPublicKey

	// rootKey gets updated by the DH ratchet.
	rootKey [32]byte

	// Header keys are used to encrypt message headers.
	sendHeaderKey     [32]byte
	recvHeaderKey     [32]byte
	nextSendHeaderKey [32]byte
	nextRecvHeaderKey [32]byte
	prevRecvHeaderKey [32]byte

	// Chain keys are used for forward secrecy updating.
	sendChainKey       [32]byte
	recvChainKey       [32]byte
	sendRatchetPrivate [32]byte
	recvRatchetPublic  [32]byte
	sendCount          uint32
	recvCount          uint32
	prevSendCount      uint32
	prevRecvCount      uint32

	// ratchet is true if we will send a new ratchet value in the next message.
	ratchet bool

	// saved is a map from a header key to a map from sequence number to
	// message key.
	saved map[[32]byte]map[uint32]savedKey

	myHalf    *[32]byte
	theirHalf *[32]byte
	kxPrivate *[32]byte

	lastEncryptTime time.Time
	lastDecryptTime time.Time

	rand io.Reader
}

func (r *Ratchet) randBytes(buf []byte) {
	if _, err := io.ReadFull(r.rand, buf); err != nil {
		panic(err)
	}
}

func New(rand io.Reader) *Ratchet {
	r := new(Ratchet)
	r.rand = rand
	r.kxPrivate = new([32]byte)
	r.randBytes(r.kxPrivate[:])
	r.saved = make(map[[32]byte]map[uint32]savedKey)
	return r
}

type KeyExchange struct {
	Public []byte                               `json:"public"`
	Cipher zkidentity.FixedSizeSntrupCiphertext `json:"cipher"`
}

// FillKeyExchange sets elements of kx with key exchange information from the
// ratchet.
func (r *Ratchet) FillKeyExchange(kx *KeyExchange) error {
	c, k, err := sntrup4591761.Encapsulate(r.rand, (*sntrup4591761.PublicKey)(r.TheirPublicKey))
	if err != nil {
		return err
	}
	pub := new([32]byte)
	curve25519.ScalarBaseMult(pub, r.kxPrivate)

	packed, err := sw.Seal(pub[:], k)
	if err != nil {
		return err
	}

	r.myHalf = k
	copy(kx.Cipher[:], c[:])
	kx.Public = packed

	return nil
}

// deriveKey takes an HMAC object and a label and calculates out = HMAC(k, label).
func deriveKey(out *[32]byte, label []byte, h hash.Hash) {
	h.Reset()
	h.Write(label)
	n := h.Sum(out[:0])
	if &n[0] != &out[0] {
		panic("hash function too large")
	}
}

// These constants are used as the label argument to deriveKey to derive
// independent keys from a master key.
var (
	chainKeyLabel          = []byte("chain key")
	headerKeyLabel         = []byte("header key")
	nextRecvHeaderKeyLabel = []byte("next receive header key")
	rootKeyLabel           = []byte("root key")
	rootKeyUpdateLabel     = []byte("root key update")
	sendHeaderKeyLabel     = []byte("next send header key")
	messageKeyLabel        = []byte("message key")
	chainKeyStepLabel      = []byte("chain key step")
)

// validateECDHpoint() performs a set of basic checks on the validity of a
// peer's randomly chosen ECDH point. The term "point" is slightly
// misleading, as all we are given are the x-coordinates of a point.
func validateECDHpoint(p []byte) error {
	if len(p) != 32 {
		return errors.New("ratchet: invalid ECDH point length")
	}
	pn := new(big.Int).SetBytes(inv32(p))
	min := big.NewInt(3)
	if pn.Cmp(min) == -1 {
		return errors.New("ratchet: invalid ECDH points") // too small
	}
	max := big.NewInt(0).Sub(big.NewInt(0).Exp(big.NewInt(2),
		big.NewInt(255), nil), big.NewInt(19))
	if pn.Cmp(max) != -1 {
		return errors.New("ratchet: invalid ECDH points") // too large
	}
	return nil
}

// CompleteKeyExchange takes a KeyExchange message from the other party and
// establishes the ratchet.
func (r *Ratchet) CompleteKeyExchange(kx *KeyExchange, alice bool) error {
	k, rv := sntrup4591761.Decapsulate((*sntrup4591761.Ciphertext)(&kx.Cipher),
		(*sntrup4591761.PrivateKey)(r.MyPrivateKey))
	if rv != 1 {
		return errors.New("CompleteKeyExchange: decapsulation error")
	}
	r.theirHalf = k

	ratchetPublic, ok := sw.Open(kx.Public, k)
	if !ok {
		return fmt.Errorf("could not open kx.Public")
	}
	err := validateECDHpoint(ratchetPublic)
	if err != nil {
		return err
	}

	d := sha256.New()
	if alice {
		d.Write(r.myHalf[:])
		d.Write(r.theirHalf[:])
	} else {
		d.Write(r.theirHalf[:])
		d.Write(r.myHalf[:])
	}
	sharedKey := d.Sum(nil)

	keyMaterial := make([]byte, 0, 32*5)
	keyMaterial = append(keyMaterial, sharedKey...)
	h := hmac.New(sha256.New, keyMaterial)
	deriveKey(&r.rootKey, rootKeyLabel, h)

	if alice {
		deriveKey(&r.recvHeaderKey, headerKeyLabel, h)
		deriveKey(&r.nextSendHeaderKey, sendHeaderKeyLabel, h)
		deriveKey(&r.nextRecvHeaderKey, nextRecvHeaderKeyLabel, h)
		deriveKey(&r.recvChainKey, chainKeyLabel, h)
		copy(r.recvRatchetPublic[:], ratchetPublic)
	} else {
		deriveKey(&r.sendHeaderKey, headerKeyLabel, h)
		deriveKey(&r.nextRecvHeaderKey, sendHeaderKeyLabel, h)
		deriveKey(&r.nextSendHeaderKey, nextRecvHeaderKeyLabel, h)
		deriveKey(&r.sendChainKey, chainKeyLabel, h)
		copy(r.sendRatchetPrivate[:], r.kxPrivate[:])
	}

	r.ratchet = alice
	r.kxPrivate = nil

	return nil
}

// ratchetRendezvous generates a rendezvous point given the specified key data.
//
// This is calculated as blake256(headerKey || msgCount).
func ratchetRendezvous(headerKey [32]byte, msgCount uint32) RVPoint {
	var msgCountLE [4]byte
	binary.LittleEndian.PutUint32(msgCountLE[0:4], msgCount)
	h := blake256.New()
	h.Write(headerKey[:])
	h.Write(msgCountLE[:])
	var res RVPoint
	if n := copy(res[:], h.Sum(nil)); n != len(res) {
		// Should never happen, but sanity check anyway.
		panic("hash did not produce required nb of bytes")
	}
	return res
}

func (r *Ratchet) sendRVKey() [32]byte {
	if isZeroKey(&r.sendHeaderKey) {
		// This happens once right after kx
		return r.nextSendHeaderKey
	}
	return r.sendHeaderKey
}

func (r *Ratchet) SendRendezvous() RVPoint {
	return ratchetRendezvous(r.sendRVKey(), r.sendCount)
}

func (r *Ratchet) SendRendezvousPlainText() string {
	return fmt.Sprintf("%x.%03d", r.sendRVKey(), r.sendCount)
}

func (r *Ratchet) recvRVKeys() ([32]byte, [32]byte) {
	rhk := r.recvHeaderKey
	if isZeroKey(&r.recvHeaderKey) {
		// This happens once right after kx
		rhk = r.nextRecvHeaderKey
	}

	dk := r.prevRecvHeaderKey
	return rhk, dk
}

func (r *Ratchet) RecvRendezvous() (RVPoint, RVPoint) {
	rhk, dk := r.recvRVKeys()
	rv := ratchetRendezvous(rhk, r.recvCount)
	var drain RVPoint
	if !isZeroKey(&dk) {
		drain = ratchetRendezvous(dk, r.prevRecvCount)
	}
	return rv, drain
}

func (r *Ratchet) RecvRendezvousPlainText() (string, string) {
	rhk, dk := r.recvRVKeys()
	rv := fmt.Sprintf("%x.%03d", rhk, r.recvCount)
	var drain string
	if !isZeroKey(&dk) {
		drain = fmt.Sprintf("%x.%03d", dk, r.prevRecvCount)
	}
	return rv, drain
}

// Encrypt acts like append() but appends an encrypted version of msg to out.
func (r *Ratchet) Encrypt(out, msg []byte) ([]byte, error) {
	if r.ratchet {
		r.randBytes(r.sendRatchetPrivate[:])
		sharedKey, err := curve25519.X25519(r.sendRatchetPrivate[:], r.recvRatchetPublic[:])
		if err != nil {
			return nil, err
		}
		copy(r.sendHeaderKey[:], r.nextSendHeaderKey[:])

		var keyMaterial [32]byte
		sha := sha256.New()
		sha.Write(rootKeyUpdateLabel)
		sha.Write(r.rootKey[:])
		sha.Write(sharedKey)
		sha.Sum(keyMaterial[:0])
		h := hmac.New(sha256.New, keyMaterial[:])
		deriveKey(&r.rootKey, rootKeyLabel, h)
		deriveKey(&r.nextSendHeaderKey, sendHeaderKeyLabel, h)
		deriveKey(&r.sendChainKey, chainKeyLabel, h)
		r.prevSendCount, r.sendCount = r.sendCount, 0
		r.ratchet = false
	}

	h := hmac.New(sha256.New, r.sendChainKey[:])
	var messageKey [32]byte
	deriveKey(&messageKey, messageKeyLabel, h)
	deriveKey(&r.sendChainKey, chainKeyStepLabel, h)

	var sendRatchetPublic [32]byte
	curve25519.ScalarBaseMult(&sendRatchetPublic, &r.sendRatchetPrivate)
	var header [headerSize]byte
	var headerNonce, messageNonce [24]byte
	r.randBytes(headerNonce[:])
	r.randBytes(messageNonce[:])

	binary.LittleEndian.PutUint32(header[0:4], r.sendCount)
	binary.LittleEndian.PutUint32(header[4:8], r.prevSendCount)
	copy(header[8:], sendRatchetPublic[:])
	copy(header[nonceInHeaderOffset:], messageNonce[:])
	out = append(out, headerNonce[:]...)
	out = secretbox.Seal(out, header[:], &headerNonce, &r.sendHeaderKey)
	r.sendCount++

	r.lastEncryptTime = time.Now()
	return secretbox.Seal(out, msg, &messageNonce, &messageKey), nil
}

// trySavedKeys tries to decrypt ciphertext using keys saved for missing messages.
func (r *Ratchet) trySavedKeys(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < sealedHeaderSize {
		return nil, errors.New("ratchet: header too small to be valid")
	}

	sealedHeader := ciphertext[:sealedHeaderSize]
	var nonce [24]byte
	copy(nonce[:], sealedHeader)
	sealedHeader = sealedHeader[len(nonce):]

	for headerKey, messageKeys := range r.saved {
		header, ok := secretbox.Open(nil, sealedHeader, &nonce, &headerKey)
		if !ok {
			continue
		}
		if len(header) != headerSize {
			continue
		}
		msgNum := binary.LittleEndian.Uint32(header[:4])
		msgKey, ok := messageKeys[msgNum]
		if !ok {
			// This is a fairly common case: the message key might
			// not have been saved because it's the next message
			// key.
			return nil, nil
		}

		sealedMessage := ciphertext[sealedHeaderSize:]
		copy(nonce[:], header[nonceInHeaderOffset:])
		msg, ok := secretbox.Open(nil, sealedMessage, &nonce, &msgKey.key)
		if !ok {
			return nil, errors.New("ratchet: corrupt message")
		}
		delete(messageKeys, msgNum)
		if len(messageKeys) == 0 {
			delete(r.saved, headerKey)
		}
		return msg, nil
	}

	return nil, nil
}

// saveKeys takes a header key, the current chain key, a received message
// number and the expected message number and advances the chain key as needed.
// It returns the message key for given given message number and the new chain
// key. If any messages have been skipped over, it also returns savedKeys, a
// map suitable for merging with r.saved, that contains the message keys for
// the missing messages.
func (r *Ratchet) saveKeys(headerKey, recvChainKey *[32]byte, messageNum, receivedCount uint32) (provisionalChainKey, messageKey [32]byte, savedKeys map[[32]byte]map[uint32]savedKey, err error) {
	if messageNum < receivedCount {
		// This is a message from the past, but we didn't have a saved
		// key for it, which means that it's a duplicate message or we
		// expired the save key.
		err = errors.New("ratchet: duplicate message or message delayed longer than tolerance")
		return
	}

	missingMessages := messageNum - receivedCount
	if missingMessages > maxMissingMessages {
		err = errors.New("ratchet: message exceeds reordering limit")
		return
	}

	// messageKeys maps from message number to message key.
	var messageKeys map[uint32]savedKey
	if missingMessages > 0 {
		messageKeys = make(map[uint32]savedKey)
	}
	copy(provisionalChainKey[:], recvChainKey[:])

	now := time.Now()
	for n := receivedCount; n <= messageNum; n++ {
		h := hmac.New(sha256.New, provisionalChainKey[:])
		deriveKey(&messageKey, messageKeyLabel, h)
		deriveKey(&provisionalChainKey, chainKeyStepLabel, h)
		if n < messageNum {
			messageKeys[n] = savedKey{now, messageKey}
		}
	}

	if messageKeys != nil {
		savedKeys = make(map[[32]byte]map[uint32]savedKey)
		savedKeys[*headerKey] = messageKeys
	}

	return
}

// mergeSavedKeys takes a map of saved message keys from saveKeys and merges it
// into r.saved.
func (r *Ratchet) mergeSavedKeys(newKeys map[[32]byte]map[uint32]savedKey) {
	for headerKey, newMessageKeys := range newKeys {
		messageKeys, ok := r.saved[headerKey]
		if !ok {
			r.saved[headerKey] = newMessageKeys
			continue
		}

		for n, messageKey := range newMessageKeys {
			messageKeys[n] = messageKey
		}
	}
}

// NbSavedKeys returns the total number of saved keys.
func (r *Ratchet) NbSavedKeys() int {
	var total int
	for _, m := range r.saved {
		total += len(m)
	}
	return total
}

// WillRatchet returns whether the next message sent will cause a ratchet op.
func (r *Ratchet) WillRatchet() bool {
	return r.ratchet
}

// isZeroKey returns true if key is all zeros.
func isZeroKey(key *[32]byte) bool {
	var x uint8
	for _, v := range key {
		x |= v
	}

	return x == 0
}

func (r *Ratchet) Decrypt(ciphertext []byte) ([]byte, error) {
	msg, err := r.trySavedKeys(ciphertext)
	if err != nil || msg != nil {
		if err == nil {
			r.lastDecryptTime = time.Now()
		}
		return msg, err
	}

	sealedHeader := ciphertext[:sealedHeaderSize]
	sealedMessage := ciphertext[sealedHeaderSize:]
	var nonce [24]byte
	copy(nonce[:], sealedHeader)
	sealedHeader = sealedHeader[len(nonce):]

	header, ok := secretbox.Open(nil, sealedHeader, &nonce, &r.recvHeaderKey)
	ok = ok && !isZeroKey(&r.recvHeaderKey)
	if ok {
		if len(header) != headerSize {
			return nil, errors.New("ratchet: incorrect header size")
		}
		messageNum := binary.LittleEndian.Uint32(header[:4])
		provisionalChainKey, messageKey, savedKeys, err := r.saveKeys(&r.recvHeaderKey, &r.recvChainKey, messageNum, r.recvCount)
		if err != nil {
			return nil, err
		}

		copy(nonce[:], header[nonceInHeaderOffset:])
		msg, ok := secretbox.Open(nil, sealedMessage, &nonce, &messageKey)
		if !ok {
			return nil, errors.New("ratchet: corrupt message")
		}

		copy(r.recvChainKey[:], provisionalChainKey[:])
		r.mergeSavedKeys(savedKeys)
		r.recvCount = messageNum + 1
		r.lastDecryptTime = time.Now()
		return msg, nil
	}

	header, ok = secretbox.Open(nil, sealedHeader, &nonce, &r.nextRecvHeaderKey)
	if !ok {
		return nil, errors.New("ratchet: cannot decrypt")
	}
	if len(header) != headerSize {
		return nil, errors.New("ratchet: incorrect header size")
	}

	if r.ratchet {
		return nil, errors.New("ratchet: received message encrypted to next header key without ratchet flag set")
	}

	messageNum := binary.LittleEndian.Uint32(header[:4])
	prevMessageCount := binary.LittleEndian.Uint32(header[4:8])

	_, _, oldSavedKeys, err := r.saveKeys(&r.recvHeaderKey, &r.recvChainKey, prevMessageCount, r.recvCount)
	if err != nil {
		return nil, err
	}

	var dhPublic, rootKey, chainKey, keyMaterial [32]byte
	copy(dhPublic[:], header[8:])

	sharedKey, err := curve25519.X25519(r.sendRatchetPrivate[:], dhPublic[:])
	if err != nil {
		return nil, err
	}
	sha := sha256.New()
	sha.Write(rootKeyUpdateLabel)
	sha.Write(r.rootKey[:])
	sha.Write(sharedKey)

	var rootKeyHMAC hash.Hash

	sha.Sum(keyMaterial[:0])
	rootKeyHMAC = hmac.New(sha256.New, keyMaterial[:])
	deriveKey(&rootKey, rootKeyLabel, rootKeyHMAC)
	deriveKey(&chainKey, chainKeyLabel, rootKeyHMAC)

	provisionalChainKey, messageKey, savedKeys, err := r.saveKeys(&r.nextRecvHeaderKey, &chainKey, messageNum, 0)
	if err != nil {
		return nil, err
	}

	copy(nonce[:], header[nonceInHeaderOffset:])
	msg, ok = secretbox.Open(nil, sealedMessage, &nonce, &messageKey)
	if !ok {
		return nil, errors.New("ratchet: corrupt message")
	}

	copy(r.rootKey[:], rootKey[:])
	copy(r.recvChainKey[:], provisionalChainKey[:])
	copy(r.prevRecvHeaderKey[:], r.recvHeaderKey[:]) // Save old recv hk
	copy(r.recvHeaderKey[:], r.nextRecvHeaderKey[:])
	deriveKey(&r.nextRecvHeaderKey, sendHeaderKeyLabel, rootKeyHMAC)
	for i := range r.sendRatchetPrivate {
		r.sendRatchetPrivate[i] = 0
	}
	copy(r.recvRatchetPublic[:], dhPublic[:])

	r.prevRecvCount = r.recvCount
	r.recvCount = messageNum + 1
	r.mergeSavedKeys(oldSavedKeys)
	r.mergeSavedKeys(savedKeys)
	r.ratchet = true

	r.lastDecryptTime = time.Now()
	return msg, nil
}

func dup32(x *[32]byte) []byte {
	if x == nil {
		return nil
	}
	ret := make([]byte, 32)
	copy(ret, x[:])
	return ret
}

func inv32(x []byte) []byte {
	if x == nil {
		return nil
	}
	ret := make([]byte, 32)
	for i := 0; i < 32; i++ {
		ret[i] = x[31-i]
	}
	return ret
}

func (r *Ratchet) LastEncDecTimes() (time.Time, time.Time) {
	return r.lastEncryptTime, r.lastDecryptTime
}

func (r *Ratchet) DiskState(lifetime time.Duration) *disk.RatchetState {
	now := time.Now()
	s := &disk.RatchetState{
		RootKey:            dup32(&r.rootKey),
		SendHeaderKey:      dup32(&r.sendHeaderKey),
		RecvHeaderKey:      dup32(&r.recvHeaderKey),
		NextSendHeaderKey:  dup32(&r.nextSendHeaderKey),
		NextRecvHeaderKey:  dup32(&r.nextRecvHeaderKey),
		PrevRecvHeaderKey:  dup32(&r.prevRecvHeaderKey),
		SendChainKey:       dup32(&r.sendChainKey),
		RecvChainKey:       dup32(&r.recvChainKey),
		SendRatchetPrivate: dup32(&r.sendRatchetPrivate),
		RecvRatchetPublic:  dup32(&r.recvRatchetPublic),
		SendCount:          r.sendCount,
		RecvCount:          r.recvCount,
		PrevSendCount:      r.prevSendCount,
		PrevRecvCount:      r.prevRecvCount,
		Ratchet:            r.ratchet,
		KXPrivate:          dup32(r.kxPrivate),
		MyHalf:             dup32(r.myHalf),
		TheirHalf:          dup32(r.theirHalf),
		LastEncryptTime:    r.lastEncryptTime.UnixMilli(),
		LastDecryptTime:    r.lastDecryptTime.UnixMilli(),
	}

	for headerKey, messageKeys := range r.saved {
		keys := make([]disk.RatchetState_SavedKeys_MessageKey, 0, len(messageKeys))
		for messageNum, savedKey := range messageKeys {
			if now.Sub(savedKey.timestamp) > lifetime {
				continue
			}
			keys = append(keys, disk.RatchetState_SavedKeys_MessageKey{
				Num:          messageNum,
				Key:          dup32(&savedKey.key),
				CreationTime: savedKey.timestamp.Unix(),
			})
		}
		s.SavedKeys = append(s.SavedKeys, disk.RatchetState_SavedKeys{
			HeaderKey:   dup32(&headerKey),
			MessageKeys: keys,
		})
	}

	return s
}

func unmarshalKey(dst *[32]byte, src []byte) bool {
	if len(src) != 32 {
		return false
	}
	copy(dst[:], src)
	return true
}

var ErrUnmarshal = errors.New("failed to unmarshal")

func (r *Ratchet) Unmarshal(s *disk.RatchetState) error {
	if !unmarshalKey(&r.rootKey, s.RootKey) ||
		!unmarshalKey(&r.sendHeaderKey, s.SendHeaderKey) ||
		!unmarshalKey(&r.recvHeaderKey, s.RecvHeaderKey) ||
		!unmarshalKey(&r.nextSendHeaderKey, s.NextSendHeaderKey) ||
		!unmarshalKey(&r.nextRecvHeaderKey, s.NextRecvHeaderKey) ||
		!unmarshalKey(&r.prevRecvHeaderKey, s.PrevRecvHeaderKey) ||
		!unmarshalKey(&r.sendChainKey, s.SendChainKey) ||
		!unmarshalKey(&r.recvChainKey, s.RecvChainKey) ||
		!unmarshalKey(&r.sendRatchetPrivate, s.SendRatchetPrivate) ||
		!unmarshalKey(&r.recvRatchetPublic, s.RecvRatchetPublic) {
		return ErrUnmarshal
	}

	r.sendCount = s.SendCount
	r.recvCount = s.RecvCount
	r.prevSendCount = s.PrevSendCount
	r.prevRecvCount = s.PrevRecvCount
	r.ratchet = s.Ratchet
	r.lastEncryptTime = time.UnixMilli(s.LastEncryptTime)
	r.lastDecryptTime = time.UnixMilli(s.LastDecryptTime)

	if len(s.KXPrivate) > 0 {
		if !unmarshalKey(r.kxPrivate, s.KXPrivate) {
			return ErrUnmarshal
		}
	} else {
		r.kxPrivate = nil
	}
	if len(s.MyHalf) > 0 {
		if r.myHalf == nil {
			r.myHalf = new([32]byte)
		}
		if !unmarshalKey(r.myHalf, s.MyHalf) {
			return ErrUnmarshal
		}
	} else {
		r.myHalf = nil
	}
	if len(s.TheirHalf) > 0 {
		if r.theirHalf == nil {
			r.theirHalf = new([32]byte)
		}
		if !unmarshalKey(r.theirHalf, s.TheirHalf) {
			return ErrUnmarshal
		}
	} else {
		r.theirHalf = nil
	}

	for _, saved := range s.SavedKeys {
		var headerKey [32]byte
		if !unmarshalKey(&headerKey, saved.HeaderKey) {
			return ErrUnmarshal
		}
		messageKeys := make(map[uint32]savedKey)
		for _, messageKey := range saved.MessageKeys {
			var savedKey savedKey
			if !unmarshalKey(&savedKey.key, messageKey.Key) {
				return ErrUnmarshal
			}
			savedKey.timestamp = time.Unix(messageKey.CreationTime, 0)
			messageKeys[messageKey.Num] = savedKey
		}

		r.saved[headerKey] = messageKeys
	}

	return nil
}

// EncryptedSize returns the estimated size for an encrypted ratched message,
// given the specified payload msg size.
func EncryptedSize(msgSize int) int {
	// The output slice for an Encrypt() call is modified by appending:
	//
	//   [headerNonce][seal(header)][seal(msg)]
	//
	// headerNonce is a 24 byte slice.
	//
	// seal(x) appends len(x) + secretbox.Overhead bytes.
	//
	// header is a headerSize byte slice.
	return 24 + // headerNonce length
		headerSize + secretbox.Overhead + // len(seal(header))
		msgSize + secretbox.Overhead // len(seal(msg))
}
