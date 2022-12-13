package client

import (
	"context"
	"errors"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
)

// The DB Send Queue is used to manage outbound messages at the highest level
// of the client code. It stores messages, such that on client restart they
// will continue to be sent to their destinations.

// addToSendQ adds the given message to the DB send queue.
func (c *Client) addToSendQ(typ string, msg interface{}, priority uint,
	dests ...clientintf.UserID) (clientdb.SendQID, error) {

	var sendqID clientdb.SendQID

	// Trick to ease storing this msg payload: compose as a full blobified
	// RM.
	blob, err := rpc.ComposeCompressedRM(c.id, msg, c.cfg.CompressLevel)
	if err != nil {
		return sendqID, err
	}

	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		sendqID, err = c.db.AddToSendQueue(tx, typ, dests, blob, priority)
		return err
	})
	if err == nil {
		c.log.Tracef("Added %q with %d dests to sendq %s", typ,
			len(dests), sendqID)
	}
	return sendqID, err
}

// removeFromSendQ marks the message of the given sendq id as sent to the given
// destination. Errors are only logged since this is called when the message
// has already been sent and there's nothing more that can be done in case of
// errors.
func (c *Client) removeFromSendQ(id clientdb.SendQID, dest clientintf.UserID) {
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.RemoveFromSendQueue(tx, id, dest)
	})
	if err != nil {
		c.log.Errorf("Unable to remove dest %s from send queue %s: %v",
			dest, id)
	} else {
		c.log.Tracef("Removed dest %s from sendq %s", dest, id)
	}
}

// sendWithSendQPriority adds the given message to the sendq with the given type and
// sends it to the specified destinations. Each sending is done asynchronously,
// so this returns immediately after enqueing.
func (c *Client) sendWithSendQPriority(typ string, msg interface{}, priority uint,
	dests ...clientintf.UserID) error {

	sqid, err := c.addToSendQ(typ, msg, priority, dests...)
	if err != nil {
		return err
	}

	// Send the msg to each destination.
	for _, dest := range dests {
		uid := dest
		ru, err := c.UserByID(uid)
		if err != nil {
			c.log.Warnf("Unable to find user for sendq entry %s: %v", typ, err)
			continue
		}

		// Helper called when we get an error.
		failed := func(err error) {
			if !errors.Is(err, clientintf.ErrSubsysExiting) {
				ru.log.Errorf("unable to queue  %T: %v",
					msg, err)
				c.removeFromSendQ(sqid, uid)
			}
		}

		// Queue synchronously to ensure outbound order.
		replyChan := make(chan error)
		err = ru.queueRMPriority(msg, priority, replyChan, typ)
		if err != nil {
			failed(err)
			continue
		}

		// Wait for reply asynchronously.
		go func() {
			err := <-replyChan
			if err != nil {
				failed(err)
			} else {
				c.removeFromSendQ(sqid, uid)
			}
		}()
	}

	return nil
}

// sendWithSendQ sends a msg using the send queue with default priority.
func (c *Client) sendWithSendQ(typ string, msg interface{}, dests ...clientintf.UserID) error {
	return c.sendWithSendQPriority(typ, msg, priorityDefault, dests...)
}

// runSendQ sends outstanding msgs from the DB send queue.
func (c *Client) runSendQ(ctx context.Context) error {
	<-c.abLoaded
	var sendq []clientdb.SendQueueElement
	err := c.db.View(c.dbCtx, func(tx clientdb.ReadTx) error {
		var err error
		sendq, err = c.db.ListSendQueue(tx)
		return err
	})
	if err != nil {
		return err
	}

	// Flatten the sendq into a single list.
	type sendEL struct {
		tries int
		msg   *clientdb.SendQueueElement
		uid   *clientintf.UserID
	}
	sendlist := make([]sendEL, 0)
	for i := range sendq {
		for d := range sendq[i].Dests {
			sendlist = append(sendlist, sendEL{
				msg: &sendq[i],
				uid: &sendq[i].Dests[d],
			})
		}
	}

	if len(sendlist) == 0 {
		c.log.Debugf("No queued messages to send")
		return nil
	}

	var i int
	removeCurrent := func() {
		c.removeFromSendQ(sendlist[i].msg.ID, *sendlist[i].uid)
		copy(sendlist[i:], sendlist[i+1:])
		sendlist = sendlist[:len(sendlist)-1]
	}

	const maxTries = 5 // Max attempts at sending same msg.

	c.log.Infof("Starting to send %d queued messages", len(sendlist))

	for len(sendlist) > 0 {
		if canceled(ctx) {
			return ctx.Err()
		}

		// Attempt to send the next message.
		el := sendlist[i]
		ru, err := c.UserByID(*el.uid)
		if err != nil {
			// User not found, drop it from queue.
			c.log.Warnf("Removing queued msg %s due to unknown user %s",
				el.msg.Type, el.uid)
			removeCurrent()
			continue
		}

		_, rm, err := rpc.DecomposeRM(&c.id.Public, el.msg.Msg)
		if err != nil {
			c.log.Warnf("Unable to decompose queued RM %s: %v",
				el.msg.Type, err)
			removeCurrent()
			continue
		}
		err = ru.sendRMPriority(rm, el.msg.Type, el.msg.Priority)
		if err != nil {
			// Failed to send this. Try next one so we're not stuck.
			ru.log.Errorf("Unable to send RM from sendq: %v", err)
			sendlist[i].tries += 1
			if sendlist[i].tries >= maxTries {
				removeCurrent()
			} else {
				i = (i + 1) % len(sendlist)
			}

			select {
			case <-time.After(time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
		} else {
			// Successfully sent this one.
			ru.log.Debugf("Sent sendq element %s. %d remaining",
				sendq[i].Type, len(sendlist)-1)
			removeCurrent()
		}
	}

	c.log.Infof("Finished sending queued messages")
	return nil
}
