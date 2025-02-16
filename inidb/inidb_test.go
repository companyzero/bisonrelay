// Copyright (c) 2016-2020 Company 0, LLC.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package inidb

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var (
	i        *INIDB
	filename string
)

func TestOpenFail(t *testing.T) {
	_, err := New("", false, -1)
	if err == nil {
		t.Fatalf("TestOpenFail should have failed")
	}
}

func TestCreateNodir(t *testing.T) {
	dir, err := os.MkdirTemp("", "inidb")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	_, err = New(filepath.Join(dir, "doesntexist", "db.ini"), true, 10)
	if err != nil && !errors.Is(err, ErrCreated) {
		t.Fatal(err)
	}
}

func TestCreate(t *testing.T) {
	dir, err := os.MkdirTemp("", "inidb")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	_, err = New(filepath.Join(dir, "db.ini"), true, 10)
	if err != nil && !errors.Is(err, ErrCreated) {
		t.Fatal(err)
	}
}

func TestOpen(t *testing.T) {
	var err error

	// copy test file to a temp location
	data, err := os.ReadFile("testdb.ini")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp("", "inidb")
	if err != nil {
		t.Fatal(err)
	}
	filename = filepath.Join(dir, "testdb.ini")
	err = os.WriteFile(filename, data, 0600)
	if err != nil {
		t.Fatal(err)
	}

	i, err = New(filename, false, 10)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGet(t *testing.T) {
	value, err := i.Get("other", "oink")
	if err != nil {
		t.Fatal(err)
	}
	if value != "pig" {
		t.Fatalf("TestGet value not found")
	}

	// not found
	_, err = i.Get("other", "oink1")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("record should not have been found")
	}

	// add
	err = i.Set("other", "oink1", "bleh")
	if err != nil {
		t.Fatal(err)
	}

	// should be found
	value, err = i.Get("other", "oink1")
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("record should have been found")
	}
	if value != "bleh" {
		t.Fatalf("TestGet value not found")
	}

	// add table
	i.NewTable("newtable")

	// add record to new table
	err = i.Set("newtable", "oink1", "bleh")
	if err != nil {
		t.Fatal(err)
	}

	// save file
	err = i.Save()
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetNewFile(t *testing.T) {
	ii, err := New(filename, false, 10)
	if err != nil {
		t.Fatal(err)
	}

	// search appended key
	value, err := ii.Get("other", "oink1")
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("record should have been found")
	}
	if value != "bleh" {
		t.Fatalf("TestGetNewFile value not found")
	}

	// search appended table
	value, err = ii.Get("newtable", "oink1")
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("record should have been found")
	}
	if value != "bleh" {
		t.Fatalf("TestGetNewFile value not found")
	}
}

func TestGetFileMax(t *testing.T) {
	ii, err := New(filename, false, 3)
	if err != nil {
		t.Fatal(err)
	}

	for x := 0; x < 5; x++ {
		// save file
		ii.dirty = true
		err = ii.Save()
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	d, err := os.ReadDir(filepath.Dir(ii.filename))
	if err != nil {
		t.Fatal(err)
	}
	// expect 4 ini files
	if len(d) != 4 {
		t.Fatalf("invalid directory count")
	}

	// Remove the global test dir.
	os.RemoveAll(filepath.Dir(ii.filename))
}

func TestDel(t *testing.T) {
	dir, err := os.MkdirTemp("", "inidb")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	idb, err := New(filepath.Join(dir, "db.ini"), true, 10)
	if err != nil && !errors.Is(err, ErrCreated) {
		t.Fatal(err)
	}

	// add
	idb.NewTable("floing")
	err = idb.Set("floing", "bar", "baz")
	if err != nil {
		t.Fatal(err)
	}

	// get to make sure
	_, err = idb.Get("floing", "bar")
	if err != nil {
		t.Fatal(err)
	}

	// Del
	err = idb.Del("floing", "bar")
	if err != nil {
		t.Fatal(err)
	}

	// test negative Get
	_, err = idb.Get("floing", "bar")
	if !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}

	// test negative Del
	err = idb.Del("doesntexist", "bar")
	if !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestRecords(t *testing.T) {
	dir, err := os.MkdirTemp("", "inidb")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	idb, err := New(filepath.Join(dir, "db.ini"), true, 10)
	if err != nil && !errors.Is(err, ErrCreated) {
		t.Fatal(err)
	}

	// add
	idb.NewTable("floing")
	err = idb.Set("floing", "bar", "baz")
	if err != nil {
		t.Fatal(err)
	}

	// get to make sure
	_, err = idb.Get("floing", "bar")
	if err != nil {
		t.Fatal(err)
	}

	// test Records
	records := idb.Records("floing")
	if len(records) != 1 {
		t.Fatalf("len")
	}

	_, found := records["bar"]
	if !found {
		t.Fatalf("!found")
	}
}
