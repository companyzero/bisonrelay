package rpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/decred/dcrd/chaincfg/chainhash"
	"golang.org/x/exp/constraints"
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

// SuggestedClientVersion is a suggested client version sent by the server.
type SuggestedClientVersion struct {
	// Client is the client identifier ("brclient", "bruig", etc).
	Client string `json:"client"`

	// Version is the suggested client version (semver compatible).
	Version string `json:"version"`
}

// SplitSuggestedClientVersions splits the given string into a list of suggested
// client versions.
func SplitSuggestedClientVersions(s string) []SuggestedClientVersion {
	// Regexp to detect comma-separated k=v strings.
	var re = regexp.MustCompile(`(?:^|,)\s*([\w]+)\s*=\s*([\d\.]+)`)

	matches := re.FindAllStringSubmatch(s, -1)
	res := make([]SuggestedClientVersion, 0, len(matches))
	for _, match := range matches {
		if len(match) != 3 {
			continue
		}

		res = append(res, SuggestedClientVersion{Client: match[1], Version: match[2]})
	}

	return res
}

// setBoolFlag sets (or clears) the bit at position bit.
func setBoolFlag[T constraints.Unsigned](current T, bit int, isSet bool) T {
	if isSet {
		return current | (1 << bit)
	} else {
		return current &^ (1 << bit)
	}
}

// isBoolFlagSet returns true if the given bit is set.
func isBoolFlagSet[T constraints.Unsigned](v T, bit int) bool {
	return v&(1<<bit) != 0
}
