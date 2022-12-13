//go:build linux && amd64
// +build linux,amd64

package golibbuilder

//go:generate go build -o ../build/linux/amd64/golib.so -buildmode=c-shared ../golib/sharedlib
