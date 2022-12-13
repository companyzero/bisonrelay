import 'definitions.dart';
import 'all_platforms.dart';
import 'desktop.dart';

class WindowsPlugin extends PluginPlatform
    with
        ChanneledPlatform,
        BaseChanneledCalls,
        NtfStreams,
        BaseDesktopPlatform {
  String get minorPlatform => "windows";

  WindowsPlugin() {
    super.readAsyncResults();
  }
}
