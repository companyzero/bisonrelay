// Copyright (c) 2016 Company 0, LLC.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package zkidentity

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
)

func TestNew(t *testing.T) {
	_, err := New("alice mcmoo", "alice")
	if err != nil {
		t.Fatalf("New alice: %v", err)
	}
}

func TestString(t *testing.T) {
	alice, err := New("alice mcmoo", "alice")
	if err != nil {
		t.Fatalf("New alice: %v", err)
	}

	s := fmt.Sprintf("%v", alice.Public)
	ss := hex.EncodeToString(alice.Public.Identity[:])
	if s != ss {
		t.Fatalf("stringer not working")
	}
}

func TestSignVerify(t *testing.T) {
	alice, err := New("alice mcmoo", "alice")
	if err != nil {
		t.Fatalf("New alice: %v", err)
	}

	message := []byte("this is a message")
	signature := alice.SignMessage(message)
	if !alice.Public.VerifyMessage(message, &signature) {
		t.Fatalf("corrupt signature")
	}
}

func TestJsonEncode(t *testing.T) {
	alice, err := New("alice mcmoo", "alice")
	if err != nil {
		t.Fatalf("New alice: %v", err)
	}

	blob, err := json.Marshal(alice)
	if err != nil {
		t.Fatal(err)
	}

	aliceRecovered := new(FullIdentity)
	if err := json.Unmarshal(blob, aliceRecovered); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(alice, aliceRecovered) {
		t.Fatalf("Unequal alice after recovery: %s vs %s",
			spew.Sdump(alice), spew.Sdump(aliceRecovered))
	}
}
