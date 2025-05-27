package audio

import (
	"math"
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

// detectSound checks if the audio buffer contains sound above the specified
// threshold.
func detectSound(buffer []int16, threshold int16, minCount int) bool {
	if len(buffer) == 0 || threshold <= 0 {
		return false
	}

	// Count samples exceeding threshold
	count := 0
	for _, sample := range buffer {
		// Take absolute value of sample
		if sample < 0 {
			sample = -sample
		}

		if sample > threshold {
			count++
			if count >= minCount {
				return true
			}
		}
	}

	return false
}

// applyGainDB modifies the volume of audio samples using decibels
// dB: gain in decibels (0 = original, 6 = 2x louder, -6 = 1/2 volume)
func applyGainDB(buffer []int16, dB float64) {
	// Convert dB to amplitude multiplier
	// Formula: gain = 10^(dB/20)
	gain := math.Pow(10, dB/20.0)

	for i := range buffer {
		sample := float64(buffer[i])
		sample = sample * gain

		// Clamp to int16 range [-32768, 32767]
		if sample > 32767 {
			sample = 32767
		} else if sample < -32768 {
			sample = -32768
		}

		buffer[i] = int16(sample)
	}
}
