//go:build android
// +build android

package golibbuilder

//go:generate mkdir -p ../build/android
//go:generate gomobile bind -target android -v -androidapi 26 -o  ../build/android/golib.aar ../golib
