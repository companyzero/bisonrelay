package lowlevel

// OutboundRM is the interface for sending routed messages via the rmq.
type OutboundRM interface {
	EncryptedLen() uint32
	EncryptedMsg() (RVID, []byte, error)
	Priority() uint
	PaidForRM(int64, int64)
}
