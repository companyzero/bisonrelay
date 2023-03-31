package rpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/decred/dcrd/chaincfg/chainhash"
)

type TxHash chainhash.Hash

// FromString decodes s into a TxHash. s must contain an hex-encoded ID of the
// correct length.
func (u *TxHash) FromString(s string) error {
	h, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	if len(h) != chainhash.HashSize {
		return fmt.Errorf("invalid TxHash length: %d", len(h))
	}
	return chainhash.Decode((*chainhash.Hash)(u), s)
}

// String returns the string representation of the tx hash.
func (u TxHash) String() string {
	return (chainhash.Hash)(u).String()
}

// MarshalJSON marshals the id into a json string.
func (u TxHash) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

// UnmarshalJSON unmarshals the json representation of a ShortID.
func (u *TxHash) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	return u.FromString(s)
}
