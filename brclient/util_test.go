package main

import (
	"testing"
)

func TestStringsCommonPrefix(t *testing.T) {
	tests := []struct {
		name string
		src  []string
		want string
	}{{
		name: "suffixed numbers",
		src:  []string{"foo", "foo1", "foo2", "foo3"},
		want: "foo",
	}, {
		name: "nil slice",
		src:  nil,
		want: "",
	}, {
		name: "none prefix",
		src:  []string{"first", "second", "third"},
		want: "",
	}, {
		name: "one wildly different",
		src:  []string{"foo", "foo1", "foo2", "bar"},
		want: "",
	}, {
		name: "empty string in slice",
		src:  []string{"foo", "", "foo2"},
		want: "",
	}}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := stringsCommonPrefix(tc.src)
			if got != tc.want {
				t.Fatalf("unexpected result: got %s, want %s",
					got, tc.want)
			}
		})
	}
}
