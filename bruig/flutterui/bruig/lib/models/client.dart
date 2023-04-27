import 'dart:async';
import 'dart:collection';
import 'package:bruig/models/menus.dart';
import 'package:bruig/models/resources.dart';
import 'package:flutter/foundation.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import '../storage_manager.dart';

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

class RequestedResourceEvent extends ChatEvent {
  final PagesSession session;

  RequestedResourceEvent(String uid, this.session)
      : super(uid, "Fetching user resources");
}

const int CMS_unknown = 0;
const int CMS_sending = 1;
const int CMS_sent = 2;
const int CMS_errored = 3;
const int CMS_canceled = 4;

const int Suggestion_received = 0;
const int Suggestion_accepted = 1;
const int Suggestion_confirmed = 2;
const int Suggestion_canceled = 3;
const int Suggestion_errored = 4;
const int Suggestion_alreadyKnown = 5;

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

  bool _firstUnread = false;
  bool get firstUnread => _firstUnread;
  void set firstUnread(bool b) {
    _firstUnread = b;
    notifyListeners();
  }

  bool _sameUser = false;
  bool get sameUser => _sameUser;
  void set sameUser(bool b) {
    _sameUser = b;
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

  int _unreadMsgCount = 0;
  int get unreadMsgCount => _unreadMsgCount;
  int _unreadEventCount = 0;
  int get unreadEventCount => _unreadEventCount;

  bool _active = false;
  bool get active => _active;
  void _setActive(bool b) {
    _active = b;
    _unreadMsgCount = 0;
    _unreadEventCount = 0;
    notifyListeners();
  }

  // return the first unread msg index and -1 if there aren't
  // unread msgs
  int firstUnreadIndex() {
    for (int i = 0; i < _msgs.length; i++) {
      if (_msgs[i].firstUnread) {
        return i;
      }
    }
    return -1;
  }

  List<ChatEventModel> _msgs = [];
  UnmodifiableListView<ChatEventModel> get msgs => UnmodifiableListView(_msgs);
  void append(ChatEventModel msg) {
    if (!_active && _unreadMsgCount == 0 && _msgs.isNotEmpty) {
      msg.firstUnread = true;
    }
    if (_msgs.isNotEmpty &&
        _msgs[_msgs.length - 1].source?.id == msg.source?.id) {
      msg.sameUser = true;
    }
    _msgs.add(msg);
    if (!_active) {
      if (msg.event is PM || msg.event is GCMsg) {
        _unreadMsgCount += 1;
      } else {
        _unreadEventCount += 1;
      }
    }
    notifyListeners();
  }

  void removeFirstUnread() {
    for (int i = 0; i < _msgs.length; i++) {
      if (_msgs[i].firstUnread) {
        _msgs[i].firstUnread = false;
        return;
      }
    }
  }

  void payTip(double amount) async {
    var tip = await Golib.payTip(id, amount);
    _msgs.add(ChatEventModel(tip, this));
    notifyListeners();
  }

  Future<void> sendMsg(String msg) async {
    // This may be triggered by autmation sending messages when the chat window
    // is not focused (for example, simplestore placed orders).
    if (!active) {
      _unreadMsgCount += 1;
    }

    if (isGC) {
      var m = GCMsg(id, nick, msg, DateTime.now().millisecondsSinceEpoch);
      var evnt = ChatEventModel(m, null);
      evnt.sentState = CMS_sending; // Track individual sending status?
      if (_msgs.isNotEmpty && _msgs[_msgs.length - 1].source == null) {
        evnt.sameUser = true;
      }
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
      if (_msgs.isNotEmpty && _msgs[_msgs.length - 1].source == null) {
        evnt.sameUser = true;
      }
      _msgs.add(evnt);
      notifyListeners();

      try {
        await Golib.pm(m);
        evnt.sentState = CMS_sent;
      } catch (exception) {
        evnt.sendError = "$exception";
      }
    }

    // This may be triggered by autmation sending messages when the chat window
    // is not focused (for example, simplestore placed orders).
    if (!active) {
      _unreadMsgCount += 1;
      notifyListeners();
    }
  }

  String workingMsg = "";

  void subscribeToPosts() {
    var event = SynthChatEvent("Subscribing to user's posts");
    event.state = SCE_sending;
    append(ChatEventModel(event, null));
    (() async {
      try {
        await Golib.subscribeToPosts(id);
        event.state = SCE_sent;
      } catch (error) {
        event.error = Exception(error);
      }
    })();
  }

  Future<void> unsubscribeToPosts() {
    var event = SynthChatEvent("Unsubscribing from user's posts");
    event.state = SCE_sending;
    append(ChatEventModel(event, null));
    return (() async {
      try {
        await Golib.unsubscribeToPosts(id);
        event.state = SCE_sent;
      } catch (error) {
        event.error = Exception(error);
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

  Future<void> resendGCList() async => await Golib.resendGCList(id);
}

class ClientModel extends ChangeNotifier {
  ClientModel() {
    _handleAcceptedInvites();
    _handleChatMsgs();
    readAddressBook();
    _handleServerSessChanged();
    _handleGCListUpdates();
    _fetchInfo();
    _handleSSOrders();
  }

  List<ChatModel> _gcChats = [];
  UnmodifiableListView<ChatModel> get gcChats => UnmodifiableListView(_gcChats);

  void set gcChats(List<ChatModel> gc) {
    _gcChats = gc;
    notifyListeners();
  }

  List<ChatModel> _userChats = [];
  UnmodifiableListView<ChatModel> get userChats =>
      UnmodifiableListView(_userChats);

  void set userChats(List<ChatModel> us) {
    _userChats = us;
    notifyListeners();
  }

  bool _loadingAddressBook = true;
  bool get loadingAddressBook => _loadingAddressBook;
  void set loadingAddressBook(bool b) {
    _loadingAddressBook = b;
    notifyListeners();
  }

  bool _hasUnreadChats = false;
  bool get hasUnreadChats => _hasUnreadChats;
  void set hasUnreadChats(bool b) {
    _hasUnreadChats = b;
    notifyListeners();
  }

  final Map<String, List<ChatMenuItem>> _subGCMenus = {};
  UnmodifiableMapView<String, List<ChatMenuItem>> get subGCMenus =>
      UnmodifiableMapView(_subGCMenus);

  final Map<String, List<ChatMenuItem>> _subUserMenus = {};
  UnmodifiableMapView<String, List<ChatMenuItem>> get subUserMenus =>
      UnmodifiableMapView(_subUserMenus);

  List<ChatMenuItem> _activeSubMenu = [];
  UnmodifiableListView<ChatMenuItem> get activeSubMenu =>
      UnmodifiableListView(_activeSubMenu);

  void set activeSubMenu(List<ChatMenuItem> sm) {
    _activeSubMenu = sm;
    notifyListeners();
  }

  void showSubMenu(bool isGC, String id) {
    if (isGC) {
      activeSubMenu = subGCMenus[id] ?? [];
    } else {
      activeSubMenu = subUserMenus[id] ?? [];
    }
    notifyListeners();
  }

  void hideSubMenu() {
    activeSubMenu = [];
    notifyListeners();
  }

  String _publicID = "";
  String get publicID => _publicID;

  String _nick = "";
  String get nick => _nick;

  ServerSessionState _connState = ServerSessionState.empty();
  ServerSessionState get connState => _connState;
  bool get isOnline => _connState.state == connStateOnline;

  String _network = "";
  String get network => _network;

  ChatModel? _active;
  ChatModel? get active => _active;

  void set active(ChatModel? c) {
    _profile = null;
    // Remove new posts messages
    _active?.removeFirstUnread();
    _active?._setActive(false);
    _active = c;
    c?._setActive(true);

    // Check for unreadMessages so we can turn off sidebar notification
    bool unreadChats = false;
    for (int i = 0; i < _gcChats.length; i++) {
      if (_gcChats[i].unreadMsgCount > 0) {
        unreadChats = true;
      }
    }

    for (int i = 0; i < _userChats.length; i++) {
      if (_userChats[i].unreadMsgCount > 0) {
        unreadChats = true;
      }
    }
    hasUnreadChats = unreadChats;
    hideSubMenu();
    notifyListeners();
  }

  ChatModel? _profile;
  ChatModel? get profile => _profile;
  set profile(ChatModel? c) {
    _profile = c;
    //c?._setShowProfile(true);
    notifyListeners();
  }

  void setActiveByNick(String nick, bool isGC) {
    try {
      var c = isGC
          ? _gcChats.firstWhere((c) => c.nick == nick)
          : _userChats.firstWhere((c) => c.nick == nick);
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
    if (isGC) {
      if (_gcChats.indexWhere((c) => c.id == id) == -1) {
        // Add to list of chats.
        _gcChats.add(c);
        _subGCMenus[c.id] = buildGCMenu(c);
      }
    } else {
      if (_userChats.indexWhere((c) => c.id == id) == -1) {
        // Add to list of chats.
        _userChats.add(c);
        _subUserMenus[c.id] = buildUserChatMenu(c);
      }
    }

    notifyListeners();

    return c;
  }

  void removeChat(ChatModel chat) {
    if (chat.isGC) {
      _gcChats.remove(chat);
      _subGCMenus.remove(chat.id);
    } else {
      _userChats.remove(chat);
      _subUserMenus.remove(chat.id);
    }
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
      // Sorting algo to attempt to retain order
      if (chat.isGC) {
        _gcChats.sort((a, b) => b.unreadMsgCount.compareTo(a.unreadMsgCount));

        String gcChatOrder = "";
        for (int i = 0; i < _gcChats.length; i++) {
          if (_gcChats[i].unreadMsgCount > 0) {
            hasUnreadChats = true;
          }
          if (i == _gcChats.length - 1) {
            gcChatOrder += _gcChats[i].nick;
          } else {
            gcChatOrder += "${_gcChats[i].nick},";
          }
        }
        StorageManager.saveData('gcListOrder', gcChatOrder);
      } else {
        _userChats.sort((a, b) => b.unreadMsgCount.compareTo(a.unreadMsgCount));

        String userChatOrder = "";
        for (int i = 0; i < _userChats.length; i++) {
          if (_userChats[i].unreadMsgCount > 0) {
            hasUnreadChats = true;
          }
          if (i == _userChats.length - 1) {
            userChatOrder += _userChats[i].nick;
          } else {
            userChatOrder += "${_userChats[i].nick},";
          }
        }
        StorageManager.saveData('userListOrder', userChatOrder);
      }
      notifyListeners();
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

    StorageManager.readData('gcListOrder').then((value) {
      if (value != null && value.length > 0) {
        List<ChatModel> sortedGCList = [];
        var gcSplitList = value.split(',');
        for (int i = 0; i < gcSplitList.length; i++) {
          for (int j = 0; j < _gcChats.length; j++) {
            if (gcSplitList[i] == _gcChats[j].nick) {
              sortedGCList.add(_gcChats[j]);
              break;
            }
          }
        }
        for (int i = 0; i < _gcChats.length; i++) {
          var found = false;
          for (int j = 0; j < gcSplitList.length; j++) {
            if (gcSplitList[j] == _gcChats[i].nick) {
              found = true;
              break;
            }
          }
          if (!found) {
            sortedGCList.add(_gcChats[i]);
          }
        }
        gcChats = sortedGCList;
      }
    });
    StorageManager.readData('userListOrder').then((value) {
      if (value != null && value.length > 0) {
        List<ChatModel> sortedUserList = [];
        var userSplitList = value.split(',');
        // First order existing users from last saved.
        for (int i = 0; i < userSplitList.length; i++) {
          for (int j = 0; j < _userChats.length; j++) {
            if (userSplitList[i] == _userChats[j].nick) {
              sortedUserList.add(_userChats[j]);
              break;
            }
          }
        }
        // Then try and find any received chats that aren't in the saved list.
        // Add them on the end.
        for (int i = 0; i < _userChats.length; i++) {
          var found = false;
          for (int j = 0; j < userSplitList.length; j++) {
            if (userSplitList[j] == _userChats[i].nick) {
              found = true;
              break;
            }
          }
          if (!found) {
            sortedUserList.add(_userChats[i]);
          }
        }
        userChats = sortedUserList;
      }
    });

    loadingAddressBook = false;
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

  void _handleSSOrderPlaced(SSPlacedOrder po) async {
    try {
      var order = po.order;
      var chat = getExistingChat(order.user);
      if (chat == null) {
        throw "user ${order.user} not found in placed simplestore order";
      }

      int totalCents = 0;
      var msg = """Thank you for placing your order #${order.id}
The following were the items in your order:
""";
      for (var item in order.cart.items) {
        var price = item.product.price.toStringAsFixed(2);
        var itemCents = (item.product.price * 100).toInt() * item.quantity;
        var itemUSD = (itemCents.toDouble() / 100).toStringAsFixed(2);
        totalCents += itemCents;
        msg +=
            "  SKU ${item.product.sku} - ${item.product.title} - ${item.quantity} units - $price/item - $itemUSD\n";
      }

      var totalUSD = (totalCents.toDouble() / 100).toStringAsFixed(2);
      msg += "Total amount: \$$totalUSD\n";
      msg += "Total DCR Amount: ${formatDCR(atomsToDCR(po.dcrAmount))}\n";
      msg += "Exchange Rate: ${po.exchangeRate} USD/DCR\n";
      if (po.onchainAddr != "") {
        msg += "OnChain payment address: ${po.onchainAddr}\n";
      } else if (po.lnInvoice != "") {
        msg += "LN Invoice: ${po.lnInvoice}";
      } else {
        msg += "You will be contacted with payment details shortly\n";
      }
      chat.sendMsg(msg);
    } catch (exception) {
      // TODO: send to snackbar model.
      print("Error while processing SimpleStore order: $exception");
    }
  }

  void _handleSSOrders() async {
    var stream = Golib.simpleStoreOrders();
    await for (var order in stream) {
      _handleSSOrderPlaced(order);
    }
  }
}
