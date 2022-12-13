import 'dart:collection';
import 'package:bruig/models/menus.dart';
import 'package:flutter/foundation.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';

const SCE_unknown = 0;
const SCE_sending = 1;
const SCE_sent = 2;
const SCE_received = 3;
const SCE_errored = 99;

class SynthChatEvent extends ChatEvent with ChangeNotifier {
  SynthChatEvent(String msg, [this._state = SCE_unknown, this._error])
      : super("", msg);

  int _state;
  int get state => _state;
  void set state(int v) {
    _state = v;
    notifyListeners();
  }

  Exception? _error;
  Exception? get error => _error;
  void set error(Exception? e) {
    if (e == null) throw Exception("Cannot set error to null");
    _error = e;
    _state = SCE_errored;
    notifyListeners();
  }
}

const ISPS_unknown = 0;
const ISPS_sending = 1;
const ISPS_subscribed = 2;
const ISPS_errored = 3;

class InflightSubscribeToPosts extends ChatEvent with ChangeNotifier {
  InflightSubscribeToPosts(uid, [this._state = ISPS_unknown, this._error])
      : super(uid, "");

  int _state;
  int get state => _state;
  void set state(int v) {
    _state = v;
    notifyListeners();
  }

  Exception? _error;
  Exception? get error => _error;
  void set error(Exception? e) {
    if (e == null) throw Exception("Cannot set error to null");
    _error = e;
    _state = ISPS_errored;
    notifyListeners();
  }
}

const int CMS_unknown = 0;
const int CMS_sending = 1;
const int CMS_sent = 2;
const int CMS_errored = 3;
const int CMS_canceled = 4;

class ChatEventModel extends ChangeNotifier {
  final ChatEvent event;
  final ChatModel? source; // null if it's from the local client.
  ChatEventModel(this.event, this.source);

  int _sentState = CMS_unknown;
  int get sentState => _sentState;
  void set sentState(int v) {
    _sentState = v;
    notifyListeners();
  }

  String? _sendError;
  String? get sendError => _sendError;
  void set sendError(String? err) {
    _sendError = err;
    _sentState = CMS_errored;
    notifyListeners();
  }
}

class ChatModel extends ChangeNotifier {
  final String id; // RemoteUID or GC ID
  final bool isGC;

  String _nick; // Nick or GC name
  String get nick => _nick;
  void set nick(String nn) {
    _nick = nn;
    notifyListeners();
  }

  ChatModel(this.id, this._nick, this.isGC);

  int _unreadCount = 0;
  int get unreadCount => _unreadCount;

  bool _active = false;
  bool get active => _active;
  void _setActive(bool b) {
    _active = b;
    _unreadCount = 0;
    notifyListeners();
  }

  List<ChatEventModel> _msgs = [];
  UnmodifiableListView<ChatEventModel> get msgs => UnmodifiableListView(_msgs);
  void append(ChatEventModel msg) {
    _msgs.add(msg);
    if (!_active) {
      _unreadCount += 1;
    }
    notifyListeners();
  }

  void payTip(double amount) async {
    var tip = await Golib.payTip(id, amount);
    _msgs.add(ChatEventModel(tip, this));
    notifyListeners();
  }

  void sendMsg(String msg) async {
    if (isGC) {
      var m = GCMsg(id, nick, msg, DateTime.now().millisecondsSinceEpoch);
      var evnt = ChatEventModel(m, null);
      evnt.sentState = CMS_sending; // Track individual sending status?
      _msgs.add(evnt);
      notifyListeners();

      try {
        await Golib.sendToGC(id, msg);
        evnt.sentState = CMS_sent;
      } catch (exception) {
        evnt.sendError = "$exception";
      }
    } else {
      var ts = DateTime.now().millisecondsSinceEpoch;
      var m = PM(id, msg, true, ts);
      var evnt = ChatEventModel(m, null);
      evnt.sentState = CMS_sending;
      _msgs.add(evnt);
      notifyListeners();

      try {
        await Golib.pm(m);
        evnt.sentState = CMS_sent;
      } catch (exception) {
        evnt.sendError = "$exception";
      }
    }
  }

  void subscribeToPosts() {
    var event = InflightSubscribeToPosts(id);
    event.state = ISPS_sending;
    append(ChatEventModel(event, null));
    (() async {
      try {
        await Golib.subscribeToPosts(id);
        event.state = ISPS_subscribed;
      } catch (error) {
        event.error = new Exception(error);
      }
    })();
  }

  void requestKXReset() {
    var event = SynthChatEvent("Requesting KX reset", SCE_sending);
    append(ChatEventModel(event, null));
    (() async {
      try {
        await Golib.requestKXReset(id);
        event.state = SCE_sent;
      } catch (error) {
        event.error = new Exception(error);
      }
    })();
  }
}

class ClientModel extends ChangeNotifier {
  ClientModel() {
    _handleAcceptedInvites();
    _handleChatMsgs();
    readAddressBook();
    _handleServerSessChanged();
    _handleGCListUpdates();
    _fetchInfo();
  }

  List<ChatModel> _chats = [];
  UnmodifiableListView<ChatModel> get chats => UnmodifiableListView(_chats);

  List<ChatMenuItem> _subGCMenu = [];
  UnmodifiableListView<ChatMenuItem> get subGCMenu =>
      UnmodifiableListView(_subGCMenu);

  void set subGCMenu(List<ChatMenuItem> sm) {
    _subGCMenu = sm;
    notifyListeners();
  }

  List<ChatMenuItem> _subUserMenu = [];
  UnmodifiableListView<ChatMenuItem> get subUserMenu =>
      UnmodifiableListView(_subUserMenu);

  void set subUserMenu(List<ChatMenuItem> sm) {
    _subUserMenu = sm;
    notifyListeners();
  }

  String _publicID = "";
  String get publicID => _publicID;

  String _nick = "";
  String get nick => _nick;

  ServerSessionState _connState = ServerSessionState.empty();
  ServerSessionState get connState => _connState;

  String _network = "";
  String get network => _network;

  ChatModel? _active;
  ChatModel? get active => _active;
  void set active(ChatModel? c) {
    _profile = null;
    _active?._setActive(false);
    _active = c;
    c?._setActive(true);
    notifyListeners();
  }

  ChatModel? _profile;
  ChatModel? get profile => _profile;
  set profile(ChatModel? c) {
    _profile = c;
    //c?._setShowProfile(true);
    notifyListeners();
  }

  void setActiveByNick(String nick) {
    try {
      var c = _chats.firstWhere((c) => c.nick == nick);
      active = c;
    } on StateError {
      // Ignore if chat doesn't exist.
    }
  }

  Map<String, ChatModel> _activeChats = Map<String, ChatModel>();
  ChatModel? getExistingChat(String uid) => _activeChats[uid];

  ChatModel _newChat(String id, String alias, bool isGC) {
    alias = alias.trim();

    var c = _activeChats[id];
    if (c != null) {
      if (alias != "" && alias != c.nick) {
        c.nick = alias;
        notifyListeners();
      }
      return c;
    }

    alias = alias == "" ? "[blank]" : alias;
    c = ChatModel(id, alias, isGC);
    _activeChats[id] = c;

    // TODO: this test should be superflous.
    if (_chats.indexWhere((c) => c.id == id) == -1) {
      // Add to list of chats.
      _chats.add(c);
    }
    notifyListeners();

    return c;
  }

  void removeChat(ChatModel chat) {
    _chats.remove(chat);
    _activeChats.remove(chat.id);
    notifyListeners();
  }

  String getNick(String uid) {
    var chat = getExistingChat(uid);
    return chat?.nick ?? "";
  }

  void _handleChatMsgs() async {
    var stream = Golib.chatEvents();
    await for (var evnt in stream) {
      if (evnt is FeedPostEvent) {
        if (evnt.sid == publicID) {
          // Ignore own relays.
          continue;
        }
      }

      var isGC = (evnt is GCMsg) || (evnt is GCUserEvent);

      var chat = _newChat(evnt.sid, "", isGC);
      ChatModel? source = null;
      if (evnt is PM) {
        if (!evnt.mine) {
          source = chat;
        }
      } else if (evnt is GCMsg) {
        source = _newChat(evnt.senderUID, "", false);
      } else if (evnt is GCUserEvent) {
        source = _newChat(evnt.uid, "", false);
      } else {
        source = chat;
      }
      chat.append(ChatEventModel(evnt, source));
    }
  }

  Future<void> readAddressBook() async {
    var info = await Golib.getLocalInfo();
    _publicID = info.id;
    _nick = info.nick;
    var ab = await Golib.addressBook();
    ab.forEach((v) => _newChat(v.id, v.nick, false));
    var gcs = await Golib.listGCs();
    gcs.forEach((v) => _newChat(v.id, v.name, true));
  }

  void acceptInvite(Invitation invite) async {
    var user = await Golib.acceptInvite(invite);
    active = _newChat(user.uid, user.nick, false);
  }

  List<String> _mediating = [];
  bool requestedMediateID(String target) => _mediating.contains(target);
  void requestMediateID(String mediator, String target) async {
    if (!requestedMediateID(target)) {
      _mediating.add(target);
      notifyListeners();
    }
    await Golib.requestMediateID(mediator, target);
  }

  void _fetchInfo() async {
    var res = await Golib.lnGetInfo();
    _network = res.chains[0].network;
  }

  void _handleAcceptedInvites() async {
    var stream = Golib.acceptedInvites();
    await for (var remoteUser in stream) {
      if (requestedMediateID(remoteUser.uid)) {
        _mediating.remove(remoteUser.uid);
      }
      var chat = _newChat(remoteUser.uid, remoteUser.nick, false);
      chat.append(
          ChatEventModel(SynthChatEvent("KX Completed", SCE_received), null));
    }
  }

  void _handleServerSessChanged() async {
    var stream = Golib.serverSessionChanged();
    await for (var state in stream) {
      _connState = state;
      notifyListeners();
    }
  }

  void _handleGCListUpdates() async {
    var stream = Golib.gcListUpdates();
    await for (var update in stream) {
      // Force creating the chat if it doesn't exist.
      _newChat(update.id, update.name, true);
    }
  }
}
