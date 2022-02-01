package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type mainWindowState struct {
	initless
	as *appState

	// Command line completion.
	completeOpts []string
	completeIdx  int

	escMode bool
	escStr  string

	viewport viewport.Model
	textArea *textAreaModel // line editor

	header string

	debug string
}

func (mws *mainWindowState) updateHeader() {
	var connMsg string
	state := mws.as.currentConnState()
	switch state {
	case connStateOnline:
		connMsg = mws.as.styles.online.Render("online")
	case connStateOffline:
		connMsg = mws.as.styles.offline.Render("offline")
	case connStateCheckingWallet:
		connMsg = mws.as.styles.checkingWallet.Render("checking wallet")
	}

	qlenMsg := mws.as.styles.header.Render(fmt.Sprintf("Q %d ", mws.as.rmqLen()))

	server := mws.as.serverAddr
	msg := fmt.Sprintf(" %s - %s", server, connMsg)
	headerMsg := mws.as.styles.header.Render(msg)
	spaces := mws.as.styles.header.Render(strings.Repeat(" ",
		max(0, mws.as.winW-lipgloss.Width(headerMsg)-lipgloss.Width(qlenMsg))))
	mws.header = headerMsg + spaces + qlenMsg
}

func (mws *mainWindowState) updateViewportContent() {
	wasAtBottom := mws.viewport.AtBottom()
	mws.viewport.SetContent(mws.as.activeWindowMsgs())
	if wasAtBottom {
		mws.viewport.GotoBottom()
	}
}

func (mws *mainWindowState) recalcViewportSize() {

	// First, update the edit line height. This is not entirely accurate
	// because textArea does its own wrapping.
	textAreaHeight := mws.textArea.recalcDynHeight(mws.as.winW, mws.as.winH)

	// Next figure out how much is left for the viewport.
	headerHeight := lipgloss.Height(mws.header)
	footerHeight := lipgloss.Height(mws.footerView())
	editHeight := textAreaHeight

	verticalMarginHeight := headerHeight + footerHeight + editHeight
	mws.viewport.YPosition = headerHeight + 1
	mws.viewport.Width = mws.as.winW
	mws.viewport.Height = mws.as.winH - verticalMarginHeight
}

func (mws *mainWindowState) onTextInputAction() {
	text := mws.textArea.Value()
	if text == "" {
		return
	}

	args := parseCommandLine(text)
	if len(args) > 0 {
		mws.as.handleCmd(text, args)
	} else {
		mws.as.msgInActiveWindow(text)
	}

	// Clear line editor
	mws.textArea.Reset()
	mws.recalcViewportSize()
}

func (mws *mainWindowState) updateCompletion() {
	// Advance to next completion option.
	if len(mws.completeOpts) != 0 {
		mws.completeIdx = (mws.completeIdx + 1) % len(mws.completeOpts)
		return
	}

	cl := mws.textArea.Value()
	mws.completeOpts = genCompleterOpts(cl, mws.as)
	if len(mws.completeOpts) != 1 {
		return
	}

	// Only have one completion option. Use it, replacing the last arg with
	// the completion.
	args := parseCommandLinePreserveQuotes(cl)
	args[len(args)-1] = mws.completeOpts[0]
	newValue := strings.Join(args, " ")
	if newValue[0] != leader {
		newValue = string(leader) + newValue
	}
	mws.textArea.SetValue(newValue)
	mws.completeOpts = nil
}

func (mws mainWindowState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	/*
		// Enable to debug msgs.
		if _, ok := msg.(logUpdated); !ok {
			mws.as.log.Infof("XXXXXX %T", msg)
		}
	*/

	// Early check for a quit msg to put us into the shutdown state (to
	// shutdown DB, etc).
	if ss, cmd := maybeShutdown(mws.as, msg); ss != nil {
		return ss, cmd
	}

	// Switch to the feed window if it got activated.
	if mws.as.isFeedWinActive() {
		return newFeedWindow(mws.as, -1, -1)
	}

	// Main msg handler. We only return early in cases where we switch to a
	// different state, otherwise only return at the end of the function.
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// mws.debug = fmt.Sprintf("%q %v", msg.String(), msg.Type)

		switch {
		case msg.Type == tea.KeyEsc:
			mws.escStr = ""
			mws.escMode = !mws.escMode

		case msg.String() == "alt+\r":
			// Alt+enter on new bubbletea version.
			msg.Type = tea.KeyEnter
			fallthrough

		case msg.Type == tea.KeyEnter && msg.Alt:
			// Alt+Enter: Add a new line to multiline edit.
			msg.Alt = false
			mws.textArea, cmd = mws.textArea.Update(msg)
			cmds = appendCmd(cmds, cmd)
			mws.recalcViewportSize()

		case msg.Type == tea.KeyEnter:
			// Execute command
			mws.onTextInputAction()

		case msg.Type == tea.KeyPgUp, msg.Type == tea.KeyPgDown:
			// Rewrite when alt is pressed to scroll a single line.
			if msg.Type == tea.KeyPgUp && msg.Alt {
				msg.Type = tea.KeyUp
				msg.Alt = false
			} else if msg.Type == tea.KeyPgDown && msg.Alt {
				msg.Type = tea.KeyDown
				msg.Alt = false
			}

			// send to viewport
			mws.viewport, cmd = mws.viewport.Update(msg)
			cmds = appendCmd(cmds, cmd)

		case msg.Type == tea.KeyTab:
			mws.updateCompletion()

		case msg.Type == tea.KeyCtrlN:
			mws.as.changeActiveWindowNext()

		case msg.Type == tea.KeyCtrlP:
			mws.as.changeActiveWindowPrev()

		case msg.Type == tea.KeyUp, msg.Type == tea.KeyDown:
			up := msg.Type == tea.KeyUp
			down := !up
			afterHistory := mws.as.cmdHistoryIdx >= len(mws.as.cmdHistory)
			atWorkingCmd := afterHistory && mws.textArea.Value() == mws.as.workingCmd
			atLastHistory := mws.as.cmdHistoryIdx == len(mws.as.cmdHistory)-1
			atStart := mws.as.cmdHistoryIdx == 0
			emptyWorkingCmd := mws.as.workingCmd == ""
			emptyHistory := len(mws.as.cmdHistory) == 0
			textAreaLineCount := mws.textArea.totalLineCount()
			textAreaInfo := mws.textArea.LineInfo()
			textAreaLineNb := mws.textArea.Line() + textAreaInfo.RowOffset
			textAreaAtLastLine := mws.textArea.Line() == (mws.textArea.LineCount()-1) &&
				textAreaInfo.RowOffset == mws.textArea.lastLineSoftLineCount()

			newValue := mws.textArea.Value()
			switch {
			case up && textAreaLineCount > 1 && textAreaLineNb > 0:
				// Move cursor up in multiline text area.
				mws.textArea, cmd = mws.textArea.Update(msg)
				cmds = appendCmd(cmds, cmd)
			case up && afterHistory && emptyWorkingCmd && emptyHistory:
				// Do nothing.
			case up && afterHistory && !atWorkingCmd:
				// Go back to working command (after scrolling
				// past it)
				newValue = mws.as.workingCmd
			case up && !atStart:
				// Go back to previous command.
				mws.as.cmdHistoryIdx -= 1
				newValue = mws.as.cmdHistory[mws.as.cmdHistoryIdx]
			case down && textAreaLineCount > 1 && !textAreaAtLastLine:
				// Move cursor down in multline text area.
				mws.textArea, cmd = mws.textArea.Update(msg)
				cmds = appendCmd(cmds, cmd)
			case down && atWorkingCmd:
				// Go past working command into a new one.
				newValue = ""
			case down && afterHistory && !atWorkingCmd:
				// Do nothing.
			case down && !afterHistory && !emptyHistory && !atLastHistory:
				// Go down to next element in history.
				mws.as.cmdHistoryIdx += 1
				newValue = mws.as.cmdHistory[mws.as.cmdHistoryIdx]
			case down && atLastHistory:
				// Go past history into working command.
				mws.as.cmdHistoryIdx += 1
				newValue = mws.as.workingCmd
			}

			if newValue != mws.textArea.Value() {
				mws.textArea.SetValue(newValue)
				mws.recalcViewportSize()

				// Moving down, go to first line of multiline
				// edit area.
				textAreaLineCount = mws.textArea.totalLineCount()
				if down {
					//mws.textArea.SetCursor(0)
					for i := 0; i < textAreaLineCount; i++ {
						mws.textArea.CursorUp()
					}
					mws.textArea.SetCursor(0)
					//mws.textArea.SetCursor(0)
				} else {
					mws.textArea.SetCursor(0) // possible panic without this
					for i := 0; i < textAreaLineCount; i++ {
						mws.textArea.CursorDown()
					}
					mws.textArea.CursorEnd()
				}

				// Force the update so it shows up correctly.
				mws.textArea, cmd = mws.textArea.Update(nil)
				cmds = appendCmd(cmds, cmd)
				cmds = appendCmd(cmds, textarea.Blink)
			}

		case mws.escMode && len(msg.Runes) == 1:
			mws.escStr += msg.String()
			return mws, func() tea.Msg {
				time.Sleep(250 * time.Millisecond)
				return msgProcessEsc{}
			}

		default:
			// Process line input.
			prevVal := mws.textArea.Value()
			mws.textArea, cmd = mws.textArea.Update(msg)
			cmds = appendCmd(cmds, cmd)
			newVal := mws.textArea.Value()

			// Store working cmd if the text input changed in
			// response to this msg.
			if prevVal != newVal {
				mws.recalcViewportSize()
				mws.as.workingCmd = newVal
				mws.as.cmdHistoryIdx = len(mws.as.cmdHistory)

				// Reset completion.
				mws.completeOpts = nil
				mws.completeIdx = 0
			}
		}

	case tea.WindowSizeMsg: // resize window
		mws.as.winW, mws.as.winH = msg.Width, msg.Height
		mws.textArea.SetWidth(msg.Width)
		mws.updateHeader()
		mws.recalcViewportSize()
		mws.updateViewportContent()

	case repaintActiveChat:
		mws.updateViewportContent()

	case logUpdated:
		if mws.as.isLogWinActive() {
			mws.updateViewportContent()
		}

	case lndLogUpdated:
		if mws.as.isLndLogWinActive() {
			mws.updateViewportContent()
		}

	case currentTimeChanged:
		mws.as.footerInvalidate()

	case showNewPostWindow:
		mws.as.workingCmd = ""
		return newNewPostWindow(mws.as)

	case showFeedWindow:
		mws.as.workingCmd = ""
		return newFeedWindow(mws.as, -1, -1)

	case msgLNRequestRecv:
		mws.as.workingCmd = ""
		return newLNRequestRecvWindow(mws.as)

	case msgLNOpenChannel:
		mws.as.workingCmd = ""
		return newLNOpenChannelWindow(mws.as, false)

	case connState, rmqLenChanged:
		mws.updateHeader()
		return mws, nil

	case msgConfirmServerCert:
		// Go back to the init step state to display the accept server
		// cert.
		return newInitStepState(mws.as, &msg), nil

	case msgPasteErr:
		mws.as.diagMsg("Unable to paste: %v", msg)
		mws.updateViewportContent()

	case msgProcessEsc:
		newWin, err := strconv.ParseInt(mws.escStr, 10, 64)
		if mws.escStr != "" && err == nil {
			// Chat windows are 0-based internally, but
			// 1-based here to preserve legacy UX.
			win := int(newWin - 1)
			mws.as.changeActiveWindow(win)
		}
		mws.escStr = ""
		mws.escMode = false

	default:
		// Handle other messages.
		mws.textArea, cmd = mws.textArea.Update(msg)
		cmds = appendCmd(cmds, cmd)
	}

	return mws, batchCmds(cmds)
}

func (mws mainWindowState) footerView() string {
	esc := ""
	if !mws.viewport.AtBottom() {
		esc = mws.as.styles.footer.Render("(more) ")
	}
	if mws.debug != "" {
		esc = mws.debug
	} else if mws.escMode {
		esc = "ESC"
	}

	return mws.as.footerView(esc)
}

func (mws mainWindowState) View() string {
	var opt string
	if mws.completeIdx < len(mws.completeOpts) {
		opt = mws.completeOpts[mws.completeIdx]
		opt = mws.as.styles.help.Render(opt)
	}

	return fmt.Sprintf("%s\n%s\n%s\n%s%s",
		mws.header,
		mws.viewport.View(),
		mws.footerView(),
		mws.textArea.View(),
		opt,
	)
}

func newMainWindowState(as *appState) (mainWindowState, tea.Cmd) {
	mws := mainWindowState{
		as: as,
	}
	mws.textArea = newTextAreaModel(as.styles)
	mws.textArea.Prompt = ""
	mws.textArea.Placeholder = ""
	mws.textArea.ShowLineNumbers = false
	mws.textArea.SetWidth(as.winW)
	mws.textArea.CharLimit = 1024 * 1024
	mws.textArea.SetValue(as.workingCmd)
	mws.textArea.Focus()

	mws.updateHeader()
	mws.recalcViewportSize()
	mws.updateViewportContent()
	return mws, nil
}
