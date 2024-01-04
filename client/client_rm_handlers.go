package client

import (
	"errors"
	"fmt"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/sw"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/companyzero/sntrup4591761"
	"github.com/davecgh/go-spew/spew"
	"github.com/decred/slog"
)

// handleRMHandshake handles all handshake messages.
func (c *Client) handleRMHandshake(ru *RemoteUser, msg interface{}) error {
	switch msg.(type) {
	case rpc.RMHandshakeSYN:
		ru.log.Infof("Received handshake SYN. Replying with SYN/ACK.")
		c.ntfns.notifyHandshakeStage(ru, "SYN")
		return c.sendWithSendQ("synack", rpc.RMHandshakeSYNACK{}, ru.ID())

	case rpc.RMHandshakeSYNACK:
		ru.log.Infof("Received handshake SYN/ACK. Replying with ACK. User ratchet is fully synced.")
		c.ntfns.notifyHandshakeStage(ru, "SYNACK")
		return c.sendWithSendQ("synack", rpc.RMHandshakeACK{}, ru.ID())

	case rpc.RMHandshakeACK:
		ru.log.Infof("Received handshake ACK. User ratchet is fully synced.")
		c.ntfns.notifyHandshakeStage(ru, "ACK")
		return nil

	default:
		return fmt.Errorf("wrong msg payload sent to handleRMHandshake: %T", msg)
	}
}

func (c *Client) handleTransitiveMsg(ru *RemoteUser, tm rpc.RMTransitiveMessage) error {
	if c.cfg.TransitiveEvent != nil {
		c.cfg.TransitiveEvent(ru.ID(), tm.For, TEMsgForward)
	}

	target, err := c.rul.byID(tm.For)
	if err != nil {
		ru.log.Warnf("Received message to forward to unknown target %s",
			tm.For)
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
	pk := (*sntrup4591761.PrivateKey)(&c.id.PrivateKey)
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
	var fromID *zkidentity.PublicIdentity
	if err != nil {
		if !errors.Is(err, userNotFoundError{}) {
			return fmt.Errorf("error finding target user %s of transitive "+
				"forward: %v", fwd.From, err)
		}

		// this should only happen during KX
	} else {
		id := from.PublicIdentity()
		fromID = &id
	}
	h, cmd, err := rpc.DecomposeRM(fromID, cleartext)
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

func (c *Client) innerHandleUserRM(ru *RemoteUser, h *rpc.RMHeader,
	p interface{}, ts time.Time) error {

	// DecomposeRM should have created the correct payload type based on
	// the command, so switch on the payload type directly.
	switch p := p.(type) {
	case rpc.RMPrivateMessage:
		if ru.IsIgnored() {
			ru.log.Tracef("Ignoring received PM")
			return nil
		}

		if filter, _ := c.FilterPM(ru.ID(), p.Message); filter {
			return nil
		}

		err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
			return c.db.LogPM(tx, ru.ID(), false, ru.Nick(), p.Message, ts)
		})
		if err != nil {
			return err
		}
		ru.log.Debugf("Received private message of length %d", len(p.Message))

		c.ntfns.notifyOnPM(ru, p, ts)

	case rpc.RMHandshakeSYN, rpc.RMHandshakeACK, rpc.RMHandshakeSYNACK:
		return c.handleRMHandshake(ru, p)

	case rpc.RMGroupInvite:
		return c.handleGCInvite(ru, p)

	case rpc.RMGroupJoin:
		return c.handleGCJoin(ru, p)

	case rpc.RMGroupList:
		return c.handleGCList(ru, p)

	case rpc.RMGroupUpgradeVersion:
		return c.handleGCUpgradeVersion(ru, p)

	case rpc.RMGroupUpdateAdmins:
		return c.handleGCUpdateAdmins(ru, p)

	case rpc.RMGroupMessage:
		if ru.IsIgnored() {
			ru.log.Tracef("Ignoring received GC message")
			return nil
		}
		return c.handleGCMessage(ru, p, ts)

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

	case rpc.RMFTGetChunkReply:
		return c.handleFTGetChunkReply(ru, p)

	case rpc.RMFTSendFile:
		return c.handleFTSendFile(ru, p)

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
		return c.handleGCKick(ru, p)

	case rpc.RMGroupPart:
		return c.handleGCPart(ru, p)

	case rpc.RMGroupKill:
		return c.handleGCKill(ru, p)

	case rpc.RMKXSearch:
		return c.handleKXSearch(ru, p)

	case rpc.RMKXSearchReply:
		return c.handleKXSearchReply(ru, p)

	case rpc.RMKXSuggestion:
		return c.handleKXSuggestion(ru, p)

	case rpc.RMFetchResource:
		return c.handleFetchResource(ru, p)

	case rpc.RMFetchResourceReply:
		return c.handleFetchResourceReply(ru, p)

	default:
		return fmt.Errorf("Received unknown command %q payload %T",
			h.Command, p)
	}

	return nil
}

// handleUserRM is the main handler for remote user RoutedMessages. It decides
// what to do with the given RM from the given user.
func (c *Client) handleUserRM(ru *RemoteUser, h *rpc.RMHeader, p interface{}, ts time.Time) {
	ru.log.Tracef("Starting to handle %T", p)
	c.gcmq.RMReceived(ru.ID(), ts)
	err := c.innerHandleUserRM(ru, h, p, ts)
	if err != nil {
		if ru.log.Level() <= slog.LevelDebug {
			ru.log.Errorf("Error handling %q (payload %T) from user: %v; %s",
				h.Command, p, err, spew.Sdump(p))
		} else {
			ru.log.Errorf("Error handling %q (payload %T) from user: %v",
				h.Command, p, err)
		}
	} else {
		ru.log.Tracef("Finished handling %T", p)
	}
}
