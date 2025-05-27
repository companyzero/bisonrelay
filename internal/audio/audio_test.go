package audio

import (
	"io"
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/companyzero/bisonrelay/internal/assert"
)

// testAudioPacketGen generates packets with random data for input into
// playback streams.
type testAudioPacketGen struct {
	rng io.Reader
	out []byte
	ts  uint32
}

func newTestAudioPacketGen() *testAudioPacketGen {
	return &testAudioPacketGen{
		out: make([]byte, 100),
		rng: rand.NewChaCha8([32]byte{31: 0xff}),
	}
}

func (tg *testAudioPacketGen) inputIntoPlayback(ps *PlaybackStream) {
	tg.rng.Read(tg.out)
	ps.Input(tg.out, tg.ts)
	tg.ts += periodSizeMS
}

type testAudioEncDec struct{}

func (t *testAudioEncDec) Decode(data []byte, frameSize int, fec bool, out []int16) ([]int16, error) {
	// This simulates the correct output size.
	return out[:len(out)/2], nil
}

func (t *testAudioEncDec) Encode(pcm []int16, frameSize int, out []byte) ([]byte, error) {
	return out, nil
}

func (t *testAudioEncDec) SetBitrate(rate int) {
}

// testAudioContext is used to test stream implementations.
type testAudioContext struct {
	t testing.TB

	samples []byte

	mtx      sync.Mutex
	started  chan struct{}
	stopped  chan struct{}
	uninited chan struct{}
	cb       dataProc
}

func newTestAudioContext(t testing.TB) *testAudioContext {
	const pcmSamplesPerPeriod = sampleRate / 1000 * periodSizeMS

	return &testAudioContext{
		t:        t,
		samples:  make([]byte, pcmSamplesPerPeriod*rawFormatSampleSize),
		started:  make(chan struct{}, 5),
		stopped:  make(chan struct{}, 5),
		uninited: make(chan struct{}, 5),
	}
}

func (tac *testAudioContext) name() string {
	return "testaudio"
}

func (tac *testAudioContext) initPlayback(deviceID DeviceID, cb dataProc) (playbackDevice, error) {
	tac.mtx.Lock()
	tac.cb = cb
	tac.mtx.Unlock()
	return tac, nil
}

func (tac *testAudioContext) initCapture(deviceID DeviceID, cb dataProc) (captureDevice, error) {
	tac.mtx.Lock()
	tac.cb = cb
	tac.mtx.Unlock()
	return tac, nil
}

func (tac *testAudioContext) free() error {
	return nil
}

func (tac *testAudioContext) newEncoder(sampleRate, channels int) (streamEncoder, error) {
	// return gopus.NewEncoder(sampleRate, channels, gopus.Voip)
	return &testAudioEncDec{}, nil
}

func (tac *testAudioContext) newDecoder(sampleRate, channels int) (streamDecoder, error) {
	// return gopus.NewDecoder(sampleRate, channels)
	return &testAudioEncDec{}, nil
}

// These are part of the playback/capture device interface.

func (tac *testAudioContext) Start() error {
	tac.started <- struct{}{}
	return nil
}
func (tac *testAudioContext) Stop() error {
	tac.stopped <- struct{}{}
	return nil
}
func (tac *testAudioContext) Uninit() {
	tac.uninited <- struct{}{}
}

// These are test functions.

// nextCBCalledChan returns a chan that is closed when the next callback is
// completed.
func (tac *testAudioContext) nextCBCalledChan() chan struct{} {
	tac.t.Helper()
	tac.mtx.Lock()
	cb := tac.cb
	tac.mtx.Unlock()

	if cb == nil {
		tac.t.Fatalf("callback not initialized")
	}

	const pcmSamplesPerPeriod = sampleRate / 1000 * periodSizeMS
	calledChan := make(chan struct{})
	go func() {
		cb(tac.samples, tac.samples, pcmSamplesPerPeriod)
		close(calledChan)
	}()
	return calledChan
}

// assertNextCBCompletes asserts that the next call to cb completes.
func (tac *testAudioContext) assertNextCBCompletes() {
	tac.t.Helper()
	calledChan := tac.nextCBCalledChan()
	assert.ChanWritten(tac.t, calledChan)
}
