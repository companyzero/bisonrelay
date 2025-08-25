package client

import (
	"errors"
	"fmt"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/sw"
	"github.com/companyzero/sntrup4591761"
	"github.com/davecgh/go-spew/spew"
	"github.com/decred/slog"
)

// clearLastHandshakeAttemptTime clears the last handshake attempt time for
// the given user.
func (c *Client) clearLastHandshakeAttemptTime(ru *RemoteUser, completed bool, completedTs time.Time) error {
	return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		entry, err := c.db.GetAddressBookEntry(tx, ru.id)
		if err != nil {
			return err
		}

		entry.LastHandshakeAttempt = time.Time{}
		if err := c.db.UpdateAddressBookEntry(tx, entry); err != nil {
			return err
		}

		if completed {
			_, err := c.db.LogPM(tx, ru.id, true, ru.Nick(),
				"Completed handshake", completedTs)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// handleRMHandshake handles all handshake messages.
func (c *Client) handleRMHandshake(ru *RemoteUser, msg interface{}, ts time.Time) error {
	switch msg.(type) {
	case rpc.RMHandshakeSYN:
		ru.log.Infof("Received handshake SYN. Replying with SYN/ACK.")
		c.ntfns.notifyHandshakeStage(ru, "SYN")
		return c.sendWithSendQ("synack", rpc.RMHandshakeSYNACK{}, ru.ID())

	case rpc.RMHandshakeSYNACK:
		ru.log.Infof("Received handshake SYN/ACK. Replying with ACK. User ratchet is fully synced.")
		if err := c.clearLastHandshakeAttemptTime(ru, true, ts); err != nil {
			return err
		}
		c.ntfns.notifyHandshakeStage(ru, "SYNACK")
		return c.sendWithSendQ("synack", rpc.RMHandshakeACK{}, ru.ID())

	case rpc.RMHandshakeACK:
		ru.log.Infof("Received handshake ACK. User ratchet is fully synced.")
		if err := c.clearLastHandshakeAttemptTime(ru, true, ts); err != nil {
			return err
		}
		c.ntfns.notifyHandshakeStage(ru, "ACK")
		return nil

	default:
		return fmt.Errorf("wrong msg payload sent to handleRMHandshake: %T", msg)
	}
}

func (c *Client) handleTransitiveMsg(ru *RemoteUser, tm rpc.RMTransitiveMessage) error {
	c.ntfns.notifyTransitiveEvent(ru.ID(), tm.For, TEMsgForward)

	target, err := c.rul.byID(tm.For)
	if err != nil {
		ru.log.Warnf("Received message to forward to unknown target %s",
			tm.For)
		return nil
	}

	ru.log.Infof("Forwarding msg to %s", target)
	fwd := rpc.RMTransitiveMessageForward{
		From:       ru.ID(),
		CipherText: tm.CipherText,
		Message:    tm.Message,
	}
	go func() {
		payEvent := fmt.Sprintf("fwdtransitive.%s", ru.ID())
		err := target.sendRM(fwd, payEvent)
		if err != nil && !errors.Is(err, clientintf.ErrSubsysExiting) {
			ru.log.Errorf("Unable to send transitive msg reply: %v", err)
		}
	}()
	return nil
}

func (c *Client) handleTransitiveMsgFwd(ru *RemoteUser, fwd rpc.RMTransitiveMessageForward) error {
	ct := (*sntrup4591761.Ciphertext)(&fwd.CipherText)
	pk := (*sntrup4591761.PrivateKey)(&c.localID.privKey)
	sk, n := sntrup4591761.Decapsulate(ct, pk)
	if n != 1 {
		return fmt.Errorf("could not decapsulate shared key from transitive msg fwd")
	}

	// Decrypt transitive command
	cleartext, ok := sw.Open(fwd.Message, sk)
	if !ok {
		return fmt.Errorf("could not open transitive command")
	}

	// Get header and command
	from, err := c.rul.byID(fwd.From)
	var msgVerifier rpc.MessageVerifier
	if err != nil {
		if !errors.Is(err, userNotFoundError{}) {
			return fmt.Errorf("error finding target user %s of transitive "+
				"forward: %v", fwd.From, err)
		}

		// This should only happen during KX.
	} else {
		msgVerifier = from.verifyMessage
	}
	h, cmd, err := rpc.DecomposeRM(msgVerifier, cleartext, uint(c.q.MaxMsgSize()))
	if err != nil {
		return fmt.Errorf("handleRMTransitiveMessageForward: "+
			"decompose %v", err)
	}
	if h.Version != rpc.RMHeaderVersion {
		return fmt.Errorf("handleRMTransitiveMessageForward: "+
			"header version %v, want %v", h.Version,
			rpc.RMHeaderVersion)
	}

	// Assume DecomposeRM generated the correct type for a given command.
	switch p := cmd.(type) {
	case rpc.OOBPublicIdentityInvite:
		return c.handleTransitiveIDInvite(ru, p)

	case rpc.RMTransitiveReset:
		return c.handleRMTransitiveReset(ru, fwd.From, p)

	case rpc.RMTransitiveResetReply:
		return c.handleRMTransitiveResetReply(ru, fwd.From, p)

	default:
		return fmt.Errorf("unknown command payload type %T for "+
			"transitive fwd cmd %q", cmd, h.Command)
	}
}

// handlePrivateMsg handles PMs.
//
// NOTE: this is called on the RV manager goroutine, so it should not block
// for long periods of time.
func (c *Client) handlePrivateMsg(ru *RemoteUser, p rpc.RMPrivateMessage, ts time.Time) error {
	if ru.IsIgnored() {
		ru.log.Tracef("Ignoring received PM")
		return nil
	}

	if filter, _ := c.FilterPM(ru.ID(), p.Message); filter {
		return nil
	}

	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		p.Message, err = c.db.LogPM(tx, ru.ID(), false, ru.Nick(), p.Message, ts)
		return err
	})
	if err != nil {
		return err
	}
	ru.log.Debugf("Received private message of length %d", len(p.Message))

	c.ntfns.notifyOnPM(ru, p, ts)
	return nil
}

func (c *Client) innerHandleUserRM(ru *RemoteUser, h *rpc.RMHeader,
	p interface{}, ts time.Time) error {

	// DecomposeRM should have created the correct payload type based on
	// the command, so switch on the payload type directly.
	switch p := p.(type) {
	case rpc.RMHandshakeSYN, rpc.RMHandshakeACK, rpc.RMHandshakeSYNACK:
		return c.handleRMHandshake(ru, p, ts)

	case rpc.RMProfileUpdate:
		return c.handleProfileUpdate(ru, p)

	case rpc.RMGroupInvite:
		return c.handleGCInvite(ru, p)

	case rpc.RMGroupJoin:
		return c.handleGCJoin(ru, p)

	case rpc.RMGroupList:
		return c.handleGCList(ru, p, ts)

	case rpc.RMGroupUpgradeVersion:
		return c.handleGCUpgradeVersion(ru, p, ts)

	case rpc.RMGroupUpdateAdmins:
		return c.handleGCUpdateAdmins(ru, p, ts)

	case rpc.RMMediateIdentity:
		return c.handleMediateID(ru, p)

	case rpc.RMBlock:
		return c.handleRMBlock(ru, p)

	case rpc.RMInvite:
		return c.handleRMInvite(ru, p)

	case rpc.RMFTList:
		return c.handleFTList(ru, p)

	case rpc.RMFTListReply:
		return c.handleFTListReply(ru, p)

	case rpc.RMFTGet:
		return c.handleFTGet(ru, p)

	case rpc.RMFTGetReply:
		return c.handleFTGetReply(ru, p)

	case rpc.RMFTGetChunk:
		return c.handleFTGetChunk(ru, p)

	case rpc.RMFTPayForChunk:
		return c.handleFTPayForChunk(ru, p)

	case rpc.RMTransitiveMessage:
		return c.handleTransitiveMsg(ru, p)

	case rpc.RMTransitiveMessageForward:
		return c.handleTransitiveMsgFwd(ru, p)

	case rpc.RMGetInvoice:
		return c.handleGetInvoice(ru, p)

	case rpc.RMInvoice:
		return c.handleInvoice(ru, p)

	case rpc.RMListPosts:
		return c.handleListPosts(ru, p)

	case rpc.RMListPostsReply:
		return c.handleListPostsReply(ru, p)

	case rpc.RMGetPost:
		return c.handleGetPost(ru, p)

	case rpc.RMPostsSubscribe:
		return c.handlePostsSubscribe(ru, p)

	case rpc.RMPostsSubscribeReply:
		return c.handlePostsSubscribeReply(ru, p)

	case rpc.RMPostsUnsubscribe:
		return c.handlePostsUnsubscribe(ru, p)

	case rpc.RMPostsUnsubscribeReply:
		return c.handlePostsUnsubscribeReply(ru, p)

	case rpc.RMPostShare:
		return c.handlePostShare(ru, p)

	case rpc.RMPostStatus:
		return c.handlePostStatus(ru, p)

	case rpc.RMPostStatusReply:
		return c.handlePostStatusReply(ru, p)

	case rpc.RMReceiveReceipt:
		return c.handleReceiveReceipt(ru, p, ts)

	case rpc.RMGroupKick:
		return c.handleGCKick(ru, p, ts)

	case rpc.RMGroupPart:
		return c.handleGCPart(ru, p, ts)

	case rpc.RMGroupKill:
		return c.handleGCKill(ru, p, ts)

	case rpc.RMKXSearch:
		return c.handleKXSearch(ru, p)

	case rpc.RMKXSearchReply:
		return c.handleKXSearchReply(ru, p)

	case rpc.RMKXSuggestion:
		return c.handleKXSuggestion(ru, p, ts)

	case rpc.RMFetchResource:
		return c.handleFetchResource(ru, p)

	case rpc.RMFetchResourceReply:
		return c.handleFetchResourceReply(ru, p)

	case rpc.RMRTDTSessionInvite:
		return c.handleRMRTDTSessionInvite(ru, p, ts)

	case rpc.RMRTDTSessionInviteAccept:
		return c.handleRMRTDTAcceptInvite(ru, p)

	case rpc.RMRTDTSessionInviteCancel:
		return c.handleRMRTDTCancelInvite(ru, p)

	case rpc.RMRTDTSession:
		return c.handleRTDTSessionUpdate(ru, p)

	case rpc.RMRTDTExitSession:
		return c.handleRMRTDTExitSession(ru, p)

	case rpc.RMRTDTDissolveSession:
		return c.handleRMRTDTDissolveSession(ru, p)

	case rpc.RMRTDTRemovedFromSession:
		return c.handleRMRTDTRemovedFromSession(ru, p)

	case rpc.RMRTDTRotateAppointCookie:
		return c.handleRMRTDTRotateAppointCookie(ru, p)

	case rpc.RMRTDTAdminCookies:
		return c.handleRMRTDTAdminCookies(ru, p)

	default:
		return fmt.Errorf("received unknown command %q payload %T",
			h.Command, p)
	}
}

// logHandlerError logs the error when handling the passed msg.
func (c *Client) logHandlerError(ru *RemoteUser, cmd string, p interface{}, err error) {
	if err == nil {
		ru.log.Tracef("Finished handling %s (%T)", cmd, p)
		return
	}

	if ru.log.Level() <= slog.LevelDebug {
		ru.log.Errorf("Error handling %q (payload %T) from user: %v; %s",
			cmd, p, err, spew.Sdump(p))
	} else {
		ru.log.Errorf("Error handling %q (payload %T) from user: %v",
			cmd, p, err)
	}

}

// handleUserRM is the main handler for remote user RoutedMessages. It decides
// what to do with the given RM from the given user.
func (c *Client) handleUserRM(ru *RemoteUser, h *rpc.RMHeader, p interface{}, ts time.Time) <-chan struct{} {
	ru.log.Tracef("Starting to handle %T", p)
	c.ntfns.notifyRMReceived(ru, h, p, ts)
	c.gcmq.RMReceived(ru.ID(), ts)

	// At this point in the execution stack, this is still in the main RV
	// manager goroutine. Some messages are deemed safe to be handled on
	// this goroutine because they do not generate replies, their
	// notification handlers are async, they otherwise do not consume
	// significant CPU resources and they benefit from sequential
	// processing (due to their reordering potentially affecting end-user
	// interactions).
	//
	// These messages are handled directly below, while others spawn a
	// separate goroutine.
	switch p := p.(type) {
	case rpc.RMPrivateMessage:
		err := c.handlePrivateMsg(ru, p, ts)
		c.logHandlerError(ru, h.Command, p, err)
		return nil

	case rpc.RMGroupMessage:
		err := c.handleGCMessage(ru, p, ts)
		c.logHandlerError(ru, h.Command, p, err)
		return nil

	case rpc.RMFTSendFile:
		err := c.handleFTSendFile(ru, p)
		c.logHandlerError(ru, h.Command, p, err)
		return nil

	case rpc.RMFTGetChunkReply:
		err := c.handleFTGetChunkReply(ru, p, ts)
		c.logHandlerError(ru, h.Command, p, err)
		return nil

	default:
		handlerDone := make(chan struct{})
		go func() {
			err := c.innerHandleUserRM(ru, h, p, ts)
			c.logHandlerError(ru, h.Command, p, err)
			close(handlerDone)
		}()
		return handlerDone
	}

}
