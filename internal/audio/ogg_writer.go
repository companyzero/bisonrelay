package audio

import (
	"encoding/binary"
	"io"
	"math/rand"
)

const oggSig = "OggS"

type OggHeader struct {
	Version     uint8
	IsContinued bool
	IsFirstPage bool
	IsLastPage  bool

	GranulePosition uint64
	BitstreamSerial uint32
	PageSequence    uint32
	CrcChecksum     uint32

	PageSegments uint8
	SegmentTable []uint8
}

type OggPage struct {
	OggHeader
	Segments [][]byte

	// Size of all segments in bytes
	SegmentTotal int
}

var checksumTable = crcChecksum()

type oggWriter struct {
	w      io.Writer
	serial uint32
}

func newOggWriter(out io.Writer) *oggWriter {
	return &oggWriter{
		w:      out,
		serial: rand.Uint32(),
	}
}

func (o *oggWriter) WritePage(p OggPage) error {
	headerSize := 27 + int(p.PageSegments)
	totalSize := headerSize + p.SegmentTotal

	buf := make([]byte, totalSize)
	headerType := uint8(0x0)
	if p.IsContinued {
		headerType = headerType | 0x1
	}
	if p.IsFirstPage {
		headerType = headerType | 0x2
	}
	if p.IsLastPage {
		headerType = headerType | 0x4
	}

	copy(buf[0:], oggSig)
	buf[4] = p.Version
	buf[5] = headerType

	binary.LittleEndian.PutUint64(buf[6:], p.GranulePosition)
	binary.LittleEndian.PutUint32(buf[14:], p.BitstreamSerial)
	binary.LittleEndian.PutUint32(buf[18:], p.PageSequence)
	// compute checksum later

	buf[26] = p.PageSegments
	for i, s := range p.SegmentTable {
		buf[27+i] = s
	}

	idx := headerSize
	for i, s := range p.Segments {
		copy(buf[idx:], s)
		idx += int(p.SegmentTable[i])
	}

	var checksum uint32
	for i := range buf {
		checksum = (checksum << 8) ^ checksumTable[byte(checksum>>24)^buf[i]]
	}
	binary.LittleEndian.PutUint32(buf[22:], checksum)

	_, err := o.w.Write(buf)
	return err
}

// partions a slice of bytes into units no bigger than 255
func partition(p []byte) ([]uint8, [][]byte) {
	segCountHint := len(p)/255 + 1
	st := make([]uint8, 0, segCountHint)
	s := make([][]byte, 0, segCountHint)

	for len(p) > 255 {
		st = append(st, 255)
		s = append(s, p[:255])
		p = p[255:]
	}

	st = append(st, uint8(len(p)))
	s = append(s, p)

	// packet of exactly 255 bytes is terminated by lacing value of 0
	if len(p) == 255 {
		st = append(st, 0)
		s = append(s, []byte{})
	}
	return st, s
}

func (o *oggWriter) NewPage(payload []byte, granulePosition uint64, pageSeqence uint32) OggPage {
	segTable, segments := partition(payload)
	total := len(payload)

	return OggPage{
		OggHeader: OggHeader{
			Version:         0,
			GranulePosition: granulePosition,
			BitstreamSerial: o.serial,
			PageSequence:    pageSeqence,

			PageSegments: uint8(len(segTable)),
			SegmentTable: segTable,
		},
		Segments:     segments,
		SegmentTotal: total,
	}
}

func (o *oggWriter) Finish(granulePosition uint64, pageSeqence uint32) error {
	page := o.NewPage([]byte{}, granulePosition, pageSeqence)
	page.IsLastPage = true
	return o.WritePage(page)
}

// https://github.com/pion/webrtc/blob/67826b19141ec9e6f1002a2267008a016a118934/pkg/media/oggwriter/oggwriter.go#L245-L261
func crcChecksum() *[256]uint32 {
	var table [256]uint32
	const poly = 0x04c11db7

	for i := range table {
		r := uint32(i) << 24
		for j := 0; j < 8; j++ {
			if (r & 0x80000000) != 0 {
				r = (r << 1) ^ poly
			} else {
				r <<= 1
			}
			table[i] = (r & 0xffffffff)
		}
	}
	return &table
}
