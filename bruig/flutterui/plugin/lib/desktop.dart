import 'dart:async';
import 'dart:convert';

import 'package:ffi/ffi.dart';
import 'package:flutter/cupertino.dart';
import 'package:golib_plugin/definitions.dart';
import 'dart:ffi';
import 'dart:isolate';
import 'desktop_dynlib.dart';

class _ReadStrData {
  SendPort sp;
  _ReadStrData(this.sp);
}

void _readStrIsolate(_ReadStrData data) async {
  final DynamicLibrary lib = DynamicLibrary.open(desktopLibPath());
  late final ReadStrNative readStr =
      lib.lookupFunction<ReadStrNative, ReadStrNative>('ReadStr');

  for (;;) {
    var s = readStr().toDartString();
    data.sp.send(s);
  }
}

void _readAsyncResultsIsolate(SendPort sp) async {
  final DynamicLibrary lib = DynamicLibrary.open(desktopLibPath());
  final NextCallResultNative nextCallResult =
      lib.lookupFunction<NextCallResultNative, NextCallResultNative>(
          'NextCallResult');
  final CopyCallResultFunc copyCallResult =
      lib.lookupFunction<CopyCallResultNative, CopyCallResultFunc>(
          'CopyCallResult');

  var buffSize = 1024 * 1024;
  var buff = calloc.allocate<Utf8>(buffSize);

  await Future.delayed(const Duration(seconds: 1));
  for (;;) {
    var nr = nextCallResult();

    // Resize response reading buffer if needed.
    if (nr.payloadLen > buffSize) {
      calloc.free(buff);
      buffSize = nr.payloadLen;
      buff = calloc.allocate<Utf8>(buffSize);
    }

    // Copy the payload.
    var rid = copyCallResult(nr.handle, buff);
    var payload = buff.toDartString(length: nr.payloadLen);

    // Send the response.
    var res = [rid, nr.isErr == 1, nr.cmdType, payload];
    sp.send(res);
  }
}

// BaseDesktopPlatform is a mixin that fulfills the GolibPluginPlatform interface
// by loading a dynamic library (.so, .dynlib, .dll) and redirecting all calls to
// that library.
mixin BaseDesktopPlatform on NtfStreams {
  String get majorPlatform => "desktop";
  int id = 1;

  final Map<int, Completer<dynamic>> calls = {};

  // Reference to the dynamic library.
  final DynamicLibrary _lib = DynamicLibrary.open(desktopLibPath());

  // The following fields are references to the dynamic library functions. They
  // are lazily initialized when first used.
  late final SetTagFunc _setTag =
      _lib.lookupFunction<SetTagNative, SetTagFunc>('SetTag');
  late final HelloFunc _hello =
      _lib.lookupFunction<HelloNative, HelloFunc>('Hello');
  late final GetURLNative _getURL =
      _lib.lookupFunction<GetURLNative, GetURLNative>('GetURL');
  late final NextTimeNative _nextTime =
      _lib.lookupFunction<NextTimeNative, NextTimeNative>('NextTime');
  late final WriteStrFunc _writeStr =
      _lib.lookupFunction<WriteStrNative, WriteStrFunc>('WriteStr');
  late final AsyncCallFunc _asyncCall =
      _lib.lookupFunction<AsyncCallNative, AsyncCallFunc>('AsyncCall');

  // From here on are the actual functions to fulfill the GolibPluginPlatform
  // interface by calling into the dynlib.

  Future<void> setTag(String tag) async => _setTag(tag.toNativeUtf8());
  Future<void> hello() async => _hello();
  Future<String> nextTime() async => _nextTime().toDartString();
  Future<void> writeStr(String s) async => _writeStr(s.toNativeUtf8());

  Stream<String> readStream() async* {
    var rp = ReceivePort();
    Isolate.spawn(_readStrIsolate, _ReadStrData(rp.sendPort));
    while (true) {
      await for (String msg in rp) {
        yield msg;
      }
    }
  }

  Future<String> getURL(String url) async {
    GetURLResultNative res = _getURL(url.toNativeUtf8());
    if (res.err.address != nullptr.address) {
      var errStr = res.err.toDartString();
      if (errStr != "") {
        throw errStr;
      }
    }

    return res.res.toDartString();
  }

  Future<dynamic> asyncCall(int cmd, dynamic payload) {
    // Use a fixed clientHandle as we currently only support a single client per UI.
    const clientHandle = 0x12131400;

    var p = jsonEncode(payload).toNativeUtf8();
    var cid = id == -1 ? 1 : id++; // skips 0 as id.
    var c = Completer<dynamic>();
    calls[cid] = c;
    _asyncCall(cmd, cid, clientHandle, p, p.length);
    calloc.free(p);
    return c.future;
  }

  void readAsyncResults() async {
    var rp = ReceivePort();
    Isolate.spawn(_readAsyncResultsIsolate, rp.sendPort);
    while (true) {
      await for (List cmdReply in rp) {
        if (cmdReply.length < 3) {
          debugPrint("Received wrong nb of elements from isolate: $cmdReply");
          continue;
        }
        int id = cmdReply[0];
        bool isError = cmdReply[1];
        int cmdType = cmdReply[2];
        String jsonPayload = cmdReply[3];

        var c = calls[id];
        if (c == null) {
          if (id == 0 && cmdType >= notificationsStartID) {
            try {
              handleNotifications(cmdType, isError, jsonPayload);
            } catch (exception, trace) {
              // Probably a decode error. Keep handling stuff.
              var err =
                  "Unable to handle notification ${cmdType.toRadixString(16)}: $exception\n$trace";
              debugPrint(
                  "Error notification from golib: $err\nPayload: $jsonPayload");
              // ignore: use_rethrow_when_possible
              (() async => throw exception)();
            }
          } else {
            debugPrint("Received reply for unknown call $id - $cmdReply");
          }

          continue;
        }
        calls.remove(id);

        dynamic payload;
        if (jsonPayload != "") {
          payload = jsonDecode(jsonPayload);
        }

        if (isError) {
          c.completeError(payload);
        } else {
          c.complete(payload);
        }
      }
    }
  }
}
