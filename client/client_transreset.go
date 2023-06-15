package client

import (
	"crypto/rand"
	"fmt"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/ratchet"
	"github.com/companyzero/bisonrelay/rpc"
)

// Client transitive reset flow is:
//
//          Alice                                    Bob
//         -------                                  -----
//
//     RequestTransitiveReset()
//             \------- RMTransitiveReset -->
//                     (via transitive msg)
//
//                                           handleRMTransitiveReset()
//                            <-- RMTransitiveResetReply ----/
//                                (via transitive msg)
//
//     handleTransitiveResetReply()
//

func (c *Client) RequestTransitiveReset(mediator, target UserID) error {
	// Ensure we know both the mediator and target.
	mediatorRU, err := c.rul.byID(mediator)
	if err != nil {
		return fmt.Errorf("mediator of trans reset: %w", err)
	}
	targetRU, err := c.rul.byID(target)
	if err != nil {
		return fmt.Errorf("target of trans reset: %w", err)
	}

	// Create new ratchet with remote identity
	r := ratchet.New(rand.Reader) // half
	r.MyPrivateKey = &c.id.PrivateKey
	r.TheirPublicKey = &targetRU.id.Key

	// Fill out half the kx
	kxA := new(ratchet.KeyExchange)
	err = r.FillKeyExchange(kxA)
	if err != nil {
		return fmt.Errorf("newHalfRatchet FillKeyExchange: %v",
			err)
	}

	// Store half-kx in DB.
	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.StoreTransResetHalfKX(tx, r, targetRU.id.Identity)
	})
	if err != nil {
		return err
	}

	c.log.Infof("Requesting transitive reset with %s via %s", targetRU, mediatorRU)

	// Ask mediator to forward msg to target.
	tr := rpc.RMTransitiveReset{HalfKX: *kxA}
	return mediatorRU.sendTransitive(tr, "transreset", *targetRU.id, priorityDefault)
}

func (c *Client) handleRMTransitiveReset(mediator *RemoteUser, target UserID, tr rpc.RMTransitiveReset) error {
	if c.cfg.TransitiveEvent != nil {
		c.cfg.TransitiveEvent(mediator.ID(), target, TEResetRequest)
	}

	ru, err := c.rul.byID(target)
	if err != nil {
		return fmt.Errorf("transitive reset for previously unknown peer: %w", err)
	}

	mediator.log.Infof("Received trans reset request from %s", ru)

	// Fill out missing bits
	r := ratchet.New(rand.Reader) // full
	r.MyPrivateKey = &c.id.PrivateKey
	r.TheirPublicKey = &ru.id.Key
	kxB := new(ratchet.KeyExchange)
	err = r.FillKeyExchange(kxB)
	if err != nil {
		return fmt.Errorf("newFullKX could not fill key "+
			"exchange: %v", err)
	}

	// Complete ratchet
	err = r.CompleteKeyExchange(&tr.HalfKX, false)
	if err != nil {
		return fmt.Errorf("newFullKX could not complete key "+
			"exchange: %v", err)
	}

	// Replace user ratchet (updates DB).
	ru.replaceRatchet(r)

	// Send reply to originator using mediator.
	trr := rpc.RMTransitiveResetReply{FullKX: *kxB}
	if err := mediator.sendTransitive(trr, "transresetreply", *ru.id, priorityDefault); err != nil {
		return err
	}

	// Send UI event.
	c.ntfns.notifyOnKXCompleted(nil, ru, false)
	return nil
}

func (c *Client) handleRMTransitiveResetReply(mediator *RemoteUser,
	target UserID, trr rpc.RMTransitiveResetReply) error {

	if c.cfg.TransitiveEvent != nil {
		c.cfg.TransitiveEvent(mediator.ID(), target, TEResetReply)
	}

	ru, err := c.rul.byID(target)
	if err != nil {
		return fmt.Errorf("transitive reset reply for previously unknown peer: %w", err)
	}

	// Fetch and delete the transitive reset half kx.
	var r *ratchet.Ratchet
	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		var err error
		r, err = c.db.LoadTransResetHalfKX(tx, target, c.id)
		if err != nil {
			return err
		}

		err = r.CompleteKeyExchange(&trr.FullKX, true)
		if err != nil {
			return fmt.Errorf("could not complete key exchange: %v",
				err)
		}

		return c.db.DeleteTransResetHalfKX(tx, target)
	})
	if err != nil {
		return err
	}

	mediator.log.Infof("Completed trans reset with %s", ru)

	// Update the ratchet (this updates the DB).
	ru.replaceRatchet(r)

	// Send UI event.
	c.ntfns.notifyOnKXCompleted(nil, ru, false)
	return nil
}
