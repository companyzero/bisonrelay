//go:build android
// +build android

package golibbuilder

//go:generate gomobile bind -target android -o ../build/android/golib.aar ../golib
