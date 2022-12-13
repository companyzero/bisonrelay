# bruig - Bison Relay UI (graphical)

# Building

Requires [flutter](https://wwww.flutter.dev). Requires xcode for macos/ios.
Requires Android NDK to build for android. 

## Native Desktop

This will build the desktop version for the current system (no cross-compiling
yet).

Replace `linux` with either `macos` or `windows`.

```shell
$ go generate ./golibbuilder
$ cd flutterui/bruig
$ flutter build linux
```

## Mobile Builds

Use the applicable tags (android, ios):

```shell
$ go generate -tags android ./golibbuilder
$ go generate -tags ios ./golibbuilder
$ cd fd/fd
$ flutter build android
$ flutter build ios
```
