package main

import (
	"testing"

	"github.com/companyzero/bisonrelay/internal/assert"
)

// TestNilConfigTheme tests that you can initialize a theme with nil config.
func TestNilConfigTheme(t *testing.T) {
	_, err := newTheme(nil)
	assert.NilErr(t, err)
}
