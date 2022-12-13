//go:build windows && amd64
// +build windows,amd64

package golibbuilder

//go:generate go build -o ../build/windows/amd64/golib.dll -buildmode=c-shared ../golib/sharedlib
