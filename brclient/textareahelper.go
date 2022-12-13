package main

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/companyzero/bisonrelay/internal/strescape"
	rw "github.com/mattn/go-runewidth"
)

// textAreaModel is a helper module to abstract a (possibly dynamically sized)
// text area. It works around quirks in the original text area implementation.
type textAreaModel struct {
	initless
	textarea.Model
}

func newTextAreaModel(theme *theme) textAreaModel {
	ta := textarea.New()
	if !theme.blink {
		ta.Cursor.SetCursorMode(cursor.CursorStatic)
	}

	// Fix for light themes not displaying colors correctly.
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	return textAreaModel{
		Model: ta,
	}
}

func (t *textAreaModel) countWrappedLines(runes []rune) int {
	repeatSpaces := func(n int) []rune {
		return []rune(strings.Repeat(string(' '), n))
	}

	var (
		lines  = [][]rune{{}}
		word   = []rune{}
		row    int
		spaces int
		width  = t.Model.Width()
	)

	// Word wrap the runes
	for _, r := range runes {
		if unicode.IsSpace(r) {
			spaces++
		} else {
			word = append(word, r)
		}

		if spaces > 0 {
			if rw.StringWidth(string(lines[row]))+rw.StringWidth(string(word))+spaces > width {
				row++
				lines = append(lines, []rune{})
				lines[row] = append(lines[row], word...)
				lines[row] = append(lines[row], repeatSpaces(spaces)...)
				spaces = 0
				word = nil
			} else {
				lines[row] = append(lines[row], word...)
				lines[row] = append(lines[row], repeatSpaces(spaces)...)
				spaces = 0
				word = nil
			}
		} else {
			// If the last character is a double-width rune, then we may not be able to add it to this line
			// as it might cause us to go past the width.
			lastCharLen := rw.RuneWidth(word[len(word)-1])
			if rw.StringWidth(string(word))+lastCharLen > width {
				// If the current line has any content, let's move to the next
				// line because the current word fills up the entire line.
				if len(lines[row]) > 0 {
					row++
					lines = append(lines, []rune{})
				}
				lines[row] = append(lines[row], word...)
				word = nil
			}
		}
	}

	if rw.StringWidth(string(lines[row]))+rw.StringWidth(string(word))+spaces >= width {
		lines = append(lines, []rune{})
		lines[row+1] = append(lines[row+1], word...)
		// We add an extra space at the end of the line to account for the
		// trailing space at the end of the previous soft-wrapped lines so that
		// behaviour when navigating is consistent and so that we don't need to
		// continually add edges to handle the last line of the wrapped input.
		spaces++
		lines[row+1] = append(lines[row+1], repeatSpaces(spaces)...)
		row += 1
	} else {
		lines[row] = append(lines[row], word...)
		spaces++
		lines[row] = append(lines[row], repeatSpaces(spaces)...)
	}

	return row
}

// totalLineCount attempts to return the correct total number of lines
// (both hard and soft wrapped) of the editing text area.
func (t *textAreaModel) totalLineCount() int {
	// This is a crappy way to calculate this, but is needed because the
	// current version of text area does not track this information by
	// itself. This basically recreates the textarea's wrap() function to be
	// able to accurately count the lines.
	lineCount := 0
	lines := strings.Split(t.Model.Value(), "\n")
	for _, line := range lines {
		lineCount += 1
		lineCount += t.countWrappedLines([]rune(line))
	}
	return lineCount
}

func (t *textAreaModel) lastLineSoftLineCount() int {
	lines := strings.Split(t.Model.Value(), "\n")
	if len(lines) == 0 {
		return 0
	}
	return t.countWrappedLines([]rune(lines[len(lines)-1]))
}

func (t *textAreaModel) recalcDynHeight(winW, winH int) int {
	maxLines := (winH - 2) / 3 // Up to 1/3 of the screen
	lineCount := t.totalLineCount()
	if t.Model.LineInfo().ColumnOffset >= winW-1 {
		// Increase height when near the end of the line to avoid
		// scrolling the first line out.
		lineCount += 1
	}
	lineCount = clamp(lineCount, 1, maxLines)
	if lineCount != t.Model.Height() {
		t.Model.SetHeight(lineCount)
	}

	return lineCount
}

func (t textAreaModel) Update(msg tea.Msg) (textAreaModel, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.String() == "alt+[":
			// Ignore (textarea bug)

		case msg.String() == "ctrl+v":
			cmds = appendCmd(cmds, paste)

		default:
			hasLN := strings.ContainsAny(msg.String(), "\n\r")
			if (msg.Type == tea.KeyRunes) && len(msg.String()) > 1 && hasLN {
				lines := strings.Split(strescape.CannonicalizeNL(msg.String()), "\n")
				for _, line := range lines {
					msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(line)}
					t.Model, cmd = t.Model.Update(msg)
					cmds = appendCmd(cmds, cmd)
					enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
					t.Model, cmd = t.Model.Update(enterMsg)
					cmds = appendCmd(cmds, cmd)
				}
			} else {
				t.Model, cmd = t.Model.Update(msg)
				cmds = appendCmd(cmds, cmd)
			}
		}

	case msgPaste:
		// Rewrite this message as if it were typed by the user, such
		// that the paste is inserted at the cursor location and handled
		// by the standard handler.
		newMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(msg)}
		cmds = appendCmd(cmds, func() tea.Msg { return newMsg })

	default:
		// Handle other messages.
		t.Model, cmd = t.Model.Update(msg)
		cmds = appendCmd(cmds, cmd)
	}

	return t, batchCmds(cmds)
}

func (t textAreaModel) View() string {
	return t.Model.View()
}
