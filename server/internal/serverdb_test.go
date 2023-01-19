package internal

import (
	"bytes"
	"context"
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/ratchet"
	brfsdb "github.com/companyzero/bisonrelay/server/internal/fsdb"
	"github.com/companyzero/bisonrelay/server/serverdb"
)

// testServerDBInterface performs interface tests in the given serverDB
// instance.
func testServerDBInterface(t *testing.T, db serverdb.ServerDB) {
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	now := time.Now()
	twoDays := time.Hour * 24 * 2
	rng := rand.New(rand.NewSource(time.Now().Unix()))

	// Trying to fetch an inexistent RV returns nil payload and no error.
	payload, err := db.FetchPayload(ctx, ratchet.RVPoint{})
	if err != nil {
		t.Fatal(err)
	}
	if payload != nil {
		t.Fatalf("unexpected payment: %x", payload)
	}

	// Storing the same content twice fails the second time with the
	// correct error, even when storing in different days.
	var rv ratchet.RVPoint
	rng.Read(rv[:])
	data1 := bytes.Repeat([]byte{0xff}, 16)
	data2 := bytes.Repeat([]byte{0xee}, 16)
	wantErr := serverdb.ErrAlreadyStoredRV
	if err := db.StorePayload(ctx, rv, data1, now); err != nil {
		t.Fatal(err)
	}
	if err := db.StorePayload(ctx, rv, data2, now); !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: got %v, want %v", err, wantErr)
	}
	if err := db.StorePayload(ctx, rv, data2, now.Add(twoDays)); !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: got %v, want %v", err, wantErr)
	}

	gotData, err := db.FetchPayload(ctx, rv)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotData.Payload, data1) {
		t.Fatalf("unexpected data: got %x, want %x",
			gotData, data1)
	}

	// Trying to fetch after deleting the content shouldn't return it
	// anymore.
	if err := db.RemovePayload(ctx, rv); err != nil {
		t.Fatal(err)
	}

	gotData, err = db.FetchPayload(ctx, rv)
	if err != nil {
		t.Fatal(err)
	}
	if gotData != nil {
		t.Fatalf("unexpected data after deleting content")
	}

	// Trying to delete twice shouldn't error
	if err := db.RemovePayload(ctx, rv); err != nil {
		t.Fatal(err)
	}

	// Test paid RV functions.
	//
	// Unpaid RV should return paid == false.
	if paid, err := db.IsSubscriptionPaid(ctx, rv); err != nil {
		t.Fatal(err)
	} else if paid {
		t.Fatalf("unexpected paid flag: got %v, want %v",
			paid, false)
	}

	// Store RV as paid.
	if err := db.StoreSubscriptionPaid(ctx, rv, now); err != nil {
		t.Fatal(err)
	}

	// Paid RV should return paid == true.
	if paid, err := db.IsSubscriptionPaid(ctx, rv); err != nil {
		t.Fatal(err)
	} else if !paid {
		t.Fatalf("unexpected paid flag: got %v, want %v",
			paid, true)
	}

	// Store RV as paid again (shouldn't error even when it's on
	// different days).
	if err := db.StoreSubscriptionPaid(ctx, rv, now); err != nil {
		t.Fatal(err)
	}
	if err := db.StoreSubscriptionPaid(ctx, rv, now.Add(twoDays)); err != nil {
		t.Fatal(err)
	}

	// Unredeemed push payment returns true.
	isRedeemed, err := db.IsPushPaymentRedeemed(ctx, rv[:])
	assert.NilErr(t, err)
	assert.DeepEqual(t, isRedeemed, false)

	// Store push payment as redeemed twice (neither should error as second
	// time is a NOP).
	assert.NilErr(t, db.StorePushPaymentRedeemed(ctx, rv[:], now))
	assert.NilErr(t, db.StorePushPaymentRedeemed(ctx, rv[:], now))

	// Push payment now shows as redeemed.
	isRedeemed, err = db.IsPushPaymentRedeemed(ctx, rv[:])
	assert.NilErr(t, err)
	assert.DeepEqual(t, isRedeemed, true)

	// Store a different rv to test expiration.
	rng.Read(rv[:])
	assert.NilErr(t, db.StorePayload(ctx, rv, data1, now))
	assert.NilErr(t, db.StoreSubscriptionPaid(ctx, rv, now))
	assert.NilErr(t, db.StorePushPaymentRedeemed(ctx, rv[:], now))

	// Expire the data and subscriptions.
	if _, err := db.Expire(ctx, now); err != nil {
		t.Fatal(err)
	}

	// Data should no longer be fetchable.
	gotData, err = db.FetchPayload(ctx, rv)
	if err != nil {
		t.Fatal(err)
	}
	if gotData != nil {
		t.Fatal("got unexpected data")
	}

	// Subscription should no longer be marked as paid.
	if paid, err := db.IsSubscriptionPaid(ctx, rv); err != nil {
		t.Fatal(err)
	} else if paid {
		t.Fatalf("unexpected paid flag: got %v, want %v",
			paid, false)
	}

	// Payment no longer shows as redeemed.
	isRedeemed, err = db.IsPushPaymentRedeemed(ctx, rv[:])
	assert.NilErr(t, err)
	assert.DeepEqual(t, isRedeemed, false)

	// Expiring an inexistent partition shouldn't error.
	if _, err := db.Expire(ctx, time.Time{}); err != nil {
		t.Fatal(err)
	}
}

func TestFSDB(t *testing.T) {
	dir, err := os.MkdirTemp("", "serverdb-fsdb")
	if err != nil {
		t.Fatal(err)
	}
	msgsDir := filepath.Join(dir, "routedmessages")
	subsDir := filepath.Join(dir, "paidrvs")

	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("Server DB dir: %s", dir)
		} else {
			os.RemoveAll(dir)
		}
	})

	db, err := brfsdb.NewFSDB(msgsDir, subsDir)
	if err != nil {
		t.Fatal(err)
	}

	testServerDBInterface(t, db)
}
