//go:build !cgo || noaudio

package audio

import (
	"context"

	"github.com/decred/slog"
)

type NoteRecorder struct{}

func NewRecorder(log slog.Logger) (*NoteRecorder, error) {
	return &NoteRecorder{}, nil
}

func (ar *NoteRecorder) FreeContext() error { return nil }

func (ar *NoteRecorder) SetCaptureDevice(dev *Device) error { return nil }

func (ar *NoteRecorder) SetPlaybackDevice(dev *Device) error { return nil }

func (ar *NoteRecorder) Busy() (recording bool, playing bool) { return false, false }

func (ar *NoteRecorder) Stop() {}

func (ar *NoteRecorder) HasRecorded() bool { return false }

func (ar *NoteRecorder) RecordInfo() RecordInfo { return RecordInfo{} }

func (ar *NoteRecorder) OpusFile() ([]byte, error) { return nil, errAudioDisabledCompilation }

func (ar *NoteRecorder) Capture(ctx context.Context) error { return errAudioDisabledCompilation }

func (ar *NoteRecorder) Playback(ctx context.Context) error { return errAudioDisabledCompilation }
