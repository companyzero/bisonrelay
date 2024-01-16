package main

import (
	"crypto/tls"
	"errors"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrlnd/lnrpc/initchainsyncrpc"
	lpclient "github.com/decred/dcrlnlpd/client"
)

// logUpdated is sent by the log backend when new log messages are generated.
type logUpdated struct {
	line string
}

// lndLogUpdated is sent when a new lnd log line was detected.
type lndLogUpdated string

// msgLNOpenChannel and Reply are used when the client lib requires the user
// to open a channel.
type msgLNOpenChannel struct{}
type msgLNOpenChannelReply struct {
}

// msgConfirmServerCert is sent when confirmation is needed from the user on
// whether to accept the new server certs.
type msgConfirmServerCert struct {
	cs        *tls.ConnectionState
	svrID     *zkidentity.PublicIdentity
	replyChan chan error
}

// msgLNRequestRecv and Reply are used when the client lib requires the user
// to input dcrlnlpd info.
type msgLNRequestRecv struct{}
type msgLNRequestRecvReply struct {
}

type msgLNFundWallet struct {
}
type msgLNFundWalletReply struct{}

// msgUnconfirmedFunds is sent when unconfirmed funds are detected for the
// wallet.
type msgUnconfirmedFunds dcrutil.Amount

// msgConfirmedFunds is sent when previously unconfirmed funds are now
// confirmed.
type msgConfirmedFunds dcrutil.Amount

type msgPaste string
type msgPasteErr error

type msgNewRecvdMsg struct{}

type msgCancelForm struct{}
type msgSubmitForm struct{}
type msgRetryAction struct{}
type msgSkipAction struct{}

type msgShowSharedFilesForLink struct{}

type msgProcessEsc struct{}

type msgDownloadCompleted clientdb.FileID

type msgActiveWindowChanged struct{}

type msgOnboardStateChanged struct{}
type msgStartOnboardErr error
type msgActiveCWRequestedPage struct{}
type msgUnwelcomeError struct {
	err error
}

func paste() tea.Msg {
	str, err := clipboard.ReadAll()
	if err != nil {
		return msgPasteErr(err)
	}
	return msgPaste(str)
}

// getClientID and reply are used when the client lib requires the user to
// input its public ID info.
type getClientID struct{}
type getClientIDReply struct {
	nick string
	name string
}

// repaintActiveChat is sent to the main window whenever the active chat window
// needs repaiting due to new messages sent/received, etc.
type repaintActiveChat struct{}

// requestShutdown requests a shutdown from the main window.
type requestShutdown struct{}

// currentTimeChanged is sent whenever the current time changes, which needs a
// UI update.
type currentTimeChanged struct{}

// showNewPostWindow shows the create post window.
type showNewPostWindow struct{}

// showFeedWindow shows the feed window.
type showFeedWindow struct{}

// feedUpdated when the feed of posts should be updated.
type feedUpdated struct {
	summ clientdb.PostSummary
}

// sentPostComment is sent when a new local comment to a post is sent.
type sentPostComment struct{}

// kxCompleted is sent when a KX process has completed with a remote peer.
type kxCompleted struct{ uid clientintf.UserID }

// crashApp is sent when we receive a signal to crash the app.
type crashApp struct{}

// kxSearchCompleted is sent when the kx search completes for the given user.
type kxSearchCompleted struct{ uid clientintf.UserID }

// lnReqRecvConfirmPayment is sent to confirm the payment of the amount
// necessary to request channel inbound volume.
type lnReqRecvConfirmPayment struct {
	chanSize        uint64
	policy          lpclient.ServerPolicy
	estimatedAmount uint64
	replyChan       chan bool
}

// lnReqRecvResult is a message sent with the result of a request receive
// liquidity call.
type lnReqRecvResult struct {
	err error
}

// lnOpenChanResult is a message sent with the result of opening a new channel.
type lnOpenChanResult struct {
	err error
}

type externalViewer struct {
	err error
}

type runDcrlndErrMsg struct{ error }

type createWalletResult struct {
	seed []byte
	err  error
}

type unlockDcrlndResult struct {
	err error
}

type lnChainSyncUpdate struct {
	update *initchainsyncrpc.ChainSyncUpdate
	err    error
}

type rmqLenChanged int

type msgPageFetched struct {
	uid  clientintf.UserID
	nick string
	req  *rpc.RMFetchResource
	res  *rpc.RMFetchResourceReply
}

var errQuitRequested = errors.New("")

func emitMsg(msg tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}

type msgRunCmd func() tea.Msg

type msgExternalCommentResult struct {
	err    error
	data   string
	parent *zkidentity.ShortID
}

// isQuitMsg returns true if the app should quit as a response to the given
// msg. It returns an error with the reason for quitting.
func isQuitMsg(msg tea.Msg) error {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		k := msg.String()
		if k == "ctrl+q" {
			return errQuitRequested
		}
	case requestShutdown:
		return errQuitRequested
	case appStateErr:
		return msg.err
	}
	return nil
}

// isCrashMsg returns true if the app should quit with a full goroutine stack
// trace as a response to que given msg.
func isCrashMsg(msg tea.Msg) bool {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+\\" {
			return true
		}
	case crashApp:
		return true
	}
	return false
}

// isEnterMsg returns true if the message is a key down message of the enter
// key.
func isEnterMsg(msg tea.Msg) bool {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return false
	}
	return keyMsg.String() == "enter"
}

// isEscMsg returns true if the message is a key down message of the ESC key.
func isEscMsg(msg tea.Msg) bool {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return false
	}
	return keyMsg.String() == "esc"
}

func markAllRead(cw *chatWindow) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(1500 * time.Millisecond)
		cw.markAllRead()
		return repaintActiveChat{}
	}
}
