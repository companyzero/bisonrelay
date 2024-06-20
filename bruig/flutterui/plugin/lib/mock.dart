import 'dart:async';
import 'dart:io';
// import 'dart:typed_data';

import 'package:flutter/cupertino.dart';
import 'package:flutter/services.dart';

import 'definitions.dart';
import 'package:path/path.dart' as path;
import 'package:shelf_web_socket/shelf_web_socket.dart';
import 'package:json_rpc_2/json_rpc_2.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

// Throws the given exception or returns null. Use it as:
// _threw(e) ?? otherval
dynamic _threw(Exception? e) {
  if (e != null) throw e;
  return null;
}

Exception? _exception(Parameter p) =>
    p.asStringOr("") != "" ? Exception(p.asString) : null;

int _lastEID = 0;
int nextEID() {
  return _lastEID++;
}

class MockPlugin with NtfStreams /*implements PluginPlatform*/ {
  /// ******************************************
  /// Fields
  ///******************************************
  Directory tempRoot = Directory.systemTemp.createTempSync("fd-mock");
  String tag = "";
  StreamController<String> streamCtrl = StreamController<String>();
  Exception? failNextGetURL;
  Exception? failNextConnect;
  StreamController<ChatEvent> chatMsgsCtrl = StreamController<ChatEvent>();
  Map<String, List<String>> gcBooks = {
    "Test Group": ["user1", "user2", "user3"],
    "group2": ["bleh", "booo", "fran"],
  };

  /// ******************************************
  /// Constructor
  ///******************************************
  MockPlugin() {
    webSocketHandler((WebSocketChannel socket) {
      final server = Server(socket.cast<String>());

      server.registerMethod('hello', rpcHello);
      server.registerMethod('failNextGetURL', rpcFailNextGetURL);
      server.registerMethod('failNextConnect', rpcFailNextConnect);
      server.registerMethod('recvMsg', rpcRecvMsg);
      server.registerMethod('feedPost', rpcFeedPost);

      server.listen();
    });

    /*
    () async {
      final address = "127.0.0.1";
      final port = 4042;
      await shelf_io.serve(handler, address, port);
      print("Mock ctrl server listening on ws://$address:$port");
    }();
    */

    // Send some initial feed events.
    /*
    () async {
      ntfPostsFeed.add(FeedPost(
          "Someone",
          "xxxxx",
          "My first content. This is a sample of the stuff I'll add in the future.",
          "test.md",
          DateTime.parse("2021-08-18 15:18:22")));
      ntfPostsFeed.add(FeedPost(
        "Someone else",
        "xxxxx",
        "This is someone else. Hope you're all fine.",
        "test.md",
        DateTime.parse("2021-08-18 15:18:22"),
      ));
      ntfPostsFeed.add(FeedPost(
        "Someone",
        "xxxxx",
        "My second content. Maybe later there will be more stuff.",
        "test.md",
        DateTime.parse("2021-08-18 15:18:22"),
      ));
      ntfPostsFeed.add(FeedPost(
        "Third Party11",
        "xxxxx",
        "Content from third party. This'll probably fail to be fetched.",
        "*bug",
        DateTime.parse("2021-08-18 15:18:22"),
      ));
    }();
    */
  }

  /// ******************************************
  ///  PluginPlatform implementation methods.
  ///******************************************

  Future<String?> get platformVersion async => "mock 1.0";
  String get majorPlatform => "mock";
  String get minorPlatform => "mock";
  Future<void> setTag(String t) async => tag = t;
  Future<void> hello() async => debugPrint("hello from mock");
  Future<String> getURL(String url) async =>
      _threw(failNextGetURL) ?? "xxx.xxx.xxx.xxx";

  Future<String> nextTime() async => "$tag ${DateTime.now().toIso8601String()}";
  Future<void> writeStr(String s) async => streamCtrl.add(s);
  Stream<String> readStream() => streamCtrl.stream;

  Future<String> asyncCall(int cmd, dynamic payload) async =>
      throw "unimplemented";

  Future<String> asyncHello(String name) async => throw "unimplemented";

  Future<void> initClient(InitClient args) async {}

  Future<void> pm(PM msg) => throw "unimplemented";

  Future<bool> hasServer() async =>
      Future.delayed(const Duration(seconds: 3), () => false);

  Future<List<AddressBookEntry>> addressBook() async => <AddressBookEntry>[
        /*
        AddressBookEntry("id01", "Someone"),
        AddressBookEntry("id02", "Someone else"),
        AddressBookEntry("id03", "Test Group"),
        AddressBookEntry("id04", "group2"),
        */
      ];

  Future<void> initID(IDInit args) async => throw "unimplemented";

  Future<void> replyConfServerCert(bool accept) async {}

  Future<String> userNick(String uid) async => throw "unimplemented";
  Future<void> commentPost(
          String from, String pid, String comment, String? parent) async =>
      throw "unimplemented";
  Future<LocalInfo> getLocalInfo() async => throw "unimplemented";
  Future<void> requestMediateID(String mediator, String target) async =>
      throw "unimplemented";
  Future<void> kxSearchPostAuthor(String from, String pid) async =>
      throw "unimplemented";
  Future<void> relayPostToAll(String from, String pid) async =>
      throw "unimplemented";
  Future<PostSummary> createPost(String content) async => throw "unimplemented";
  Future<Map<String, dynamic>> getGCBlockList(String gcID) async =>
      throw "unimplemented";
  Future<void> addToGCBlockList(String gcID, String uid) async =>
      throw "unimplemented";
  Future<void> removeFromGCBlockList(String gcID, String uid) async =>
      throw "unimplemented";
  Future<void> partFromGC(String gcID) async => throw "unimplemented";
  Future<void> killGC(String gcID) async => throw "unimplemented";
  Future<void> blockUser(String uid) async => throw "unimplemented";
  Future<void> ignoreUser(String uid) async => throw "unimplemented";
  Future<void> unignoreUser(String uid) async => throw "unimplemented";
  Future<bool> isIgnored(String uid) async => throw "unimplemented";
  Future<List<String>> listSubscribers() async => throw "unimplemented";
  Future<List<String>> listSubscriptions() async => throw "unimplemented";
  Future<List<OutstandingFileDownload>> listDownloads() async =>
      throw "unimplemented";
  Future<LNInfo> lnGetInfo() async => throw "unimplemented";
  Future<List<LNChannel>> lnListChannels() async => throw "unimplemented";
  Future<LNPendingChannelsList> lnListPendingChannels() async =>
      throw "unimplemented";
  Future<LNGenInvoiceResponse> lnGenInvoice(
          double dcrAmount, String memo) async =>
      throw "unimplemented";
  Future<LNPayInvoiceResponse> lnPayInvoice(
          String invoice, double dcrAmount) async =>
      throw "unimplemented";
  Future<String> lnGetServerNode() async => throw "unimplemented";
  Future<LNQueryRouteResponse> lnQueryRoute(
          double dcrAmount, String target) async =>
      throw "unimplemented";
  Future<LNBalances> lnGetBalances() async => throw "unimplemented";
  Future<LNDecodedInvoice> lnDecodeInvoice(String invoice) async =>
      throw "unimplemented";
  Future<List<LNPeer>> lnListPeers() async => throw "unimplemented";
  Future<void> lnConnectToPeer(String addr) async => throw "unimplemented";
  Future<void> lnDisconnectFromPeer(String pubkey) async =>
      throw "unimplemented";
  Future<void> lnOpenChannel(
          String pubkey, double amount, double pushAmount) async =>
      throw "unimplemented";
  Future<void> lnCloseChannel(String channelPoint, bool force) async =>
      throw "unimplemented";
  Future<LNInfo> lnTryExternalDcrlnd(
          String rpcHost, String tlsCertPath, String macaroonPath) =>
      throw "unimplemented";
  Future<LNNewWalletSeed> lnInitDcrlnd(
          String rootPath, String network, String password) async =>
      throw "unimplemented";
  Future<String> lnRunDcrlnd(String rootPath, String network, String password,
          String proxyaddr, bool torisolation) async =>
      throw "unimplemented";
  void captureDcrlndLog() => throw "unimplemented";
  Future<String> lnGetDepositAddr() async => throw "unimplemented";
  Future<void> lnRequestRecvCapacity(
          String server, String key, double chanSize) async =>
      throw "unimplemented";
  Future<void> lnConfirmPayReqRecvChan(bool value) async =>
      throw "unimplemented";
  Future<void> confirmFileDownload(String fid, bool confirm) async =>
      throw "unimplemented";
  Future<void> sendFile(String uid, String filepath) async =>
      throw "unimplemented";

  /// ******************************************
  ///  Mock-only Methods (to be added to PluginPlatform)
  ///******************************************

  Future<ServerInfo> connectToServer(
          String server, String name, String nick) async =>
      Future<ServerInfo>.delayed(
          const Duration(seconds: 3),
          () =>
              _threw(failNextConnect) ??
              ServerInfo(
                  innerFingerprint: "XXYY",
                  outerFingerprint: "LLOOOO",
                  serverAddr: server));

  Future<ChatEvent> sendToChat(String nick, String msg, int timestamp) async {
    ChatEvent cm = PM(nick, msg, true, timestamp);
    /*
    Future.delayed(Duration(seconds: 1), () {
      if (msg == "*bug1") {
        cm.error = new Exception("errored before sending");
        return;
      }
      cm.sentState = CMS_sending;
      Future.delayed(Duration(seconds: 1), () {
        if (msg == "*bug2") {
          cm.error = new Exception("errored before sent");
          return;
        }
        cm.sentState = CMS_sent;
      });
    });
    */
    return cm;
  }

  @override
  Stream<ChatEvent> chatEvents() => chatMsgsCtrl.stream;

  Future<void> generateInvite(String filepath) async {
    var f = File(filepath);
    await f.writeAsString("mock-invitation-file");
  }

  Future<Invitation> decodeInvite(String filepath) async {
    return Invitation(
        OOBPublicIdentityInvite(
            PublicIdentity("user", "User to Confirm", "xx-xx-xx-xx-xx-xx"),
            "",
            "",
            null),
        Uint8List(5));
  }

  Future<RemoteUser> acceptInvite(Invitation info) => throw "unimplemented";
  /*
      // kx happens. Assume it completed successfuly and the server sent a diag msg.
      Future.delayed(Duration(seconds: 3), () {
        chatMsgsCtrl.add(ChatMsg.pm(
            nextEID(), info.nick, info.nick, "kx performed!",
            isServerMsg: true));
        return info;
      });
      */

  Future<void> createGC(String name) => throw "unimplemented";
  /*
      Future.delayed(Duration(seconds: 3), () {
        // gc creation happens. Assume it completed successfully and the server
        // sent a gc diag msg.
        chatMsgsCtrl.add(ChatMsg.gc(nextEID(), name, name, "GC Created!",
            isServerMsg: true));
      });
      */

  Future<void> inviteToGC(InviteToGC inv) => throw "unimplemented";

  Future<void> acceptGCInvite(int iid) => throw "unimplemented";

  Future<List<GCAddressBookEntry>> listGCs() async => throw "unimplemented";

  Future<ChatEvent> sendToGC(String gc, String msg) async {
    throw "unimplemented";

    /*
    ChatMsg cm = ChatMsg.gc(nextEID(), gc, gc, msg, mine: true);
    Future.delayed(Duration(seconds: 1), () {
      if (msg == "*bug1") {
        cm.error = new Exception("errored before sending");
        return;
      }
      cm.sentState = CMS_sending;
      Future.delayed(Duration(seconds: 1), () {
        if (msg == "*bug2") {
          cm.error = new Exception("errored before sent");
          return;
        }
        cm.sentState = CMS_sent;
      });
    });

    return cm;
    */
  }

  Future<GCAddressBookEntry> getGC(String gc) async => throw "unimplemented";

  Future<void> inviteGcUser(String gc, String nick) async {
    if (nick == "*bug") throw Exception("bugging out as requested");
    return Future.delayed(
        const Duration(seconds: 3), () => gcBooks[gc]?.add(nick));
  }

  Future<void> removeGcUser(String gc, String nick) async {
    if (nick == "fran") throw Exception("bugging out as requested");
    return Future.delayed(const Duration(seconds: 3),
        () => debugPrint("${gcBooks[gc]?.remove(nick)}"));
  }

  Future<void> confirmGCInvite(InviteToGC invite) => throw "unimplemented";
  /*
      Future.delayed(Duration(seconds: 3), () {
        if (invite.name == "*bug") throw Exception("Bugging out as requested");

        // Assume the invitation was successfully completed.
        chatMsgsCtrl.add(ChatMsg.gc(
            nextEID(), invite.name, invite.name, "Joined GC!",
            isServerMsg: true));
      });
      **/

  InflightTip payTip(String nick, double amount) {
    var tip = InflightTip(nextEID(), nick, amount);
    Future.delayed(const Duration(seconds: 1), () {
      tip.state = ITS_started;
      Future.delayed(const Duration(seconds: 2), () {
        if (amount == 666) {
          tip.error = Exception("Bugging out as requested");
          return;
        }

        tip.state = ITS_completed;
      });
    });
    return tip;
  }

  Future<void> transitiveInvite(String destNick, String targetNick) async {
    throw "unimplemented";
    /*
    chatMsgsCtrl.add(ChatMsg.gc(
        nextEID(), destNick, destNick, "Sent invite for user $targetNick",
        isServerMsg: true));
    */
  }

  Future<void> requestKXReset(String uid) async => throw "unimplemented";

  Future<void> shareFile(
          String filename, String? uid, double cost, String descr) =>
      throw "unimplemented";

  Future<void> unshareFile(String filename, String? uid) =>
      throw "unimplemented";

  Future<List<SharedFileAndShares>> listSharedFiles() => throw "unimplemented";

  Future<List<ReceivedFile>> listUserContent(String uid) =>
      throw "unimplemented";

  Future<ReceivedFile> getUserContent(String uid, String filename) =>
      throw "unimplemented";

  Future<List<PostSummary>> listPosts() async => throw "uimplemented";
  Future<PostMetadata> readPost(String from, String pid) async =>
      throw "unimplemented";
  Future<List<PostMetadataStatus>> listPostStatus(
          String from, String pid) async =>
      throw "unimplemented";

  // extractMdContent extracts the given content (which must be a native bundle
  // format) and returns the dir to the extracted temp bundle.
  Future<String> extractMdContent(String nick, String filename) async =>
      Future.delayed(const Duration(seconds: 3), () async {
        if (filename == "*bug") {
          throw Exception("Bugging out as requested");
        }

        var dir = Directory(path.join(tempRoot.path, "sample_md"));
        if (dir.existsSync()) {
          return dir.path;
        }

        // Extract sample md data to it.
        dir.createSync(recursive: true);
        List<String> files = ["index.md", "bunny_small.mp4", "pixabay.jpg"];
        for (var fname in files) {
          File f = File(path.join(dir.path, fname));
          var content = await rootBundle.load("assets/sample_md/$fname");
          var buffer = content.buffer;
          var bytes =
              buffer.asUint8List(content.offsetInBytes, content.lengthInBytes);
          await f.writeAsBytes(bytes);
        }

        return dir.path;
      });

  Future<void> subscribeToPosts(String uid) => throw "unimplemented";

  /// ******************************************
  ///  Mock JSON-RPC handlers.
  ///******************************************
  String rpcHello() => "is it me you're looking for?";

  void rpcFailNextGetURL(Parameters params) {
    failNextGetURL = _exception(params[0]);
  }

  void rpcFailNextConnect(Parameters params) {
    failNextConnect = _exception(params[0]);
  }

  void rpcRecvMsg(Parameters params) {
    /*
    var nick = params[0].asString;
    var msg = params[1].asString;
    chatMsgsCtrl.add(ChatMsg.pm(nextEID(), nick, nick, msg, mine: false));
    */
    throw "unimplemented";
  }

  void rpcFeedPost(Parameters params) {
    /*
    ntfPostsFeed.add(PostSummary(params[0].asString, "xxxxx", params[1].asString,
        "test.md", DateTime.now()));
        */
    throw "unimplemented";
  }
}
