package rpc

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
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
