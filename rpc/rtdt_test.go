package rpc

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/zkidentity"
)

// TestRTDTJoinCookieEncDec tests that the join cookie can be encrypted and
// decrypted.
func TestRTDTJoinCookieEncDec(t *testing.T) {
	t.Parallel()

	want := &RTDTJoinCookie{
		ServerSecret:     zkidentity.RandomShortID(),
		OwnerSecret:      zkidentity.RandomShortID(),
		Size:             rand.Uint32(),
		PeerID:           RTDTPeerID(rand.Uint32()),
		EndTimestamp:     rand.Int64(),
		PaymentTag:       rand.Uint64(),
		PublishAllowance: rand.Uint64(),
		IsAdmin:          true,
	}
	key := zkidentity.NewFixedSizeSymmetricKey()
	buf := want.Encrypt(nil, key)
	got := new(RTDTJoinCookie)
	err := got.Decrypt(buf, key, nil)
	assert.NilErr(t, err)
	assert.DeepEqual(t, got, want)
}

// TestRTDTPeerIDNextSkipsZero ensures Next() skips the zero value.
func TestRTDTPeerIDNextSkipsZero(t *testing.T) {
	id := RTDTPeerID(math.MaxUint32)
	assert.DeepEqual(t, id+1, RTDTPeerID(0))      // Simple addition would get to zero.
	assert.DeepEqual(t, id.Next(), RTDTPeerID(1)) // Next() does not.
}

// TestRTDTPushSessionRates tests the rate calculation for pushing data in RTDT
// sessions.
func TestRTDTPushSessionRates(t *testing.T) {
	const defaultJoinRate = 1000 // MilliAtoms

	tests := []struct {
		joinRate   uint64
		rateMAtoms uint64
		rateMB     uint64
		sessMB     uint32
		sessSize   uint32
		want       int64
		wantErr    error
		wantMB     uint32
	}{{
		joinRate:   defaultJoinRate,
		rateMAtoms: 1, // 1 MAtom/MB
		rateMB:     1,
		sessMB:     1,
		sessSize:   2,
		want:       defaultJoinRate + 2,
		wantMB:     1,
	}, {
		joinRate:   0,
		rateMAtoms: 1e11, // 1 DCR/MB
		rateMB:     1,
		sessMB:     10000, // 10 GB
		sessSize:   1e6,   // 1MM clients
		wantErr:    errPushCostOverflows,
	}, {
		joinRate:   0,
		rateMAtoms: 1e11, // 1 DCR/MB
		rateMB:     1,
		sessMB:     100, // 100 MB
		sessSize:   1e6, // 1MM clients
		wantErr:    errPushCostOverflowsInt64,
	}, {
		joinRate:   defaultJoinRate,
		rateMAtoms: 10000, // 10 atoms/MB
		rateMB:     1,
		sessMB:     10000, // 10GB
		sessSize:   1e6,   // 1MM clients
		want:       defaultJoinRate + 1e4*1e4*1e6,
		wantMB:     10000,
	}}

	for _, tc := range tests {
		name := fmt.Sprintf("%d/%d/%d/%d/%d", tc.joinRate, tc.rateMAtoms,
			tc.rateMB, tc.sessMB, tc.sessSize)
		t.Run(name, func(t *testing.T) {
			got, gotErr := CalcRTDTSessPushMAtoms(tc.joinRate, tc.rateMAtoms, tc.rateMB,
				tc.sessMB, tc.sessSize)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Unexpected error: got %v, want %v", gotErr, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("Unexpected value: got %d, want %d",
					got, tc.want)
			}

			if tc.wantErr != nil {
				return
			}

			// The reverse calculation should yield the same input
			// MB.
			gotMB, gotMBErr := CalcRTDTSessPushMB(tc.joinRate, tc.rateMAtoms,
				tc.rateMB, tc.sessSize, got)
			if gotMBErr != nil {
				t.Fatalf("Unexpected error in reverse calc: %v", gotMBErr)
			}
			if gotMB != tc.wantMB {
				t.Fatalf("Unexpected value: got %d, want %d", gotMB, tc.wantMB)
			}
		})
	}
}

// TestCalcRTDTSessionPushMB tests calculating back the amount allowed to be
// pushed in a session.
func TestCalcRTDTSessionPushMB(t *testing.T) {
	const defaultJoinRate = 1000 // MilliAtoms

	tests := []struct {
		joinRate   uint64
		rateMAtoms uint64
		rateMB     uint64
		sessSize   uint32
		paidMAtoms int64
		want       uint32
		wantErr    error
	}{{
		joinRate:   defaultJoinRate,
		rateMAtoms: 1000,
		rateMB:     1,
		sessSize:   2,
		paidMAtoms: defaultJoinRate - 1,
		wantErr:    errRTDTSessPaymentLowerThanMin,
	}, {
		joinRate:   defaultJoinRate,
		rateMAtoms: 1000,
		rateMB:     1,
		sessSize:   2,
		paidMAtoms: -1,
		wantErr:    errRTDTSessPaymentNegative,
	}, {
		joinRate:   defaultJoinRate,
		rateMAtoms: 1000, // 1 atom/MB
		rateMB:     1,
		sessSize:   2,
		paidMAtoms: defaultJoinRate, // Paid only the join rate.
		want:       0,               // No push allowance.
	}, {
		joinRate:   defaultJoinRate,
		rateMAtoms: 1000, // 1 atom/MB
		rateMB:     1,
		sessSize:   2,
		paidMAtoms: defaultJoinRate + 1999, // Not exactly 1 MB payment.
		want:       0,                      // No push allowance.
	}, {
		joinRate:   defaultJoinRate,
		rateMAtoms: 1000, // 1 atom/MB
		rateMB:     1,
		sessSize:   2,
		paidMAtoms: defaultJoinRate + 2000, // 1 MB payment for a 2-party session.
		want:       1,
	}, {
		joinRate:   defaultJoinRate,
		rateMAtoms: 1000, // 1 atom/MB
		rateMB:     1,
		sessSize:   2,
		paidMAtoms: defaultJoinRate + 3999, // Just shy of 2 MB.
		want:       1,
	}, {
		joinRate:   defaultJoinRate,
		rateMAtoms: 1000, // 1 atom/MB
		rateMB:     1,
		sessSize:   2,
		paidMAtoms: defaultJoinRate + 4000, // 2 MB payment.
		want:       2,
	}, {
		joinRate:   defaultJoinRate,
		rateMAtoms: 1, // 1 atom/MB
		rateMB:     1,
		sessSize:   2,
		paidMAtoms: defaultJoinRate + math.MaxUint32*2, // Very large payment.
		want:       math.MaxInt32,                      // Clamped to maxInt32.
	}, {
		joinRate:   defaultJoinRate,
		rateMAtoms: 10000, // 10 atoms / 10 MB
		rateMB:     10,
		sessSize:   1e6,                           // 1 MM clients
		paidMAtoms: defaultJoinRate + 1e6*1e3*1e4, // Pay for 10 GB.
		want:       10000,                         // 10GB
	}}

	for _, tc := range tests {
		name := fmt.Sprintf("%d/%d/%d/%d/%d", tc.joinRate, tc.rateMAtoms,
			tc.rateMB, tc.sessSize, tc.paidMAtoms)
		t.Run(name, func(t *testing.T) {
			got, gotErr := CalcRTDTSessPushMB(tc.joinRate, tc.rateMAtoms,
				tc.rateMB, tc.sessSize, tc.paidMAtoms)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Unexpected error: got %v, want %v", gotErr, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("Unexpected value: got %d, want %d",
					got, tc.want)
			}
		})
	}
}
