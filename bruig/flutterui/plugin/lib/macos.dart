import 'definitions.dart';
import 'all_platforms.dart';
import 'desktop.dart';

class MacOSPlugin extends PluginPlatform
    with
        ChanneledPlatform,
        BaseChanneledCalls,
        NtfStreams,
        BaseDesktopPlatform {
  @override
  String get minorPlatform => "macos";

  MacOSPlugin() {
    super.readAsyncResults();
  }
}
