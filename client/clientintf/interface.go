package clientintf

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrutil/v4"
)

// ID is a 32-byte global ID. This is used as an alias for all 32-byte arrays
// that are interpreted as unique IDs.
type ID = zkidentity.ShortID

func RandomID() ID {
	var id ID
	n, err := rand.Read(id[:])
	if err != nil {
		panic(err)
	}
	if n != len(id) {
		panic("insufficient entropy to generate ID")
	}
	return id
}

type UserID = ID
type PostID = ID
type RawRVID = ID
type FileID = ID

// Conn represents the required functions for a remote connection to a server.
type Conn interface {
	io.Reader
	io.Writer
	io.Closer
	RemoteAddr() net.Addr
}

// Dialer is a function that can generate new connections to a server.
type Dialer func(context.Context) (Conn, *tls.ConnectionState, error)

// CertConfirmer is a functiion that can be called to confirm whether a given
// server is safe.
type CertConfirmer func(context.Context, *tls.ConnectionState, *zkidentity.PublicIdentity) error

// ServerPolicy is the policy for a given server session.
type ServerPolicy struct {
	PushPaymentLifetime time.Duration `json:"push_payment_lifetime"`
	MaxPushInvoices     int           `json:"max_push_invoices"`

	// MaxMsgSizeVersion is the version of the max message size accepted
	// by the server.
	MaxMsgSizeVersion rpc.MaxMsgSizeVersion `json:"max_msg_size_version"`

	// MaxMsgSize is the maximum message size accepted by the server.
	MaxMsgSize uint `json:"max_msg_size"`

	// ExpirationDays is the number of days after which data pushed to the server
	// is removed if not fetched.
	ExpirationDays int `json:"expiration_days"`

	// PushPayRateMAtoms is the rate (in milli-atoms per PushPayRateBytes) to
	// push data to the server.
	PushPayRateMAtoms uint64 `json:"push_pay_rate_matoms"`

	// PushPayRateBytes is the number of bytes used in calculating the final
	// push pay rate.
	PushPayRateBytes uint64 `json:"push_pay_rate_bytes"`

	// PushPayRateMinMAtoms is the minimum payment amount (in MAtoms)
	// independently of size.
	PushPayRateMinMAtoms uint64 `json:"push_pay_rate_min_matoms"`

	// SubPayRate is the rate (in milli-atoms) to subscribe to an RV point
	// on the server.
	SubPayRate uint64 `json:"sub_pay_rate"`

	// PingLimit is the deadline for writing messages (including ping) to
	// the server.
	PingLimit time.Duration `json:"ping_limit"`

	// ClientVersions is a list of server-provided client and versions
	// suggested for use when connecting to this server. This can be used
	// by the client to determine if it should be updated.
	ClientVersions []rpc.SuggestedClientVersion `json:"client_versions"`

	MilliAtomsPerRTSess     uint64
	MilliAtomsPerUserRTSess uint64
	MilliAtomsGetCookie     uint64
	MilliAtomsPerUserCookie uint64
	MilliAtomsRTJoin        uint64
	MilliAtomsRTPushRate    uint64
	RTPushRateMBytes        uint64
}

// CalcPushCostMAtoms calculates the cost to push a message with the given size.
func (sp *ServerPolicy) CalcPushCostMAtoms(msgSizeBytes int) (int64, error) {
	if msgSizeBytes < 0 {
		panic("msgSizeBytes cannot be negative")
	}

	return rpc.CalcPushCostMAtoms(sp.PushPayRateMinMAtoms, sp.PushPayRateMAtoms,
		sp.PushPayRateBytes, uint64(msgSizeBytes))
}

// PushDcrPerGB returns the rate to push data to the server, in DCR/GB. Assumes
// the push policy rates are valid.
func (sp *ServerPolicy) PushDcrPerGB() float64 {
	matoms, _ := sp.CalcPushCostMAtoms(1e9) // 1e9 bytes == 1GB
	return float64(matoms) / 1e11           // 1e11 == milliatoms / dcr
}

// MaxPayloadSize returns the max payload size for the server policy.
func (sp *ServerPolicy) MaxPayloadSize() int {
	return int(rpc.MaxPayloadSizeForVersion(sp.MaxMsgSizeVersion))
}

// CalcRTPushMAtoms calculates how much to pay to push the given number of bytes
// in a session of the given size using the current server policy.
func (sp *ServerPolicy) CalcRTPushMAtoms(sessSize, pushMB uint32) (int64, error) {
	return rpc.CalcRTDTSessPushMAtoms(sp.MilliAtomsRTJoin, sp.MilliAtomsRTPushRate,
		sp.RTPushRateMBytes, pushMB, sessSize)
}

// ServerSessionIntf is the interface available from serverSession to
// consumers.
type ServerSessionIntf interface {
	SendPRPC(msg rpc.Message, payload interface{}, reply chan<- interface{}) error
	RequestClose(err error)
	PayClient() PaymentClient
	Policy() ServerPolicy

	// Context returns a context that gets cancelled once this session stops
	// running.
	Context() context.Context
}

// DecodedInvoice represents an invoice that was successfully decoded by the
// PaymentClient
type DecodedInvoice struct {
	ID         []byte
	MAtoms     int64
	ExpiryTime time.Time
}

// isExpired is similar to IsExpired, but with a parametrized nowFunc to allow
// unit testing.
func (inv *DecodedInvoice) isExpired(affordance time.Duration, nowFunc func() time.Time) bool {
	now := nowFunc()
	return inv.ExpiryTime.Before(now.Add(affordance))
}

// IsExpired returns whether this invoice has expired taking
// into account the specified affordance. In other words, it returns true if
// the expiration time of the invoice is before time.Now().Add(affordance).
func (inv *DecodedInvoice) IsExpired(affordance time.Duration) bool {
	return inv.isExpired(affordance, time.Now)
}

// PaymentClient is the interface for clients that can pay for invoices.
type PaymentClient interface {
	PayScheme() string
	PayInvoice(context.Context, string) (int64, error)
	PayInvoiceAmount(context.Context, string, int64) (int64, error)
	GetInvoice(context.Context, int64, func(int64)) (string, error)
	DecodeInvoice(context.Context, string) (DecodedInvoice, error)
	IsInvoicePaid(context.Context, int64, string) error
	TrackInvoice(context.Context, string, int64) (int64, error)
	IsPaymentCompleted(context.Context, string) (int64, error)
}

// FreePaymentClient implements the PaymentClient interface for servers that
// offer the "free" payment scheme: namely, invoices are requested but there is
// nothing to pay for.
type FreePaymentClient struct{}

func (pc FreePaymentClient) PayScheme() string                                 { return rpc.PaySchemeFree }
func (pc FreePaymentClient) PayInvoice(context.Context, string) (int64, error) { return 0, nil }
func (pc FreePaymentClient) PayInvoiceAmount(context.Context, string, int64) (int64, error) {
	return 0, nil
}
func (pc FreePaymentClient) GetInvoice(ctx context.Context, mat int64, cb func(int64)) (string, error) {
	return fmt.Sprintf("free invoice for %d milliatoms", mat), nil
}
func (pc FreePaymentClient) IsInvoicePaid(context.Context, int64, string) error {
	return nil
}
func (pc FreePaymentClient) IsPaymentCompleted(context.Context, string) (int64, error) {
	return 0, nil
}
func (pc FreePaymentClient) TrackInvoice(ctx context.Context, inv string, minMAtoms int64) (int64, error) {
	return 0, nil
}

// farFutureExpiryTime is a time far in the future for the expiration of free
// invoices.
var farFutureExpiryTime = time.Date(2200, 01, 01, 0, 0, 0, 0, time.UTC)

func (pc FreePaymentClient) DecodeInvoice(_ context.Context, invoice string) (DecodedInvoice, error) {
	var id [32]byte
	copy(id[:], invoice)
	return DecodedInvoice{ExpiryTime: farFutureExpiryTime, ID: id[:]}, nil
}

// OnboardStage tracks stages of the client onboarding process.
type OnboardStage string

const (
	StageFetchingInvite      OnboardStage = "fetching_invite"
	StageInviteUnpaid        OnboardStage = "invite_unpaid"
	StageInviteFetchTimeout  OnboardStage = "invite_fetch_timeout"
	StageInviteNoFunds       OnboardStage = "invite_no_funds"
	StageRedeemingFunds      OnboardStage = "redeeming_funds"
	StageWaitingFundsConfirm OnboardStage = "waiting_funds_confirm"
	StageOpeningOutbound     OnboardStage = "opening_outbound"
	StageWaitingOutMined     OnboardStage = "waiting_out_mined"
	StageWaitingOutConfirm   OnboardStage = "waiting_out_confirm"
	StageOpeningInbound      OnboardStage = "opening_inbound"
	StageInitialKX           OnboardStage = "initial_kx"
	StageOnboardDone         OnboardStage = "done"
)

// OnboardState tracks a state of the client onboarding process.
type OnboardState struct {
	Stage        OnboardStage                 `json:"stage"`
	Key          *PaidInviteKey               `json:"key"`
	Invite       *rpc.OOBPublicIdentityInvite `json:"invite"`
	RedeemTx     *chainhash.Hash              `json:"redeem_tx"`
	RedeemAmount dcrutil.Amount               `json:"redeem_amount"`
	OutChannelID string                       `json:"out_channel_id"`
	InChannelID  string                       `json:"in_channel_id"`

	// The following fields were added in the second version of onboarding.
	// Onboards done on older versions do not have these fields set.

	OutChannelHeightHint  uint32 `json:"out_channel_height_hint"`
	OutChannelMinedHeight uint32 `json:"out_channel_mined_height"`
	OutChannelConfsLeft   int32  `json:"out_channel_confs_left"`
}

// PagesSessionID is a type that represents a page navigation session.
type PagesSessionID uint64

func (id PagesSessionID) String() string {
	return fmt.Sprintf("%08d", id)
}

// ReceivedGCMsg is an individual GC message received by a local client.
type ReceivedGCMsg struct {
	MsgID   zkidentity.ShortID `json:"msg_id"`
	UID     UserID             `json:"uid"`
	GCM     rpc.RMGroupMessage `json:"gcm"`
	TS      time.Time          `json:"ts"`
	GCAlias string             `json:"gc_alias"`
}

var (
	ErrSubsysExiting             = errors.New("subsys exiting")
	ErrInvoiceInsufficientlyPaid = errors.New("invoice insufficiently paid")
	ErrInvoiceExpired            = errors.New("invoice expired")
	ErrOnboardNoFunds            = errors.New("onboarding invite does not have any funds")
	ErrRetriablePayment          = errors.New("retriable payment error")
)
