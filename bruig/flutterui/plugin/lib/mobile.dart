import 'package:flutter/services.dart';
import 'all_platforms.dart';

T _cast<T>(x) => x is T ? x : throw "Not a $T";

mixin BaseMobilePlatform on ChanneledPlatform {
  String get majorPlatform => "mobile";

  Future<void> hello() async {
    await channel.invokeMethod('hello');
  }

  Future<String> getURL(String url) async =>
      await channel.invokeMethod('getURL', <String, dynamic>{'url': url});

  Future<void> setTag(String tag) async =>
      await channel.invokeMethod('setTag', <String, dynamic>{'tag': tag});

  Future<String> nextTime() async => await channel.invokeMethod('nextTime');

  Future<void> writeStr(String s) async =>
      await channel.invokeMethod('writeStr', <String, dynamic>{'s': s});

  Stream<String> readStream() {
    var channel = EventChannel('readStream');
    var stream = channel.receiveBroadcastStream();
    return stream.map<String>((e) => _cast<String>(e));
  }

  Future<String> asyncCall(int cmd, dynamic payload) async =>
      throw "unimplemented";
}
