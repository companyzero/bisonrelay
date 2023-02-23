package singlesetmap

import (
	"testing"

	"github.com/companyzero/bisonrelay/internal/assert"
)

func TestSingleSetMap(t *testing.T) {
	m := &Map[string]{}
	k1 := "key 1"
	k2 := "key 2"
	assert.DeepEqual(t, m.Set(k1), false)
	assert.DeepEqual(t, m.Set(k1), true)
	assert.DeepEqual(t, m.Set(k2), false)
	assert.DeepEqual(t, m.Set(k2), true)
}
