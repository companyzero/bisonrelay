package testutils

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
)

// MockPayClient fulfills the [clientintf.PaymentClient] interface, while
// allowing to fail certain calls. It is used for tests.
//
// It behaves as the free payment client, except for hooked calls.
type MockPayClient struct {
	mtx            sync.Mutex
	payInvoice     func(string) (int64, error)
	isPayCompleted func(string) (int64, error)
	getInvoice     func(int64, func(int64)) (string, error)
	decodeInvoice  func(string) (clientintf.DecodedInvoice, error)
}

func (pc *MockPayClient) PayScheme() string {
	return rpc.PaySchemeFree
}

func (pc *MockPayClient) HookPayInvoice(hook func(string) (int64, error)) {
	pc.mtx.Lock()
	pc.payInvoice = hook
	pc.mtx.Unlock()
}

func (pc *MockPayClient) PayInvoice(_ context.Context, invoice string) (int64, error) {
	pc.mtx.Lock()
	hook := pc.payInvoice
	pc.mtx.Unlock()
	if hook != nil {
		return hook(invoice)
	}
	return 0, nil
}

func (pc *MockPayClient) PayInvoiceAmount(_ context.Context, invoice string, _ int64) (int64, error) {
	pc.mtx.Lock()
	hook := pc.payInvoice
	pc.mtx.Unlock()
	if hook != nil {
		return hook(invoice)
	}
	return 0, nil
}

func (pc *MockPayClient) HookGetInvoice(hook func(int64, func(int64)) (string, error)) {
	pc.mtx.Lock()
	pc.getInvoice = hook
	pc.mtx.Unlock()
}

func (pc *MockPayClient) GetInvoice(ctx context.Context, mat int64, cb func(int64)) (string, error) {
	pc.mtx.Lock()
	hook := pc.getInvoice
	pc.mtx.Unlock()
	if hook != nil {
		return hook(mat, cb)
	}

	return fmt.Sprintf("free invoice for %d milliatoms", mat), nil
}

func (pc *MockPayClient) IsInvoicePaid(context.Context, int64, string) error {
	return nil
}

func (pc *MockPayClient) HookIsPayCompleted(hook func(string) (int64, error)) {
	pc.mtx.Lock()
	pc.isPayCompleted = hook
	pc.mtx.Unlock()
}

func (pc *MockPayClient) IsPaymentCompleted(_ context.Context, invoice string) (int64, error) {
	pc.mtx.Lock()
	hook := pc.isPayCompleted
	pc.mtx.Unlock()
	if hook != nil {
		return hook(invoice)
	}

	return 0, nil
}

// farFutureExpiryTime is a time far in the future for the expiration of free
// invoices.
var farFutureExpiryTime = time.Date(2200, 01, 01, 0, 0, 0, 0, time.UTC)

func (pc *MockPayClient) HookDecodeInvoice(hook func(string) (clientintf.DecodedInvoice, error)) {
	pc.mtx.Lock()
	pc.decodeInvoice = hook
	pc.mtx.Unlock()
}

// DefaultDecodeInvoice is the default behavior of the MockPayClient
// DecodeInvoice(). Useful when DecodeInvoice is hooked and the default behavior
// is desired for a particular invoice.
func (pc *MockPayClient) DefaultDecodeInvoice(invoice string) (clientintf.DecodedInvoice, error) {
	var id [32]byte
	copy(id[:], invoice)
	return clientintf.DecodedInvoice{ExpiryTime: farFutureExpiryTime, ID: id[:]}, nil
}

func (pc *MockPayClient) DecodeInvoice(_ context.Context, invoice string) (clientintf.DecodedInvoice, error) {
	pc.mtx.Lock()
	hook := pc.decodeInvoice
	pc.mtx.Unlock()
	if hook != nil {
		return hook(invoice)
	}

	return pc.DefaultDecodeInvoice(invoice)
}
