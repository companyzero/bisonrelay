package main

import (
	"reflect"
	"testing"
)

func TestParseCommandLine(t *testing.T) {
	tests := []struct {
		name string
		cl   string
		args []string
	}{{
		name: "no leader",
		cl:   "test",
		args: nil,
	}, {
		name: "leader with no cmd",
		cl:   "/",
		args: []string{},
	}, {
		name: "cmd with no arguments",
		cl:   "/test",
		args: []string{"test"},
	}, {
		name: "cmd with single argument",
		cl:   "/test foo",
		args: []string{"test", "foo"},
	}, {
		name: "cmd with single argument and trailing space",
		cl:   "/test foo    ",
		args: []string{"test", "foo"},
	}, {
		name: "cmd with two arguments",
		cl:   "/test foo bar",
		args: []string{"test", "foo", "bar"},
	}, {
		name: "cmd with two arguments and multiple spaces",
		cl:   "/test      foo      bar     ",
		args: []string{"test", "foo", "bar"},
	}, {
		name: "cmd with quoted argument",
		cl:   "/test foo \"   a quoted arg    \" bar \"second quoted arg\" baz   ",
		args: []string{"test", "foo", "   a quoted arg    ", "bar",
			"second quoted arg", "baz"},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotArgs := parseCommandLine(tc.cl)
			if !reflect.DeepEqual(gotArgs, tc.args) {
				t.Fatalf("Unexpected result: got %v, want %v",
					gotArgs, tc.args)
			}
		})
	}
}

func TestPopNargs(t *testing.T) {
	tests := []struct {
		name string
		cl   string
		n    int
		args []string
		rest string
	}{{
		name: "no leader",
		cl:   "test",
		n:    1,
		args: []string{"test"},
		rest: "",
	}, {
		name: "leader with no cmd",
		cl:   "/",
		n:    1,
		args: []string{"/"},
		rest: "",
	}, {
		name: "cmd with no arguments",
		cl:   "/test",
		n:    1,
		args: []string{"/test"},
		rest: "",
	}, {
		name: "cmd with single argument and one pop",
		cl:   "/test foo",
		n:    1,
		args: []string{"/test"},
		rest: "foo",
	}, {
		name: "cmd with single argument and two pops",
		cl:   "/test foo",
		n:    2,
		args: []string{"/test", "foo"},
		rest: "",
	}, {
		name: "cmd with single argument and trailing space",
		cl:   "/test foo    ",
		n:    2,
		args: []string{"/test", "foo"},
		rest: "",
	}, {
		name: "cmd with two arguments n=1",
		cl:   "/test foo bar",
		n:    1,
		args: []string{"/test"},
		rest: "foo bar",
	}, {
		name: "cmd with two arguments n=2",
		cl:   "/test foo bar",
		n:    2,
		args: []string{"/test", "foo"},
		rest: "bar",
	}, {
		name: "cmd with two arguments n=3",
		cl:   "/test foo bar",
		n:    3,
		args: []string{"/test", "foo", "bar"},
		rest: "",
	}, {
		name: "cmd with two arguments n=4",
		cl:   "/test foo bar",
		n:    4,
		args: nil,
		rest: "",
	}, {
		name: "cmd with two arguments and multiple spaces",
		cl:   "/test      foo      bar     ",
		n:    3,
		args: []string{"/test", "foo", "bar"},
		rest: "",
	}, {
		name: "cmd with quoted argument",
		cl:   "/test foo \"   a quoted arg    \" bar   \"second quoted arg\" baz   ",
		n:    3,
		args: []string{"/test", "foo", "   a quoted arg    "},
		rest: "bar   \"second quoted arg\" baz",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotArgs, gotRest := popNArgs(tc.cl, tc.n)
			if !reflect.DeepEqual(gotArgs, tc.args) {
				t.Fatalf("Unexpected args: got %#v, want %#v",
					gotArgs, tc.args)
			}
			if gotRest != tc.rest {
				t.Fatalf("Unexpected rest: got %v, want %v",
					gotRest, tc.rest)
			}
		})
	}
}
