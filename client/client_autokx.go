package client

import (
	"errors"
	"fmt"
	"time"

	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
)

// Autokx flow (where Alice is asking Bob for an invite to Charlie):
//
//          Alice                       Bob                    Charlie
//         -------                     -----                  ---------
//
//      RequestMediateIdentity()
//           \--- RMMediateIdentity -->
//
//                                handleMediateID()
//                                       \------- RMInvite -->
//
//                                                         handleRMInvite()
//                                        <-- RMTransitiveMessage -----/
//                                          [RMPublicIdentityInvite]
//                                                         guestList.runInvite()
//
//
//                                handleTransitiveMsg()
//              <-- RMTransitiveMessageFwd -----/
//                 [RMPublicIdentityInvite]
//
//     handleTransitiveMsgFwd()
//     handleTransitiveIDInvite()
//     hostList.runInvite()
//
//                   (rest of flow is similar to standard kx)

// RequestMediateIdentity attempts to start a kx process with target by asking
// mediator for an introduction.
func (c *Client) RequestMediateIdentity(mediator, target UserID) error {
	mu, err := c.rul.byID(mediator)
	if err != nil {
		return fmt.Errorf("cannot request mediate identity: no session "+
			"with mediator %s: %v", mediator, err)
	}

	_, err = c.rul.byID(target)
	if !errors.Is(err, userNotFoundError{}) {
		if err == nil {
			return fmt.Errorf("cannot request mediate identity: already have"+
				"session with target %s", target)
		} else {
			return fmt.Errorf("unexpected error fetching user: %v", err)
		}
	}

	// Track that we requested this mediate ID request.
	err = c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.StoreMediateIDRequested(tx, mediator, target)
	})
	if err != nil {
		return err
	}

	mu.log.Infof("Asking to mediate identity to target %s", target)
	mi := rpc.RMMediateIdentity{Identity: target}
	payEvent := fmt.Sprintf("mediateid.%s", target)
	return c.sendWithSendQ(payEvent, mi, mediator)
}

func (c *Client) handleMediateID(ru *RemoteUser, mi rpc.RMMediateIdentity) error {
	if c.cfg.TransitiveEvent != nil {
		c.cfg.TransitiveEvent(ru.ID(), mi.Identity, TEMediateID)
	}

	target, err := c.rul.byID(mi.Identity)
	if err != nil {
		ru.log.Warnf("Asked to mediate id to unknown user %s",
			zkidentity.ShortID(mi.Identity))
		return err
	}
	ru.log.Infof("Asked to mediate id to %s", target)

	// Ask target to generate an identity invite.
	rm := rpc.RMInvite{Invitee: ru.PublicIdentity()}
	payEvent := fmt.Sprintf("mediateid.%s", ru.ID())
	return target.sendRM(rm, payEvent)
}

func (c *Client) handleRMInvite(ru *RemoteUser, iv rpc.RMInvite) error {
	ru.log.Infof("Requested invite on behalf of %s (%q)", iv.Invitee.Identity,
		iv.Invitee.Nick)
	if c.cfg.TransitiveEvent != nil {
		c.cfg.TransitiveEvent(ru.ID(), iv.Invitee.Identity, TERequestInvite)
	}

	// Generate an invite.
	mediatorID := ru.ID()
	pii, err := c.kxl.createInvite(nil, &iv.Invitee, &mediatorID, false, nil)
	if err != nil {
		return err
	}

	return ru.sendTransitive(pii, "RMInvite", iv.Invitee, priorityDefault)
}

func (c *Client) handleTransitiveIDInvite(ru *RemoteUser, pii rpc.OOBPublicIdentityInvite) error {

	// Double check we actually requested this invitation.
	err := c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		_, err := c.db.HasMediateID(tx, ru.ID(), pii.Public.Identity)
		if errors.Is(err, clientdb.ErrNotFound) {
			return fmt.Errorf("local client did not request MI to %s"+
				"by way of %s", pii.Public.Identity, ru.ID())
		}
		return err
	})
	if err != nil {
		return err
	}

	if !pii.Public.VerifyIdentity() {
		return fmt.Errorf("received pii with key different then expected by identity")
	}

	ru.log.Infof("Accepting transitive invite from %s (%q)", pii.Public.Identity,
		pii.Public.Nick)
	if c.cfg.TransitiveEvent != nil {
		c.cfg.TransitiveEvent(ru.ID(), pii.Public.Identity, TEReceivedInvite)
	}

	err = c.kxl.acceptInvite(pii, false, true)
	if errors.Is(err, errUserBlocked) {
		ru.log.Infof("Canceled invite from blocked identity %s (%q)", pii.Public.Identity,
			pii.Public.Nick)
		return nil
	} else if errKX := new(errHasOngoingKX); errors.As(err, errKX) {
		ru.log.Infof("Skipping accepting invite for kx %s from %s (%q) due to "+
			"already ongoing kx (RV %s)", pii.InitialRendezvous,
			pii.Public.Identity, pii.Public.Nick, errKX.otherRV)
	} else if err != nil {
		return err
	}

	// Everything ok, remove this mediate id request.
	return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		return c.db.RemoveMediateID(tx, ru.ID(), pii.Public.Identity)
	})
}

// ListMediateIDs lists mediate id requests made by the local client.
func (c *Client) ListMediateIDs() ([]clientdb.MediateIDRequest, error) {
	var res []clientdb.MediateIDRequest
	err := c.dbView(func(tx clientdb.ReadTx) error {
		var err error
		res, err = c.db.ListMediateIDs(tx)
		return err
	})
	return res, err
}

// clearOldMediateIDs removes all mediate id requests that were requested over 7
// days ago.
func (c *Client) clearOldMediateIDs(miExpiryDuration time.Duration) error {
	limitDate := time.Now().Add(-miExpiryDuration)
	return c.dbUpdate(func(tx clientdb.ReadWriteTx) error {
		mis, err := c.db.ListMediateIDs(tx)
		if err != nil {
			return err
		}

		for _, mi := range mis {
			if !mi.Date.Before(limitDate) {
				continue
			}

			err := c.db.RemoveMediateID(tx, mi.Mediator, mi.Target)
			if err != nil {
				c.log.Warnf("Unable to remove mediate id to %s "+
					"via %s: %v", mi.Target, mi.Mediator, err)
			}
		}

		return nil
	})
}
