package audio

import (
	"errors"
	"strings"
)

// DeviceID is a generic id for playback and capture devices.
type DeviceID string

const DefaultDeviceID DeviceID = ""

func (id DeviceID) String() string {
	return strings.TrimRight(string(id), "\u0000 \t\n\r")
}

// DeviceType type.
//
// Note: This MUST match malgo.DeviceType definition.
type DeviceType uint32

// rawFormatSampleSize is how many bytes per sample on the raw audio interface.
//
// Fixed to 2 bytes per sample (16 bits per sample) and assumes the samples are
// ints (int16 samples).
const rawFormatSampleSize = 2

// DeviceType enumeration.
//
// Note: This MUST match malgo's device type enumeration.
const (
	Playback DeviceType = iota + 1
	Capture
	Duplex
	Loopback
)

// Device identifies capture/playback device.
type Device struct {
	ID        DeviceID `json:"id"`
	Name      string   `json:"name"`
	IsDefault bool     `json:"is_default"`
}

// Devices is the list of devices in the computer.
type Devices struct {
	Playback []Device `json:"playback"`
	Capture  []Device `json:"capture"`
}

// RecordInfo tracks information about a recording session.
type RecordInfo struct {
	SampleCount int `json:"sample_count"`
	DurationMs  int `json:"duration_ms"`
	EncodedSize int `json:"encoded_size"`
	PacketCount int `json:"packet_count"`
}

// dataProc matches malgo.DataProc.
type dataProc func(pOutputSample, pInputSamples []byte, framecount uint32)

// streamEncoder is the interface of voice encoders (i.e. gopus).
type streamEncoder interface {
	Encode(pcm []int16, frameSize int, out []byte) ([]byte, error)
	SetBitrate(rate int)
}

// streamDecoder is the interface of voice decoders (i.e. gopus).
type streamDecoder interface {
	Decode(data []byte, frameSize int, fec bool, out []int16) ([]int16, error)
}

// audioContext is an abstraction over malgo or simulation audio capture and
// playback subsystem.
type audioContext interface {
	initPlayback(deviceID DeviceID, cb dataProc) (playbackDevice, error)
	initCapture(deviceID DeviceID, cb dataProc) (captureDevice, error)
	free() error
	name() string

	newEncoder(sampleRate, channels int) (streamEncoder, error)
	newDecoder(sampleRate, channels int) (streamDecoder, error)
}

// playbackDevice lists the calls needed for a playback device (malgo or sim).
type playbackDevice interface {
	Start() error
	Stop() error
	Uninit()
}

// playbackDevice lists the calls needed for a capture device (malgo or sim).
type captureDevice interface {
	Start() error
	Stop() error
	Uninit()
}

// newAudioContext is set by an initer in either malgoaudio.go or noaudio.go
// depending on the build config.
var newAudioContext = func() (audioContext, error) {
	return nil, errors.New("no audio context initer set")
}
