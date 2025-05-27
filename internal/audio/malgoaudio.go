//go:build cgo && !noaudio

package audio

import (
	"encoding/binary"
	"fmt"
	"runtime"
	"strconv"

	"github.com/companyzero/gopus"
	"github.com/decred/slog"

	"github.com/gen2brain/malgo"
)

// rawFormat needs to be agreed upon between capture()/playback()
var rawFormat = malgo.FormatS16

// toMalgoDeviceId converts a device id to a malgo device id.
func (id DeviceID) toMalgoDeviceId() malgo.DeviceID {
	var res malgo.DeviceID
	if runtime.GOOS == "android" {
		i, err := strconv.ParseInt(string(id), 10, 32)
		if err == nil {
			binary.LittleEndian.PutUint32(res[:], uint32(i))
		}

	} else {
		copy(res[:], id)
	}
	return res
}

func init() {
	newAudioContext = newMalgoContext
}

func listMalgoDevices(typ malgo.DeviceType, malgoCtx *malgo.AllocatedContext, log slog.Logger) ([]Device, error) {
	devices, err := malgoCtx.Devices(typ)
	if err != nil {
		return nil, err
	}

	res := make([]Device, 0, len(devices))
	setIds := make(map[DeviceID]struct{}, len(devices))
	for _, dev := range devices {
		full, err := malgoCtx.DeviceInfo(typ, dev.ID, malgo.Shared)
		if err != nil {
			log.Warnf("Unable to get audio device info: %v", err)
			continue
		}

		// Avoid duplicate device IDs.
		id := DeviceID(string(append([]byte(nil), full.ID[:]...)))
		if _, ok := setIds[id]; ok {
			continue
		}
		setIds[id] = struct{}{}

		res = append(res, Device{
			ID:        id,
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
func FindDevice(typ DeviceType, id DeviceID) *Device {
	malgoCtx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil
	}
	defer func() {
		_ = malgoCtx.Uninit()
		malgoCtx.Free()
	}()

	devices, err := listMalgoDevices(malgo.DeviceType(typ), malgoCtx, slog.Disabled)
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

// malgoContext is an implementation of audioContext which offloads the
// work to malgo library.
type malgoContext struct {
	malgoCtx *malgo.AllocatedContext
}

// emptyDeviceID is an empty malgo device id.
var emptyDeviceID malgo.DeviceID

// newMalgoContext creates a new audioContext using malgo.
func newMalgoContext() (audioContext, error) {
	malgoCtx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, err
	}

	return &malgoContext{malgoCtx: malgoCtx}, nil
}

func (mpc *malgoContext) name() string {
	return "malgo"
}

func (mpc *malgoContext) free() error {
	if err := mpc.malgoCtx.Uninit(); err != nil {
		return err
	}
	mpc.malgoCtx.Free()
	return nil
}

// initPlayback is part of the audioContext interface.
func (mpc *malgoContext) initPlayback(deviceID DeviceID, cb dataProc) (playbackDevice, error) {
	// Sanity check.
	sampleSizeInBytes := malgo.SampleSizeInBytes(rawFormat)
	if sampleSizeInBytes != rawFormatSampleSize {
		return nil, fmt.Errorf("malgo raw format has wrong sample size "+
			"(got %d, want %d)", sampleSizeInBytes, rawFormatSampleSize)
	}

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	malgoDeviceID := deviceID.toMalgoDeviceId()
	if malgoDeviceID != emptyDeviceID {
		deviceConfig.Playback.DeviceID = malgoDeviceID.Pointer()
	}
	deviceConfig.PeriodSizeInMilliseconds = periodSizeMS
	deviceConfig.SampleRate = sampleRate
	deviceConfig.Playback.Format = rawFormat
	deviceConfig.Playback.Channels = channels
	deviceConfig.Alsa.NoMMap = 1

	playbackCallbacks := malgo.DeviceCallbacks{
		Data: malgo.DataProc(cb),
	}

	device, err := malgo.InitDevice(mpc.malgoCtx.Context, deviceConfig, playbackCallbacks)
	if err != nil {
		return nil, err
	}
	return device, nil
}

func (mpc *malgoContext) initCapture(deviceID DeviceID, cb dataProc) (captureDevice, error) {
	// Sanity check.
	sampleSizeInBytes := malgo.SampleSizeInBytes(rawFormat)
	if sampleSizeInBytes != rawFormatSampleSize {
		return nil, fmt.Errorf("malgo raw format has wrong sample size "+
			"(got %d, want %d)", sampleSizeInBytes, rawFormatSampleSize)
	}

	malgoDeviceID := deviceID.toMalgoDeviceId()
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.SampleRate = sampleRate
	deviceConfig.PeriodSizeInMilliseconds = periodSizeMS
	if malgoDeviceID != emptyDeviceID {
		deviceConfig.Capture.DeviceID = malgoDeviceID.Pointer()
	}
	deviceConfig.Capture.Format = rawFormat
	deviceConfig.Capture.Channels = channels
	deviceConfig.Alsa.NoMMap = 1 // Needed for capture?

	captureCallbacks := malgo.DeviceCallbacks{
		Data: malgo.DataProc(cb),
	}

	device, err := malgo.InitDevice(mpc.malgoCtx.Context, deviceConfig, captureCallbacks)
	if err != nil {
		return nil, err
	}
	return device, nil
}

func (mpc *malgoContext) newEncoder(sampleRate, channels int) (streamEncoder, error) {
	return gopus.NewEncoder(sampleRate, channels, gopus.Voip)
}

func (mpc *malgoContext) newDecoder(sampleRate, channels int) (streamDecoder, error) {
	return gopus.NewDecoder(sampleRate, channels)
}
