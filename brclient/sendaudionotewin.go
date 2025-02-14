package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/mdembeds"
	"github.com/decred/dcrd/dcrutil/v4"
	"golang.org/x/net/context"
)

type sendAudioNoteWin struct {
	initless

	as          *appState
	target      msgSendAudioNote
	opusFile    []byte
	uploadCost  dcrutil.Amount
	embedArgs   mdembeds.EmbeddedArgs
	targetCount int
	indicator   spinner.Model
	btns        formHelper
	err         error
}

func (w *sendAudioNoteWin) updateButtons() {
	styles := w.as.styles.Load()
	btns := newFormHelper(styles)

	recording, playing := w.as.noterec.Busy()
	hasRecorded := w.as.noterec.HasRecorded()

	if recording || playing {
		btns.AddInputs(newButtonHelper(
			styles,
			btnWithLabel("[ Stop ]"),
			btnWithTrailing(" "),
			btnWithFixedMsgAction(msgCancelForm{}),
		))
	} else {
		btns.AddInputs(newButtonHelper(
			styles,
			btnWithLabel("[ Record ]"),
			btnWithTrailing(" "),
			btnWithFixedMsgAction(msgRecordNote{}),
		))

		if hasRecorded {
			btns.AddInputs(newButtonHelper(
				styles,
				btnWithLabel("[ Play ]"),
				btnWithTrailing(" "),
				btnWithFixedMsgAction(msgPlaybackNote{}),
			))

			btns.AddInputs(newButtonHelper(
				styles,
				btnWithLabel("[ Accept ]"),
				btnWithTrailing(" "),
				btnWithFixedMsgAction(msgSubmitForm{}),
			))
		}
	}

	w.btns = btns
	w.btns.setFocus(0)
}

func (w *sendAudioNoteWin) updateRecordData() error {
	if !w.as.noterec.HasRecorded() {
		return nil
	}

	data, err := w.as.noterec.OpusFile()
	if err != nil {
		return err
	}

	var args mdembeds.EmbeddedArgs
	args.Alt = "Audio note"
	args.Typ = "audio/ogg"
	args.Filename = time.Now().Format("2006-01-02-15_04_05") + "-audionote.opus"
	args.Data = data
	msg := args.String()

	policy := w.as.serverPolicy()
	estCost, err := clientintf.EstimatePMCost(msg, &policy)
	if err != nil {
		return err
	}

	if w.target.targetIsGC {
		w.targetCount = w.as.c.GetGCDestCount(w.target.targetID)
	} else {
		w.targetCount = 1
	}

	w.opusFile = data
	w.uploadCost = dcrutil.Amount(1 + estCost*uint64(w.targetCount)/1000)
	w.embedArgs = args
	return nil
}

func (w *sendAudioNoteWin) submitToTarget() error {
	var cw *chatWindow
	if w.target.targetIsGC {
		cw = w.as.findOrNewGCWindow(w.target.targetID)
	} else {
		cw = w.as.findOrNewChatWindow(w.target.targetID, "")
	}

	w.as.pm(cw, w.embedArgs.String())
	return nil
}

func (w sendAudioNoteWin) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Early check for a quit msg to put us into the shutdown state (to
	// shutdown DB, etc).
	if ss, cmd := maybeShutdown(w.as, msg); ss != nil {
		return ss, cmd
	}

	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyEsc:
			w.as.noterec.Stop()
			return newMainWindowState(w.as)

		default:
			w.btns, cmd = w.btns.Update(msg)
		}

	case msgAudioError:
		w.err = error(msg)

	case msgRecordNote:
		go func() {
			err := w.as.noterec.Capture(w.as.ctx)
			if err != nil && !errors.Is(err, context.Canceled) {
				w.as.sendMsg(msgAudioError(err))
			} else {
				w.as.sendMsg(msgRecordComplete{})
			}
		}()
		cmd = batchCmds([]tea.Cmd{
			w.indicator.Tick,
			emitAfter(msgRefreshAudioNoteUI{}, 20*time.Millisecond),
		})

	case msgPlaybackNote:
		go func() {
			err := w.as.noterec.Playback(w.as.ctx)
			if err != nil && !errors.Is(err, context.Canceled) {
				w.as.sendMsg(msgAudioError(err))
			} else {
				w.as.sendMsg(msgPlaybackComplete{})
			}

		}()
		cmd = batchCmds([]tea.Cmd{
			w.indicator.Tick,
			emitAfter(msgRefreshAudioNoteUI{}, 20*time.Millisecond),
		})

	case msgCancelForm:
		w.as.noterec.Stop()
		w.err = nil
		cmd = emitAfter(msgRefreshAudioNoteUI{}, 20*time.Millisecond)

	case msgSubmitForm:
		w.err = w.submitToTarget()
		if w.err == nil {
			return newMainWindowState(w.as)
		}

	case msgRecordComplete:
		w.err = w.updateRecordData()
		w.updateButtons()
		if w.err == nil {
			// Set focus on "play" button after recording.
			w.btns.setFocus(1)
		}

	case msgPlaybackComplete:
		w.updateButtons()
		w.btns.setFocus(1)

	case msgRefreshAudioNoteUI:
		w.updateButtons()

	case spinner.TickMsg:
		w.indicator, cmd = w.indicator.Update(msg)
	}

	return w, cmd
}

func (w sendAudioNoteWin) View() string {
	b := new(strings.Builder)

	styles := w.as.styles.Load()
	headerMsg := styles.header.Render(" Record and send audio note")
	spaces := styles.header.Render(strings.Repeat(" ",
		max(0, w.as.winW-lipgloss.Width(headerMsg))))
	b.WriteString(headerMsg + spaces)
	b.WriteRune('\n')

	nbLines := 1

	recording, playing := w.as.noterec.Busy()
	if recording || playing {
		msg := "Recording"
		if playing {
			msg = "Playing"
		}
		b.WriteString(msg + " ")
		b.WriteString(w.indicator.View())
		b.WriteString("\n\n\n")
		nbLines += 3
	} else {
		hasRecorded := w.as.noterec.HasRecorded()
		recInfo := w.as.noterec.RecordInfo()

		if hasRecorded {
			recDuration := time.Millisecond * time.Duration(recInfo.DurationMs)
			fmt.Fprintf(b, "Record size: %s (%s)\n",
				hbytes(int64(recInfo.EncodedSize)), recDuration)
			fmt.Fprintf(b, "Estimated cost: %s (%d %s)",
				w.uploadCost, w.targetCount,
				plural(w.targetCount, "target", "targets"))
		} else {
			b.WriteString("\n")
		}
		b.WriteString("\n\n")
		nbLines += 3
	}

	b.WriteString(w.btns.View())
	b.WriteRune('\n')
	nbLines += 1

	if w.err != nil {
		b.WriteRune('\n')
		b.WriteString(styles.err.Render(w.err.Error()))
		b.WriteRune('\n')
		nbLines += 2
	}

	b.WriteString(blankLines(w.as.winH - nbLines - 2))
	b.WriteString(w.as.footerView(styles, ""))

	return b.String()
}

func newSendAudioNoteWindow(as *appState, target msgSendAudioNote) (sendAudioNoteWin, tea.Cmd) {
	indicator := spinner.New(spinner.WithSpinner(spinner.Points))
	w := sendAudioNoteWin{
		as:        as,
		target:    target,
		indicator: indicator,
	}
	w.updateRecordData()
	w.updateButtons()
	return w, nil
}
