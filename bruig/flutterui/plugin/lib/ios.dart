import 'definitions.dart';
import 'all_platforms.dart';
import 'mobile.dart';

class IOSPlugin extends PluginPlatform
    with ChanneledPlatform, BaseChanneledCalls, NtfStreams, BaseMobilePlatform {
  @override
  String get minorPlatform => "ios";

  IOSPlugin() {
    super.readAsyncResults();
  }
}
