package rtdtserver

import (
	"encoding/binary"

	"github.com/companyzero/bisonrelay/rpc"
	"golang.org/x/crypto/nacl/secretbox"
)

// framedPktBuffer allows modifying an outbound buffer that contains a
// FramedPacket without having to go through encoding/decoding.
//
// This is used in the server to avoid having to keep rewriting and/or
// allocating buffers for multiple messages.
//
// Note: the set/get functions assume a valid packet has been either written to
// or read into the buffer.
type framedPktBuffer struct {
	// b is the raw buffer of the framed packet. This should have
	// len == rpc.RTDTMaxMessageSize.
	b []byte

	// n is the current number of bytes used from b.
	n int
}

// hasValidSize returns true if this buffer has enough bytes to be considered a
// valid framed packet..
func (fpb *framedPktBuffer) hasValidSize() bool {
	return fpb.n >= 12
}

// toPacket decodes this framed buffer into a full packet structure.
func (fpb *framedPktBuffer) toPacket(pkt *rpc.RTDTFramedPacket) {
	pkt.Target = fpb.getTarget()
	pkt.Source = fpb.getSource()
	pkt.Sequence = fpb.getSequence()
	pkt.Data = fpb.getData()
}

// getTarget returns the "Target" field of the framed packet.
func (fpb *framedPktBuffer) getTarget() rpc.RTDTPeerID {
	return rpc.RTDTPeerID(binary.BigEndian.Uint32(fpb.b))
}

// setTarget replaces the "Target" field of the framed packet.
func (fpb *framedPktBuffer) setTarget(id rpc.RTDTPeerID) {
	binary.BigEndian.PutUint32(fpb.b, uint32(id))
}

// getSource returns the "Source" field of the framed packet.
func (fpb *framedPktBuffer) getSource() rpc.RTDTPeerID {
	return rpc.RTDTPeerID(binary.BigEndian.Uint32(fpb.b[4:]))
}

// setSource replaces the "Source" field of the framed packet.
func (fpb *framedPktBuffer) setSource(id rpc.RTDTPeerID) {
	binary.BigEndian.PutUint32(fpb.b[4:], uint32(id))
}

// getSequence returns the value of the sequence field of the framed packet.
func (fpb *framedPktBuffer) getSequence() uint32 {
	return binary.BigEndian.Uint32(fpb.b[8:])
}

// setSequence replaces the "Sequence" field of the framed packet.
func (fpb *framedPktBuffer) setSequence(seq uint32) {
	binary.BigEndian.PutUint32(fpb.b[8:], seq)
}

// framedPktBuffer returns the "Data" field of the framed packet.
func (fpb *framedPktBuffer) getData() []byte {
	return fpb.b[12:fpb.n]
}

// setCmdPayload sets the payload as if it were a server command of the
// following type and content.
func (fpb *framedPktBuffer) setCmdPayload(cmd rpc.RTDTServerCmdType, b []byte) {
	fpb.b[12] = byte(cmd)         // Header (command type).
	copied := copy(fpb.b[13:], b) // Payload
	fpb.n = 13 + copied
}

// setFullData sets the full packet data as the passed source buffer.
func (fpb *framedPktBuffer) setFullData(src []byte) {
	fpb.n = copy(fpb.b, src)
}

// decryptFrom decrypts the data packet from the passed source buffer. Returns
// true if decryption succeeded.
func (fpb *framedPktBuffer) decryptFrom(src []byte, key *[32]byte) bool {
	out, ok := secretbox.Open(fpb.b[:0], src[24:], aliasNonce(src), key)
	fpb.n = len(out)
	return ok
}

// outBuffer returns the full output buffer.
func (fpb *framedPktBuffer) outBuffer() []byte {
	return fpb.b[:fpb.n]
}

// newFramedPktBuffer creates a new framed packet buffer, presized to messages
// of the maximum allowed packet size.
func newFramedPktBuffer() *framedPktBuffer {
	return &framedPktBuffer{
		b: make([]byte, rpc.RTDTMaxMessageSize),
	}
}
