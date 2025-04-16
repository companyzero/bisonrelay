package clientdb

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/assert"
)

// TestBundledUserResponsesMapJSON tests that the BundledUserResponsesMap
// structure can be encoded to/from JSON.
func TestBundledUserResponsesMapJSON(t *testing.T) {
	// Build a sample bundled response object.
	var burmap BundledUserResponsesMap
	uid := clientintf.UserID{0: 0x01}
	bundleID := clientintf.PagesSessionID(0x1011)
	pageID := clientintf.PagesSessionID(0x1012)
	path := "/path/to/item"
	burmap.initBundledResponse(uid, bundleID)
	burmap.setBundledUserReponse(uid, path, bundleID, pageID)

	// Encode it as JSON.
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	if err := enc.Encode(&burmap); err != nil {
		t.Fatalf("Unable to encode BundledUserResponsesMap as JSON: %v", err)
	}
	jsonEncoded := buf.Bytes()
	// t.Log(string(jsonEncoded))

	// Decode it from JSON.
	var gotBurmap BundledUserResponsesMap
	dec := json.NewDecoder(bytes.NewBuffer(jsonEncoded))
	if err := dec.Decode(&gotBurmap); err != nil {
		t.Fatalf("Unable to decode BundledUserResponsesMap from JSON: %v", err)
	}
	// t.Logf("%s", spew.Sdump(gotBurmap))

	// Verify we can get back the id.
	gotPageID, _ := gotBurmap.getBundledUserPageID(uid, path)
	assert.DeepEqual(t, gotPageID, pageID)
	assert.DeepEqual(t, gotBurmap, burmap)
}
