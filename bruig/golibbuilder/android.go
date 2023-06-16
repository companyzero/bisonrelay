//go:build android
// +build android

package golibbuilder

//go:generate mkdir -p ../build/android
//go:generate gomobile bind -target android -o ../build/android/golib.aar ../golib
