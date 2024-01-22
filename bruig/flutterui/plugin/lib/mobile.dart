import 'dart:async';
import 'dart:convert';

import 'package:flutter/services.dart';
import 'all_platforms.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/mock.dart';

T _cast<T>(x) => x is T ? x : throw "Not a $T";

mixin BaseMobilePlatform on ChanneledPlatform, NtfStreams {
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

  void readAsyncResults() async {
    var channel = const EventChannel('cmdResultLoop');
    var stream = channel.receiveBroadcastStream();
    await for (var e in stream) {
      int id = e["id"] ?? 0;
      String err = e["error"] ?? "";
      String jsonPayload = e["payload"] ?? "";
      int cmdType = e["type"] ?? 0;
      bool isError = err != "";

      // Pseudo-encode errors as json to imitate desktop.
      if (isError) {
        jsonPayload = "\"$err\"";
      }

      var c = calls[id];
      if (c == null) {
        if (id == 0 && cmdType >= notificationsStartID) {
          try {
            handleNotifications(cmdType, isError, jsonPayload);
          } catch (exception, trace) {
            // Probably a decode error. Keep handling stuff.
            var err =
                "Unable to handle notification ${cmdType.toRadixString(16)}: $exception\n$trace";
            print(err);
            print(jsonPayload);
            (() async => throw exception)();
          }
        } else {
          print("Received reply for unknown call $id - $e");
        }

        continue;
      }
      calls.remove(id);

      dynamic payload;
      if (jsonPayload != "") {
        jsonPayload = jsonPayload.trim().replaceAll("\n", "");
        jsonPayload = jsonPayload.trim().replaceAll("\t", "");
        payload = jsonDecode(jsonPayload.trim());
      }

      if (isError) {
        c.completeError(err);
      } else {
        c.complete(payload);
      }
    }
  }

  int id = 1; // id of the next command to send to the lib.

  // map of oustanding calls.
  final Map<int, Completer<dynamic>> calls = {};

  Future<dynamic> asyncCall(int cmd, dynamic payload) async {
    var jsonPayload = jsonEncode(payload);

    // Use a fixed clientHandle as we currently only support a single client per UI.
    const clientHandle = 0x12131400;
    var cid = id == -1 ? 1 : id++; // skips 0 as id.

    var c = Completer<dynamic>();
    calls[cid] = c;
    Map<String, dynamic> args = {
      'typ': cmd,
      'id': cid,
      'handle': clientHandle,
      'payload': jsonPayload,
    };
    channel.invokeMethod('asyncCall', args);
    return c.future;
  }
}
