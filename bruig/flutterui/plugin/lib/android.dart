import 'dart:convert';

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

  // Miniaudio does not support listing audio devices on Android, so this needs
  // a platform-specific implementation.
  @override
  Future<AudioDevices> listAudioDevices() async {
    var jsonRes = await channel.invokeMethod("listAudioDevices") as String;
    var jsonPayload = jsonDecode(jsonRes);
    return AudioDevices.fromJson(jsonPayload);
  }
}
