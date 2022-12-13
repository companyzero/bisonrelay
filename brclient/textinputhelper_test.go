package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTextInputHelperIsFocusable(t *testing.T) {
	styles, _ := newTheme(nil)
	ti := newTextInputHelper(styles)
	inputs := []tea.Model{
		ti,
	}
	if _, ok := inputs[0].(focusableWidget); !ok {
		t.Fatalf("textInputHelper not focusable")
	}
}
