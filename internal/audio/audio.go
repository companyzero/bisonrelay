//go:build cgo && !noaudio

package audio

import (
	"github.com/decred/slog"

	"github.com/gen2brain/malgo"
)

func listMalgoDevices(typ malgo.DeviceType, malgoCtx *malgo.AllocatedContext, log slog.Logger) ([]Device, error) {
	devices, err := malgoCtx.Devices(typ)
	if err != nil {
		return nil, err
	}

	res := make([]Device, 0, len(devices))
	for _, dev := range devices {
		full, err := malgoCtx.DeviceInfo(typ, dev.ID, malgo.Shared)
		if err != nil {
			log.Warnf("Unable to get audio device info: %v", err)
			continue
		}

		res = append(res, Device{
			ID:        string(append([]byte(nil), full.ID[:]...)),
			Name:      full.Name(),
			IsDefault: full.IsDefault == 1,
		})
	}

	return res, nil
}

// ListAudioDevices lists available audio devices.
func ListAudioDevices(log slog.Logger) (Devices, error) {
	malgoCtx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return Devices{}, err
	}
	defer func() {
		_ = malgoCtx.Uninit()
		malgoCtx.Free()
	}()

	// Devices.
	playbackDevs, err := listMalgoDevices(malgo.Playback, malgoCtx, log)
	if err != nil {
		return Devices{}, err
	}
	captureDevs, err := listMalgoDevices(malgo.Capture, malgoCtx, log)
	if err != nil {
		return Devices{}, err
	}

	return Devices{
		Playback: playbackDevs,
		Capture:  captureDevs,
	}, nil
}

// FindDevice finds the device with the given ID or returns nil.
func FindDevice(typ DeviceType, id string) *Device {
	malgoCtx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil
	}
	defer func() {
		_ = malgoCtx.Uninit()
		malgoCtx.Free()
	}()

	malgoDt := malgo.Capture
	if typ == DeviceTypePlayback {
		malgoDt = malgo.Playback
	}
	devices, err := listMalgoDevices(malgoDt, malgoCtx, slog.Disabled)
	if err != nil {
		return nil
	}

	for i := range devices {
		if devices[i].ID == id {
			out := new(Device)
			*out = devices[i]
			return out
		}
	}

	return nil
}
