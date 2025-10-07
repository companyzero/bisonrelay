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
const minPlaybackBufferPackets = 20

// EncodedCapturedFunc is the signature for the callback function that processes
// captured and opus-encoded packets.
type EncodedCapturedFunc func(ctx context.Context, data []byte, timestamp uint32) error

type capturedPacket struct {
	samples *[]int16
	size    int
}

// CaptureStream captures data from an input device for some time.
type CaptureStream struct {
	audioCtx       audioContext
	log            slog.Logger
	deviceID       DeviceID
	int16Buffers   sync.Pool
	encodeChan     chan capturedPacket
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
		samplesSize := readSize / 2
		samples := cs.int16Buffers.Get().(*[]int16)
		bytesToLES16Slice(inSamples[:readSize], (*samples))

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
		case cs.encodeChan <- capturedPacket{samples: samples, size: samplesSize}:
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
	cs.encodeChan <- capturedPacket{}

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

	var volumeGain = initialVolGain

	var timestamp uint32

nextPacket:
	for {
		var cp capturedPacket
		select {
		case cp = <-cs.encodeChan:
			if cp.samples == nil {
				break nextPacket
			}

		case newGain := <-cs.volumeGainChan:
			cs.log.Debugf("Changing capture volume gain to %.2f", newGain)
			volumeGain = newGain
			continue nextPacket
		}

		samples := (*cp.samples)[:cp.size]
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
		// cs.log.Tracef("Encoded packet of size %d ts %d frameSize %d",
		//	len(encoded), timestamp, cp.size)

		if err := cs.encodedFunc(ctx, encoded, timestamp); err != nil {
			return err
		}

		timestamp += periodSizeMS
		packetCount++
		inputSamples += len(samples)
		inputSize += len(samples) * 2
		encodedSize += len(encoded)

		cs.int16Buffers.Put(cp.samples)
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
		encodeChan:     make(chan capturedPacket),
		captureDone:    make(chan struct{}),
		stopChan:       make(chan struct{}, 1),
		volumeGainChan: make(chan float64, 1),
		log:            log,
		audioCtx:       audioCtx,
		deviceID:       deviceID,
		int16Buffers: sync.Pool{New: func() interface{} {
			b := make([]int16, 0, sampleCount)
			return &b
		}},
		encodedFunc: f,
	}

	go cs.run(ctx, initialVolGain)
	return cs
}

// inputPacket tracks an individual packet and timestamp.
type inputPacket struct {
	data     *[]byte
	size     int
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
	getBufCount       chan chan int
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
	buf := ps.bytesBuffers.Get().(*[]byte)
	bufSlice := *buf
	if len(data) > cap(bufSlice) {
		bufSlice = make([]byte, 0, len(data))
		buf = &bufSlice
	}
	copy(bufSlice[:len(data)], data)
	packet := inputPacket{data: buf, size: len(data), ts: ts}

	// Send it to decodeLoop().
	select {
	case ps.inputChan <- packet:
	default:
		// Stall! Audio decoder stuck or too many input packets.
		ps.log.Warnf("Input channel is full when attempting to send "+
			"packet ts %d (input %d, playback %d)", ts, len(ps.inputChan),
			len(ps.playbackChan))
	}
}

// BufferedCount returns the number of buffered packets.
func (ps *PlaybackStream) BufferedCount() int64 {
	replyChan := make(chan int, 1)
	select {
	case ps.getBufCount <- replyChan:
	case <-ps.playbackDone:
		return 0
	}

	select {
	case res := <-replyChan:
		return int64(res)
	case <-ps.playbackDone:
		return 0
	}
}

// inputBlocking inputs a playback packet but blocks if the input buffer is
// full.  This is used when playing back recordings.
func (ps *PlaybackStream) inputBlocking(ctx context.Context, data []byte, ts uint32) {
	// Copy the input data to an internal bytes buffer.
	buf := ps.bytesBuffers.Get().(*[]byte)
	bufSlice := *buf
	if len(data) > cap(bufSlice) {
		bufSlice = make([]byte, 0, len(data))
		buf = &bufSlice
	}
	copy(bufSlice[:len(data)], data)
	packet := inputPacket{data: buf, size: len(data), ts: ts}

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
func (ps *PlaybackStream) decodeLoop(ctx context.Context) error {
	decoder, err := ps.audioCtx.newDecoder(sampleRate, channels)
	if err != nil {
		return fmt.Errorf("newDecoder: %v", err)
	}

	// Track total decoding time.
	var startTime = time.Now()

	// Must be agreed upon.
	const frameSize = sampleRate / 1000 * periodSizeMS

	// The minimum number of items to maintain in the output queue.
	const minOutqueueTime = 80 * time.Millisecond
	const minOutQueueLen = int(minOutqueueTime/time.Millisecond) / periodSizeMS

	// The maximum amount of packets/time to buffer on the input (undecoded)
	// queue.
	const maxUndecodedInputTime = 800 * time.Millisecond
	const maxUndecodedInputLen = int(maxUndecodedInputTime/time.Millisecond) / periodSizeMS

	// The maximum amount of packets/time to buffer on the output (decoded)
	// queue.
	const maxOutQueueLenTime = 500 * time.Millisecond
	const maxOutQueueLen = int(maxOutQueueLenTime/time.Millisecond) / periodSizeMS

	// The minimum amount to buffer within inQueue, when fillQueue == true,
	// before decoding starts.
	const minFillQueueTime = 400 * time.Millisecond
	const minFillQueueLen = int(minFillQueueTime/time.Millisecond) / periodSizeMS

	// fillQueue is set to true when the decode queue needs to be filled
	// with some packets before starting to decode again. This happens at
	// startup and if playback stalled and no data was available in
	// inQueue.
	var fillQueue = true

	// Stats.
	var inSize, outSize, outSamples, inPackets int

	// Buffer that receives the results of a decoder.Decode() call.
	var decodeBuffer = make([]int16, frameSize*channels*2)

	var lastDecTs uint32

	var volumeGain float64 = 0

	var didFEC bool

	waitWorkTicker := time.NewTicker(periodSizeMS * time.Millisecond)

	inQueue := newTsBufferQueue(maxUndecodedInputLen + cap(ps.inputChan))
	outQueue := newInputPacketQueue(maxOutQueueLen)

nextAction:
	for {
		// Read as many packets as are available for reading.
		for readIn := true; readIn; {
			select {
			case newGain := <-ps.volumeGainChan:
				ps.log.Debugf("Changing volume gain to %.2f", newGain)
				volumeGain = newGain

			case replyChan := <-ps.getBufCount:
				replyChan <- len(ps.inputChan) + len(ps.playbackChan) +
					outQueue.len() + inQueue.len()

			case input := <-ps.inputChan:
				if input.dataDone {
					ps.log.Tracef("Playback input marked done ts %d", input.ts)
					break nextAction
				}

				// Reject when this is an old timestamp.
				if lastDecTs > 0 && input.ts <= lastDecTs {
					ps.bytesBuffers.Put(input.data)
					if addDebugTrace {
						ps.log.Tracef("Rejecting to decode packet with "+
							"timestamp %d due to last timestamp %d",
							input.ts, lastDecTs)
					}
					continue
				}

				inQueue.enq(input.data, input.size, input.ts)

				if addDebugTrace {
					ps.log.Tracef("Read input packet ts %d len %d inql %d outql %d",
						input.ts, input.size, inQueue.len(), outQueue.len())
				}

			case <-waitWorkTicker.C:
				// Time to check for additional work.
				readIn = false

			case <-ctx.Done():
				return ctx.Err()
			}
		}

		if addDebugTrace {
			totQueueLen := outQueue.len() + inQueue.len() + len(ps.playbackChan) + len(ps.inputChan)
			if lastDecTs > 0 && totQueueLen > 0 && totQueueLen < 5 {
				ps.log.Debugf("Queues getting close to depleted: ts %d inql %d outql %d pql %d ipql %d",
					lastDecTs, inQueue.len(), outQueue.len(), len(ps.playbackChan), len(ps.inputChan))
			} else if totQueueLen > maxOutQueueLen+maxUndecodedInputLen-5 {
				ps.log.Debugf("Queues getting close to needing pruning: ts %d inql %d outql %d pql %d ipql %d",
					lastDecTs, inQueue.len(), outQueue.len(), len(ps.playbackChan), len(ps.inputChan))
			} else if lastDecTs > 0 && lastDecTs%5000 == 0 {
				ps.log.Debugf("Queues running: ts %d inql %d outql %d pql %d ipql %d",
					lastDecTs, inQueue.len(), outQueue.len(), len(ps.playbackChan), len(ps.inputChan))

			}
		}

		// If not in filling mode, send as many decoded packets as
		// available to output. We do the len() < cap() chan to ensure
		// we don't lose the pop'd entry.
		for sendOut := true; !fillQueue && sendOut && !outQueue.isEmpty() && len(ps.playbackChan) < cap(ps.playbackChan); {
			item := outQueue.deq()
			select {
			case ps.playbackChan <- item:
				if outQueue.isEmpty() {
					if addDebugTrace {
						ps.log.Warnf("Setting fillQueue to true due to empty queues")
					}

					// Empty queues. Start buffering up
					// again from whatever timestamp the
					// remote sends us.
					fillQueue = true
					lastDecTs = 0
				}
				if addDebugTrace {
					ps.log.Tracef("Pushed ts %d inql %d outql %d",
						item.ts, inQueue.len(), outQueue.len())
				}
			case <-ctx.Done():
				return ctx.Err()
			default:
				sendOut = false
			}
		}

		// If both outQueue and inQueue are full, drop some packets from
		// outQueue (one inQueue.len worth of packets) to make room for
		// more recent packets. Hitting this case means we're slower
		// processing the packets than the remote end is sending them or
		// the network/remote are bursty.
		if outQueue.isFull() && inQueue.len() >= maxUndecodedInputLen {
			var dropped int
			inql, outql := inQueue.len(), outQueue.len()
			for i := 0; !outQueue.isEmpty() && i < inQueue.len(); i++ {
				item := outQueue.deq()
				ps.bytesBuffers.Put(item.data)
				dropped++
			}
			if addDebugTrace {
				ps.log.Infof("Pruned %d pkts due to full queues inql %d outql %d pql %d",
					dropped, inql, outql, len(ps.playbackChan))
			}
		}

		// Decode as many packets as needed (or available) depending on
		// each case.
		for !outQueue.isFull() && inQueue.len() > 0 {
			availableTs := inQueue.firstTs()

			doInitialDecode := fillQueue &&
				inQueue.len() >= minFillQueueLen
			outQueueAlmostDepleted := !fillQueue && outQueue.len() < minOutQueueLen
			doFEC := !didFEC && outQueueAlmostDepleted && availableTs == lastDecTs+periodSizeMS*2
			doDecode := availableTs == lastDecTs+periodSizeMS

			if !doDecode && !doFEC && !outQueueAlmostDepleted && !doInitialDecode {
				// If we decided not to decode and not generate
				// a FEC packet, nothing to do.
				break
			}

			var opusFrame *[]byte
			var opusFrameSlice []byte
			var opusFrameSize int
			var ts uint32

			// If playback is stalled and missing just one packet
			// (likely lost to the network), then generate a FEC
			// packet by passing a nil packet to Decode().
			//
			// Otherwise, use whatever we have that is waiting for
			// decoding.
			if doFEC {
				ts = lastDecTs + periodSizeMS
				if addDebugTrace {
					ps.log.Infof("FEC packet at ts %d inql %d outql %d",
						ts, inQueue.len(), outQueue.len())
				}
			} else {
				opusFrame, opusFrameSize, ts, _ = inQueue.deq()
				if addDebugTrace {
					ps.log.Tracef("Decoding packet ts %d inql %d outql %d",
						ts, inQueue.len(), outQueue.len())
				}
				opusFrameSlice = (*opusFrame)[:opusFrameSize]
			}

			// Do the actual stateful decoding.
			decoded, err := decoder.Decode(opusFrameSlice, frameSize, doFEC, decodeBuffer)
			if err != nil {
				return err
			}
			didFEC = doFEC

			// Determine if this has sound above a noise level.
			applyGainDB(decoded, volumeGain)
			hasSound := detectSound(decoded, 250, 5)

			// Convert from 16 bit samples to byte samples. Reuse
			// buffer fetched in Input() if possible.
			var samples = opusFrame
			if samples == nil {
				samples = ps.bytesBuffers.Get().(*[]byte)
			}
			leS16SliceToBytes(decoded, (*samples))
			samplesSize := len(decoded) * 2

			// Stats.
			inSize += opusFrameSize
			outSize += samplesSize
			outSamples += len(decoded)

			// Add to outQueue.
			outQueue.enq(inputPacket{data: samples, size: samplesSize, ts: ts, hasSound: hasSound})

			if fillQueue && outQueue.len() > minOutQueueLen+cap(ps.playbackChan) {
				fillQueue = false
				if addDebugTrace {
					ps.log.Infof("Considering outQueue initially filled "+
						"with %d pkts ts %d remaining %d in inQueue",
						outQueue.len(), ts, inQueue.len())
				}
			}

			// Track timestamp of last decoded packet.
			lastDecTs = ts
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
			inSize += input.size
			lastTimestamp = input.ts

			if input.size != bytesToRead {
				ps.log.Warnf("Received Samples %d is different than read size %d",
					input.size, bytesToRead)
				continue
			}

			// Received a valid set of samples.
			if addDebugTrace {
				ps.log.Tracef("Filling output buffer with data from "+
					"timestamp %d (plen %d)", lastTimestamp, len(ps.playbackChan))
			}
			copy(outSample, (*input.data)[:input.size])
			ps.bytesBuffers.Put(input.data)
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
			buf := make([]byte, 0, sampleCount*2)
			return &buf
		}},
		playbackChan:     make(chan inputPacket, 4),  // 80ms
		inputChan:        make(chan inputPacket, 20), // 400ms
		getBufCount:      make(chan chan int, 5),
		volumeGainChan:   make(chan float64, 1),
		inputDone:        make(chan struct{}),
		playbackDone:     make(chan struct{}),
		changeDeviceChan: make(chan DeviceID),
	}
}

// playbackOpusFrames creates and runs a playback stream through a set of opus
// packets (likely a recording).
func playbackOpusFrames(ctx context.Context, audioCtx audioContext,
	deviceID DeviceID, gain float64, opusFrames [][]byte, log slog.Logger) *PlaybackStream {

	ps := newPlaybackStream(audioCtx, deviceID)
	ps.log = log

	go ps.run(ctx)
	if gain != 0 {
		ps.SetVolumeGain(gain)
	}

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
