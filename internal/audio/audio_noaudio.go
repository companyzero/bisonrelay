//go:build !cgo || noaudio

package audio

import (
	"errors"

	"github.com/decred/slog"
)

var errAudioDisabledCompilation = errors.New("audio was disabled during compilation")

func ListAudioDevices(log slog.Logger) (Devices, error) {
	return Devices{}, errAudioDisabledCompilation
}

func FindDevice(typ DeviceType, id string) *Device { return nil }
