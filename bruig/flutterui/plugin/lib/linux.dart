import 'dart:io';
import 'package:path/path.dart' as path;
import 'definitions.dart';
import 'all_platforms.dart';
import 'desktop.dart';

class LinuxPlugin extends PluginPlatform
    with
        ChanneledPlatform,
        BaseChanneledCalls,
        NtfStreams,
        BaseDesktopPlatform {
  String get minorPlatform => "linux";

  LinuxPlugin() {
    super.readAsyncResults();
  }
}
