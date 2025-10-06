package server

import (
	"context"
	"fmt"
	"time"

	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/dcrlnd/lnrpc"
)

func (z *ZKS) handleCreateRTDTSession(ctx context.Context, sc *sessionContext,
	msg rpc.Message, r rpc.CreateRTDTSession) error {

	// Hardcoded to a low number for the moment, until more tests are made.
	if r.Size > 64 {
		return fmt.Errorf("client requested too large session")
	}

	if z.rtServerAddr == "" {
		return fmt.Errorf("RTDT sessions are disabled")
	}

	// Verify payment.
	switch z.settings.PayScheme {
	case rpc.PaySchemeFree:
		// Keep going.

	case rpc.PaySchemeDCRLN:
		sc.Lock()
		if sc.lnCreateRTSessHash == nil {
			sc.Unlock()
			return fmt.Errorf("tried to create RTDT session without payment")
		}

		// Want payment of base rate for new session + per user-day
		// of the desired session size.
		wantMAtoms := z.settings.MilliAtomsPerRTSess +
			uint64(r.Size)*z.settings.MilliAtomsPerUserRTSess
		lookupReq := &lnrpc.PaymentHash{RHash: sc.lnCreateRTSessHash}
		lookupRes, err := z.lnRpc.LookupInvoice(ctx, lookupReq)
		if lookupRes != nil {
			switch {
			case lookupRes.State == lnrpc.Invoice_CANCELED:
				err = fmt.Errorf("LN invoice canceled")

			case lookupRes.State != lnrpc.Invoice_SETTLED:
				err = fmt.Errorf("unexpected LN state: %d",
					lookupRes.State)

			case lookupRes.AmtPaidMAtoms < int64(wantMAtoms):
				// Also have upper limit if
				// overpaid?
				err = fmt.Errorf("LN invoice not "+
					"sufficiently paid (got %d, want %d)",
					lookupRes.AmtPaidMAtoms, wantMAtoms)
			}
		}
		sc.lnCreateRTSessHash = nil
		sc.Unlock()

		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("unsupported pay scheme %s", z.settings.PayScheme)
	}

	// Generate the session cookie.
	scookie := rpc.RTDTSessionCookie{
		ServerSecret: zkidentity.RandomShortID(),
		Size:         r.Size,
	}

	// Send reply of created session.
	sc.writer <- &RPCWrapper{
		Message: rpc.Message{
			Command: rpc.TaggedCmdCreateRTDTSessionReply,
			Tag:     msg.Tag,
		},
		Payload: rpc.CreateRTDTSessionReply{
			SessionCookie: scookie.Encrypt(nil, z.rtCookieKey),
		},
	}

	return nil
}

func (z *ZKS) handleGetRTDTAppointCookie(ctx context.Context, sc *sessionContext,
	msg rpc.Message, r rpc.GetRTDTAppointCookies) error {

	// Verify payment.
	var paidCookies int64
	switch z.settings.PayScheme {
	case rpc.PaySchemeFree:
		// Assume paid for all requested cookies.
		paidCookies = int64(len(r.Peers))

	case rpc.PaySchemeDCRLN:
		sc.Lock()
		if sc.lnGetRTCookieHash == nil {
			sc.Unlock()
			return fmt.Errorf("tried to get RTDT cookies without payment")
		}

		// Want payment of base rate for new session + per user-day
		// of the desired session size.
		wantMAtoms := z.settings.MilliAtomsGetCookie
		lookupReq := &lnrpc.PaymentHash{RHash: sc.lnGetRTCookieHash}
		lookupRes, err := z.lnRpc.LookupInvoice(ctx, lookupReq)
		if lookupRes != nil {
			switch {
			case lookupRes.State == lnrpc.Invoice_CANCELED:
				err = fmt.Errorf("LN invoice canceled")

			case lookupRes.State != lnrpc.Invoice_SETTLED:
				err = fmt.Errorf("unexpected LN state: %d",
					lookupRes.State)

			case lookupRes.AmtPaidMAtoms < int64(wantMAtoms):
				err = fmt.Errorf("LN invoice not "+
					"sufficiently paid (got %d, want %d)",
					lookupRes.AmtPaidMAtoms, wantMAtoms)

			default:
				// Figure out how many cookies this payment was
				// for.
				paidCookies = (lookupRes.AmtPaidMAtoms - int64(z.settings.MilliAtomsGetCookie)) /
					int64(z.settings.MilliAtomsPerUserCookie)
			}
		}
		sc.lnGetRTCookieHash = nil
		sc.Unlock()

		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("unsupported pay scheme %s", z.settings.PayScheme)
	}

	if paidCookies < int64(len(r.Peers)) {
		return fmt.Errorf("user paid for less cookies (%d) than requested (%d)",
			paidCookies, len(r.Peers))
	}

	var scookie rpc.RTDTSessionCookie
	err := scookie.Decrypt(r.SessionCookie, z.rtCookieKey, z.rtDecodeCookieKeys)
	if err != nil {
		return fmt.Errorf("unable to decrypt session cookie: %v", err)
	}

	reply := rpc.GetRTDTAppointCookiesReply{
		AppointCookies: make([][]byte, 0, len(r.Peers)),
	}
	for _, peer := range r.Peers {
		ac := rpc.RTDTAppointCookie{
			ServerSecret:       scookie.ServerSecret,
			OwnerSecret:        r.OwnerSecret,
			Size:               scookie.Size,
			PeerID:             peer.ID,
			AllowedAsPublisher: peer.AllowedAsPublisher,
			IsAdmin:            peer.IsAdmin,
		}
		encrypted := ac.Encrypt(nil, z.rtCookieKey)
		reply.AppointCookies = append(reply.AppointCookies, encrypted)
	}

	// If the user requested it, generate a rotate cookie.
	if r.OldOwnerSecret != nil {
		var rc rpc.RTDTRotateCookie
		copy(rc.OldOwnerSecret[:], r.OldOwnerSecret[:])
		copy(rc.NewOwnerSecret[:], r.OwnerSecret[:])
		rc.Timestamp = time.Now().Unix()
		rc.Size = scookie.Size
		rc.ServerSecret = scookie.ServerSecret
		rc.PaymentTag = randomUint64()
		reply.RotateCookie = rc.Encrypt(nil, z.rtCookieKey)
	}

	// Send reply with appoint cookies.
	sc.writer <- &RPCWrapper{
		Message: rpc.Message{
			Command: rpc.TaggedCmdGetRTDTAppointCookieReply,
			Tag:     msg.Tag,
		},
		Payload: reply,
	}
	return nil
}

func (z *ZKS) handleAppointRTDTServer(ctx context.Context, sc *sessionContext,
	msg rpc.Message, r rpc.AppointRTDTServer) error {

	// Verify payment.
	var publishAllowanceMAtoms int64
	switch z.settings.PayScheme {
	case rpc.PaySchemeFree:
		// When using the free payment scheme, just assume the amount
		// of bytes.
		publishAllowanceMAtoms = int64(rpc.PublishFreePayRTPublishAllowanceMB*
			z.settings.MilliAtomsRTPushRate + z.settings.MilliAtomsRTJoin)

	case rpc.PaySchemeDCRLN:
		sc.Lock()
		if sc.lnPublishInRTSessHash == nil {
			sc.Unlock()
			return fmt.Errorf("tried to get RTDT server appointment without payment")
		}

		// Determine net payment amount for allowance (paid amount
		// minus the minimum payment to join the session).
		lookupReq := &lnrpc.PaymentHash{RHash: sc.lnPublishInRTSessHash}
		lookupRes, err := z.lnRpc.LookupInvoice(ctx, lookupReq)
		if lookupRes != nil {
			switch {
			case lookupRes.State == lnrpc.Invoice_CANCELED:
				err = fmt.Errorf("LN invoice canceled")

			case lookupRes.State != lnrpc.Invoice_SETTLED:
				err = fmt.Errorf("unexpected LN state: %d",
					lookupRes.State)
			}

			// The allowance is whatever the user paid, minus the
			// base payment fee.
			publishAllowanceMAtoms = lookupRes.AmtPaidMAtoms
		}
		sc.lnPublishInRTSessHash = nil
		sc.Unlock()

		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("unsupported pay scheme %s", z.settings.PayScheme)
	}

	// Decrypt the appointment cookie to determine session details.
	var acookie rpc.RTDTAppointCookie
	err := acookie.Decrypt(r.AppointCookie, z.rtCookieKey, z.rtDecodeCookieKeys)
	if err != nil {
		return fmt.Errorf("error decrypting appoint cookie: %v", err)
	}

	if z.settings.PayScheme == rpc.PaySchemeFree {
		// Special case free payment to be 1MB per user.
		publishAllowanceMAtoms *= int64(acookie.Size)
	}

	// Determine the final allowance in bytes.
	// sessionRate := int64(z.settings.MilliAtomsRTPubPerUserMB) * (int64(acookie.Size) - 1)
	// allowanceBytes := publishAllowanceMAtoms * 1000000 / sessionRate
	allowanceMB, err := rpc.CalcRTDTSessPushMB(z.settings.MilliAtomsRTJoin,
		z.settings.MilliAtomsRTPushRate, z.settings.RTPushRateMBytes, acookie.Size,
		publishAllowanceMAtoms)
	if err != nil {
		return fmt.Errorf("errored calculating session publish allowance: %v", err)
	}
	allowanceBytes := uint64(allowanceMB) * 1000000

	// When not allowed as publisher, zero the allowance to prevent relaying
	// any data.
	if !acookie.AllowedAsPublisher {
		allowanceBytes = 0
	}

	z.log.Debugf("Sending join cookie to peer %s with allowance %d, size %d, "+
		"isPublisher %v, isAdmin %v", acookie.PeerID, allowanceBytes,
		acookie.Size, acookie.AllowedAsPublisher, acookie.IsAdmin)

	// Encode join cookie.
	jc := rpc.RTDTJoinCookie{
		ServerSecret:     acookie.ServerSecret,
		OwnerSecret:      acookie.OwnerSecret,
		PeerID:           acookie.PeerID,
		Size:             acookie.Size,
		EndTimestamp:     time.Now().Add(time.Hour).Unix(),
		PublishAllowance: allowanceBytes,
		PaymentTag:       randomUint64(),
		IsAdmin:          acookie.IsAdmin,
	}

	reply := rpc.AppointRTDTServerReply{
		JoinCookie:    jc.Encrypt(nil, z.rtCookieKey),
		ServerAddress: z.rtServerAddr,
		ServerPubKey:  z.rtServerPubKey,
	}

	// Send reply of created session.
	sc.writer <- &RPCWrapper{
		Message: rpc.Message{
			Command: rpc.TaggedCmdAppointRTDTServerReply,
			Tag:     msg.Tag,
		},
		Payload: reply,
	}
	return nil
}
