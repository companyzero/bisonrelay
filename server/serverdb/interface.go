package serverdb

import (
	"context"
	"errors"
	"time"

	"github.com/companyzero/bisonrelay/ratchet"
)

var ErrAlreadyStoredRV = errors.New("already stored payload at the RV point")

type FetchPayloadResult struct {
	Payload    []byte
	InsertTime time.Time
}

type ServerDB interface {
	StorePayload(ctx context.Context, rv ratchet.RVPoint, payload []byte, insertTime time.Time) error
	FetchPayload(ctx context.Context, rv ratchet.RVPoint) (*FetchPayloadResult, error)
	RemovePayload(ctx context.Context, rv ratchet.RVPoint) error
	IsSubscriptionPaid(ctx context.Context, rv ratchet.RVPoint) (bool, error)
	StoreSubscriptionPaid(ctx context.Context, rv ratchet.RVPoint, insertTime time.Time) error
	Expire(ctx context.Context, date time.Time) (uint64, error)
	IsPushPaymentRedeemed(ctx context.Context, payID []byte) (bool, error)
	StorePushPaymentRedeemed(ctx context.Context, payID []byte, insertTime time.Time) error
	IsMaster(ctx context.Context) (bool, error)
	HealthCheck(ctx context.Context) error
}
