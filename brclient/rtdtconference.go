package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/strescape"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/muesli/reflow/wordwrap"
)

type rtdtConferenceWin struct {
	initless

	// indexRV is the selected session in the list of sessions.
	indexRV zkidentity.ShortID

	sessionsView viewport.Model
	infoView     viewport.Model

	selectingPeer bool
	selPeerIndex  int
	kickingPeer   bool
	banDuration   time.Duration

	attemptingJoinSessRV zkidentity.ShortID

	as  *appState
	err error

	// lastRTT tracks the last RTT determined to the server.
	//
	// NOTE: assumes only a single server is used for all connections, in
	// the future this might not be true.
	lastRTT time.Duration
}

func (w *rtdtConferenceWin) recalcViewportSizes() {
	w.sessionsView.Width = w.as.winW
	w.sessionsView.Height = w.as.winH/2 - 2

	w.infoView.Width = w.as.winW
	w.infoView.Height = w.as.winH/2 - 2
}

// selectedSessIsLive returns true if there is a selected session and that
// sessions is live.
func (w *rtdtConferenceWin) selectedSessIsLive() bool {
	if w.indexRV.IsEmpty() {
		return false
	}

	isLive, _ := w.as.c.IsLiveAndHotRTSession(&w.indexRV)
	return isLive
}

// selectedLiveSessLocalIsAdmin returns true if the local client is an admin of
// the current selected live session.
func (w *rtdtConferenceWin) selectedLiveSessLocalIsAdmin() bool {
	if !w.selectedSessIsLive() {
		return false
	}

	sess, _ := w.as.c.GetRTDTSession(&w.indexRV)
	if sess == nil {
		return false
	}

	return sess.LocalIsAdmin()
}

// selectedPeerInfo returns info of the selected publisher/member of the
// currently selected session.
func (w *rtdtConferenceWin) selectedPeerInfo() (rpc.RTDTPeerID, clientintf.UserID, string) {
	if w.indexRV.IsEmpty() {
		return 0, clientintf.UserID{}, ""
	}

	sess, _ := w.as.c.GetRTDTSession(&w.indexRV)
	if sess == nil {
		return 0, clientintf.UserID{}, ""
	}

	if w.selPeerIndex < 0 || w.selPeerIndex >= len(sess.Metadata.Publishers) {
		return 0, clientintf.UserID{}, ""
	}

	pub := sess.Metadata.Publishers[w.selPeerIndex]
	nick, _ := w.as.c.UserNick(pub.PublisherID)
	if nick == "" {
		nick = pub.Alias
	}
	return pub.PeerID, pub.PublisherID, nick
}

func (w *rtdtConferenceWin) renderSessionsView() {
	styles := w.as.styles.Load()

	var minOffset, maxOffset int

	liveSessions := w.as.c.ListLiveRTSessions()
	attemptingJoin := w.as.attemptingJoinRTDTSessions()

	b := strings.Builder{}
	w.as.rangeRtSessions(func(i int, sess *rtdtSession) {
		rv := sess.rv
		style := styles.noStyle
		if rv == w.indexRV {
			style = styles.focused
			minOffset = strings.Count(b.String(), "\n")
		}

		livePrefix, hotPrefix, padPrefix := "  ", "   ", "     "
		hasHotAudio, isLiveSess := liveSessions[rv]
		_, isAttemptingJoin := attemptingJoin[rv]
		if isLiveSess {
			livePrefix = "👤"
		} else if isAttemptingJoin {
			livePrefix = "↪"
		}
		if hasHotAudio {
			hotPrefix = "🎤 "
		}
		descr := wordwrap.String(sess.descr, w.sessionsView.Width-len(padPrefix)-1)
		descrLines := strings.Split(descr, "\n")

		b.WriteString(style.Render(livePrefix + hotPrefix + rv.String()))
		b.WriteRune('\n')
		for _, line := range descrLines {
			b.WriteString(style.Render(padPrefix + line))
			b.WriteRune('\n')
		}
		if sess.err != nil {
			b.WriteString(padPrefix)
			b.WriteString(styles.err.Render(sess.err.Error()))
			b.WriteRune('\n')
		}
		b.WriteRune('\n')

		if rv == w.indexRV {
			maxOffset = strings.Count(b.String(), "\n")
		}
	})
	if b.Len() == 0 {
		b.WriteString("\nNo realtime chat rooms\n")
	}

	// Ensure the currently selected index is visible.
	if w.sessionsView.YOffset > minOffset {
		// Move viewport up until top of selected item is visible.
		w.sessionsView.SetYOffset(minOffset)
	} else if bottom := w.sessionsView.YOffset + w.sessionsView.Height; bottom < maxOffset {
		// Move viewport down until bottom of selected item is visible.
		w.sessionsView.SetYOffset(w.sessionsView.YOffset + (maxOffset - bottom))
	}

	oldOff := w.sessionsView.YOffset
	w.sessionsView.SetContent(b.String())
	w.sessionsView.YOffset = oldOff
}

// selSessionPublisherCount returns the number of publishers in the given session.
func (w *rtdtConferenceWin) selSessionPublisherCount() int {
	sess, err := w.as.c.GetRTDTSession(&w.indexRV)
	if err != nil || sess == nil {
		return 0
	}

	return len(sess.Metadata.Publishers)
}

// modifySelPeerGain modifies the volumen gain for the currently selected peer.
func (w *rtdtConferenceWin) modifySelPeerGain(delta float64) {
	sess, err := w.as.c.GetRTDTSession(&w.indexRV)
	if err != nil || sess == nil {
		return
	}

	lenPublishers := len(sess.Metadata.Publishers)
	if w.selPeerIndex < 0 || w.selPeerIndex > lenPublishers-1 {
		return
	}

	pub := sess.Metadata.Publishers[w.selPeerIndex]
	w.as.c.ModifyRTDTLivePeerVolumeGain(&w.indexRV, pub.PeerID, delta)
}

func (w *rtdtConferenceWin) renderInfoView() {
	if w.indexRV.IsEmpty() {
		w.infoView.SetContent("")
		return
	}

	sess, err := w.as.c.GetRTDTSession(&w.indexRV)
	if err != nil || sess == nil {
		w.infoView.SetContent("")
		return
	}

	b := &strings.Builder{}
	pf := func(s string, args ...interface{}) {
		fmt.Fprintf(b, s, args...)
	}

	styles := w.as.styles.Load()
	if w.kickingPeer && w.selPeerIndex >= 0 && w.selPeerIndex < len(sess.Metadata.Publishers) {
		kickPeer := sess.Metadata.Publishers[w.selPeerIndex]
		style := styles.focused

		userNick, _ := w.as.c.UserNick(kickPeer.PublisherID)
		if userNick == "" {
			userNick = kickPeer.Alias
		}

		pf("%s", style.Render("Confirm kicking peer "))
		pf("%s", styles.nick.Render(strescape.Nick(userNick)))
		pf("%s %s", style.Render(" (%s)?"), kickPeer.PublisherID)
		pf("\n\n")
		pf("Temporary ban duration: %s (press +/- to change)\n", w.banDuration)
		pf("  Note: ban is lifted immediately if all members leave session\n")
		pf("\n")
		pf("Press <enter> to confirm, <esc> to cancel\n")
		w.infoView.SetContent(b.String())
		w.infoView.SetYOffset(0)
		return
	}

	ownerNick := w.as.c.UserLogNick(sess.Metadata.Owner)
	myID := w.as.c.PublicID()

	liveSess := w.as.c.GetLiveRTSession(&w.indexRV)
	isLive := liveSess != nil
	isHotAudio := liveSess != nil && liveSess.HotAudio
	isAdmin := sess.LocalIsAdmin()

	if sess.Metadata.IsInstant {
		pf("%s", styles.mention.Render("Instant Call - will be removed once left"))
		pf("\n")
	}

	if isLive && isHotAudio {
		pf("Live & hot mic session - <Enter> to turn off mic\n")
	} else if isLive {
		pf("Live session - <Enter> to make mic hot, <q> to leave session\n")
	} else {
		pf("Not in session - <Enter> to join\n")
	}

	if isLive {
		pf("Press <m> to modify per-publisher volume\n")
		pf("Press <c> to open chat window on this session\n")
	}
	if isAdmin {
		pf("Press <m>, select peer, then <k> to kick a peer\n")
	}
	pf("\n")

	var minOffset, maxOffset int

	pf("RV: %s\n", w.indexRV)
	pf("Descr: %s\n", sess.Metadata.Description)
	pf("Size: %d\n", sess.Metadata.Size)
	pf("Session Owner: %s\n", strescape.Nick(ownerNick))
	pf("Local Peer ID: %s\n", sess.LocalPeerID)
	pf("Session Publishers (%d):\n", len(sess.Metadata.Publishers))
	for i, pub := range sess.Metadata.Publishers {
		isSelectedPeer := liveSess != nil && w.selectingPeer && w.selPeerIndex == i
		if isSelectedPeer {
			minOffset = strings.Count(b.String(), "\n")
		}

		liveIcon := " "
		pubNick, _ := w.as.c.UserNick(pub.PublisherID)
		if pubNick == "" && pub.PublisherID == myID {
			pubNick = "local client"
			if isHotAudio {
				liveIcon = "●"
			} else if isLive {
				liveIcon = "○"
			}
		} else if pub.Alias != "" {
			pubNick = pub.Alias
		} else {
			pubNick = pub.PublisherID.String()
		}

		isPeerLive := liveSess != nil && liveSess.IsPeerLive(pub.PeerID)
		pubIsLocalClient := pub.PublisherID == myID
		if !pubIsLocalClient {
			if liveSess.PeerHasSound(pub.PeerID) {
				liveIcon = "●"
			} else if isPeerLive {
				liveIcon = "○"
			}
		}

		suffix := fmt.Sprintf("(%s)", pub.PublisherID)
		style := styles.noStyle
		if liveSess != nil && w.selectingPeer && w.selPeerIndex == i {
			style = styles.focused
			livePeer, ok := liveSess.Peers[pub.PeerID]
			if ok && !pubIsLocalClient {
				suffix = fmt.Sprintf(" 🔊 %+.0f (press +/- to change peer volume)", livePeer.VolumeGain)
			}
		} else if liveSess != nil && isPeerLive {
			peerBufCount := liveSess.Peers[pub.PeerID].BufferedCount
			suffix += fmt.Sprintf(" buf: %s", time.Duration(peerBufCount)*time.Millisecond*20)
		}

		line := fmt.Sprintf("  %s %s %s", liveIcon,
			strescape.Nick(pubNick), suffix)
		pf("%s\n", style.Render(line))

		if isSelectedPeer {
			maxOffset = strings.Count(b.String(), "\n")
		}
	}

	w.infoView.SetContent(b.String())

	// Ensure the currently selected index is visible.
	if liveSess != nil {
		if w.infoView.YOffset > minOffset {
			// Move viewport up until top of selected item is visible.
			w.infoView.SetYOffset(minOffset)
		} else if bottom := w.infoView.YOffset + w.infoView.Height; bottom < maxOffset {
			// Move viewport down until bottom of selected item is visible.
			w.infoView.SetYOffset(w.infoView.YOffset + (maxOffset - bottom))
		}
	} else {
		w.infoView.SetYOffset(0)
	}
}

func (w *rtdtConferenceWin) updateSessions() {

	newLen, firstRV := w.as.updateRtdtSessions()
	_, stillExists := w.as.rtSessFrom(w.indexRV, 0)
	switch {
	case stillExists:
		// Do nothing, keep it selected.
	case newLen == 0:
		// No sessions, clear index rv.
		w.indexRV = zkidentity.ShortID{}
	default:
		// Switch to first RV.
		w.indexRV = firstRV
	}

	// Update the viewport.
	w.renderSessionsView()
	w.renderInfoView()
}

func (w *rtdtConferenceWin) activateSelected() {
	w.err = nil
	if w.indexRV.IsEmpty() {
		return
	}

	rv := w.indexRV
	sess, err := w.as.c.GetRTDTSession(&rv)
	if err != nil || sess == nil {
		return
	}

	liveSessions := w.as.c.ListLiveRTSessions()

	hasHotAudio, isLive := liveSessions[rv]
	switch {
	case !isLive:
		// Mark as attempting to join this session. When we actually
		// join it, make the audio hot by default.
		w.attemptingJoinSessRV = rv
		w.as.attemptJoinRTDTSession(rv)

	case !hasHotAudio:
		// Live session without hot audio, make it audio hot.
		w.as.rtSessClearError(rv)
		err = w.as.c.SwitchHotAudio(rv)

	default:
		// Live session with hot audio, make it audio not hot.
		w.as.rtSessClearError(rv)
		err = w.as.c.SwitchHotAudio(zkidentity.ShortID{})

	}

	w.err = err
	if err != nil {
		w.as.diagMsg("Unable to make session %s audio hot: %v", rv, err)
		return
	}

	w.renderSessionsView()
	w.renderInfoView()
}

func (w *rtdtConferenceWin) leaveSelected() {
	if w.indexRV.IsEmpty() {
		return
	}

	go w.as.rtLeaveLiveSession(w.indexRV)

	w.renderSessionsView()
	w.renderInfoView()
}

func (w rtdtConferenceWin) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Early check for a quit msg to put us into the shutdown state (to
	// shutdown DB, etc).
	if ss, cmd := maybeShutdown(w.as, msg); ss != nil {
		return ss, cmd
	}

	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg: // resize window
		w.as.winW, w.as.winH = msg.Width, msg.Height
		w.recalcViewportSizes()
		w.renderSessionsView()
		w.renderInfoView()

	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyEsc:
			if w.kickingPeer {
				w.kickingPeer = false
				w.renderInfoView()
			} else if w.selectingPeer {
				w.selectingPeer = false
				w.renderInfoView()
			} else {
				return newMainWindowState(w.as)
			}

		case w.kickingPeer && (msg.Type == tea.KeyUp || msg.Type == tea.KeyDown):
			// Ignore.

		case w.selectingPeer && msg.Type == tea.KeyUp:
			w.selPeerIndex = max(w.selPeerIndex-1, 0)
			w.renderInfoView()

		case w.selectingPeer && msg.Type == tea.KeyDown:
			w.selPeerIndex = min(w.selPeerIndex+1, w.selSessionPublisherCount()-1)
			w.renderInfoView()

		case w.kickingPeer && (msg.String() == "+" || msg.String() == "-"):
			switch {
			case msg.String() == "+" && w.banDuration == 0:
				w.banDuration = time.Second * 30
			case msg.String() == "+" && w.banDuration > time.Hour:
				w.banDuration = 64 * time.Minute
			case msg.String() == "+":
				w.banDuration *= 2
			case msg.String() == "-" && w.banDuration <= 30*time.Second:
				w.banDuration = 0
			case msg.String() == "-":
				w.banDuration /= 2
			}
			w.renderInfoView()

		case w.selectingPeer && msg.String() == "+":
			w.modifySelPeerGain(+1)
			w.renderInfoView()

		case w.selectingPeer && msg.String() == "-":
			w.modifySelPeerGain(-1)
			w.renderInfoView()

		case msg.Type == tea.KeyUp:
			next, ok := w.as.rtSessFrom(w.indexRV, -1)
			if ok {
				w.indexRV = next
			}
			w.renderSessionsView()
			w.renderInfoView()

		case msg.Type == tea.KeyDown:
			prev, ok := w.as.rtSessFrom(w.indexRV, +1)
			if ok {
				w.indexRV = prev
			}

			w.renderSessionsView()
			w.renderInfoView()

		case w.kickingPeer && msg.Type == tea.KeyEnter:
			// Do kick
			pid, uid, nick := w.selectedPeerInfo()
			go w.as.rtKickMember(w.indexRV, pid, w.banDuration, uid, nick)
			w.kickingPeer = false
			w.banDuration = 0
			w.renderInfoView()

		case msg.Type == tea.KeyEnter:
			w.activateSelected()

		case msg.String() == "c" && w.selectedSessIsLive():
			sess, _ := w.as.c.GetRTDTSession(&w.indexRV)
			var cw *chatWindow
			if sess != nil && sess.GC != nil {
				cw = w.as.findOrNewGCWindow(*sess.GC)
			} else {
				cw = w.as.findOrNewRTWindow(w.indexRV)
			}
			w.as.changeActiveWindowCW(cw)
			return newMainWindowState(w.as)

		case msg.String() == "m" && w.selectedSessIsLive():
			w.selectingPeer = !w.selectingPeer
			w.renderInfoView()

		case msg.String() == "k" && w.selectingPeer && w.selectedLiveSessLocalIsAdmin():
			w.kickingPeer = true
			w.banDuration = 0
			w.renderInfoView()

		case msg.String() == "q":
			w.leaveSelected()

		}

	case msgJoinedLiveRTChat:
		w.updateSessions()
		if zkidentity.ShortID(msg) == w.attemptingJoinSessRV && w.as.rtAutoHotAudio {
			if !w.indexRV.IsEmpty() && w.indexRV == w.attemptingJoinSessRV {
				w.activateSelected()
			}
		}

	case msgLeftLiveRTChatRes:
		w.updateSessions()
		w.err = msg.res

	case msgJoinLiveRTChatRes:
		w.updateSessions()
		w.err = msg.res

	case msgRTLivePeersChanged:
		rv := zkidentity.ShortID(msg)
		if w.indexRV == rv {
			w.renderInfoView()
		}

	case msgRTLiveSessSendErrored:
		w.updateSessions()
		w.renderSessionsView()
		w.renderInfoView()

	case msgRTLocalClientKicked:
		w.updateSessions()
		w.renderSessionsView()
		w.renderInfoView()

	case msgRTSessionsChanged:
		w.updateSessions()
		w.renderSessionsView()
		w.renderInfoView()

	case msgRTRTTCalculated:
		w.lastRTT = time.Duration(msg)

	case msgTick:
		// Keep sending this tick to update the UI (bufferedCount).
		w.renderInfoView()
		return w, emitOrCancelAfter(w.as.ctx, msgTick{}, time.Second)
	}

	return w, cmd
}

func (w rtdtConferenceWin) View() string {
	b := new(strings.Builder)

	styles := w.as.styles.Load()
	headerMsg := styles.header.Render(" Real Time Conference")
	if w.lastRTT > 0 {
		headerMsg += styles.header.Render(" (RTT " + w.lastRTT.String() + ")")
	}
	spaces := styles.header.Render(strings.Repeat(" ",
		max(0, w.as.winW-lipgloss.Width(headerMsg))))
	b.WriteString(headerMsg + spaces)
	b.WriteRune('\n')

	nbLines := 1

	/*
		if w.err != nil {
			b.WriteRune('\n')
			b.WriteString(styles.err.Render(w.err.Error()))
			b.WriteRune('\n')
			nbLines += 2
		}
	*/

	b.WriteString(w.sessionsView.View())
	nbLines += w.sessionsView.Height

	b.WriteString("\n")
	b.WriteString(styles.help.Render("═════ Selected Conference "))
	b.WriteString(styles.help.Render(strings.Repeat("═", w.as.winW-21)))
	b.WriteString("\n\n")
	nbLines += 3

	b.WriteString(w.infoView.View())
	nbLines += w.infoView.Height

	nbLines += 1
	b.WriteString(blankLines(w.as.winH - nbLines))
	footerErr := ""
	if w.err != nil {
		footerErr = w.as.styles.Load().err.Render(w.err.Error())
	}
	b.WriteString(w.as.footerView(styles, footerErr))

	return b.String()
}

func newRtdtConferenceWin(as *appState) (rtdtConferenceWin, tea.Cmd) {
	w := rtdtConferenceWin{
		as:           as,
		sessionsView: viewport.New(as.winW, as.winH/2-2),
		infoView:     viewport.New(as.winW, as.winH/2-2),
	}
	w.recalcViewportSizes()
	w.updateSessions()

	return w, emitOrCancelAfter(as.ctx, msgTick{}, time.Second)
}
