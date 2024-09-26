package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/companyzero/bisonrelay/internal/strescape"
)

type textInputHelperOption func(model *textinput.Model)

func tihWithPrompt(prompt string) textInputHelperOption {
	return func(model *textinput.Model) {
		model.Prompt = prompt
	}
}

func tihWithValue(value string) textInputHelperOption {
	return func(model *textinput.Model) {
		model.SetValue(value)
	}

}

// textInputHelper is a helper to work around textinput.Model quirks and to
// ease creating forms.
type textInputHelper struct {
	initless
	textinput.Model
	styles *theme
}

func (input *textInputHelper) clear() {
	input.Model.SetValue("")
}

func (input *textInputHelper) Focus() tea.Cmd {
	input.Model.PromptStyle = input.styles.focused
	input.Model.TextStyle = input.styles.focused
	return input.Model.Focus()
}

func (input *textInputHelper) Blur() tea.Cmd {
	input.Model.PromptStyle = input.styles.noStyle
	input.Model.TextStyle = input.styles.noStyle
	input.Model.Blur()
	return nil
}

func (input *textInputHelper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		default:
			msgStr := msg.String()
			if msg.Paste {
				msgStr = sanitizePastedMsgString(msgStr)
			}
			hasLN := strings.ContainsAny(msgStr, "\n\r")
			if hasLN {
				lines := strescape.CannonicalizeNL(msgStr)
				msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(lines)}
				input.Model, cmd = input.Model.Update(msg)
				cmds = appendCmd(cmds, cmd)
			} else {
				input.Model, cmd = input.Model.Update(msg)
				cmds = appendCmd(cmds, cmd)
			}
		}
	}

	return input, batchCmds(cmds)
}

func newTextInputHelper(styles *theme, opts ...textInputHelperOption) *textInputHelper {
	// TODO: parametrize based on styles.blink
	c := cursor.New()
	c.Style = styles.cursor
	c.SetMode(cursor.CursorBlink)

	model := textinput.New()
	model.Cursor = c

	input := textInputHelper{
		styles: styles,
		Model:  model,
	}

	for _, opt := range opts {
		opt(&input.Model)
	}

	return &input
}
