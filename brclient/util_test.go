package main

import (
	"reflect"
	"testing"

	"github.com/companyzero/bisonrelay/zkidentity"
)

func mustDecodeID(s string) zkidentity.ShortID {
	var r zkidentity.ShortID
	if err := r.FromString(s); err != nil {
		panic(err)
	}
	return r
}

func TestReplaceEmbeds(t *testing.T) {
	testRawArg := `--embed[name=textfile.txt,type=text/plain,download=891534a17af07aacd247a78e33ea93de5c5c590138af784eef3d9a7164968f4c,alt=some%20alt,data=dGVzdA==]--`
	testArg := embeddedArgs{
		name:     `textfile.txt`,
		typ:      `text/plain`,
		download: mustDecodeID("891534a17af07aacd247a78e33ea93de5c5c590138af784eef3d9a7164968f4c"),
		alt:      "some alt",
		data:     []byte("test"),
	}

	// All tags in the tests below will be replaced by "xxx".
	tests := []struct {
		name     string
		src      string
		wantArgs []embeddedArgs
		wantDst  string
	}{{
		name:     "empty string",
		src:      "",
		wantArgs: nil,
		wantDst:  "",
	}, {
		name:     "one replacement",
		src:      "start " + testRawArg + " end",
		wantArgs: []embeddedArgs{testArg},
		wantDst:  "start xxx end",
	}, {
		name:     "two replacements",
		src:      "first " + testRawArg + " second " + testRawArg + " end",
		wantArgs: []embeddedArgs{testArg, testArg},
		wantDst:  "first xxx second xxx end",
	}, {
		name:     "broken download id",
		src:      "start --embed[alt=alt,download=broken]-- end",
		wantArgs: []embeddedArgs{{alt: "alt"}},
		wantDst:  "start xxx end",
	}}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			wantArgs := tc.wantArgs
			gotDst := replaceEmbeds(tc.src, func(args embeddedArgs) string {
				t.Helper()
				if len(wantArgs) < 1 {
					t.Fatalf("got arg %v when none was expected", args)
				}
				if !reflect.DeepEqual(args, wantArgs[0]) {
					t.Fatalf("unexpected args: got %#v, want %#v",
						args, wantArgs[0])
				}
				wantArgs = wantArgs[1:]
				return "xxx"
			})
			if len(wantArgs) != 0 {
				t.Fatalf("did not get final %d expected args", len(wantArgs))
			}

			if gotDst != tc.wantDst {
				t.Fatalf("unexpected final string: got %s, want %s",
					gotDst, tc.wantDst)
			}
		})
	}
}

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
