package rtdtserver

import (
	"context"
	"fmt"
	randv2 "math/rand/v2"
	"slices"
	"sync"
	"time"

	"github.com/companyzero/bisonrelay/rpc"
)

// peerSession represents a peer inside one session.
type peerSession struct {
	peerID  rpc.RTDTPeerID
	sess    *session
	size    int
	c       *conn
	isAdmin bool

	paymentsMtx sync.Mutex
	payments    []*payment
}

// addPayment dedupes and adds a payment to this peerSession.
func (ps *peerSession) addPayment(p *payment) {
	added := false

	ps.paymentsMtx.Lock()
	for i := range ps.payments {
		if p.tag == ps.payments[i].tag {
			added = true
			break
		}
	}
	if !added {
		ps.payments = append(ps.payments, p)
	}
	ps.paymentsMtx.Unlock()
}

// deductAllowance deducts the given allowance from this peer in this session.
// Returns true if there was enough allowance to deduct the entire n.
func (ps *peerSession) deductAllowance(n int64) bool {
	var res bool
	ps.paymentsMtx.Lock()
	for i := 0; n > 0 && i < len(ps.payments); {
		newAllowance := ps.payments[i].bytesAllowance.Add(-n)
		if newAllowance < 0 {
			// Payment exhausted.
			ps.payments = slices.Delete(ps.payments, i, i+1)

			// Keep deducting from other payments.
			n = -newAllowance
		} else {
			res = true
			break
		}
	}
	ps.paymentsMtx.Unlock()
	return res
}

// validateJoinCookie decrypts and validates the given join cookie from a
// source peer..
func (s *Server) validateJoinCookie(sourceID rpc.RTDTPeerID, rawJc []byte, jc *rpc.RTDTJoinCookie) (*payment, error) {
	if err := jc.Decrypt(rawJc, s.cfg.cookieKey, s.cfg.decodeCookieKeys); err != nil {
		return nil, makeCodedError(errCodeJoinCookieInvalid, err)
	}

	if jc.PeerID != sourceID {
		return nil, errCodeJoinCookieWrongPeerID
	}

	endTime := time.Unix(jc.EndTimestamp, 0)
	if time.Now().After(endTime) {
		return nil, errCodeJoinCookieExpired
	}

	// Track payment allowance.
	s.paymentsMtx.Lock()
	pay := s.payments[jc.PaymentTag]
	if pay == nil {
		// Redeeming payment.
		pay = &payment{
			tag:     jc.PaymentTag,
			endTime: time.Unix(jc.EndTimestamp, 0),
		}
		pay.bytesAllowance.Store(int64(jc.PublishAllowance))
		s.payments[jc.PaymentTag] = pay
		s.log.Debugf("Redeeming payment tag %d with allowance %d",
			jc.PaymentTag, jc.PublishAllowance)
	} else {
		s.log.Debugf("Reusing payment tag %d with allowance %d",
			jc.PaymentTag, pay.bytesAllowance.Load())
	}
	s.paymentsMtx.Unlock()

	return pay, nil
}

// connHandleInternalPkt is the handler for internal server commands received
// from a remote peer.
func (s *Server) connHandleInternalPkt(_ context.Context, c *conn,
	inPkt *framedPktBuffer, pktTs time.Time, tempBuf []byte) error {

	// Decode into a packet to simplify the code.
	var pkt rpc.RTDTFramedPacket
	inPkt.toPacket(&pkt)

	// Preprare reply buffer. We can reuse the input buffer for this because
	// every reply is written after the input data is fully processed.
	replyBuf := inPkt.b[:0]

	cmdType, payload := pkt.ServerCmd()
	switch cmdType {
	case rpc.RTDTServerCmdTypePing:
		// Limit ping interval duration.
		oldPingTs, pinged := c.lastRecvPingTime.LoadAndStore(pktTs)
		if pinged && pktTs.Sub(oldPingTs) < s.cfg.minPingInterval {
			return errPingTooRecent
		}

		// Limit ping payload size.
		if len(payload) > rpc.RTDTMaxPingPayloadSize {
			return errPingTooLarge
		}

		// Decode ping data.
		var cmd rpc.RTDTPingCmd
		if err := cmd.FromBytes(payload); err != nil {
			return err
		}

		// Send Reply (echo back any payload).
		reply := rpc.RTDTPongCmd(cmd)
		pkt.Target = pkt.Source
		pkt.Source = 0
		pkt.Sequence = c.nextSendSeq()
		out := reply.AppendFramed(&pkt, replyBuf)
		n, err := s.listenerWrite(c, out, tempBuf)
		s.stats.bytesWritten.Add(float64(n))
		return err

	case rpc.RTDTServerCmdTypePong:
		if len(payload) > rpc.RTDTMaxPingPayloadSize {
			c.banScore.Add(1) // Pongs should have limited data.
		}

		return nil

	case rpc.RTDTServerCmdTypeLeaveSession:
		err := s.removeFromSession(c, pkt.Source, false)
		if err != nil && s.cfg.logReadLoopErrors {
			s.log.Warnf("Error processing session leave command: %v", err)
		}

		// Send Reply.
		pkt.Target = pkt.Source
		pkt.Source = 0
		pkt.Sequence = c.nextSendSeq()
		var reply rpc.RTDTServerCmdLeaveSessionReply
		out := reply.AppendFramed(&pkt, replyBuf)
		n, err := s.listenerWrite(c, out, tempBuf)
		s.stats.bytesWritten.Add(float64(n))
		return err

	case rpc.RTDTServerCmdTypeJoinSession:
		var cmd rpc.RTDTServerCmdJoinSession
		if err := cmd.FromBytes(payload); err != nil {
			return err
		}

		var size uint32
		var sessID sessionID
		var err error
		var pay *payment
		var isAdmin bool
		if s.cfg.cookieKey != nil {
			// Validate join cookie.
			var jc rpc.RTDTJoinCookie
			pay, err = s.validateJoinCookie(pkt.Source, cmd.JoinCookie, &jc)
			if err == nil {
				size = jc.Size
				sessID = sessionIDFromJoinCookie(&jc)
				isAdmin = jc.IsAdmin
			}
		} else {
			// Not validating cookies, simulate these values.
			size = 1 << 16                      // Only used to limit # of forwards
			sessID[28] = byte(pkt.Source >> 24) // 16 bit session
			sessID[29] = byte(pkt.Source >> 16)
			pay = &payment{
				tag:     randv2.Uint64(),
				endTime: time.Now().Add(s.cfg.dropPaymentLoopInterval),
			}
			pay.bytesAllowance.Store(10000000000) // 10GB
			isAdmin = pkt.Source&1 == 1           // Anyone with odd id is admin
		}

		var sess *session
		if err == nil {
			// Bind peer to session.
			sess, err = s.bindToSession(c, pkt.Source, pay,
				sessID, size, isAdmin)
		} else {
			c.banScore.Add(1)
		}

		// Send Reply.
		pkt.Target = pkt.Source
		pkt.Source = 0
		pkt.Sequence = c.nextSendSeq()
		var reply rpc.RTDTServerCmdJoinSessionReply
		if s.cfg.replyErrorCodes {
			// Only send reply error when debugging flag is set.
			reply.ErrCode = uint64(errorCodeFromError(err))
		}
		if err != nil && s.cfg.logReadLoopErrors {
			s.log.Warnf("Error processing session bind command: %v", err)
		}
		out := reply.AppendFramed(&pkt, replyBuf)

		n, err := s.listenerWrite(c, out, tempBuf)
		s.stats.bytesWritten.Add(float64(n))

		// Force send list of updated members after sending the reply
		// so that the source also receives an updated list.
		if sess != nil {
			s.forceSessListChan <- sess
		}

		return err

	case rpc.RTDTServerCmdTypeKickPeer:
		var cmd rpc.RTDTServerCmdKickPeer
		if err := cmd.FromBytes(payload); err != nil {
			return err
		}

		kickSource := pkt.Source
		targetConn, err := s.kickFromSession(c, kickSource, cmd.KickTarget, cmd.BanDurationSeconds)
		if err != nil {
			c.banScore.Add(1)
			if s.cfg.logReadLoopErrors {
				s.log.Warnf("Error processing kick peer command: %v", err)
			}
		} else {
			// Inform target they were kicked.
			pkt.Target = cmd.KickTarget
			pkt.Source = 0
			pkt.Sequence = targetConn.nextSendSeq()
			out := cmd.AppendFramed(&pkt, replyBuf)
			n, err := s.listenerWrite(targetConn, out, tempBuf)
			if err != nil {
				s.log.Warnf("Unable to write report to kicked target %s: %v",
					targetConn, err)
			} else {
				s.log.Debugf("Sent report to kicked target %s", cmd.KickTarget)
			}
			s.stats.bytesWritten.Add(float64(n))
		}

		// Send Reply.
		pkt.Target = kickSource
		pkt.Source = 0
		pkt.Sequence = c.nextSendSeq()
		reply := rpc.RTDTServerCmdKickPeerReply{KickTarget: cmd.KickTarget}
		if s.cfg.replyErrorCodes {
			// Only send reply error when debugging flag is set.
			reply.ErrCode = uint64(errorCodeFromError(err))

		}
		out := reply.AppendFramed(&pkt, replyBuf)

		n, err := s.listenerWrite(c, out, tempBuf)
		s.stats.bytesWritten.Add(float64(n))

		return err

	case rpc.RTDTServerCmdTypeRotateCookies:
		var cmd rpc.RTDTServerCmdRotateSessionCookies
		if err := cmd.FromBytes(payload); err != nil {
			return err
		}

		err := s.rotateSessionCookies(c, pkt.Source, cmd.RotateCookie)
		if err != nil {
			return err
		}

		// Send Reply.
		pkt.Target = pkt.Source
		pkt.Source = 0
		pkt.Sequence = c.nextSendSeq()
		reply := rpc.RTDTServerCmdRotateSessionCookiesReply{}
		if s.cfg.replyErrorCodes {
			// Only send reply error when debugging flag is set.
			reply.ErrCode = uint64(errorCodeFromError(err))

		}
		out := reply.AppendFramed(&pkt, replyBuf)

		n, err := s.listenerWrite(c, out, tempBuf)
		s.stats.bytesWritten.Add(float64(n))
		return err

	default:
		return fmt.Errorf("unknown server cmd type %d", cmdType)
	}
}
