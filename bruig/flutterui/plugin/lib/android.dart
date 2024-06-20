import 'definitions.dart';
import 'all_platforms.dart';
import 'mobile.dart';

class AndroidPlugin extends PluginPlatform
    with ChanneledPlatform, BaseChanneledCalls, NtfStreams, BaseMobilePlatform {
  @override
  String get minorPlatform => "android";

  AndroidPlugin() {
    readAsyncResults();
  }

  @override
  Future<void> startForegroundSvc() async =>
      await channel.invokeMethod('startFgSvc');

  @override
  Future<void> stopForegroundSvc() async =>
      await channel.invokeMethod('stopFgSvc');

  @override
  Future<void> setNtfnsEnabled(bool enabled) async => await channel
      .invokeMethod('setNtfnsEnabled', <String, dynamic>{"enabled": enabled});
}
