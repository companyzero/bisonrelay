package rpc

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/zkidentity"
)

// TestMaxSizeVersions verifies the estimated RM wire size for a message of the
// maximum payload size is less than the max msg size for every version and
// that an actually encoded message also respects this size.
func TestMaxSizeVersions(t *testing.T) {
	id, err := zkidentity.New("Alice McMalice", "alice")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		version MaxMsgSizeVersion
	}{{
		name:    "v0",
		version: MaxMsgSizeV0,
	}, {
		name:    "v1",
		version: MaxMsgSizeV1,
	}}
	for i := range tests {
		tc := tests[i]
		t.Run(tc.name, func(t *testing.T) {
			// Generate an RM with max payload for this version.
			maxPayload := MaxPayloadSizeForVersion(tc.version)
			data := make([]byte, maxPayload)
			n, err := rand.Read(data[:])
			assert.NilErr(t, err)
			if n != len(data) {
				t.Fatal("too few bytes read")
			}
			rm := RMFTGetChunkReply{
				FileID: zkidentity.ShortID{}.String(),
				Index:  1<<32 - 1,
				Chunk:  data,
				Tag:    1<<32 - 1,
			}

			// Sign, compress and encode this RM.
			compressed, err := ComposeCompressedRM(id.SignMessage, rm, zlib.NoCompression)
			assert.NilErr(t, err)
			maxSize := MaxMsgSizeForVersion(tc.version)
			estSize := uint(EstimateRoutedRMWireSize(len(compressed)))
			if estSize > maxSize {
				t.Fatalf("Estimated size %d > max msg size %d",
					estSize, maxSize)
			}

			// Create a PRPC message to push this RM from server
			// to client.
			prpcMsg := Message{
				Command:   strings.Repeat("s", 256),
				TimeStamp: 1<<63 - 1,
			}
			prpcPayload := PushRoutedMessage{
				Payload:   compressed,
				Timestamp: 1<<63 - 1,
			}

			// Encode this message as it would be seen on the wire.
			var bb bytes.Buffer
			enc := json.NewEncoder(&bb)
			assert.NilErr(t, enc.Encode(prpcMsg))
			assert.NilErr(t, enc.Encode(prpcPayload))
			wireBytes := bb.Bytes()

			// Ensure the wire message is not larger than the max
			// size.
			if uint(len(wireBytes)) > maxSize {
				t.Fatalf("RM with payload size %d compressed to "+
					"%d bytes and sent in the wire with %d bytes "+
					"which is > max size %d",
					maxPayload, len(compressed), len(wireBytes), maxSize)
			}

			// Decompose the compressed message as it would in the
			// receiving client.
			_, decomposed, err := DecomposeRM(id.Public.VerifyMessage, compressed, maxSize)
			assert.NilErr(t, err)
			decomposedRM := decomposed.(RMFTGetChunkReply)
			if !bytes.Equal(decomposedRM.Chunk, rm.Chunk) {
				t.Fatalf("Composed and decomposed chunks do not match")
			}
		})
	}
}

// TestCalcPushCostMAtoms tests the correctness of CalcPushCostMAtosm for
// various sample sizes.
func TestCalcPushCostMAtoms(t *testing.T) {
	kb := uint64(1000)
	mb := kb * 1000
	gb := mb * 1000
	tb := gb * 1000

	tests := []struct {
		minRate uint64
		rate    uint64 // MAtoms/<bytes>
		bytes   uint64
		size    uint64
		want    uint64
		wantErr error
	}{{
		minRate: PropPushPaymentRateMinMAtomsDefault,
		rate:    1, // 1 MAtom/byte
		bytes:   PropPushPaymentRateBytesDefault,
		size:    1,
		want:    PropPushPaymentRateMinMAtomsDefault,
	}, {
		minRate: PropPushPaymentRateMinMAtomsDefault,
		rate:    5, // 5 MAtoms/byte
		bytes:   PropPushPaymentRateBytesDefault,
		size:    mb,
		want:    mb * 5,
	}, {
		minRate: PropPushPaymentRateMinMAtomsDefault,
		rate:    5, // 5 MAtoms/kb
		bytes:   kb,
		size:    kb / 2,
		want:    PropPushPaymentRateMinMAtomsDefault,
	}, {
		minRate: PropPushPaymentRateMinMAtomsDefault,
		rate:    5, // 5 MAtoms/kb
		bytes:   kb,
		size:    mb,
		want:    mb * 5 / kb,
	}, {
		minRate: PropPushPaymentRateMinMAtomsDefault,
		rate:    1e11, // 1e11 MAtoms/mb == 1 dcr/mb
		bytes:   mb,
		size:    gb,
		wantErr: errPushCostOverflows,
	}, {
		minRate: PropPushPaymentRateMinMAtomsDefault,
		rate:    1e5, // 100,000 MAtoms/byte == 1 dcr/mb
		bytes:   1,
		size:    gb,
		want:    1000 * 1e8 * 1000, // 1000 MB * 1e8 atoms (1 dcr) * 1000 (matoms)
	}, {
		minRate: PropPushPaymentRateMinMAtomsDefault,
		rate:    1, // 1 MAtom/10 bytes == 0.001 dcr/gb
		bytes:   10,
		size:    10 * gb,
		want:    10 * 1e5 * 1000, // 10 GB * 1e5 atoms * 1000 (matoms)
	}, {
		minRate: PropPushPaymentRateMinMAtomsDefault,
		rate:    1, // 1 MAtom/10 bytes == 0.001 dcr/gb
		bytes:   10,
		size:    tb,
		want:    1000 * 1e5 * 1000, // 1000 GB (1 TB) * 1e5 atoms * 1000 (matoms)
	}, {
		minRate: PropPushPaymentRateMinMAtomsDefault,
		rate:    1, // 1 MAtom/10 bytes == 0.001 dcr/gb
		bytes:   10,
		size:    1,
		want:    PropPushPaymentRateMinMAtomsDefault,
	}, {
		minRate: PropPushPaymentRateMinMAtomsDefault,
		rate:    1, // 1 MAtom/10 bytes == 0.001 dcr/gb
		bytes:   10,
		size:    10009,
		want:    PropPushPaymentRateMinMAtomsDefault,
	}, {
		minRate: PropPushPaymentRateMinMAtomsDefault,
		rate:    1, // 1 MAtom/10 bytes == 0.001 dcr/gb
		bytes:   10,
		size:    10010,
		want:    1001,
	}, {
		minRate: PropPushPaymentRateMinMAtomsDefault,
		rate:    1,
		bytes:   1,
		size:    math.MaxInt64 + 1,
		wantErr: errPushCostOverflowsInt64,
	}}

	for _, tc := range tests {
		name := fmt.Sprintf("%d/%d/%d/%d", tc.minRate, tc.rate, tc.bytes, tc.size)
		t.Run(name, func(t *testing.T) {
			got, gotErr := CalcPushCostMAtoms(tc.minRate, tc.rate, tc.bytes,
				tc.size)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Unexpected error: got %v, want %v", gotErr, tc.wantErr)
			}
			if got != int64(tc.want) {
				t.Fatalf("Unexpected value: got %d, want %d",
					got, tc.want)
			}
		})
	}
}
