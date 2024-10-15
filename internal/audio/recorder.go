//go:build cgo && !noaudio

package audio

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/companyzero/gopus"
	"github.com/decred/slog"
	"github.com/gen2brain/malgo"
	"golang.org/x/sync/errgroup"
)

// rawFormat needs to be agreed upon between capture()/playback()
var rawFormat = malgo.FormatS16

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
	malgoCtx *malgo.AllocatedContext
	log      slog.Logger

	int16Buffers sync.Pool
	bytesBuffers sync.Pool
	encodeChan   chan []int16
	playbackChan chan []byte

	mtx              sync.Mutex
	stop             func()
	captureDeviceID  malgo.DeviceID
	playbackDeviceID malgo.DeviceID
	recording        bool
	playing          bool
	opusOutput       [][]byte
	recInfo          RecordInfo
}

func NewRecorder(log slog.Logger) (*NoteRecorder, error) {
	malgoCtx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, err
	}

	sampleCount := sampleRate / 1000 * periodSizeMS

	return &NoteRecorder{
		log:      log,
		malgoCtx: malgoCtx,
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
	if err := ar.malgoCtx.Uninit(); err != nil {
		return err
	}
	ar.malgoCtx.Free()
	return nil
}

// SetCaptureDevice sets the capture device to use for recording. If nil, uses
// the default device.
func (ar *NoteRecorder) SetCaptureDevice(dev *Device) error {
	ar.mtx.Lock()
	defer ar.mtx.Unlock()

	if ar.recording {
		return errors.New("cannot change capture device while recording")
	}

	if dev == nil {
		ar.captureDeviceID = malgo.DeviceID{}
	} else {
		copy(ar.captureDeviceID[:], dev.ID)
	}

	return nil
}

// SetPlaybackDevice sets the playback device to use for playing. If nil, uses
// the default device.
func (ar *NoteRecorder) SetPlaybackDevice(dev *Device) error {
	ar.mtx.Lock()
	defer ar.mtx.Unlock()

	if ar.playing {
		return errors.New("cannot change playback device while playing")
	}

	if dev == nil {
		ar.playbackDeviceID = malgo.DeviceID{}
	} else {
		copy(ar.playbackDeviceID[:], dev.ID)
	}

	return nil
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

// captureLoop captures raw samples from the capture device and sends them
// to the encoding loop.
func (ar *NoteRecorder) captureLoop(ctx context.Context) error {
	var emptyDeviceID malgo.DeviceID

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.SampleRate = sampleRate
	deviceConfig.PeriodSizeInMilliseconds = periodSizeMS
	if ar.captureDeviceID != emptyDeviceID {
		deviceConfig.Capture.DeviceID = ar.captureDeviceID.Pointer()
	}
	deviceConfig.Capture.Format = rawFormat
	deviceConfig.Capture.Channels = channels
	deviceConfig.Alsa.NoMMap = 1 // Needed for capture?

	sampleSizeInBytes := uint32(malgo.SampleSizeInBytes(rawFormat))

	sendingDone := make(chan struct{})

	var inFrames, inSize int

	onRecvFrames := func(_, inSamples []byte, framecount uint32) {
		readSize := int(framecount * sampleSizeInBytes)
		if len(inSamples) < readSize {
			ar.log.Warnf("inSamples buffer has len %d when expected %d",
				len(inSamples), readSize)
			readSize = len(inSamples)
		}
		buf := ar.int16Buffers.Get().([]int16)
		samples := bytesToLES16Slice(inSamples[:readSize], buf)

		inFrames += 1
		inSize += len(inSamples)

		// Double check sending hasn't finished first.
		select {
		case <-sendingDone:
			return
		default:
		}

		// Send to encode loop.
		select {
		case ar.encodeChan <- samples:
		case <-sendingDone:
		}
	}

	captureCallbacks := malgo.DeviceCallbacks{
		Data: onRecvFrames,
	}
	device, err := malgo.InitDevice(ar.malgoCtx.Context, deviceConfig, captureCallbacks)
	if err != nil {
		return err
	}

	ar.log.Debug("Starting to capture raw samples")
	if err := device.Start(); err != nil {
		return err
	}

	<-ctx.Done()
	if err := device.Stop(); err != nil {
		return err
	}
	device.Uninit()

	// Wait for some time for any outstanding callback to be executed.
	time.Sleep(time.Millisecond * time.Duration(periodSizeMS) * 2)

	// Signal the encoding loop that all data has been captured.
	close(sendingDone)
	ar.encodeChan <- nil

	ar.log.Debugf("Finished capturing loop: %d frames, %d bytes", inFrames,
		inSize)

	if inFrames == 0 {
		return errors.New("captured no data")
	}

	return nil
}

// encodeLoop opus-encodes raw samples from the recording loop.
func (ar *NoteRecorder) encodeLoop(ctx context.Context) error {
	encoder, err := gopus.NewEncoder(sampleRate, channels, gopus.Voip)
	if err != nil {
		return fmt.Errorf("gopus.NewEcoder: %v", err)
	}

	encoder.SetBitrate(encodeBitRate)
	const samplesPerChannel = sampleRate / 1000 * periodSizeMS

	ar.log.Debug("Starting encoding loop")

	var encodeBuffer = make([]byte, 1024*1024)
	var opusPackets [][]byte
	var encodedSize, inputSize, inputSamples int

	for samples := range ar.encodeChan {
		if samples == nil {
			break
		}

		if len(samples) != samplesPerChannel {
			ar.log.Warnf("Wrong len of samples to encode "+
				"(want %d, got %d)", samplesPerChannel,
				len(samples))
		}
		encoded, err := encoder.Encode(samples, len(samples), encodeBuffer)
		if err != nil {
			return err
		}

		encoded = append([]byte(nil), encoded...) // Copy bytes from encodeBuffer.
		opusPackets = append(opusPackets, encoded)

		inputSamples += len(samples)
		inputSize += len(samples) * 2
		encodedSize += len(encoded)

		ar.int16Buffers.Put(samples[:0])
	}

	// Done!
	ar.log.Debugf("Finished encoding loop: %d samples "+
		"(%d in bytes), %d opus packets (%d out size)",
		inputSamples, inputSize, len(opusPackets),
		encodedSize)

	ar.mtx.Lock()
	ar.opusOutput = opusPackets
	ar.recInfo = RecordInfo{
		SampleCount: inputSamples,
		DurationMs:  len(opusPackets) * periodSizeMS,
		EncodedSize: encodedSize,
		PacketCount: len(opusPackets),
	}
	ar.mtx.Unlock()
	return nil
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

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return ar.captureLoop(gctx) })
	g.Go(func() error { return ar.encodeLoop(gctx) })

	ar.mtx.Unlock()

	err := g.Wait()

	ar.mtx.Lock()
	ar.recording = false
	ar.mtx.Unlock()

	ar.log.Infof("Finished audio capture. Captured for %s", time.Since(start))
	return err
}

// decodeLoop decodes opus-encoded packets and sends them to the playback loop.
func (ar *NoteRecorder) decodeLoop(ctx context.Context) error {
	decoder, err := gopus.NewDecoder(sampleRate, channels)
	if err != nil {
		return fmt.Errorf("gopus.NewDecoder: %v", err)
	}

	// Must be agreed upon.
	const frameSize = sampleRate / 1000 * periodSizeMS

	ar.mtx.Lock()
	opusFrames := ar.opusOutput
	ar.mtx.Unlock()

	ar.log.Debugf("Starting decode loop")

	var inSize, outSize, outSamples int
	var decodeBuffer = make([]int16, frameSize*channels*2)

	for i := 0; i < len(opusFrames); i++ {
		decoded, err := decoder.Decode(opusFrames[i], frameSize, false, decodeBuffer)
		if err != nil {
			return err
		}

		samples := ar.bytesBuffers.Get().([]byte)
		samples = leS16SliceToBytes(decoded, samples)

		inSize += len(opusFrames[i])
		outSize += len(samples)
		outSamples += len(decoded)

		select {
		case <-ctx.Done():
			// Early return.
			i = len(opusFrames)
		case ar.playbackChan <- samples:
		}
	}

	// Send an empty message to signal that we finished decoding.
	ar.playbackChan <- nil
	ar.log.Debugf("Finished decoder loop with %d in packets (%d bytes), "+
		"%d samples (%d bytes)", len(opusFrames), inSize, outSamples,
		outSize)
	return nil
}

// playbackLoop plays samples deocded from the decode loop in the playback
// device.
func (ar *NoteRecorder) playbackLoop(ctx context.Context) error {
	var emptyDeviceID malgo.DeviceID

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	if ar.playbackDeviceID != emptyDeviceID {
		deviceConfig.Playback.DeviceID = ar.playbackDeviceID.Pointer()
	}
	deviceConfig.PeriodSizeInMilliseconds = periodSizeMS
	deviceConfig.SampleRate = sampleRate
	deviceConfig.Playback.Format = rawFormat
	deviceConfig.Playback.Channels = channels
	deviceConfig.Alsa.NoMMap = 1

	sampleSizeInBytes := uint32(malgo.SampleSizeInBytes(rawFormat))
	playbackDone := make(chan struct{})

	var samples []byte

	ar.log.Debugf("Starting playback loop")

	var cbCount, inPackets, inSize, outSize int

	onSendFrames := func(outSample, _ []byte, framecount uint32) {
		// How many bytes to read in this callback.
		bytesToRead := int(framecount * channels * sampleSizeInBytes)
		if len(outSample) < bytesToRead {
			ar.log.Warnf("Buffer size %d is smaller than read size %d",
				len(outSample), bytesToRead)
			bytesToRead = len(outSample)
		}

		cbCount += 1

		for bytesToRead > 0 {
			// Fetch samples if needed.
			if len(samples) == 0 {
				select {
				case <-playbackDone:
					return
				case <-ctx.Done():
					return
				case samples = <-ar.playbackChan:
					if samples == nil {
						close(playbackDone)
						return
					}

					inPackets += 1
					inSize += len(samples)
				default:
				}
			}

			if len(samples) >= bytesToRead {
				// Remaining samples are sufficient.
				copy(outSample, samples[:bytesToRead])
				outSize += bytesToRead
				if len(samples) == bytesToRead {
					samples = nil
				} else {
					samples = samples[bytesToRead:]
				}
				return
			}

			// Need more decoded packets.
			copy(outSample, samples)
			outSize += len(samples)
			bytesToRead -= len(samples)
			outSample = outSample[len(samples):]
			samples = nil
		}

		ar.bytesBuffers.Put(samples[:0])
	}

	playbackCallbacks := malgo.DeviceCallbacks{
		Data: onSendFrames,
	}

	device, err := malgo.InitDevice(ar.malgoCtx.Context, deviceConfig, playbackCallbacks)
	if err != nil {
		return err
	}

	err = device.Start()
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		// Stop playback immediately.
		device.Uninit()

		// Drain encoder channel.
		for drained := false; !drained; {
			select {
			case <-playbackDone:
				drained = true
			case s := <-ar.playbackChan:
				drained = s == nil
			}
		}

		return ctx.Err()
	case <-playbackDone:
		time.Sleep(time.Millisecond * periodSizeMS)
	}

	ar.log.Debugf("Finished playback loop with %d callbacks, %d in "+
		"packets, %d in bytes, %d out bytes", cbCount, inPackets,
		inSize, outSize)

	device.Uninit()
	return nil
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

	ar.log.Infof("Starting playback")
	start := time.Now()

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return ar.decodeLoop(gctx) })
	g.Go(func() error { return ar.playbackLoop(gctx) })

	ar.mtx.Unlock()

	err := g.Wait()

	ar.log.Infof("Finished playback. Played for %s", time.Since(start))

	ar.mtx.Lock()
	ar.playing = false
	ar.mtx.Unlock()

	return err

}
