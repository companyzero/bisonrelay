//go:build darwin && amd64
// +build darwin,amd64

package golibbuilder

//go:generate go build -o ../build/macos/amd64/golib.dylib -buildmode=c-shared ../golib/sharedlib
//go:generate cp -r ../build/macos/amd64/golib.dylib ../flutterui/plugin/macos/libs
