package main

import (
	"fmt"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/rpc"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/erikgeiser/promptkit/selection"
	"github.com/mitchellh/go-homedir"
)

// embedWidget is used to display the new embed screen in new posts and other
// places that allow adding an embed.
type embedWidget struct {
	initless
	as *appState

	embedding bool
	formEmbed formHelper
	embedErr  error

	sharing        bool
	sharedFiles    []clientdb.SharedFileAndShares
	selSharedFiles *selection.Model
	idxSharedFile  int

	addEmbedCB func(id string, data []byte, embedStr string) error
}

func (ew *embedWidget) listSharedFiles() tea.Cmd {
	files, err := ew.as.c.ListLocalSharedFiles()
	if err != nil {
		ew.as.diagMsg("Unable to list local shared files: %v", err)
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

	ew.selSharedFiles = selSharedFiles
	ew.sharedFiles = sharedFiles
	ew.idxSharedFile = -1
	return selSharedFiles.Init()
}

func (ew *embedWidget) active() bool {
	return ew.sharing || ew.embedding
}

func (ew *embedWidget) activate() []tea.Cmd {
	ew.idxSharedFile = -1
	ew.embedding = true
	ew.embedErr = nil
	return ew.formEmbed.setFocus(0)
}

// tryEmbed tries to create the embed and calls the addEmbedCB.
func (ew *embedWidget) tryEmbed() error {
	var args embeddedArgs

	args.alt = url.PathEscape(ew.formEmbed.inputs[1].(*textInputHelper).Value())

	filename, err := homedir.Expand(ew.formEmbed.inputs[0].(*textInputHelper).Value())
	if err != nil {
		return err
	}

	var data []byte
	var id string

	if filename != "" {
		data, err = os.ReadFile(filename)
		if err != nil {
			return err
		}

		if uint64(len(data)) > rpc.MaxChunkSize {
			return fmt.Errorf("file too big to embed")
		}

		args.typ = mime.TypeByExtension(filepath.Ext(filename))
		id = chainhash.HashH(data).String()[:8]
		pseudoData := fmt.Sprintf("[content %s]", id)
		args.data = []byte(pseudoData)
	}

	if ew.idxSharedFile > -1 && ew.idxSharedFile < len(ew.sharedFiles) {
		sf := ew.sharedFiles[ew.idxSharedFile]
		args.download = sf.SF.FID
		args.cost = sf.Cost
		args.size = sf.Size
	}

	embedStr := args.String()
	return ew.addEmbedCB(id, data, embedStr)
}

func (ew *embedWidget) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	if ew.sharing {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch {
			case msg.Type == tea.KeyEnter:
				ew.sharing = false
				choice, err := ew.selSharedFiles.Value()
				if err == nil {
					ew.idxSharedFile = choice.Index
				}
				return ew, nil

			case msg.Type == tea.KeyEsc:
				ew.sharing = false
				return ew, nil
			}
		}

		_, cmd = ew.selSharedFiles.Update(msg)

		return ew, cmd
	}

	if ew.embedding {
		switch msg := msg.(type) {
		case msgCancelForm:
			ew.embedding = false
			ew.formEmbed.clear()
			cmds = ew.formEmbed.setFocus(-1)
			return ew, batchCmds(cmds)

		case msgSubmitForm:
			err := ew.tryEmbed()
			if err == nil {
				return ew, emitMsg(msgCancelForm{})
			}
			ew.embedErr = err

		case msgShowSharedFilesForLink:
			ew.sharing = true
			cmd = ew.listSharedFiles()
			return ew, cmd

		case tea.KeyMsg:
			switch {
			case msg.Type == tea.KeyF2 || msg.Type == tea.KeyEsc:
				// Simulate canceling the form.
				return ew, emitMsg(msgCancelForm{})
			}
		}

		ew.formEmbed, cmd = ew.formEmbed.Update(msg)
		return ew, cmd
	}

	return ew, cmd
}

func (ew *embedWidget) embeddingView() string {
	var b strings.Builder

	nbLines := 2 + 1 + ew.formEmbed.lineCount() + 2

	b.WriteString(ew.formEmbed.View())
	b.WriteString("\n")
	if ew.embedErr != nil {
		b.WriteString(ew.as.styles.err.Render(ew.embedErr.Error()))
	}
	b.WriteString("\n")

	if ew.idxSharedFile > -1 && ew.idxSharedFile < len(ew.sharedFiles) {
		b.WriteString(fmt.Sprintf("Linking to shared file %s",
			ew.sharedFiles[ew.idxSharedFile].SF.Filename))
	}

	for i := 0; i < ew.as.winH-nbLines-1; i++ {
		b.WriteString("\n")
	}

	return b.String()
}

func (ew *embedWidget) sharingView() string {
	var b strings.Builder

	b.WriteString(ew.selSharedFiles.View())

	nbLines := 2 + 2 + 5
	for i := 0; i < ew.as.winH-nbLines-1; i++ {
		b.WriteString("\n")
	}

	return b.String()
}

func (ew *embedWidget) View() string {
	if ew.sharing {
		return ew.sharingView()
	} else if ew.embedding {
		return ew.embeddingView()
	}
	return ""
}

func newEmbedWidget(as *appState, addEmbedCB func(string, []byte, string) error) *embedWidget {
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

	ew := &embedWidget{
		as:             as,
		formEmbed:      formEmbed,
		selSharedFiles: selSharedFiles,
		addEmbedCB:     addEmbedCB,
	}

	return ew
}
