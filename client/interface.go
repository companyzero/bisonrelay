package client

import (
	"github.com/companyzero/bisonrelay/client/internal/lowlevel"
)

// rmqIntf is the public interface for the rmq that clients can use to send
// RMs.
type rmqIntf interface {
	SendRM(orm lowlevel.OutboundRM) error
	QueueRM(orm lowlevel.OutboundRM, replyChan chan error) error
}

// rdzvManagerIntf is the public interface for the rendezvous manager that
// clients can use to register for rendezvous pushes.
type rdzvManagerIntf interface {
	Sub(rdzv lowlevel.RVID, handler lowlevel.RVHandler, paidHandler lowlevel.SubPaidHandler) error
	Unsub(rdzv lowlevel.RVID) error
}

// lnNodeSession fulfilled by the actual serverSession to return the ln node of
// the server.
type lnNodeSession interface {
	LNNode() string
}
