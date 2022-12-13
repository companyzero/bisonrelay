package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// focusableWidget are widgets that can get/lose focus.
type focusableWidget interface {
	Focus() tea.Cmd
	Blur() tea.Cmd
}

// trailingWidget are widgets with a trailing() function to draw their line
// ending.
type trailingWidget interface {
	trailing() string
}

// actionableWidget are widgets that handle <enter> directly.
type actionableWidget interface {
	action(msg tea.Msg) tea.Cmd
}

type clearableWidget interface {
	clear()
}

type formHelper struct {
	initless

	focusIndex int
	inputs     []tea.Model
}

func (fh *formHelper) setFocus(fi int) []tea.Cmd {
	var cmds []tea.Cmd

	fh.focusIndex = fi
	for i := 0; i < len(fh.inputs); i++ {
		if i == fh.focusIndex {
			// Set focused state
			if fw, ok := fh.inputs[i].(focusableWidget); ok {
				cmd := fw.Focus()
				cmds = appendCmd(cmds, cmd)
			}
			continue
		}

		// Remove focused state
		if fw, ok := fh.inputs[i].(focusableWidget); ok {
			cmds = appendCmd(cmds, fw.Blur())
		}
	}

	return cmds
}

func (fh *formHelper) focused() tea.Model {
	i := fh.focusIndex
	if i < 0 || i >= len(fh.inputs) {
		return nil
	}

	return fh.inputs[i]
}

func (fh *formHelper) updateFocused(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	i := fh.focusIndex
	if i < 0 || i >= len(fh.inputs) {
		return cmd
	}

	fh.inputs[i], cmd = fh.inputs[i].Update(msg)

	return cmd
}

func (fh *formHelper) cycleFocused(msg tea.KeyMsg) []tea.Cmd {
	// Call action if enter was pressed.
	if focused := fh.focused(); focused != nil && msg.Type == tea.KeyEnter {
		if aw, ok := focused.(actionableWidget); ok {
			cmd := aw.action(msg)
			return appendCmd(nil, cmd)
		}
	}

	// Cycle indexes
	s := msg.String()
	fi := fh.focusIndex
	if s == "up" || s == "shift+tab" {
		fi--
	} else {
		fi++
	}

	if fi >= len(fh.inputs) {
		fi = 0
	} else if fi < 0 {
		fi = len(fh.inputs) - 1
	}

	return fh.setFocus(fi)
}

func (fh formHelper) Update(msg tea.Msg) (formHelper, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab", "enter", "up", "down":
			cmds := fh.cycleFocused(msg)
			return fh, batchCmds(cmds)
		}
	}

	cmd := fh.updateFocused(msg)
	return fh, cmd
}

// lineCount returns the number of lines used for the View() method.
func (fh formHelper) lineCount() int {
	count := 0
	for i := range fh.inputs {
		if tw, ok := fh.inputs[i].(trailingWidget); ok {
			count += strings.Count(tw.trailing(), "\n")
		} else {
			count += 2
		}
	}
	return count
}

func (fh *formHelper) clear() {
	for i := range fh.inputs {
		if cw, ok := fh.inputs[i].(clearableWidget); ok {
			cw.clear()
		}
	}
}

func (fh formHelper) View() string {
	var b strings.Builder
	for i := range fh.inputs {
		b.WriteString(fh.inputs[i].View())
		if tw, ok := fh.inputs[i].(trailingWidget); ok {
			b.WriteString(tw.trailing())
		} else {
			b.WriteString("\n\n")
		}
	}

	return b.String()
}

func newFormHelper(styles *theme, inputs ...tea.Model) formHelper {
	fh := formHelper{
		inputs: inputs,
	}

	if len(inputs) > 0 {
		if fw, ok := inputs[0].(focusableWidget); ok {
			fw.Focus()
		}
	}
	return fh
}

type buttonHelperOption func(model *buttonHelper)

func btnWithLabel(label string) buttonHelperOption {
	return func(model *buttonHelper) {
		model.label = label
	}
}

func btnWithTrailing(trailing string) buttonHelperOption {
	return func(model *buttonHelper) {
		model.trailingTxt = trailing
	}
}

func btnWithFixedMsgAction(msg tea.Msg) buttonHelperOption {
	return func(model *buttonHelper) {
		model.actionCmd = func() tea.Msg {
			return msg
		}
	}
}

type buttonHelper struct {
	initless
	label       string
	trailingTxt string
	focus       bool
	actionCmd   tea.Cmd
	styles      *theme
}

func (btn *buttonHelper) Focus() tea.Cmd {
	btn.focus = true
	return nil
}

func (btn *buttonHelper) Blur() tea.Cmd {
	btn.focus = false
	return nil
}

func (btn *buttonHelper) trailing() string {
	return btn.trailingTxt
}

func (btn *buttonHelper) action(msg tea.Msg) tea.Cmd {
	return btn.actionCmd
}

func (btn *buttonHelper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return btn, nil
}

func (btn *buttonHelper) View() string {
	if btn.focus {
		return btn.styles.focused.Render(btn.label)
	}
	return btn.label
}

func newButtonHelper(styles *theme, opts ...buttonHelperOption) *buttonHelper {
	btn := &buttonHelper{
		styles: styles,
	}
	for _, opt := range opts {
		opt(btn)
	}
	return btn
}
