package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// embedWidget is used to display the new embed screen in new posts and other
// places that allow adding an embed.
type pluginWidget struct {
	initless
	as *appState

	showing   bool
	formEmbed formHelper
	embedErr  error

	addEmbedCB func(id string, data string) error
}

func (ew *pluginWidget) active() bool {
	return ew.showing
}

func (ew *pluginWidget) activate() []tea.Cmd {
	ew.showing = true
	ew.embedErr = nil
	return ew.formEmbed.setFocus(0)
}

func (ew *pluginWidget) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd tea.Cmd
	)
	if ew.showing {

		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch {
			case msg.Type == tea.KeyEsc:
				ew.showing = false
				return ew, nil
			}
		default:
			// fmt.Printf("%s", msg)
			ew.formEmbed.Update(msg)
		}

		return ew, cmd
	}

	return ew, cmd
}

func (ew *pluginWidget) showingView() string {
	var b strings.Builder

	b.WriteString("plugin view\n\n")
	b.WriteString(ew.formEmbed.View())
	nbLines := 2 + 2 + 5
	for i := 0; i < ew.as.winH-nbLines-1; i++ {
		b.WriteString("\n")
	}

	return b.String()
}

func (ew *pluginWidget) View() string {
	if ew.showing {
		return ew.showingView()
	}
	return ""
}

func newPluginWidget(as *appState, addEmbedCB func(string, string) error) *pluginWidget {
	styles := as.styles.Load()

	formEmbed := newFormHelper(styles,
		newTextInputHelper(styles,
			tihWithPrompt("Plugin: "),
		),
		newTextInputHelper(styles,
			tihWithPrompt(""),
		),
		newButtonHelper(styles,
			btnWithLabel("[ Cancel ]"),
			btnWithTrailing(" "),
			btnWithFixedMsgAction(msgCancelForm{}),
		),
		newButtonHelper(styles,
			btnWithLabel(" [ Add Embed ]"),
			btnWithTrailing("\n"),
			btnWithFixedMsgAction(msgSubmitForm{}),
		),
	)

	ew := &pluginWidget{
		as:         as,
		formEmbed:  formEmbed,
		addEmbedCB: addEmbedCB,
	}

	return ew
}
