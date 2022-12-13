import 'package:flutter/services.dart';

mixin ChanneledPlatform {
  final MethodChannel channel = const MethodChannel('golib_plugin');
}

mixin BaseChanneledCalls on ChanneledPlatform {
  Future<String?> get platformVersion async {
    final String? version = await channel.invokeMethod('getPlatformVersion');
    return version;
  }
}
