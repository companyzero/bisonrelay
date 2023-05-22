package zkidentity

import (
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// ShortID is a 32-byte global ID. This is used as an alias for all 32-byte
// arrays that are interpreted as unique IDs.
type ShortID [32]byte

// Bytes returns the ID as a slice of bytes.
func (u ShortID) Bytes() []byte {
	return u[:]
}

// String returns the hex encoding of the ShortID.
func (u ShortID) String() string {
	return hex.EncodeToString(u[:])
}

// ShortLogID returns the first 8 bytes in hex format (16 chars), useful as a
// short log ID.
func (u ShortID) ShortLogID() string {
	return hex.EncodeToString(u[:8])
}

// MarshalJSON marshals the id into a json string.
func (u ShortID) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

// UnmarshalJSON unmarshals the json representation of a ShortID.
func (u *ShortID) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	return u.FromString(s)
}

// FromString decodes s into an ShortID. s must contain an hex-encoded ID of the
// correct length.
func (u *ShortID) FromString(s string) error {
	h, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	if len(h) != len(u) {
		return fmt.Errorf("invalid ShortID length: %d", len(h))
	}
	copy(u[:], h)
	return nil
}

// FromBytes copies the short id from the given byte slice. The passed slice
// must have the correct length.
func (u *ShortID) FromBytes(b []byte) error {
	if len(b) != len(u) {
		return fmt.Errorf("invalid ShortID length: %d", len(b))
	}
	copy(u[:], b)
	return nil
}

// Less returns whether this is less then the passed ID. other must be non-nil,
// otherwise this panics. The comparison is made in a big-endian way.
func (u *ShortID) Less(other *ShortID) bool {
	for i := range other {
		if u[i] < other[i] {
			return true
		}
		if u[i] > other[i] {
			return false
		}
	}
	return false
}

// ConstantTimeEq returns 1 when the two ids are equal. The comparison is done
// in constant time.
func (u ShortID) ConstantTimeEq(other *ShortID) bool {
	return subtle.ConstantTimeCompare(u[:], other[:]) == 1
}

// IsEmpty returns true if the short ID is empty (i.e. all zero).
func (u ShortID) IsEmpty() bool {
	var empty ShortID = ShortID{}
	return u.ConstantTimeEq(&empty)
}
