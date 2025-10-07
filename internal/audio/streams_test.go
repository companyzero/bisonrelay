package audio

import (
	"context"
	"testing"
	"time"

	"github.com/companyzero/bisonrelay/internal/assert"
	"github.com/companyzero/bisonrelay/internal/testutils"
)

// TestPlaybackStream tests successful reception of subsequent packets.
func TestPlaybackStream(t *testing.T) {
	t.Parallel()

	audioCtx := newTestAudioContext(t)
	inStream := newTestAudioPacketGen()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ps := streamPlaybackOpusFrames(ctx, audioCtx, "", nil, testutils.TestLoggerSys(t, "XXXX"))

	// Send some packets for playback. This causes the stream to be
	// initialized.
	nbPackets := minPlaybackBufferPackets + 1
	for i := 0; i < nbPackets; i++ {
		inStream.inputIntoPlayback(ps)
	}
	assert.ChanWritten(t, audioCtx.started)

	// Assert packets are drained.
	for i := 0; i < nbPackets; i++ {
		audioCtx.assertNextCBCompletes()
	}

	// Assert after cancelling, the playback stream ends.
	cancel()
	assert.ChanWritten(t, audioCtx.uninited)
}

// TestPlaybackPacketsBuffered tests that the playback stream buffers packets
// before starting playback.
func TestPlaybackPacketsBuffered(t *testing.T) {
	t.Parallel()

	audioCtx := newTestAudioContext(t)
	inStream := newTestAudioPacketGen()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ps := streamPlaybackOpusFrames(ctx, audioCtx, "", nil, testutils.TestLoggerSys(t, "XXXX"))

	// Audio is not initialized until at least 2 packets are received.
	assert.ChanNotWritten(t, audioCtx.started, time.Second)
	for i := 0; i < minPlaybackBufferPackets-1; i++ {
		inStream.inputIntoPlayback(ps)
		assert.ChanNotWritten(t, audioCtx.started, 100*time.Millisecond)
	}

	// Audio is initialized after the last buffered packet is received.
	inStream.inputIntoPlayback(ps)
	assert.ChanWritten(t, audioCtx.started)
}
