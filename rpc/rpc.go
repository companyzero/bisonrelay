// Copyright (c) 2016 Company 0, LLC.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// rpc contains all structures required by the ZK protocol.
//
// A ZK session has two discrete phases:
//	1. pre session phase, used to obtain brserver key
//	2. session phase, used for all other RPC commands
//	3. once the key exchange is complete the server shall issue a Welcome
//         command.  The welcome command also transfer additional settings such
//         as tag depth etc.

package rpc

import (
	"encoding/base64"
	"strconv"
	"time"

	"github.com/companyzero/bisonrelay/ratchet"
)

type MessageMode uint32

const (
	// pre session phase
	InitialCmdIdentify = "identify"
	InitialCmdSession  = "session"

	// session phase
	SessionCmdWelcome = "welcome"

	// tagged server commands
	TaggedCmdAcknowledge = "ack"
	TaggedCmdPing        = "ping"
	TaggedCmdPong        = "pong"

	// payment cmds
	TaggedCmdGetInvoice      = "getinvoice"
	TaggedCmdGetInvoiceReply = "getinvoicereply"

	TaggedCmdRouteMessage      = "routemessage"
	TaggedCmdRouteMessageReply = "routemessagereply"

	TaggedCmdSubscribeRoutedMessages      = "subscriberoutedmessages"
	TaggedCmdSubscribeRoutedMessagesReply = "subscriberoutedmessagesreply"

	TaggedCmdPushRoutedMessage = "pushroutedmessage"

	// misc
	MessageModeNormal MessageMode = 0
	MessageModeMe     MessageMode = 1

	PaySchemeFree  = "free"
	PaySchemeDCRLN = "dcrln"

	// PingLimit is how long to wait for a ping before disconnect.
	// DefaultPingInterval is how long to wait to send the next ping.
	PingLimit           = 45 * time.Second
	DefaultPingInterval = 30 * time.Second

	// MaxChunkSize is the maximum size of a file chunk used in file
	// downloads.
	MaxChunkSize = 1024 * 1024 // 1 MiB

	// MaxMsgSize is the maximum size of a message. This was determined as
	// enough to contain a base64 encoded version of MaxChunkSize bytes,
	// along with the necessary overhead of headers, encodings and frames
	// needed by the encrypted routed messages with some room to spare, when
	// sending with compression turned off.
	MaxMsgSize = 1887437 // ~1.8 MiB

	// MinRMPushPayment is the minimum payment amount required to push a payment
	// to the server (in milliatoms).
	MinRMPushPayment uint64 = 1000

	// InvoiceExpiryAffordance is the time before the expiry a client may
	// request a new invoice.
	InvoiceExpiryAffordance = 15 * time.Second
)

// Message is the generic command that flows between a server and client and
// vice versa.  Its purpose is to add a discriminator to simplify payload
// decoding.  Additionally it has a tag that the recipient shall return
// unmodified when replying.  The tag is originated by the sender and shall be
// unique provided an answer is expected.  The receiver shall not interpret or
// use the tag in any way.  The Cleartext flag indicates that the payload is in
// clear text. This flag should only be used for proxy commands (e.g. ratchet
// reset).
type Message struct {
	Command   string // discriminator
	TimeStamp int64  // originator timestamp
	Cleartext bool   // If set Payload is in clear text, proxy use only
	Tag       uint32 // client generated tag, shall be unique
	//followed by Payload []byte
}

// RouteMessage is a hack
type RouteMessage struct {
	Rendezvous ratchet.RVPoint
	Message    []byte
}

type RouteMessageReply struct {
	Error       string
	NextInvoice string
}

type SubscribeRoutedMessages struct {
	AddRendezvous []ratchet.RVPoint // Add to subscribed RVs
	DelRendezvous []ratchet.RVPoint // Del from subscribed RVs
}

type SubscribeRoutedMessagesReply struct {
	NextInvoice string
	Error       string
}

type PushRoutedMessage struct {
	Payload   []byte
	RV        ratchet.RVPoint
	Timestamp int64
	Error     string
}

// Acknowledge is sent to acknowledge commands and Error is set if the command
// failed.
type Acknowledge struct {
	Error     string
	ErrorCode int // optional error to be used as a hint
}

// GetInvoiceAction is the action the client wants to perform and needs an
// invoice for.
type GetInvoiceAction string

const (
	InvoiceActionPush GetInvoiceAction = "push"
	InvoiceActionSub  GetInvoiceAction = "sub"
)

type GetInvoice struct {
	PaymentScheme string           // LN, on-chain, whatever
	Action        GetInvoiceAction // push or subscribe
}

type GetInvoiceReply struct {
	Invoice string // Depends on payment scheme
}

const (
	ProtocolVersion = 10
)

// Welcome is written immediately following a key exchange.  This command
// purpose is to detect if the key exchange completed on the client side.  If
// the key exchange failed the server will simply disconnect.
type Welcome struct {
	Version    int   // protocol version
	ServerTime int64 // server timestamp

	// Client shall ensure it is compatible with the server requirements
	Properties []ServerProperty // server properties
}

type ServerProperty struct {
	Key      string // name of property
	Value    string // value of property
	Required bool   // if true client must handle this entry
}

const (
	// Tag Depth is a required property.  It defines maximum outstanding
	// commands.
	PropTagDepth        = "tagdepth"
	PropTagDepthDefault = "10"

	// Server Time is a required property.  It contains the server time
	// stamp.  The client shall warn the user if the client is not time
	// synced.  Clients and proxies really shall run NTP.
	PropServerTime = "servertime"

	// Payment Scheme is a required property. It defines whether the type
	// of payment that the server expects before routing messages.
	PropPaymentScheme        = "payscheme"
	PropPaymentSchemeDefault = "free"

	// Push Payment rate is the required payment rate to push RMs when the
	// payment scheme is not free (in milli-atoms per byte).
	PropPushPaymentRate        = "pushpayrate"
	PropPushPaymentRateDefault = 100 // MilliAtoms/byte

	// Sub payment rate is the required payment rate to sub to RVs when the
	// payment scheme is not free (in milli-atoms per byte).
	PropSubPaymentRate        = "subpayrate"
	PropSubPaymentRateDefault = 1000 // MilliAtoms/sub

	// PropServerLNNode returns the node id of the server.
	PropServerLNNode = "serverlnnode"

	// PropExpirationDays is the number of days after which data is expired
	// from the server automatically.
	PropExpirationDays        = "expirationdays"
	PropExpirationDaysDefault = 7
)

var (
	// required
	DefaultPropTagDepth = ServerProperty{
		Key:      PropTagDepth,
		Value:    PropTagDepthDefault,
		Required: true,
	}
	DefaultServerTime = ServerProperty{
		Key:      PropServerTime,
		Value:    "", // int64 unix time
		Required: true,
	}
	DefaultPropPaymentScheme = ServerProperty{
		Key:      PropPaymentScheme,
		Value:    PropPaymentSchemeDefault,
		Required: true,
	}
	DefaultPropPushPaymentRate = ServerProperty{
		Key:      PropPushPaymentRate,
		Value:    strconv.Itoa(PropPushPaymentRateDefault),
		Required: true,
	}
	DefaultPropSubPaymentRate = ServerProperty{
		Key:      PropSubPaymentRate,
		Value:    strconv.Itoa(PropSubPaymentRateDefault),
		Required: true,
	}

	// TODO: make this a required prop once clients have updated.
	DefaultPropExpirationDays = ServerProperty{
		Key:      PropExpirationDays,
		Value:    strconv.Itoa(PropExpirationDaysDefault),
		Required: false,
	}

	// optional
	DefaultPropServerLNNode = ServerProperty{
		Key:      PropServerLNNode,
		Value:    "",
		Required: false,
	}

	// All properties must exist in this array.
	SupportedServerProperties = []ServerProperty{
		// required
		DefaultPropTagDepth,
		DefaultServerTime,
		DefaultPropPaymentScheme,
		DefaultPropPushPaymentRate,
		DefaultPropSubPaymentRate,

		// TODO: Add it here once the clients are updated.
		//DefaultPropExpirationDays,

		// optional
		DefaultPropServerLNNode,
	}
)

// Ping is a PRPC that is used to determine if the server is alive.
// This command must be acknowledged by the remote side.
type Ping struct{}
type Pong struct{}

// EstimateRoutedRMWireSize estimates the final wire size of a compressed RM
// (with compression set to its lowest value, which effectively disables it).
func EstimateRoutedRMWireSize(compressedRMSize int) int {
	// Estimation of the overhead in all the various framings used for a
	// message encoded by ComposeCompressedRM to be sent on the wire.
	const overheadEstimate = 512

	// The compressed RM will end up encoded as base64 within a json
	// message.
	b64size := base64.StdEncoding.EncodedLen(compressedRMSize)
	return b64size + overheadEstimate
}
