import 'definitions.dart';
import 'all_platforms.dart';
import 'desktop.dart';

class LinuxPlugin extends PluginPlatform
    with
        ChanneledPlatform,
        BaseChanneledCalls,
        NtfStreams,
        BaseDesktopPlatform {
  @override
  String get minorPlatform => "linux";

  LinuxPlugin() {
    super.readAsyncResults();
  }
}
