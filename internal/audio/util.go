package audio

import (
	"slices"
)

func bytesToLES16Slice(src []byte, dst []int16) []int16 {
	s16len := len(src) / 2
	dst = slices.Grow(dst, s16len)
	for i := 0; i < s16len; i++ {
		dst = append(dst, int16(src[i*2])|(int16(src[i*2+1])<<8))
	}
	return dst
}

func leS16SliceToBytes(src []int16, dst []byte) []byte {
	s8len := len(src) * 2
	dst = slices.Grow(dst, s8len)
	for i := 0; i < len(src); i++ {
		dst = append(dst, byte(src[i]), byte(src[i]>>8))
	}
	return dst
}
