import 'dart:convert';

import 'package:flutter/services.dart';
import 'package:golib_plugin/golib_plugin.dart';

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

  @override
  Future<dynamic> lastAndroidIntent() async {
    return await channel.invokeMethod("getLastIntent");
  }
}

class AndroidIntent {
  final String action;
  final String data;
  final String? sessRV;
  final String? inviter;

  AndroidIntent(this.action, this.data, {this.sessRV, this.inviter});
}

// Get the last Android intent directed to the app and stored in either the
// MainActivity or by the golib plugin. Both need to be checked because the
// receiver of the intent depends on the state of the app (whether it was in the
// background, flutter attached or not, foreground process running or not, etc).
Future<AndroidIntent?> lastAndroidIntent() async {
  const methodChan = MethodChannel("org.bisonrelay.bruig.mainActivityChannel");
  var res = await methodChan.invokeMethod("getLastIntent") ??
      await Golib.lastAndroidIntent();
  if (res == null) {
    return null;
  }
  return AndroidIntent(res["action"] ?? "", res["data"] ?? "",
      sessRV: res["sessRV"], inviter: res["inviter"]);
}

Future<void> closeAllAndroidNativeNotifications() async {
  const methodChan = MethodChannel("org.bisonrelay.bruig.mainActivityChannel");
  await methodChan.invokeMethod("closeAllNotifications");
}
