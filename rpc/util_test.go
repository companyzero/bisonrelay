package rpc

import (
	"testing"

	"github.com/companyzero/bisonrelay/internal/assert"
)

// TestSplitSuggestedClientVersions tests the split function works.
func TestSplitSuggestedClientVersions(t *testing.T) {
	mkwant := func(kvs ...string) []SuggestedClientVersion {
		if len(kvs)%2 != 0 {
			panic("wrong call")
		}
		res := make([]SuggestedClientVersion, len(kvs)/2)
		for i := 0; i < len(kvs)/2; i++ {
			res[i] = SuggestedClientVersion{Client: kvs[i*2], Version: kvs[i*2+1]}
		}
		return res
	}

	tests := []struct {
		in   string
		want []SuggestedClientVersion
	}{{
		in:   "", // Empty string
		want: mkwant(),
	}, {
		in:   "str", // No "="
		want: mkwant(),
	}, {
		in:   "justkey=", // Just key value
		want: mkwant(),
	}, {
		in:   "key=1.3.4", // Valid
		want: mkwant("key", "1.3.4"),
	}, {
		in:   "spaces and key   =   1.3.4", // Invalid spaces
		want: mkwant(),
	}, {
		in:   "   underline_and_key   =   1.3.4", // Valid underline and spaces
		want: mkwant("underline_and_key", "1.3.4"),
	}, {
		in:   "key=v1.3.4", // Invalid "v" char in version
		want: mkwant(),
	}, {
		in:   "key=1.3.4+boo", // Discarded meta section
		want: mkwant("key", "1.3.4"),
	}, {
		in:   "key1 = 1.4.3, key2= 1\t, key3= 1.2.3.4.5.6", // Multiple valid
		want: mkwant("key1", "1.4.3", "key2", "1", "key3", "1.2.3.4.5.6"),
	}, {
		in:   "key = 1.3.4, foo, bar=2.3", // Discarded invalid key-value
		want: mkwant("key", "1.3.4", "bar", "2.3"),
	}}

	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			got := SplitSuggestedClientVersions(test.in)
			assert.DeepEqual(t, got, test.want)
		})
	}
}
