package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/companyzero/bisonrelay/internal/mdembeds"
	"golang.org/x/exp/maps"
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

	// embedded data for images/files/links
	embedContent map[string][]byte
	ew           *embedWidget

	formInput textinput.Model

	isPage bool
	isChat bool

	header string

	debug string

	// windowMsgs is only used during debug.
	windowMsgs map[string]int
}

func (mws *mainWindowState) updateHeader(styles *theme) {
	var connMsg string
	state := mws.as.currentConnState()
	switch state {
	case connStateOnline:
		connMsg = styles.online.Render("online")
	case connStateOffline:
		connMsg = styles.offline.Render("offline")
	case connStateCheckingWallet:
		connMsg = styles.checkingWallet.Render("checking wallet")
	}

	var helpStr string
	if mws.isPage {
		helpStr = " - tab/shift+tab to navigate form, enter to select, ctrl+pgup/pgdown to change windows, ctrl+w to close"
	} else {
		helpStr = " - F2 to embed, ctrl+up/down to select, ctrl+v to view"
	}
	helpMsg := styles.header.Render(helpStr)
	qlenMsg := styles.header.Render(fmt.Sprintf("Q %d ", mws.as.rmqLen()))

	server := mws.as.serverAddr
	msg := fmt.Sprintf(" %s - %s%s", server, connMsg, helpMsg)
	headerMsg := styles.header.Render(msg)
	spaces := styles.header.Render(strings.Repeat(" ",
		max(0, mws.as.winW-lipgloss.Width(headerMsg)-lipgloss.Width(qlenMsg))))
	mws.header = headerMsg + spaces + qlenMsg
}

func (mws *mainWindowState) updateViewportContent() {
	wasAtBottom := mws.viewport.AtBottom()
	mws.viewport.SetContent(mws.as.activeWindowMsgs())
	cw := mws.as.activeChatWindow()
	mws.isPage = cw != nil && cw.page != nil
	mws.isChat = !mws.isPage
	if wasAtBottom && !mws.isPage {
		mws.viewport.GotoBottom()
	}
}

func (mws *mainWindowState) recalcViewportSize() {
	styles := mws.as.styles.Load()

	// First, update the edit line height. This is not entirely accurate
	// because textArea does its own wrapping.
	textAreaHeight := mws.textArea.recalcDynHeight(mws.as.winW, mws.as.winH)

	// Next figure out how much is left for the viewport.
	headerHeight := lipgloss.Height(mws.header)
	footerHeight := lipgloss.Height(mws.footerView(styles))
	editHeight := textAreaHeight

	// Ensure viewport width is zero to disable all padding/wrapping.
	// Wrapping is done when building the messages (in chatwindow or
	// appstate).
	mws.viewport.Style = mws.viewport.Style.Width(0).
		Underline(false).
		UnderlineSpaces(false).
		Strikethrough(false).
		StrikethroughSpaces(false)
	mws.viewport.Width = 0

	verticalMarginHeight := headerHeight + footerHeight + editHeight
	mws.viewport.YPosition = headerHeight + 1
	mws.viewport.Height = mws.as.winH - verticalMarginHeight
}

func (mws *mainWindowState) addEmbedCB(id string, data []byte, embedStr string) error {
	if id != "" && data != nil {
		mws.embedContent[id] = data
	}

	mws.textArea.InsertString(embedStr)
	return nil
}

// resetFormInput reset the currently selected form input to the form field's
// value.
//
// This must be called _after_ an updateViewportWindow(), otherwise the currently
// selected form field might be wrong.
func (mws *mainWindowState) resetFormInput() {
	cw := mws.as.activeChatWindow()
	if cw == nil || cw.selEl == nil || cw.selEl.formField == nil {
		return
	}
	cw.selEl.formField.resetInputModel(&mws.formInput)

	// It's really crappy to udpate the viewport after having just upadted
	// it. This design needs to be improved.
	mws.updateViewportContent()
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

		// Replace pseudo-data with data.
		text = mdembeds.ReplaceEmbeds(text, func(args mdembeds.EmbeddedArgs) string {
			data := string(args.Data)
			if strings.HasPrefix(data, "[content ") {
				id := data[9 : len(args.Data)-1]
				args.Data = mws.embedContent[id]
			}

			return args.String()
		})

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

	args := parseCommandLinePreserveQuotes(cl)
	if len(args) == 0 {
		// Nothing to complete.
		return
	}

	var lastArgRepl string
	if len(mws.completeOpts) == 1 {
		// Only have one completion option. Use it, replacing the last
		// arg with the completion.
		lastArgRepl = mws.completeOpts[0]
		mws.completeOpts = nil
	} else {
		// Multiple completion options. Find out the common prefix
		// for all of them (if there is one), and pre-complete with
		// this prefix.
		lastArgRepl = stringsCommonPrefix(mws.completeOpts)
		if lastArgRepl == "" {
			lastArgRepl = args[len(args)-1]
		}
	}

	args[len(args)-1] = lastArgRepl
	newValue := strings.Join(args, " ")
	if newValue[0] != leader {
		newValue = string(leader) + newValue
	}
	mws.textArea.SetValue(newValue)
}

func (mws mainWindowState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	if mws.windowMsgs != nil {
		msgKey := fmt.Sprintf("%T", msg)
		mws.windowMsgs[msgKey] = mws.windowMsgs[msgKey] + 1
	}

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
		return newFeedWindow(mws.as, -1, -1, mws.as.feedAuthor)
	}

	if mws.ew.active() {
		_, cmd = mws.ew.Update(msg)
		return mws, cmd
	}
	styles := mws.as.styles.Load()

	cw := mws.as.activeChatWindow()
	if cw != nil && cw.isPage {
		cw.Lock()
		pageRequested := cw.pageRequested
		cw.Unlock()
		if pageRequested != nil {
			cw.pageSpinner, cmd = cw.pageSpinner.Update(msg)
			cmds = appendCmd(cmds, cmd)
		}
	}

	// Main msg handler. We only return early in cases where we switch to a
	// different state, otherwise only return at the end of the function.
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// mws.debug = fmt.Sprintf("%q %v", msg.String(), msg.Type)

		switch {
		case msg.Type == tea.KeyCtrlW:
			mws.as.closeActiveWindow()

		case msg.Type == tea.KeyEsc:
			mws.escStr = ""
			mws.escMode = !mws.escMode

		case msg.String() == "alt+\r":
			// Alt+enter on new bubbletea version.
			msg.Type = tea.KeyEnter
			fallthrough

		case mws.isChat && (msg.Type == tea.KeyEnter && msg.Alt):
			// Alt+Enter: Add a new line to multiline edit.
			msg.Alt = false
			mws.textArea, cmd = mws.textArea.Update(msg)
			cmds = appendCmd(cmds, cmd)
			mws.recalcViewportSize()

		case msg.Type == tea.KeyEnter:
			if mws.isPage {
				if cw.selEl != nil && cw.selEl.embed != nil {
					// View selected embed.
					embedded := *cw.selEl.embed
					cmd, err := mws.as.viewEmbed(embedded)
					if err == nil {
						return mws, cmd
					}
					cw.newHelpMsg("Unable to view embed: %v", err)
					mws.updateViewportContent()

				} else if cw.selEl != nil && cw.selEl.link != nil {
					// Navigate to other page.
					uid := cw.page.UID
					err := mws.as.fetchPage(uid, *cw.selEl.link,
						cw.page.SessionID, cw.page.PageID, nil, "")
					if err != nil {
						mws.as.diagMsg("Unable to fetch page: %v", err)
					}
				} else if cw.selEl != nil && cw.selEl.form != nil &&
					cw.selEl.formField != nil && cw.selEl.formField.typ == "submit" {
					// Don't allow submit if any form fields has an err
					for _, field := range cw.selEl.form.fields {
						if field.err != nil {
							break
						}
					}
					// Submit form.
					uid := cw.page.UID
					action := cw.selEl.form.action()

					err := mws.as.fetchPage(uid, action,
						cw.page.SessionID, cw.page.PageID,
						cw.selEl.form, cw.selEl.form.asyncTarget())
					if err != nil {
						mws.as.diagMsg("Unable to fetch page: %v", err)
					}

				}

				break
			}

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

			wasAtBottom := mws.viewport.AtBottom()

			// send to viewport
			mws.viewport, cmd = mws.viewport.Update(msg)
			cmds = appendCmd(cmds, cmd)

			if !wasAtBottom && mws.viewport.AtBottom() {
				cw := mws.as.activeChatWindow()
				if cw != nil {
					cmds = appendCmd(cmds, markAllRead(cw))
					mws.updateViewportContent()
				}
			}

		case msg.Type == tea.KeyCtrlDown:
			if !mws.isPage && cw != nil {
				if cw.changeSelected(1) {
					mws.updateViewportContent()
					mws.resetFormInput()
				}
			}

		case msg.Type == tea.KeyCtrlUp:
			if !mws.isPage && cw != nil {
				if cw.changeSelected(-1) {
					mws.updateViewportContent()
					mws.resetFormInput()
				}
			}

		case msg.Type == tea.KeyTab:
			if mws.isPage {
				if cw.changeSelected(1) {
					mws.updateViewportContent()
					mws.resetFormInput()
				}

				break
			}

			mws.updateCompletion()

		case msg.Type == tea.KeyShiftTab:
			if mws.isPage {
				if cw.changeSelected(-1) {
					mws.updateViewportContent()
					mws.resetFormInput()
				}

				break
			}

		case msg.Type == tea.KeyCtrlPgUp:
			mws.as.changeActiveWindowNext()

		case msg.Type == tea.KeyCtrlPgDown:
			mws.as.changeActiveWindowPrev()

		case msg.Type == tea.KeyUp, msg.Type == tea.KeyDown:
			if mws.isPage {
				// send to viewport
				mws.viewport, cmd = mws.viewport.Update(msg)
				cmds = appendCmd(cmds, cmd)

				break
			} else if !mws.isChat {
				break
			}

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

		case msg.Type == tea.KeyF2:
			cmds = mws.ew.activate()

		case !mws.isPage && cw != nil && cw.selEl != nil && cw.selEl.embed != nil && msg.Type == tea.KeyCtrlV:
			// View selected embed.
			embedded := *cw.selEl.embed
			cmd, err := mws.as.viewEmbed(embedded)
			if err == nil {
				return mws, cmd
			}
			cw.newHelpMsg("Unable to view embed: %v", err)
			mws.updateViewportContent()

		case !mws.isPage && cw != nil && cw.selEl != nil && cw.selEl.link != nil && msg.Type == tea.KeyCtrlV:
			// Navigate to other page.
			uid := cw.page.UID
			err := mws.as.fetchPage(uid, *cw.selEl.link,
				cw.page.SessionID, cw.page.PageID, nil, "")
			if err != nil {
				mws.as.diagMsg("Unable to fetch page: %v", err)
			}

		case !mws.isPage && cw != nil && cw.selEl != nil && cw.selEl.formField != nil && cw.selEl.formField.typ == "submit" && cw.selEl.form != nil && msg.Type == tea.KeyCtrlV:
			// Submit form.
			uid := cw.page.UID
			action := cw.selEl.form.action()

			// mws.as.diagMsg("uid: %s", uid)
			// for _, ff := range cw.selEl.form.fields {
			//	mws.as.diagMsg("form %s - %v", ff.typ, ff.value)
			// }

			err := mws.as.fetchPage(uid, action,
				cw.page.SessionID, cw.page.PageID, cw.selEl.form,
				cw.selEl.form.asyncTarget())
			if err != nil {
				mws.as.diagMsg("Unable to fetch page: %v", err)
			}

		case cw != nil && cw.selEl != nil && cw.selEl.url != nil && cw.selEl.payReq != nil && msg.Type == tea.KeyCtrlV:
			// Pay invoice.
			mws.as.payPayReq(cw, *cw.selEl.url, cw.selEl.payReq)

		case msg.Type == tea.KeyCtrlD:
			if cw != nil && cw.selEl != nil && cw.selEl.embed != nil {
				embedded := *cw.selEl.embed
				if embedded.Uid != nil {
					err := mws.as.downloadEmbed(*embedded.Uid, embedded)
					if err == nil {
						cw.newHelpMsg("Starting to download file %s", embedded.Uid)
						mws.updateViewportContent()
						return mws, nil
					}
					cw.newHelpMsg("Unable to download embed: %v", err)
					mws.updateViewportContent()
				}
			}

		case mws.isPage && cw != nil && cw.selEl != nil && cw.selEl.formField != nil:
			// Process form input in page.
			if cw.selEl.formField.updateInputModel(&mws.formInput, msg) {
				mws.updateViewportContent()
			}

		case mws.isPage:
			cw.pageSpinner, _ = cw.pageSpinner.Update(msg)

			// Do not process command input when a page is active
			// (to capture form input).

		case msg.Type == tea.KeyF8:
			// Debug window message counts.
			if mws.windowMsgs == nil {
				mws.windowMsgs = make(map[string]int)
			}
			keys := maps.Keys(mws.windowMsgs)
			sort.Slice(keys, func(i, j int) bool {
				ci, cj := mws.windowMsgs[keys[i]], mws.windowMsgs[keys[j]]
				return ci > cj
			})
			var maxKey int
			for i := 0; i < len(keys); i++ {
				maxKey = max(maxKey, len(keys[i]))
			}
			mws.as.manyDiagMsgsCb(func(pf printf) {
				pf("")
				if len(mws.windowMsgs) == 0 {
					pf("Starting to capture window message counts.")
				}
				for _, k := range keys {
					pf("%[1]*[2]s: %[3]d", maxKey, k,
						mws.windowMsgs[k])
				}
			})

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
		mws.updateHeader(styles)
		mws.recalcViewportSize()
		mws.updateViewportContent()

	case msgActiveWindowChanged:
		cw := mws.as.activeChatWindow()
		if cw != nil {
			if cw.unreadCount() < mws.as.winH {
				cmds = appendCmd(cmds, markAllRead(cw))
			}

			mws.updateViewportContent()
			mws.resetFormInput()
		}

	case msgNewRecvdMsg:
		// Clear unread count from active chat if we're following at
		// the bottom of the screen.
		if mws.viewport.AtBottom() {
			cw := mws.as.activeChatWindow()
			if cw != nil {
				cmds = appendCmd(cmds, markAllRead(cw))
				mws.updateViewportContent()
			}
		}

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
		return newFeedWindow(mws.as, -1, -1, msg.author)

	case msgLNRequestRecv:
		mws.as.workingCmd = ""
		return newLNRequestRecvWindow(mws.as, false)

	case msgLNOpenChannel:
		mws.as.workingCmd = ""
		return newLNOpenChannelWindow(mws.as, false)

	case connState, rmqLenChanged:
		mws.updateHeader(styles)
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

	case msgDownloadCompleted:
		mws.updateViewportContent()

	case msgPageFetched:
		mws.updateViewportContent()
		mws.resetFormInput()

	case msgActiveCWRequestedPage:
		for i := range msg.cmds {
			cmds = appendCmd(cmds, msg.cmds[i])
		}
		mws.updateViewportContent()

	case msgRunCmd:
		return mws, tea.Cmd(msg)

	default:
		// Handle other messages.
		mws.textArea, cmd = mws.textArea.Update(msg)
		cmds = appendCmd(cmds, cmd)
	}

	return mws, batchCmds(cmds)
}

func (mws mainWindowState) footerView(styles *theme) string {
	esc := ""
	if !mws.viewport.AtBottom() {
		esc = styles.footer.Render("(more) ")
	}
	if mws.debug != "" {
		esc = mws.debug
	} else if mws.escMode {
		esc = "ESC"
	}

	return mws.as.footerView(styles, esc)
}

func (mws mainWindowState) View() string {
	styles := mws.as.styles.Load()

	if mws.ew.active() {
		return fmt.Sprintf("%s\n%s\n%s\n",
			mws.header,
			mws.ew.View(),
			mws.footerView(styles))
	}

	if mws.isPage {
		cw := mws.as.activeChatWindow()
		if cw != nil {
			cw.Lock()
			if cw.pageRequested != nil {
				var b strings.Builder
				b.WriteString(mws.header)
				b.WriteRune('\n')
				cw.renderPageHeader(mws.as, &b)
				cw.Unlock()
				b.WriteString(strings.Repeat("\n", mws.viewport.Height-4))
				b.WriteString(mws.footerView(styles))
				b.WriteRune('\n')
				return b.String()
			}
			cw.Unlock()
		}
	}

	textAreaView := mws.textArea.View()
	var opt string
	if mws.completeIdx < len(mws.completeOpts) {
		// TrimRight is needed to remove textArea suffix stuff. This may
		// break in the future if textArea changes.
		textAreaView = strings.TrimRightFunc(textAreaView, unicode.IsSpace)
		opt = mws.completeOpts[mws.completeIdx]
		opt = styles.help.Render(opt)
	}

	vwview := mws.viewport.View()

	res := fmt.Sprintf("%s\n%s\n%s\n%s%s",
		mws.header,
		vwview,
		mws.footerView(styles),
		textAreaView,
		opt,
	)

	return res
}

func newMainWindowState(as *appState) (mainWindowState, tea.Cmd) {
	mws := mainWindowState{
		as:           as,
		embedContent: make(map[string][]byte),
		formInput:    textinput.New(),
	}
	styles := as.styles.Load()

	mws.textArea = newTextAreaModel(styles)
	mws.textArea.Prompt = ""
	mws.textArea.Placeholder = ""
	mws.textArea.ShowLineNumbers = false
	mws.textArea.SetWidth(as.winW)
	mws.textArea.CharLimit = 1024 * 1024
	mws.textArea.SetValue(as.workingCmd)
	mws.textArea.Focus()

	mws.formInput.Focus()

	mws.ew = newEmbedWidget(as, mws.addEmbedCB)
	mws.updateHeader(styles)
	mws.recalcViewportSize()
	mws.updateViewportContent()
	return mws, nil
}
