package audio

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/decred/slog"
	"golang.org/x/sync/errgroup"
)

// minPlaybackBufferPackets is how many packets to buffer before starting
// playback of a stream.
const minPlaybackBufferPackets = 5

// EncodedCapturedFunc is the signature for the callback function that processes
// captured and opus-encoded packets.
type EncodedCapturedFunc func(ctx context.Context, data []byte, timestamp uint32) error

// CaptureStream captures data from an input device for some time.
type CaptureStream struct {
	audioCtx       audioContext
	log            slog.Logger
	deviceID       DeviceID
	int16Buffers   sync.Pool
	encodeChan     chan []int16
	volumeGainChan chan float64
	recInfo        RecordInfo
	encodedFunc    EncodedCapturedFunc
	captureDone    chan struct{}
	stopChan       chan struct{}
	runErr         error
}

// RecordInfo is the information about the finished recording.
func (cs *CaptureStream) RecordInfo() RecordInfo {
	select {
	case <-cs.captureDone:
		return cs.recInfo
	default:
		return RecordInfo{}
	}
}

// CaptureDone is closed once capturing is completed.
func (cs *CaptureStream) CaptureDone() <-chan struct{} {
	return cs.captureDone
}

// Stop stops the capture stream independently of the run context stopping.
func (cs *CaptureStream) Stop() {
	select {
	case cs.stopChan <- struct{}{}:
	case <-cs.captureDone:
	}
}

// Err is the capturing error. It is only set after capturing is done.
func (cs *CaptureStream) Err() error {
	select {
	case <-cs.captureDone:
		return cs.runErr
	default:
		return nil
	}
}

// captureLoop loops capturing data from an audio context and sends it to be
// encoded in encodeLoop.
func (cs *CaptureStream) captureLoop(ctx context.Context) error {
	sendingDone := make(chan struct{})

	var inFrames, inSize int

	cs.log.Debug("Starting capture loop")

	onRecvFrames := func(_, inSamples []byte, framecount uint32) {
		readSize := int(framecount * rawFormatSampleSize)
		if len(inSamples) < readSize {
			cs.log.Warnf("inSamples buffer has len %d when expected %d",
				len(inSamples), readSize)
			readSize = len(inSamples)
		}
		buf := cs.int16Buffers.Get().([]int16)
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
		case cs.encodeChan <- samples:
		case <-sendingDone:
		}
	}

	device, err := cs.audioCtx.initCapture(cs.deviceID, onRecvFrames)
	if err != nil {
		return err
	}

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
	cs.encodeChan <- nil

	if inFrames == 0 {
		return errors.New("captured no data")
	}

	cs.log.Debug("Finished capture loop")
	return nil
}

// encodeLoop opus-encodes raw samples from the recording loop.
func (cs *CaptureStream) encodeLoop(ctx context.Context, initialVolGain float64) error {
	encoder, err := cs.audioCtx.newEncoder(sampleRate, channels)
	if err != nil {
		return fmt.Errorf("newEcoder: %v", err)
	}

	cs.log.Debug("Starting encode loop")

	encoder.SetBitrate(encodeBitRate)
	const samplesPerChannel = sampleRate / 1000 * periodSizeMS

	var encodeBuffer = make([]byte, 1024*1024)
	var encodedSize, inputSize, inputSamples, packetCount int

	var volumeGain float64 = initialVolGain

	var timestamp uint32

nextPacket:
	for {
		var samples []int16
		select {
		case samples = <-cs.encodeChan:
			if samples == nil {
				break nextPacket
			}

		case newGain := <-cs.volumeGainChan:
			cs.log.Debugf("Changing capture volume gain to %.2f", newGain)
			volumeGain = newGain
			continue nextPacket
		}

		if volumeGain != 0 {
			applyGainDB(samples, volumeGain)
		}

		if len(samples) != samplesPerChannel {
			cs.log.Warnf("Wrong len of samples to encode "+
				"(want %d, got %d)", samplesPerChannel,
				len(samples))
		}
		encoded, err := encoder.Encode(samples, len(samples), encodeBuffer)
		if err != nil {
			return err
		}

		// Debug log commented out for performance reasons.
		// cs.log.Tracef("Encoded packet of size %d ts %d", len(encoded), timestamp)

		if err := cs.encodedFunc(ctx, encoded, timestamp); err != nil {
			return err
		}

		timestamp += periodSizeMS
		packetCount++
		inputSamples += len(samples)
		inputSize += len(samples) * 2
		encodedSize += len(encoded)

		cs.int16Buffers.Put(samples[:0])
	}

	// Done!
	cs.log.Debugf("Finished encoding loop: %d samples "+
		"(%d in bytes), %d opus packets (%d out size)",
		inputSamples, inputSize, packetCount,
		encodedSize)

	cs.recInfo = RecordInfo{
		SampleCount: inputSamples,
		DurationMs:  packetCount * periodSizeMS,
		EncodedSize: encodedSize,
		PacketCount: packetCount,
	}
	return nil
}

func (cs *CaptureStream) run(ctx context.Context, initialVolGain float64) {
	ctx, cancel := context.WithCancel(ctx)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return cs.encodeLoop(gctx, initialVolGain) })
	g.Go(func() error { return cs.captureLoop(gctx) })
	g.Go(func() error {
		select {
		case <-gctx.Done():
		case <-cs.stopChan:
		}
		cancel()
		return nil
	})
	cs.runErr = g.Wait()
	close(cs.captureDone)
}

// SetVolumeGain sets the volume gain for captured samples. The new gain is
// specified in dB.
func (cs *CaptureStream) SetVolumeGain(gainDB float64) {
	select {
	case cs.volumeGainChan <- gainDB:
	case <-cs.captureDone:
	}
}

// streamCaptureOpusFrames runs a new capturing stream.
func streamCaptureOpusFrames(ctx context.Context, audioCtx audioContext,
	deviceID DeviceID, f EncodedCapturedFunc, initialVolGain float64, log slog.Logger) *CaptureStream {

	sampleCount := sampleRate / 1000 * periodSizeMS
	cs := &CaptureStream{
		encodeChan:     make(chan []int16),
		captureDone:    make(chan struct{}),
		stopChan:       make(chan struct{}, 1),
		volumeGainChan: make(chan float64, 1),
		log:            log,
		audioCtx:       audioCtx,
		deviceID:       deviceID,
		int16Buffers: sync.Pool{New: func() interface{} {
			return make([]int16, 0, sampleCount)
		}},
		encodedFunc: f,
	}

	go cs.run(ctx, initialVolGain)
	return cs
}

// inputPacket tracks an individual packet and timestamp.
type inputPacket struct {
	data     []byte
	ts       uint32
	dataDone bool
	hasSound bool
}

// PlaybackStream plays back opus-encoded data.
type PlaybackStream struct {
	log               slog.Logger
	inputChan         chan inputPacket
	inputDone         chan struct{}
	playbackDone      chan struct{}
	playbackChan      chan inputPacket
	stallChan         chan uint32
	volumeGainChan    chan float64
	runErr            error
	bytesBuffers      sync.Pool
	playbackDeviceID  DeviceID
	changeDeviceChan  chan DeviceID
	audioCtx          audioContext
	soundStateChanged func(bool)
}

// MarkInputDone signals that input data for playback in this stream is done.
func (ps *PlaybackStream) MarkInputDone(ctx context.Context) {
	select {
	case ps.inputChan <- inputPacket{dataDone: true}:
	case <-ctx.Done():
	case <-ps.playbackDone:
	}
}

// PlaybackDone is closed when playback of this stream is finished or canceled.
func (ps *PlaybackStream) PlaybackDone() <-chan struct{} {
	return ps.playbackDone
}

// ChangePlaybackDevice changes the playback device of this stream to the given
// one.
func (ps *PlaybackStream) ChangePlaybackDevice(devID DeviceID) {
	select {
	case ps.changeDeviceChan <- devID:
	case <-ps.playbackDone:
	}
}

// Input data into the playback stream. Data should be an opus-encoded packet
// and the timestamp should be following the standard periodSizeMS period.
//
// Note: If the input buffer is full, this drops the packet.
func (ps *PlaybackStream) Input(data []byte, ts uint32) {
	// Copy the input data to an internal bytes buffer.
	buf := ps.bytesBuffers.Get().([]byte)
	buf = append(buf, data...)
	packet := inputPacket{data: buf, ts: ts}

	// Send it to decodeLoop().
	select {
	case ps.inputChan <- packet:
	default:
		// Stall! Audio decoder stuck or too many input packets.
		ps.log.Warnf("Input channel is full when attempting to send packet ts %d", ts)
	}
}

// inputBlocking inputs a playback packet but blocks if the input buffer is
// full.  This is used when playing back recordings.
func (ps *PlaybackStream) inputBlocking(ctx context.Context, data []byte, ts uint32) {
	// Copy the input data to an internal bytes buffer.
	buf := ps.bytesBuffers.Get().([]byte)
	buf = append(buf, data...)
	packet := inputPacket{data: buf, ts: ts}

	// Send it to decodeLoop().
	select {
	case ps.inputChan <- packet:
	case <-ctx.Done():
	}
}

// Err returns the playback error. It is only set after playback is done.
func (ps *PlaybackStream) Err() error {
	select {
	case <-ps.playbackDone:
		return ps.runErr
	default:
		return nil
	}
}

// decodeLoop runs a loop receiving input packets and opus-decoding them.
func (ps *PlaybackStream) decodeLoop(ctx context.Context) error {
	decoder, err := ps.audioCtx.newDecoder(sampleRate, channels)
	if err != nil {
		return fmt.Errorf("newDecoder: %v", err)
	}

	// Must be agreed upon.
	const frameSize = sampleRate / 1000 * periodSizeMS

	// Stats.
	var inSize, outSize, outSamples, inPackets int

	// Buffer that receives the results of a decoder.Decode() call.
	var decodeBuffer = make([]int16, frameSize*channels*2)

	var startTime = time.Now()   // Track total decoding time.
	var lastTimestamp uint32 = 0 // Last decoded ts.
	var stallStartTs uint32      // Used to log stats about stalls.

	// Whether playback loop is stalled. This starts as true to ensure the
	// first packet is decoded if it has a timestamp of 0.
	var stalled bool = true

	// fillQueue is set to true when the decode queue needs to be filled
	// with some packets before starting to decode again. This happens at
	// startup and if playback stalled and no data was available in
	// decodeQueue.
	var fillQueue bool = true

	// Adaptative buffer length. Starts by buffering a small number of
	// packets, but every time a stall occurs (dropped or delayed packets),
	// it doubles the size of buffering done for the next round (up to a
	// maximum).
	var stallTargetQLen int = minPlaybackBufferPackets
	const maxTargetQLen = 1000 / periodSizeMS // 1 second.

	var volumeGain float64 = 0

	decodeQueue := newTsBufferQueue(20)

nextFrame:
	for {
		// Take next action.
		select {
		case newGain := <-ps.volumeGainChan:
			ps.log.Debugf("Changing volume gain to %.2f", newGain)
			volumeGain = newGain
			continue nextFrame

		case input := <-ps.inputChan:
			if input.dataDone {
				ps.log.Tracef("Playback input marked done")
				break nextFrame
			}

			// Reject when this is an old timestamp.
			if lastTimestamp > 0 && input.ts <= lastTimestamp {
				ps.bytesBuffers.Put(input.data[:0])
				ps.log.Tracef("Rejecting to decode packet with "+
					"timestamp %d due to last timestamp %d",
					input.ts, lastTimestamp)
				continue nextFrame
			}

			if addDebugTrace {
				if decodeQueue.len() == 0 && input.ts > lastTimestamp+periodSizeMS {
					ps.log.Debugf("Got packet with ts %d qlen %d inlen %d playlen %d ahead of time (wanted %d)",
						input.ts, decodeQueue.len(), len(ps.inputChan),
						len(ps.playbackChan), lastTimestamp+periodSizeMS)
				} else {
					ps.log.Tracef("Got input ts %d qlen %d inlen %d playlen %d",
						input.ts, decodeQueue.len(), len(ps.inputChan),
						len(ps.playbackChan))
				}
			}

			// Put the received packet into the sorted queue.
			decodeQueue.enq(input.data, input.ts)
			inPackets++

			// If we're waiting for a full queue before proceeding,
			// wait for the next action.
			if fillQueue && decodeQueue.len() < stallTargetQLen {
				if addDebugTrace {
					ps.log.Debugf("Filling buffer after stallts %d lastts %d qlen %d target %d",
						stallStartTs, lastTimestamp, decodeQueue.len(),
						stallTargetQLen)
				}
				continue nextFrame
			}

		case stallTs := <-ps.stallChan:
			// Playback stalled waiting for a packet.
			stalled = true
			if stallStartTs == 0 {
				// ps.log.Tracef("Started stalling decoding "+
				//	"stall ts %d last ts %d qlen %d",
				//	stallTs, lastTimestamp, decodeQueue.len())
				stallStartTs = stallTs

				// When the decoding queue is empty, this is a
				// trigger to wait for a full queue before
				// resuming playback (maybe the network lost a
				// lot of packets or it has degraded its
				// conditions) or the remote stopped
				// transmitting altogether.
				if decodeQueue.len() == 0 {
					fillQueue = true

					// Reset the timestamp. This handles
					// cases where the remote will rewind
					// their stream timestamps after
					// returning.
					lastTimestamp = 0

					// Take it as hint to increase the
					// adaptive queue size.
					stallTargetQLen = min(stallTargetQLen*2, maxTargetQLen)
					continue nextFrame
				}
			} else if ((stallTs - stallStartTs) % 1000) == 0 {
				if addDebugTrace {
					ps.log.Debugf("Still stalling decode stream at "+
						"stall ts %d last ts %d", stallTs, lastTimestamp)
				}
			}

		case <-ctx.Done():
			break nextFrame
		}

		// Decode and send to playback every packet, as long as they
		// are in order (the first packet in the queue is the next
		// expected one).
		//
		// We stop early in case the packets are NOT ordered to give it
		// a chance for the packet to arrive.
		for (decodeQueue.len() > 0 && decodeQueue.firstTs() == lastTimestamp+periodSizeMS) || stalled {
			var opusFrame []byte
			var ts uint32
			var dequeued bool
			var fec bool

			// If playback is stalled and missing just one packet
			// (likely lost to the network), then generate a FEC
			// packet by passing a nil packet to Decode().
			//
			// Otherwise, use whatever we have that is waiting for
			// decoding.
			if stalled && decodeQueue.firstTs() == lastTimestamp+periodSizeMS*2 {
				if addDebugTrace {
					ps.log.Debugf("Generating fec packet lastts %d firstts %d",
						lastTimestamp, decodeQueue.firstTs())
				}
				ts = lastTimestamp + periodSizeMS
				lastTimestamp = ts
				fec = true
			} else {
				opusFrame, ts, dequeued = decodeQueue.deq()
			}

			// Do the actual stateful decoding.
			decoded, err := decoder.Decode(opusFrame, frameSize, fec, decodeBuffer)
			if err != nil {
				return err
			}

			// Determine if this has sound above a noise level.
			applyGainDB(decoded, volumeGain)
			hasSound := detectSound(decoded, 500, 5)

			// Convert from 16 bit samples to byte samples. Reuse
			// buffer fetched in Input() if possible.
			var samples []byte
			if opusFrame != nil {
				samples = opusFrame[:0]
			} else {
				samples = ps.bytesBuffers.Get().([]byte)
			}
			samples = leS16SliceToBytes(decoded, samples[:0])

			// Stats.
			inSize += len(opusFrame)
			outSize += len(samples)
			outSamples += len(decoded)

			// Send to playbackLoop.
			select {
			case <-ctx.Done():
				break nextFrame
			case ps.playbackChan <- inputPacket{
				data:     samples,
				ts:       max(lastTimestamp, ts),
				hasSound: hasSound,
			}:
				if addDebugTrace {
					ps.log.Tracef("Pushed ts %d dequed %v", ts, dequeued)
				}
			}

			// If we sent a packet that was dequeued (as opposed to
			// a FEC packet), track that and stop waiting to fill
			// the decode queue (send packets as fast as they are
			// coming in).
			if dequeued {
				lastTimestamp = ts
				if stallStartTs > 0 {
					if addDebugTrace {
						ps.log.Debugf("Stopped stalling at timestamp %d (total %d ms)",
							ts, ts-stallStartTs)
					}
					stallStartTs = 0
				}
				fillQueue = false
			}
			stalled = false
		}
	}

	// Send an empty message to signal that we finished decoding.
	ps.log.Debugf("Playback decoding ended in %s after decoding %d packets (%d in size, %d out size)",
		time.Since(startTime), inPackets, inSize, outSize)
	ps.playbackChan <- inputPacket{data: nil, ts: math.MaxUint32}
	return nil
}

// playbackLoopWithDevice runs a loop that picks up decoded packets and plays
// them on the audio context in the specified device.
func (ps *PlaybackStream) playbackLoopWithDevice(ctx context.Context, devID DeviceID) error {

	// Wait until the first packet has been decoded to init playback.
	for len(ps.playbackChan) == 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(periodSizeMS) * time.Millisecond):
		}
	}
	ps.log.Debugf("Initializing playback after buffering %d decoded packets",
		len(ps.playbackChan))

	playbackDone := make(chan struct{})

	var cbCount, inPackets, inSize int
	var lastTimestamp uint32

	// Track to notify when sound is starting/ending.
	var hasSound bool
	var noSoundCount int

	onSendFrames := func(outSample, _ []byte, framecount uint32) {
		// How many bytes to read in this callback.
		bytesToRead := int(framecount * channels * rawFormatSampleSize)
		if len(outSample) < bytesToRead {
			ps.log.Warnf("Buffer size %d is smaller than read size %d",
				len(outSample), bytesToRead)
			bytesToRead = len(outSample)
		}

		cbCount += 1

		// Fetch next set of samples.
		for {
			var input inputPacket

			select {
			case <-playbackDone:
				return
			case <-ctx.Done():
				return
			case input = <-ps.playbackChan:
				// Fetched next set of samples.
			default:
				// Stall! Request FEC or comfort noise packet
				// from decoder.
				stallTs := lastTimestamp + periodSizeMS
				select {
				case ps.stallChan <- stallTs:
					// decodeLoop ack'd the stall, wait for
					// packet.
					select {
					case input = <-ps.playbackChan:
					case <-ctx.Done():
						return
					}

				case input = <-ps.playbackChan:
					// decodeLoop was in the process of
					// decoding when playbackLoop stalled,
					// accept this input.

				case <-ctx.Done():
					return
				}
			}

			if input.data == nil || input.ts == math.MaxUint32 {
				// Finished playback.
				close(playbackDone)
				ps.log.Debugf("Finished playback after timestamp %d", lastTimestamp)
				return
			}

			// Trigger events when sound is starting/ending.
			switch {
			case !hasSound && input.hasSound:
				hasSound = true
				noSoundCount = 0
				if ps.soundStateChanged != nil {
					ps.soundStateChanged(true)
				}
			case hasSound && !input.hasSound && noSoundCount < 20: // 20 == 400ms
				noSoundCount++
			case hasSound && !input.hasSound:
				hasSound = false
				if ps.soundStateChanged != nil {
					ps.soundStateChanged(false)
				}
			}

			inPackets += 1
			inSize += len(input.data)
			lastTimestamp = input.ts

			if len(input.data) != bytesToRead {
				ps.log.Warnf("Received Samples %d is different than read size %d",
					len(input.data), bytesToRead)
				continue
			}

			// Received a valid set of samples.
			if addDebugTrace {
				ps.log.Tracef("Filling output buffer with data from "+
					"timestamp %d", lastTimestamp)
			}
			copy(outSample, input.data)
			ps.bytesBuffers.Put(input.data[:0])
			return
		}
	}

	device, err := ps.audioCtx.initPlayback(devID, onSendFrames)
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

		return ctx.Err()
	case <-playbackDone:
		time.Sleep(time.Millisecond * periodSizeMS)
	}

	ps.log.Debugf("Finished playback loop with %d callbacks, %d "+
		"packets, %d bytes", cbCount, inPackets, inSize)

	device.Uninit()
	return nil
}

// playbackLoop runs the playback loop with variable devices. It starts with
// the device defined during stream init, but allows modifying the device by
// re-running playbackLoopWithDevice.
func (ps *PlaybackStream) playbackLoop(ctx context.Context) error {
	devID := ps.playbackDeviceID
	errChan := make(chan error, 1)
	for ctx.Err() == nil {
		playDevCtx, cancel := context.WithCancel(ctx)
		go func() { errChan <- ps.playbackLoopWithDevice(playDevCtx, devID) }()

		select {
		case err := <-errChan:
			cancel()
			if ctx.Err() == nil {
				// An error that did not depend on context
				// being done (i.e. an actual error).
				//
				// This includes err==nil, which happens when
				// MarkInputDone() is called.
				return err
			}

		case <-ctx.Done():
			cancel()

		case devID = <-ps.changeDeviceChan:
			ps.log.Debugf("Changing playback device ID to %q in playback stream", devID)
			cancel()

			// Drain errChan
			select {
			case err := <-errChan:
				if err != nil && !errors.Is(err, context.Canceled) {
					return err
				}
			case <-ctx.Done():
			}
		}
	}
	return ctx.Err()
}

func (ps *PlaybackStream) run(ctx context.Context) {
	ps.log.Debugf("Running new playback loop")

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return ps.decodeLoop(gctx) })
	g.Go(func() error { return ps.playbackLoop(gctx) })
	ps.runErr = g.Wait()
	close(ps.playbackDone)
}

// SetVolumeGain modifies the volume gain of this stream. Gain is expressed
// in dB units.
func (ps *PlaybackStream) SetVolumeGain(gainDB float64) {
	select {
	case ps.volumeGainChan <- gainDB:
	case <-ps.playbackDone:
	}
}

func newPlaybackStream(audioCtx audioContext, deviceID DeviceID) *PlaybackStream {
	sampleCount := sampleRate / 1000 * periodSizeMS
	return &PlaybackStream{
		log:              slog.Disabled,
		audioCtx:         audioCtx,
		playbackDeviceID: deviceID,
		bytesBuffers: sync.Pool{New: func() interface{} {
			return make([]byte, 0, sampleCount*2)
		}},
		playbackChan:     make(chan inputPacket, 1000/periodSizeMS), // Buffer up to 1 second of decoded frames.
		inputChan:        make(chan inputPacket, 1000/periodSizeMS), // Buffer up to 1 second of input frames.
		volumeGainChan:   make(chan float64, 1),
		stallChan:        make(chan uint32),
		inputDone:        make(chan struct{}),
		playbackDone:     make(chan struct{}),
		changeDeviceChan: make(chan DeviceID),
	}
}

// playbackOpusFrames creates and runs a playback stream through a set of opus
// packets (likely a recording).
func playbackOpusFrames(ctx context.Context, audioCtx audioContext,
	deviceID DeviceID, opusFrames [][]byte, log slog.Logger) *PlaybackStream {

	ps := newPlaybackStream(audioCtx, deviceID)
	ps.log = log

	// Input opus frames.
	go func() {
	nextFrame:
		for i, frame := range opusFrames {
			buf := ps.bytesBuffers.Get().([]byte)
			buf = append(buf, frame...)

			ps.inputBlocking(ctx, buf, uint32(i*periodSizeMS))

			if ctx.Err() != nil {
				break nextFrame
			}
		}
		ps.MarkInputDone(ctx)
	}()

	go ps.run(ctx)
	return ps
}

// streamPlaybackOpusFrames runs a playback stream that expects input packets
// to be sent to the Input() method of the stream.
func streamPlaybackOpusFrames(ctx context.Context, audioCtx audioContext,
	deviceID DeviceID, soundStateChanged func(bool), log slog.Logger) *PlaybackStream {

	if log == nil {
		log = slog.Disabled
	}

	ps := newPlaybackStream(audioCtx, deviceID)
	ps.log = log
	ps.soundStateChanged = soundStateChanged
	go ps.run(ctx)
	return ps
}
