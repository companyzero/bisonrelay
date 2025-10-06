package seederserver

import "errors"

var (
	errNoAlias          = errors.New("alias is empty")
	errLastUpdateTooOld = errors.New("last update is too old")
)
