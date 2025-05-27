package client

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
)

// The DB Send Queue is used to manage outbound messages at the highest level
// of the client code. It stores messages, such that on client restart they
// will continue to be sent to their destinations.

// addToSendQ adds the given message to the DB send queue.
//
// This does NOT include the message in the outbound RMQ, it only adds the
// message to the db.
func (c *Client) addToSendQ(typ string, rmOrFileChunk interface{}, priority uint,
	dests ...clientintf.UserID) (clientdb.SendQID, error) {

	var sendqID clientdb.SendQID
	var blob []byte
	var fileChunk *clientdb.SendQueueFileChunk
	var estSize int

	// Filter out the local user ID from targets, just in case.
	myID := c.PublicID()
	dests = slices.DeleteFunc(dests, func(id clientintf.UserID) bool { return id == myID })

	// Determine the type of sendq item.
	if fc, ok := rmOrFileChunk.(*clientdb.SendQueueFileChunk); ok {
		fileChunk = fc
		estSize = rpc.EstimateRoutedRMWireSize(int(fc.Size))
	} else {
		// Trick to ease storing this msg payload: compose as a full blobified
		// RM.
		var err error
		blob, err = rpc.ComposeCompressedRM(c.localID.signMessage,
			rmOrFileChunk, c.cfg.CompressLevel)
		if err != nil {
			return sendqID, err
		}

		estSize = rpc.EstimateRoutedRMWireSize(len(blob))
	}

	maxMsgSize := int(c.q.MaxMsgSize())
	if estSize > maxMsgSize {
		return sendqID, fmt.Errorf("cannot enqueue message %T "+
			"estimated as larger than max message size %d > %d: %w",
			rmOrFileChunk, estSize, maxMsgSize, errRMTooLarge)

	}

	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		sendqID, err = c.db.AddToSendQueue(tx, typ, dests, blob,
			fileChunk, priority)
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
			dest, id, err)
	} else {
		c.log.Tracef("Removed dest %s from sendq %s", dest, id)
	}
}

// preparedSendqItem is a sendq entry that has been saved to the DB and is ready
// to be enqueued.
type preparedSendqItem struct {
	id            clientdb.SendQID
	typ           string
	rmOrFileChunk interface{}
	priority      uint
	progressChan  chan SendProgress
	dests         []clientintf.UserID
}

// rm returns the actual RM for this prepared item.
func (prep *preparedSendqItem) rm() (interface{}, error) {
	var err error
	var rm interface{} = prep.rmOrFileChunk
	if fc, ok := prep.rmOrFileChunk.(*clientdb.SendQueueFileChunk); ok {
		rm, err = fc.RM()
	}
	return rm, err
}

// prepareSendqItem prepares a sendq item to be sent, by saving it in the DB.
func (c *Client) prepareSendqItem(typ string, rmOrFileChunk interface{}, priority uint,
	progressChan chan SendProgress, dests ...clientintf.UserID) (*preparedSendqItem, error) {

	id, err := c.addToSendQ(typ, rmOrFileChunk, priority, dests...)
	if err != nil {
		return nil, err
	}

	return &preparedSendqItem{
		id:            id,
		typ:           typ,
		rmOrFileChunk: rmOrFileChunk,
		priority:      priority,
		progressChan:  progressChan,
		dests:         dests,
	}, nil
}

// sendPreparedSendqItem sends a previously prepared sendq item.
func (c *Client) sendPreparedSendqItem(prep *preparedSendqItem) error {
	var sent, total atomic.Int64
	total.Store(int64(len(prep.dests)))

	rm, err := prep.rm()
	if err != nil {
		return err
	}

	// Send the msg to each destination.
	for _, dest := range prep.dests {
		uid := dest
		ru, err := c.UserByID(uid)
		if err != nil {
			c.log.Warnf("Unable to find user for sendq entry %s: %v",
				prep.typ, err)
			c.removeFromSendQ(prep.id, uid)
			continue
		}

		// Helper called when we get an error.
		failed := func(err error) {
			if !errors.Is(err, clientintf.ErrSubsysExiting) {
				ru.log.Errorf("unable to queue  %T: %v",
					prep.rmOrFileChunk, err)
				c.removeFromSendQ(prep.id, uid)
			} else {
				ru.log.Tracef("Unable to queue %T due to %v",
					prep.rmOrFileChunk, err)
			}
		}

		// Queue synchronously to ensure outbound order.
		replyChan := make(chan error)
		err = ru.queueRMPriority(rm, prep.priority, replyChan, prep.typ,
			&prep.id)
		if err != nil {
			failed(err)
			total.Add(-1)
			continue
		}

		// Wait for reply asynchronously.
		go func() {
			// On success sending, the RemoteUser instance removes
			// from the sendq.
			err := <-replyChan
			if err != nil {
				failed(err)
			}

			// Alert about progress.
			if !errors.Is(err, clientintf.ErrSubsysExiting) && (prep.progressChan != nil) {
				prep.progressChan <- SendProgress{
					Sent:  int(sent.Add(1)),
					Total: int(total.Load()),
					Err:   err,
				}
			}
		}()
	}

	return nil
}

// sendWithSendQPriority adds the given message to the sendq with the given
// type and sends it to the specified destinations. Each sending is done
// asynchronously, so this returns immediately after enqueing.
//
// If progressChan is specified, updates about the sending progress are sent
// there.
func (c *Client) sendWithSendQPriority(typ string, rmOrFileChunk interface{}, priority uint,
	progressChan chan SendProgress, dests ...clientintf.UserID) error {

	prep, err := c.prepareSendqItem(typ, rmOrFileChunk, priority,
		progressChan, dests...)
	if err != nil {
		return err
	}

	return c.sendPreparedSendqItem(prep)
}

// sendWithSendQ sends a msg using the send queue with default priority.
func (c *Client) sendWithSendQ(typ string, rmOrFileChunk interface{}, dests ...clientintf.UserID) error {
	return c.sendWithSendQPriority(typ, rmOrFileChunk, priorityDefault, nil, dests...)
}

// sendPreparedSendqItemSync sends a previously prepared sendq item in a sync
// fashion (waiting until the server acks the RM).
func (c *Client) sendPreparedSendqItemSync(prep *preparedSendqItem) error {
	rm, err := prep.rm()
	if err != nil {
		return err
	}

	itemSent, itemTotal := 0, len(prep.dests)

	// Send the msg to each destination.
	for _, dest := range prep.dests {
		uid := dest
		ru, err := c.UserByID(uid)
		if err != nil {
			c.log.Warnf("Unable to find user for sendq entry %s: %v",
				prep.typ, err)
			c.removeFromSendQ(prep.id, uid)
			itemSent++
			continue
		}

		// Queue outbound.
		replyChan := make(chan error)
		err = ru.queueRMPriority(rm, prep.priority, replyChan, prep.typ, &prep.id)
		if errors.Is(err, clientintf.ErrSubsysExiting) {
			// Item will be sent on restart.
			return err
		} else if err != nil {
			ru.log.Errorf("unable to queue  %T: %v",
				prep.rmOrFileChunk, err)
			c.removeFromSendQ(prep.id, uid)
		}

		// Wait for server ack. On success, the RM is automatically
		// removed from the sendq.
		err = <-replyChan
		if errors.Is(err, clientintf.ErrSubsysExiting) {
			return err
		}

		if prep.progressChan != nil {
			itemSent++
			prep.progressChan <- SendProgress{
				Sent:  itemSent,
				Total: itemTotal,
				Err:   err,
			}
		}
	}

	return nil
}

// sendWithSendQPrioritySync prepares and sends an RM using the sendq in a
// sync fashion (waiting until brserver acks the RM).
func (c *Client) sendWithSendQPrioritySync(typ string, rmOrFileChunk interface{}, priority uint,
	progressChan chan SendProgress, dests ...clientintf.UserID) error {

	prep, err := c.prepareSendqItem(typ, rmOrFileChunk, priority,
		progressChan, dests...)
	if err != nil {
		return err
	}

	return c.sendPreparedSendqItemSync(prep)
}

// sendPreparedSendqItemListSync sends a list of prepared sendq items. Sending
// is done in a synchronous way (one item is only sent after the previous one
// is ack'd) and this returns only after all items were sent and ack'd by the
// server.
//
// This is usually used in situations where many prepared items are sent to one
// destination each.
func (c *Client) sendPreparedSendqItemListSync(items []*preparedSendqItem, progressChan chan SendProgress) error {

	sent, total := 0, len(items)

	for _, prep := range items {
		err := c.sendPreparedSendqItemSync(prep)
		if err != nil {
			return err
		}

		if progressChan != nil {
			sent++
			prep.progressChan <- SendProgress{
				Sent:  sent,
				Total: total,
			}
		}
	}
	return nil
}

// SendQueueLen returns the number of items in the sendqueue (total items and
// total number of targets).
func (c *Client) SendQueueLen() (items, dests int) {
	c.dbView(func(tx clientdb.ReadTx) error {
		items, dests = c.db.SendQueueLen(tx)
		return nil
	})
	return
}

// runSendQ sends outstanding msgs from the DB send queue.
func (c *Client) runSendQ(ctx context.Context) error {
	<-c.abLoaded
	sendq := c.startupSendq
	c.startupSendq = nil

	// Local helper type for one sendq element to be sent to one user.
	type sendEL struct {
		tries int
		qel   *clientdb.SendQueueElement
		uid   *clientintf.UserID
		rm    interface{}
	}
	prepSendElRM := func(sel *sendEL) error {
		var err error
		if sel.rm != nil {
			return nil // Already prepared.
		}
		if sel.qel.FileChunk != nil {
			sel.rm, err = sel.qel.FileChunk.RM()
			return err
		} else {
			_, sel.rm, err = rpc.DecomposeRM(c.localID.verifyMessage,
				sel.qel.Msg, uint(c.q.MaxMsgSize()))
			if err != nil {
				return fmt.Errorf("unable to decompose queued RM %s: %v",
					sel.qel.Type, err)
			}
		}
		return nil
	}

	// Flatten the sendq into a single list.
	sendlist := make([]sendEL, 0)
	for i := range sendq {
		for d := range sendq[i].Dests {
			sendlist = append(sendlist, sendEL{
				qel: &sendq[i],
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
		c.removeFromSendQ(sendlist[i].qel.ID, *sendlist[i].uid)
		sendlist = slices.Delete(sendlist, i, i+1)
		if i >= len(sendlist) {
			i = 0
		}
	}

	const maxTries = 5 // Max attempts at sending same msg.

	c.log.Infof("Starting to send %d queued messages", len(sendlist))

	for len(sendlist) > 0 {
		if canceled(ctx) {
			return ctx.Err()
		}

		// Attempt to send the next message.
		el := &sendlist[i]
		ru, err := c.UserByID(*el.uid)
		if err != nil {
			// User not found, drop it from queue.
			c.log.Warnf("Removing queued msg %s due to unknown user %s",
				el.qel.Type, el.uid)
			removeCurrent()
			continue
		}

		err = prepSendElRM(el)
		if err != nil {
			// Unable to prepare outbound RM. Remove and go to next
			// one.
			c.log.Warnf("Removing queued msg %s due to failure to "+
				"prepare RM: %v", el.qel.Type, err)
			removeCurrent()
			continue
		}

		err = ru.sendRMPriority(el.rm, el.qel.Type, el.qel.Priority, &el.qel.ID)
		if err != nil {
			// Failed to send this. Try next one so we're not stuck.
			ru.log.Errorf("Unable to send RM from sendq: %v", err)
			el.tries += 1
			if el.tries >= maxTries {
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
