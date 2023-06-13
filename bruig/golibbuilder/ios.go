//go:build ios
// +build ios

package golibbuilder

//go:generate gomobile bind -target ios -o ../build/ios/Golib.xcframework ../golib
//go:generate cp -r ../build/ios/Golib.xcframework ../flutterui/plugin/ios/Frameworks/
