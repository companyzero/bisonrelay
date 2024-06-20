library golib_plugin;

import 'dart:io';
import 'package:golib_plugin/android.dart';
import 'package:golib_plugin/ios.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/linux.dart';
import 'package:golib_plugin/macos.dart';
import 'package:golib_plugin/windows.dart';

PluginPlatform _newPluginPlatform() {
  if (Platform.isLinux) {
    return LinuxPlugin();
  } else if (Platform.isMacOS) {
    return MacOSPlugin();
  } else if (Platform.isWindows) {
    return WindowsPlugin();
  } else if (Platform.isAndroid) {
    return AndroidPlugin();
  } else if (Platform.isIOS) {
    return IOSPlugin();
  }

  throw "unknown platform OS ${Platform.operatingSystem}";
}

/*
// Disabled due to being out of date.
MockPlugin _mockGolib() {
  return new MockPlugin();
}

// MockGolib is used for in-developement functions that have not yet been
// included in the Golib interface.
late final MockPlugin MockGolib = _mockGolib();
*/

// ignore: non_constant_identifier_names
final Golib = _newPluginPlatform();
