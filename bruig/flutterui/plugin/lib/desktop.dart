import 'dart:async';
import 'dart:convert';

import 'package:ffi/ffi.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/mock.dart';
import 'dart:ffi';
import 'dart:isolate';
import 'desktop_dynlib.dart';

class _readStrData {
  SendPort sp;
  _readStrData(this.sp);
}

void _readStrIsolate(_readStrData data) async {
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

  await Future.delayed(Duration(seconds: 1));
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

  final Map<int, Completer<dynamic>> calls = Map<int, Completer<dynamic>>();

  // Reference to the dynamic library.
  final DynamicLibrary _lib = DynamicLibrary.open(desktopLibPath());

  // The following fields are references to the dynamic library functions. They
  // are lazily initialized when first used.
  late final SetTagFunc _setTag =
      this._lib.lookupFunction<SetTagNative, SetTagFunc>('SetTag');
  late final HelloFunc _hello =
      this._lib.lookupFunction<HelloNative, HelloFunc>('Hello');
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
    var rp = new ReceivePort();
    Isolate.spawn(_readStrIsolate, _readStrData(rp.sendPort));
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
    var c = new Completer<dynamic>();
    calls[cid] = c;
    _asyncCall(cmd, cid, clientHandle, p, p.length);
    calloc.free(p);
    return c.future;
  }

  handleNotifications(int cmd, bool isError, String jsonPayload) {
    dynamic payload;
    if (jsonPayload != "") {
      payload = jsonDecode(jsonPayload);
    }
    //print("XXXXXXXX $payload");

    switch (cmd) {
      case NTInviteAccepted:
        isError
            ? ntfAcceptedInvites.addError(payload)
            : ntfAcceptedInvites.add(RemoteUser.fromJson(payload));
        break;

      case NTInviteErrored:
        throw Exception(payload);

      case NTPM:
        isError
            ? ntfChatEvents.addError(payload)
            : ntfChatEvents.add(PM.fromJson(payload));
        break;

      case NTLocalIDNeeded:
        ntfConfs.add(ConfNotification(cmd, null));
        break;

      case NTFConfServerCert:
        ntfConfs.add(ConfNotification(cmd, ServerCert.fromJson(payload)));
        break;

      case NTServerSessChanged:
        ntfServerSess.add(ServerSessionState.fromJson(payload));
        break;

      case NTNOP:
        // NOP.
        break;

      case NTInvitedToGC:
        var evnt = GCInvitation.fromJson(payload);
        ntfChatEvents.add(evnt);
        break;

      case NTUserAcceptedGCInvite:
        var evnt = InviteToGC.fromJson(payload);
        ntfChatEvents.add(GCUserEvent(
            evnt.uid, evnt.gc, "Accepted our invitation to join the GC"));
        break;

      case NTGCJoined:
        var gc = GCAddressBookEntry.fromJson(payload);
        ntfGCListUpdates.add(gc);
        break;

      case NTGCMessage:
        var gcm = GCMsg.fromJson(payload);
        ntfChatEvents.add(gcm);
        break;

      case NTKXCompleted:
        ntfAcceptedInvites.add(RemoteUser.fromJson(payload));
        break;

      case NTTipReceived:
        var args = PayTipArgs.fromJson(payload);
        var it = InflightTip(nextEID(), args.uid, args.amount);
        it.state = ITS_received;
        ntfChatEvents.add(it);
        break;

      case NTPostReceived:
        var pr = PostSummary.fromJson(payload);
        ntfPostsFeed.add(pr);
        ntfChatEvents.add(FeedPostEvent(pr.from, pr.id, pr.title));
        break;

      case NTPostStatusReceived:
        var psr = PostStatusReceived.fromJson(payload);
        ntfPostStatusFeed.add(psr);
        break;

      case NTFileDownloadCompleted:
        var rf = ReceivedFile.fromJson(payload);
        ntfDownloadCompleted.add(rf);
        ntfChatEvents.add(FileDownloadedEvent(rf.uid, rf.diskPath));
        break;

      case NTFileDownloadProgress:
        var fdp = FileDownloadProgress.fromJson(payload);
        ntfDownloadProgress.add(fdp);
        break;

      case NTLogLine:
        ntfLogLines.add(payload);
        break;

      case NTLNConfPayReqRecvChan:
        var est = LNReqChannelEstValue.fromJson(payload);
        ntfConfs.add(ConfNotification(NTLNConfPayReqRecvChan, est));
        break;

      case NTLNInitialChainSyncUpdt:
        isError
            ? ntfLNInitChainSync.addError(payload)
            : ntfLNInitChainSync
                .add(LNInitialChainSyncUpdate.fromJson(payload));
        break;

      case NTConfFileDownload:
        var data = ConfirmFileDownload.fromJson(payload);
        ntfConfs.add(ConfNotification(NTConfFileDownload, data));
        break;

      case NTLNDcrlndStopped:
        ntfConfs.add(ConfNotification(NTLNDcrlndStopped, payload));
        break;

      case NTClientStopped:
        ntfConfs.add(ConfNotification(NTClientStopped, payload));
        break;

      case NTUserPostsList:
        var event = UserPostList.fromJson(payload);
        ntfChatEvents.add(event);
        break;

      case NTUserContentList:
        var event = UserContentList.fromJson(payload);
        ntfChatEvents.add(event);
        break;

      case NTPostSubscriptionResult:
        var event = PostSubscriptionResult.fromJson(payload);
        ntfChatEvents.add(event);
        break;

      case NTInvoiceGenFailed:
        ntfConfs.add(ConfNotification(
            NTInvoiceGenFailed, InvoiceGenFailed.fromJson(payload)));
        break;

      case NTGCVersionWarn:
        var event = GCVersionWarn.fromJson(payload);
        ntfChatEvents.add(event);
        break;

      case NTGCAddedMembers:
        var event = GCAddedMembers.fromJson(payload);
        ntfChatEvents.add(event);
        break;

      case NTGCUpgradedVersion:
        var event = GCUpgradedVersion.fromJson(payload);
        ntfChatEvents.add(event);
        break;

      case NTGCMemberParted:
        var event = GCMemberParted.fromJson(payload);
        ntfChatEvents.add(event);
        break;

      case NTGCAdminsChanged:
        var event = GCAdminsChanged.fromJson(payload);
        ntfChatEvents.add(event);
        break;

      case NTKXCSuggested:
        var event = KXSuggested.fromJson(payload);
        ntfChatEvents.add(event);
        break;

      default:
        print("Received unknown notification ${cmd.toRadixString(16)}");
    }
  }

  void readAsyncResults() async {
    var rp = new ReceivePort();
    Isolate.spawn(_readAsyncResultsIsolate, rp.sendPort);
    while (true) {
      await for (List cmdReply in rp) {
        if (cmdReply.length < 3) {
          print("Received wrong nb of elements from isolate: $cmdReply");
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
              print(err);
              print(jsonPayload);
              (() async => throw exception)();
            }
          } else {
            print("Received reply for unknown call $id - $cmdReply");
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
