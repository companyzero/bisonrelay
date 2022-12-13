package clientintf

import (
	"testing"

	"github.com/companyzero/bisonrelay/rpc"
)

func TestPostTitle(t *testing.T) {
	tests := []struct {
		name      string
		titleAttr string
		mainAttr  string
		wantTitle string
	}{{
		name:      "empty post",
		wantTitle: "",
	}, {
		name:      "prefer title to content",
		titleAttr: "test Title",
		mainAttr:  "test content",
		wantTitle: "test Title",
	}, {
		name:      "single line title",
		titleAttr: "test title",
		wantTitle: "test title",
	}, {
		name:      "title ends in newline",
		titleAttr: "first line\n",
		wantTitle: "first line",
	}, {
		name:      "title starts with newline",
		titleAttr: "\nfirst line",
		wantTitle: "first line",
	}, {
		name:      "multiline title",
		titleAttr: "first line\nsecond line",
		wantTitle: "first line",
	}, {
		name:      "multiline title with CR",
		titleAttr: "first line\rsecond line",
		wantTitle: "first line",
	}, {
		name:      "multiline content",
		mainAttr:  "first line\nsecond line",
		wantTitle: "first line",
	}, {
		name:      "space at the start and end",
		mainAttr:  "     test title    ",
		wantTitle: "test title",
	}, {
		name:      "multiple empty lines at the start",
		mainAttr:  "\n  \n\n\n  \n   first line   \nsecond line",
		wantTitle: "first line",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pm := rpc.PostMetadata{
				Attributes: map[string]string{
					rpc.RMPTitle: tc.titleAttr,
					rpc.RMPMain:  tc.mainAttr,
				},
			}
			gotTitle := PostTitle(&pm)
			if gotTitle != tc.wantTitle {
				t.Fatalf("unexpected title: got %q, want %q",
					gotTitle, tc.wantTitle)
			}
		})
	}
}
