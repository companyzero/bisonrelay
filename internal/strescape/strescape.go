package strescape

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Nick returns s escaped from chars that don't don't belong in a nick.
func Nick(s string) string {
	return strings.Map(func(r rune) rune {
		if !strconv.IsPrint(r) {
			return -1
		}
		if r == utf8.RuneError {
			return -1
		}
		return r
	}, s)
}

// Content returns s escaped from chars that don't belong in content.
func Content(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return r
		}
		if !strconv.IsGraphic(r) {
			return -1
		}
		if r == utf8.RuneError {
			return -1
		}
		return r
	}, s)
}

var pathElementNonChars = map[rune]struct{}{
	':':  {},
	'\\': {},
	'/':  {},
	'*':  {},
	'?':  {},
	'<':  {},
	'>':  {},
	'|':  {},
	';':  {},
}

// PathElement returns s escaped from chars that modify a path element.
func PathElement(s string) string {
	if s == "." {
		return "dot"
	}

	if s == ".." {
		return "dotdot"
	}

	return strings.Map(func(r rune) rune {
		if !strconv.IsPrint(r) {
			return -1
		}
		if _, ok := pathElementNonChars[r]; ok {
			return -1
		}
		if r == utf8.RuneError {
			return -1
		}
		return r
	}, s)
}

// CannonicalizeNL converts all newline char sequences to \n. Additionally, it
// trims all empty newlines from the right of the string.
func CannonicalizeNL(val string) string {
	val = strings.ReplaceAll(val, "\r\n", "\n")
	val = strings.ReplaceAll(val, "\r", "\n")
	val = strings.TrimRight(val, "\n")
	return val
}

// ResourcesPathElements escapes each individual path element.
func ResourcesPathElements(path []string) []string {
	escaped := make([]string, len(path))
	for i := range path {
		v := path[i]
		v = strconv.Quote(v)
		v = v[1 : len(v)-1]
		v = strings.ReplaceAll(v, "/", `\x2f`)
		escaped[i] = v
	}
	return escaped
}

// ResourcesPath escapes and joins the specified path into a single string.
func ResourcesPath(path []string) string {
	return strings.Join(ResourcesPathElements(path), "/")
}
