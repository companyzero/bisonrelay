package strescape

import (
	"testing"
)

func TestNick(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{{
		name: "empty string",
		s:    "",
		want: "",
	}, {
		name: "all ascii string",
		s:    "all ascii chars",
		want: "all ascii chars",
	}, {
		name: "unicode graphic string",
		s:    "∀x∈ℝ ⌈x⌉ = −⌊−x⌋, α ∧ ¬β = ¬(¬α ∨ β)",
		want: "∀x∈ℝ ⌈x⌉ = −⌊−x⌋, α ∧ ¬β = ¬(¬α ∨ β)",
	}, {
		name: "new line",
		s:    "new\nline",
		want: "newline",
	}, {
		name: "tab",
		s:    "nick\ttab",
		want: "nicktab",
	}, {
		name: "nick with slashes",
		s:    "my / new \\ nick",
		want: `my / new \ nick`,
	}, {
		name: "null char",
		s:    "null\x00char",
		want: "nullchar",
	}, {
		name: "ansi escape",
		s:    "ansi\x1b[1D code",
		want: "ansi[1D code",
	}, {
		name: "invalid utf8",
		s:    "invalid\xa0\xa1 utf8",
		want: "invalid utf8",
	}, {
		name: "4 byte utf-8 chars",
		s:    "🀲 🀼 🁏",
		want: "🀲 🀼 🁏",
	}}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := Nick(tc.s)
			if got != tc.want {
				t.Fatalf("Unexpected result: got %q, want %q",
					got, tc.want)
			}
		})
	}
}

func TestContent(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{{
		name: "empty string",
		s:    "",
		want: "",
	}, {
		name: "all ascii string",
		s:    "all ascii chars",
		want: "all ascii chars",
	}, {
		name: "unicode graphic string",
		s:    "∀x∈ℝ ⌈x⌉ = −⌊−x⌋, α ∧ ¬β = ¬(¬α ∨ β)",
		want: "∀x∈ℝ ⌈x⌉ = −⌊−x⌋, α ∧ ¬β = ¬(¬α ∨ β)",
	}, {
		name: "new line",
		s:    "new\nline",
		want: "new\nline",
	}, {
		name: "tab",
		s:    "content\ttab",
		want: "content\ttab",
	}, {
		name: "content with slashes",
		s:    "my / new \\ content",
		want: `my / new \ content`,
	}, {
		name: "null char",
		s:    "null\x00char",
		want: "nullchar",
	}, {
		name: "ansi escape",
		s:    "ansi\x1b[1D code",
		want: "ansi[1D code",
	}, {
		name: "invalid utf8",
		s:    "invalid\xa0\xa1 utf8",
		want: "invalid utf8",
	}, {
		name: "4 byte utf-8 chars",
		s:    "🀲 🀼 🁏",
		want: "🀲 🀼 🁏",
	}}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := Content(tc.s)
			if got != tc.want {
				t.Fatalf("Unexpected result: got %q, want %q",
					got, tc.want)
			}
		})
	}
}

func TestPathElement(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{{
		name: "empty string",
		s:    "",
		want: "",
	}, {
		name: "all ascii string",
		s:    "all ascii chars",
		want: "all ascii chars",
	}, {
		name: "unicode graphic string",
		s:    "∀x∈ℝ ⌈x⌉ = −⌊−x⌋, α ∧ ¬β = ¬(¬α ∨ β)",
		want: "∀x∈ℝ ⌈x⌉ = −⌊−x⌋, α ∧ ¬β = ¬(¬α ∨ β)",
	}, {
		name: "new line",
		s:    "new\nline",
		want: "newline",
	}, {
		name: "windows path",
		s:    "c:\\foo\\bar.txt",
		want: "cfoobar.txt",
	}, {
		name: "unix path",
		s:    "/path/to/foo",
		want: "pathtofoo",
	}, {
		name: "windows special chars",
		s:    "am; < I* > test|ing?",
		want: "am  I  testing",
	}, {
		name: "relative path",
		s:    "../../foo/bar",
		want: "....foobar",
	}, {
		name: "current dir",
		s:    ".",
		want: "dot",
	}, {
		name: "parent dir",
		s:    "..",
		want: "dotdot",
	}, {
		name: "null char",
		s:    "null\x00char",
		want: "nullchar",
	}, {
		name: "ansi escape",
		s:    "ansi\x1b[1D code",
		want: "ansi[1D code",
	}, {
		name: "invalid utf8",
		s:    "invalid\xa0\xa1 utf8",
		want: "invalid utf8",
	}, {
		name: "4 byte utf-8 chars",
		s:    "🀲 🀼 🁏",
		want: "🀲 🀼 🁏",
	}}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := PathElement(tc.s)
			if got != tc.want {
				t.Fatalf("Unexpected result: got %q, want %q",
					got, tc.want)
			}
		})
	}
}

func TestCannonicalizeNLs(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{{
		name: "empty string",
		s:    "",
		want: "",
	}, {
		name: "single <LF>",
		s:    "\n ",
		want: "\n ",
	}, {
		name: "multiple <LF>s",
		s:    "\n\n\n\n ",
		want: "\n\n\n\n ",
	}, {
		name: "single <CR>",
		s:    "\r ",
		want: "\n ",
	}, {
		name: "multiple <CR>s",
		s:    "\r\r\r\r ",
		want: "\n\n\n\n ",
	}, {
		name: "single <CR><LF>",
		s:    "\r\n ",
		want: "\n ",
	}, {
		name: "multiple <CR><LF>s",
		s:    "\r\n\r\n\r\n\r\n ",
		want: "\n\n\n\n ",
	}, {
		name: "multiple <LF><CR>s",
		s:    "\n\r\n\r\n\r\n\r ",
		want: "\n\n\n\n\n ",
	}, {
		name: "literal escape chars",
		s:    `\n \r \r\n \n\r`,
		want: `\n \r \r\n \n\r`,
	}}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := CannonicalizeNL(tc.s)
			if got != tc.want {
				t.Fatalf("Unexpected result: got %q, want %q",
					got, tc.want)
			}
		})
	}
}

func TestResourcesPath(t *testing.T) {
	tests := []struct {
		name string
		path []string
		want string
	}{{
		name: "nil path",
		path: nil,
		want: "",
	}, {
		name: "empty path",
		path: []string{},
		want: "",
	}, {
		name: "single path element",
		path: []string{"first"},
		want: "first",
	}, {
		name: "multiple path element",
		path: []string{"first", "second", "third"},
		want: "first/second/third",
	}, {
		name: "unprintable path element",
		path: []string{"start", "ansi \x1b escape", "end"},
		want: `start/ansi \x1b escape/end`,
	}, {
		name: "null char in path element",
		path: []string{"start", "null \x00 char", "end"},
		want: `start/null \x00 char/end`,
	}, {
		name: "graphical unicode path element",
		path: []string{"start", "∀x∈ℝ ⌈x⌉", "end"},
		want: "start/∀x∈ℝ ⌈x⌉/end",
	}, {
		name: "newline in path element",
		path: []string{"start", "new \n line", "end"},
		want: `start/new \n line/end`,
	}, {
		name: "invalid utf-8 in path element",
		path: []string{"start", "invalid \xa0\xa1 utf8", "end"},
		want: `start/invalid \xa0\xa1 utf8/end`,
	}, {
		name: "escape sequence in path element",
		path: []string{"start", `escape \x01 sequence`, "end"},
		want: `start/escape \\x01 sequence/end`,
	}, {
		name: "path separator in path element",
		path: []string{"start", "path / separator", "end"},
		want: `start/path \x2f separator/end`,
	}}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := ResourcesPath(tc.path)
			if got != tc.want {
				t.Fatalf("Unexpected result: got %q, want %q",
					got, tc.want)
			}
		})
	}
}
