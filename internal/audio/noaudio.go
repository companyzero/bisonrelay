//go:build !cgo || noaudio

// This audio context is only used in cgo-less and noaudio builds.

package audio

import (
	"errors"

	"github.com/decred/slog"
)

func init() {
	newAudioContext = newNullAudioContext
}

type nullAudioContext struct{}

func newNullAudioContext() (audioContext, error) {
	return nullAudioContext{}, nil
}

func (_ nullAudioContext) name() string { return "nullaudio" }

type nullAudioDevice struct{}

func (_ nullAudioDevice) Start() error { return nil }
func (_ nullAudioDevice) Stop() error  { return nil }
func (_ nullAudioDevice) Uninit()      {}

func (_ nullAudioContext) initPlayback(deviceID DeviceID, cb dataProc) (playbackDevice, error) {
	return nullAudioDevice{}, nil
}

func (_ nullAudioContext) initCapture(deviceID DeviceID, cb dataProc) (captureDevice, error) {
	return nullAudioDevice{}, nil
}

func (_ nullAudioContext) free() error {
	return nil
}

type nullAudioEncDec struct{}

func (_ nullAudioEncDec) Encode(pcm []int16, frameSize int, out []byte) ([]byte, error) {
	return out, nil
}

func (_ nullAudioEncDec) SetBitrate(rate int) {}

func (_ nullAudioEncDec) Decode(data []byte, frameSize int, fec bool, out []int16) ([]int16, error) {
	return out, nil
}

func (_ nullAudioContext) newEncoder(sampleRate, channels int) (streamEncoder, error) {
	return nullAudioEncDec{}, nil
}

func (_ nullAudioContext) newDecoder(sampleRate, channels int) (streamDecoder, error) {
	return nullAudioEncDec{}, nil
}

var errAudioDisabledCompilation = errors.New("audio was disabled during compilation")

func ListAudioDevices(log slog.Logger) (Devices, error) {
	return Devices{}, errAudioDisabledCompilation
}

func FindDevice(typ DeviceType, id string) *Device { return nil }
