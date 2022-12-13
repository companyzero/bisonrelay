package client

import (
	"github.com/companyzero/bisonrelay/client/internal/lowlevel"
	"github.com/companyzero/bisonrelay/ratchet"
)

// rawRM is a routed message that is considered already encrypted and is sent
// as-is to the remote side.
type rawRM struct {
	pri      uint
	msg      []byte
	rv       lowlevel.RVID
	paidRMCB func(amount, fees int64)
}

func (rm rawRM) Priority() uint {
	return rm.pri
}

func (rm rawRM) EncryptedLen() uint32 {
	return uint32(len(rm.msg))
}

func (rm rawRM) EncryptedMsg() (lowlevel.RVID, []byte, error) {
	return rm.rv, rm.msg, nil
}

func (rm rawRM) String() string {
	return "rawRM"
}

func (rm rawRM) PaidForRM(amount, fees int64) {
	if rm.paidRMCB != nil {
		rm.paidRMCB(amount, fees)
	}
}

// remoteUserRM is an outbound RM type that encrypts messages via a ratchet
// associated with a remote user. This encrypts the msg just before being sent.
type remoteUserRM struct {
	pri      uint
	msg      []byte
	payloadT string

	encrypted []byte
	sendRV    lowlevel.RVID
	ru        *RemoteUser
	payEvent  string
}

// Assert remoteUserRM fulfills the outboundRM interface.
var _ lowlevel.OutboundRM = (*remoteUserRM)(nil)

func (rm *remoteUserRM) Priority() uint {
	return rm.pri
}

func (rm *remoteUserRM) EncryptedLen() uint32 {
	return uint32(ratchet.EncryptedSize(len(rm.msg)))
}

func (rm *remoteUserRM) EncryptedMsg() (lowlevel.RVID, []byte, error) {
	return rm.ru.encryptRM(rm)
}

func (rm *remoteUserRM) String() string {
	return rm.payloadT
}

func (rm *remoteUserRM) PaidForRM(amount, fees int64) {
	go rm.ru.paidForRM(rm.payEvent, amount, fees)
}
