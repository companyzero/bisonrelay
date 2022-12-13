package main

import (
	"fmt"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/erikgeiser/promptkit/selection"
	"github.com/mitchellh/go-homedir"
)

type newPostWindow struct {
	initless
	as *appState

	errMsg string
	debug  string

	focusIdx int
	textArea textAreaModel

	embedding    bool
	formEmbed    formHelper
	embedErr     error
	embedContent map[string][]byte

	sharing        bool
	sharedFiles    []clientdb.SharedFileAndShares
	selSharedFiles *selection.Model
	idxSharedFile  int

	estSize uint64
}

func (pw *newPostWindow) updateTextAreaSize() {
	// marginHeight is header+footer+estimated length comment
	marginHeight := 2 + 2 + 3
	pw.textArea.SetWidth(pw.as.winW)
	pw.textArea.SetHeight(pw.as.winH - marginHeight)
}

func (pw *newPostWindow) listSharedFiles() tea.Cmd {
	files, err := pw.as.c.ListLocalSharedFiles()
	if err != nil {
		pw.as.diagMsg("Unable to list local shared files: %v", err)
		return nil
	}

	choices := make([]*selection.Choice, 0, len(files))
	sharedFiles := make([]clientdb.SharedFileAndShares, 0, len(files))
	for _, file := range files {
		if !file.Global {
			continue
		}

		txt := fmt.Sprintf("%s - %s - %s (%s)",
			file.SF.Filename, hbytes(int64(file.Size)),
			dcrutil.Amount(int64(file.Cost)), file.SF.FID.ShortLogID())
		c := selection.NewChoice(txt)

		choices = append(choices, c)
		sharedFiles = append(sharedFiles, file)
	}

	sel := selection.New("Select shared file", choices)
	selSharedFiles := selection.NewModel(sel)
	selSharedFiles.Filter = nil
	selSharedFiles.Selection.PageSize = 5

	pw.selSharedFiles = selSharedFiles
	pw.sharedFiles = sharedFiles
	pw.idxSharedFile = -1
	return selSharedFiles.Init()
}

// tryEmbed tries to embed the file in the formEmbed to the current post.
// Returns true if successful.
func (pw *newPostWindow) tryEmbed() error {
	var args embeddedArgs

	args.alt = url.PathEscape(pw.formEmbed.inputs[1].(*textInputHelper).Value())

	filename, err := homedir.Expand(pw.formEmbed.inputs[0].(*textInputHelper).Value())
	if err != nil {
		return err
	}

	if filename != "" {
		data, err := os.ReadFile(filename)
		if err != nil {
			return err
		}

		if pw.estSize+uint64(len(data)) >= rpc.MaxChunkSize {
			return fmt.Errorf("file too big to embed")
		}

		args.typ = mime.TypeByExtension(filepath.Ext(filename))
		id := chainhash.HashH(data).String()[:8]
		pw.embedContent[id] = data
		pseudoData := fmt.Sprintf("[content %s]", id)
		args.data = []byte(pseudoData)
	}

	if pw.idxSharedFile > -1 && pw.idxSharedFile < len(pw.sharedFiles) {
		sf := pw.sharedFiles[pw.idxSharedFile]
		args.download = sf.SF.FID
		args.cost = sf.Cost
		args.size = sf.Size
	}

	embedStr := args.String()
	pw.textArea.InsertString(embedStr)
	return nil
}

func (pw *newPostWindow) createPost(post string) {
	// Replace pseudo-data with data.
	fullPost := replaceEmbeds(post, func(args embeddedArgs) string {
		data := string(args.data)
		if strings.HasPrefix(data, "[content ") {
			id := data[9 : len(args.data)-1]
			args.data = pw.embedContent[id]
		}

		return args.String()

	})
	go pw.as.createPost(fullPost)
}

func (pw newPostWindow) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	// Early check for a quit msg to put us into the shutdown state (to
	// shutdown DB, etc).
	if ss, cmd := maybeShutdown(pw.as, msg); ss != nil {
		return ss, cmd
	}

	// Common handlers for both main post area and embed form.

	switch msg := msg.(type) {
	case tea.WindowSizeMsg: // resize window
		pw.as.winW = msg.Width
		pw.as.winH = msg.Height
		pw.updateTextAreaSize()

	case currentTimeChanged:
		pw.as.footerInvalidate()

	case msgPasteErr:
		pw.as.diagMsg("Unable to paste: %v", msg)
	}

	// Handlers when the embedding form is active.
	if pw.sharing {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch {
			case msg.Type == tea.KeyEnter:
				pw.sharing = false
				choice, err := pw.selSharedFiles.Value()
				if err == nil {
					pw.idxSharedFile = choice.Index
				}
				return pw, nil

			case msg.Type == tea.KeyEsc:
				pw.sharing = false
				return pw, nil
			}
		}

		_, cmd = pw.selSharedFiles.Update(msg)

		return pw, cmd
	}

	if pw.embedding {
		switch msg := msg.(type) {
		case msgCancelForm:
			pw.embedding = false
			pw.formEmbed.clear()
			cmds = pw.formEmbed.setFocus(-1)
			return pw, batchCmds(cmds)

		case msgSubmitForm:
			err := pw.tryEmbed()
			if err == nil {
				return pw, emitMsg(msgCancelForm{})
			}
			pw.embedErr = err

		case msgShowSharedFilesForLink:
			pw.sharing = true
			cmd = pw.listSharedFiles()
			return pw, cmd

		case tea.KeyMsg:
			switch {
			case msg.Type == tea.KeyF2 || msg.Type == tea.KeyEsc:
				// Simulate canceling the form.
				return pw, emitMsg(msgCancelForm{})
			}
		}

		pw.formEmbed, cmd = pw.formEmbed.Update(msg)
		return pw, cmd
	}

	// Handlers when the main post typing form is active.
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyF2:
			pw.embedding = true
			cmds = pw.formEmbed.setFocus(0)

		case msg.Type == tea.KeyEsc:
			// Cancel post.
			return newMainWindowState(pw.as)

		case pw.focusIdx == 1 && msg.Type == tea.KeyEnter:
			post := pw.textArea.Value()
			if post != "" {
				go pw.createPost(post)
			}

			return newMainWindowState(pw.as)

		case msg.Type == tea.KeyTab:
			pw.focusIdx = (pw.focusIdx + 1) % 2
			if pw.focusIdx == 0 {
				pw.textArea.Focus()
			} else {
				pw.textArea.Blur()
			}

		default:
			pw.textArea, cmd = pw.textArea.Update(msg)
			cmds = appendCmd(cmds, cmd)

			var err error
			post := pw.textArea.Value()
			pw.estSize, err = clientintf.EstimatePostSize(post, "")
			if err != nil {
				pw.errMsg = err.Error()
			}
		}

	default:
		// Handle other messages.
		pw.textArea, cmd = pw.textArea.Update(msg)
		cmds = appendCmd(cmds, cmd)
	}

	return pw, batchCmds(cmds)
}

func (pw *newPostWindow) headerView() string {
	msg := " Create Post - F2 to Embed/Link File"
	headerMsg := pw.as.styles.header.Render(msg)
	spaces := pw.as.styles.header.Render(strings.Repeat(" ",
		max(0, pw.as.winW-lipgloss.Width(headerMsg))))
	return headerMsg + spaces
}

func (pw *newPostWindow) footerView() string {
	return pw.as.footerView(pw.debug)
}

func (pw *newPostWindow) embeddingView() string {
	var b strings.Builder

	nbLines := 2 + 1 + pw.formEmbed.lineCount() + 2

	b.WriteString(pw.formEmbed.View())
	b.WriteString("\n")
	if pw.embedErr != nil {
		b.WriteString(pw.as.styles.err.Render(pw.embedErr.Error()))
	}
	b.WriteString("\n")

	if pw.idxSharedFile > -1 && pw.idxSharedFile < len(pw.sharedFiles) {
		b.WriteString(fmt.Sprintf("Linking to shared file %s",
			pw.sharedFiles[pw.idxSharedFile].SF.Filename))
	}

	for i := 0; i < pw.as.winH-nbLines-1; i++ {
		b.WriteString("\n")
	}

	return b.String()
}

func (pw *newPostWindow) sharingView() string {
	var b strings.Builder

	b.WriteString(pw.selSharedFiles.View())

	nbLines := 2 + 2 + 5
	for i := 0; i < pw.as.winH-nbLines-1; i++ {
		b.WriteString("\n")
	}

	return b.String()
}

func (pw newPostWindow) View() string {
	var b strings.Builder

	b.WriteString(pw.headerView())
	b.WriteString("\n\n")

	if pw.sharing {
		b.WriteString(pw.sharingView())
		b.WriteString(pw.footerView())
		return b.String()
	} else if pw.embedding {
		b.WriteString(pw.embeddingView())
		b.WriteString(pw.footerView())
		return b.String()
	}

	b.WriteString(pw.textArea.View())
	b.WriteString("\n\n")

	if pw.errMsg != "" {
		b.WriteString(pw.as.styles.err.Render(pw.errMsg))
	} else {
		estSizeMsg := fmt.Sprintf(" Estimated post size: %s.", hbytes(int64(pw.estSize)))
		if pw.estSize > rpc.MaxChunkSize {
			estSizeMsg = pw.as.styles.err.Render(estSizeMsg)
		}
		b.WriteString(estSizeMsg)
	}
	if pw.focusIdx == 1 {
		b.WriteString(pw.as.styles.focused.Render(" [ Submit ]"))
	} else {
		b.WriteString(" [ Submit ]")
	}
	b.WriteString("\n\n")

	b.WriteString(pw.footerView())

	return b.String()
}

func newNewPostWindow(as *appState) (newPostWindow, tea.Cmd) {
	var cmds []tea.Cmd

	t := newTextAreaModel(as.styles)
	t.Placeholder = "Post"
	t.CharLimit = 0
	t.FocusedStyle.Prompt = as.styles.focused
	t.FocusedStyle.Text = as.styles.focused
	t.BlurredStyle.Prompt = as.styles.noStyle
	t.BlurredStyle.Text = as.styles.noStyle
	t.Focus()

	formEmbed := newFormHelper(as.styles,
		newTextInputHelper(as.styles,
			tihWithPrompt("File to embed: "),
		),
		newTextInputHelper(as.styles,
			tihWithPrompt("Alt Text: "),
		),
		newButtonHelper(as.styles,
			btnWithLabel("[ Link to Shared File ]"),
			btnWithTrailing("\n\n"),
			btnWithFixedMsgAction(msgShowSharedFilesForLink{}),
		),
		newButtonHelper(as.styles,
			btnWithLabel("[ Cancel ]"),
			btnWithTrailing(" "),
			btnWithFixedMsgAction(msgCancelForm{}),
		),
		newButtonHelper(as.styles,
			btnWithLabel(" [ Add Embed ]"),
			btnWithTrailing("\n"),
			btnWithFixedMsgAction(msgSubmitForm{}),
		),
	)

	sel := selection.New("Select shared file", selection.Choices([]string{""}))
	selSharedFiles := selection.NewModel(sel)
	selSharedFiles.Filter = nil
	//selSharedFiles.Update(tea.WindowSizeMsg{Width: as.winW, Height: 10})
	//selSharedFiles.Selection.PageSize = 10

	cmds = appendCmd(cmds, selSharedFiles.Init())

	nw := newPostWindow{
		as:           as,
		textArea:     t,
		formEmbed:    formEmbed,
		embedContent: make(map[string][]byte),

		selSharedFiles: selSharedFiles,
	}

	nw.updateTextAreaSize()

	return nw, batchCmds(cmds)
}
