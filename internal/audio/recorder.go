package audio

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/decred/slog"
)

// sampleRate must be agreed everywhere
const sampleRate = 48000

// channels must be agreed everywhere
const channels = 1

// periodSizeMS is the captured frame size in milliseconds
const periodSizeMS = 20

// encodeBitRate is the bitrate (in bps) to use as encoder output.
const encodeBitRate = 40000

// NoteRecorder can record and playback audio notes.
type NoteRecorder struct {
	audioCtx audioContext
	log      slog.Logger

	int16Buffers sync.Pool
	bytesBuffers sync.Pool
	encodeChan   chan []int16
	playbackChan chan []byte

	mtx              sync.Mutex
	stop             func()
	captureDeviceID  DeviceID
	playbackDeviceID DeviceID
	recording        bool
	playing          bool
	opusOutput       [][]byte
	recInfo          RecordInfo
	capGain          float64
	playGain         float64
}

func NewRecorder(log slog.Logger) (*NoteRecorder, error) {
	audioCtx, err := newAudioContext()
	if err != nil {
		return nil, err
	}

	sampleCount := sampleRate / 1000 * periodSizeMS

	if addDebugTrace {
		log.Infof("Initializing audio recorder with driver %s WITH DEBUG TRACE",
			audioCtx.name())
	} else {
		log.Infof("Initializing audio recorder with driver %s",
			audioCtx.name())
	}

	return &NoteRecorder{
		log:      log,
		audioCtx: audioCtx,
		int16Buffers: sync.Pool{New: func() interface{} {
			return make([]int16, 0, sampleCount)
		}},
		bytesBuffers: sync.Pool{New: func() interface{} {
			return make([]byte, 0, sampleCount*2)
		}},
		encodeChan:   make(chan []int16, 1000/periodSizeMS), // Buffer 1 second
		playbackChan: make(chan []byte, 1000/periodSizeMS),
	}, nil
}

// FreeContext releases all resources.
func (ar *NoteRecorder) FreeContext() error {
	return ar.audioCtx.free()
}

// SetCaptureDevice sets the capture device to use for recording. If nil, uses
// the default device.
func (ar *NoteRecorder) SetCaptureDevice(devID DeviceID) error {
	ar.mtx.Lock()
	defer ar.mtx.Unlock()

	if ar.recording {
		return errors.New("cannot change capture device while recording")
	}

	ar.captureDeviceID = devID
	ar.log.Infof("Setting capturing device to %q", devID)

	return nil
}

// CaptureDeviceID returns the ID of the device used for capturing mic data. If
// empty, the system-wide default device is used.
func (ar *NoteRecorder) CaptureDeviceID() DeviceID {
	ar.mtx.Lock()
	res := ar.captureDeviceID
	ar.mtx.Unlock()
	return res
}

// SetPlaybackDevice sets the playback device to use for playing. If nil, uses
// the default device.
func (ar *NoteRecorder) SetPlaybackDevice(devID DeviceID) error {
	ar.mtx.Lock()
	defer ar.mtx.Unlock()

	if ar.playing {
		return errors.New("cannot change playback device while playing")
	}

	ar.playbackDeviceID = devID
	ar.log.Infof("Setting playback device to %q", devID)
	return nil
}

// PlaybackDeviceID returns the ID of the device used for playing back audio
// data. If empty, the system-wide default device is used.
func (ar *NoteRecorder) PlaybackDeviceID() DeviceID {
	ar.mtx.Lock()
	res := ar.playbackDeviceID
	ar.mtx.Unlock()
	return res
}

// SetCaptureGain sets the capture gain for captures. This only applies to new
// capture streams.
func (ar *NoteRecorder) SetCaptureGain(gain float64) {
	ar.mtx.Lock()
	ar.capGain = gain
	ar.mtx.Unlock()
}

// GetCaptureGain returns the currently set capture gain.
func (ar *NoteRecorder) GetCaptureGain() float64 {
	ar.mtx.Lock()
	res := ar.capGain
	ar.mtx.Unlock()
	return res
}

// SetPlaybackGain sets the global playback gain. This is added to the
// per-stream playback gain.
func (ar *NoteRecorder) SetPlaybackGain(gain float64) {
	ar.mtx.Lock()
	ar.playGain = gain
	ar.mtx.Unlock()
}

// GetPlaybackGain returns the global playback gain.
func (ar *NoteRecorder) GetPlaybackGain() float64 {
	ar.mtx.Lock()
	res := ar.playGain
	ar.mtx.Unlock()
	return res
}

// Busy returns the state of the recorder.
func (ar *NoteRecorder) Busy() (recording bool, playing bool) {
	ar.mtx.Lock()
	recording = ar.recording
	playing = ar.playing
	ar.mtx.Unlock()
	return
}

// Stop the current operation (record or playback).
func (ar *NoteRecorder) Stop() {
	ar.mtx.Lock()
	stop := ar.stop
	ar.stop = nil
	ar.mtx.Unlock()
	if stop != nil {
		ar.log.Infof("Stopping activity")
		stop()
	}
}

// HasRecorded returns whether there's a recorded note.
func (ar *NoteRecorder) HasRecorded() bool {
	ar.mtx.Lock()
	res := len(ar.opusOutput) > 0
	ar.mtx.Unlock()
	return res
}

// RecordInfo returns information about the latest recording.
func (ar *NoteRecorder) RecordInfo() RecordInfo {
	ar.mtx.Lock()
	res := ar.recInfo
	ar.mtx.Unlock()
	return res
}

// OpusFile encodes the recorded audio note as an opusfile (a .ogg file
// with opus-encoded audio data).
func (ar *NoteRecorder) OpusFile() ([]byte, error) {
	ar.mtx.Lock()
	opusFrames := ar.opusOutput
	ar.mtx.Unlock()

	if len(opusFrames) == 0 {
		return nil, errors.New("no data to encode")
	}

	buf := bytes.NewBuffer(nil)
	w, err := newOpusWriter(buf)
	if err != nil {
		return nil, err
	}

	pcmSamplesPerOpusPkt := sampleRate / 1000 * periodSizeMS
	for i := range opusFrames {
		isLast := i == len(opusFrames)-1
		err := w.WritePacket(opusFrames[i], uint64(pcmSamplesPerOpusPkt), isLast)
		if err != nil {
			return nil, err
		}
	}

	ar.log.Debugf("Opusfile wrote %d pages, %d PCM samples per pkt, %d total bytes\n",
		len(opusFrames), pcmSamplesPerOpusPkt, buf.Len())

	return buf.Bytes(), nil
}

// Capture audio data until the context is canceled or Stop() is called.
func (ar *NoteRecorder) Capture(ctx context.Context) error {
	ar.mtx.Lock()
	if ar.recording {
		ar.mtx.Unlock()
		return errors.New("already recording")
	}
	if ar.playing {
		ar.mtx.Unlock()
		return errors.New("cannot record while playing back")
	}

	ctx, ar.stop = context.WithCancel(ctx)
	ar.recording = true

	start := time.Now()
	ar.log.Infof("Starting to capture audio")

	// Init a capture stream that stores the full output in memory.
	var opusPackets [][]byte
	encodedFunc := func(ctx context.Context, data []byte, timestamp uint32) error {
		// Store a copy of the encoded data.
		opusPackets = append(opusPackets, append([]byte(nil), data...))
		return nil
	}
	cs := streamCaptureOpusFrames(ctx, ar.audioCtx, ar.captureDeviceID,
		encodedFunc, ar.capGain, ar.log)
	ar.mtx.Unlock()

	// Wait until capture finishes.
	<-cs.CaptureDone()

	// Store result.
	ar.mtx.Lock()
	ar.recInfo = cs.RecordInfo()
	ar.opusOutput = opusPackets
	ar.recording = false
	ar.mtx.Unlock()

	ar.log.Infof("Finished audio capture. Captured for %s", time.Since(start))
	return cs.Err()
}

// Playback the recorded audio until it ends or the context is canceled or
// Stop() is called.
func (ar *NoteRecorder) Playback(ctx context.Context) error {
	ar.mtx.Lock()
	if ar.recording {
		ar.mtx.Unlock()
		return errors.New("cannot play while recording")
	}
	if ar.playing {
		ar.mtx.Unlock()
		return errors.New("already playing")
	}
	if len(ar.opusOutput) == 0 {
		ar.mtx.Unlock()
		return errors.New("no recorded audio note")
	}

	ctx, ar.stop = context.WithCancel(ctx)
	ar.playing = true

	// Init playback stream with a copy of the opus packets.
	ar.log.Infof("Starting playback (%d opus frames to playback)", len(ar.opusOutput))
	ps := playbackOpusFrames(ctx, ar.audioCtx, ar.playbackDeviceID,
		ar.playGain, ar.opusOutput[:], ar.log)
	start := time.Now()
	ar.mtx.Unlock()

	// Wait until playback finishes or is canceled.
	<-ps.PlaybackDone()

	ar.log.Infof("Finished playback. Played for %s", time.Since(start))
	ar.mtx.Lock()
	ar.playing = false
	ar.mtx.Unlock()

	return ps.Err()
}

// CaptureStream runs a new capture stream, sending data to the callback. This
// capture stream is independent of other operations.
func (ar *NoteRecorder) CaptureStream(ctx context.Context, f EncodedCapturedFunc) (*CaptureStream, error) {
	ar.mtx.Lock()
	cs := streamCaptureOpusFrames(ctx, ar.audioCtx, ar.captureDeviceID, f,
		ar.capGain, ar.log)
	ar.mtx.Unlock()
	return cs, nil
}

// PlaybackStream creates a new playback stream. This playback stream is
// independent of other operations.
func (ar *NoteRecorder) PlaybackStream(ctx context.Context, soundStateChanged func(bool)) *PlaybackStream {
	ar.mtx.Lock()
	ps := streamPlaybackOpusFrames(ctx, ar.audioCtx, ar.playbackDeviceID,
		soundStateChanged, ar.log)
	playGain := ar.playGain
	ar.mtx.Unlock()

	if playGain != 0 {
		ps.SetVolumeGain(playGain)
	}
	return ps

}
