import 'dart:async';
import 'dart:collection';

import 'package:bruig/components/gc_context_menu.dart';
import 'package:bruig/models/menus.dart';
import 'package:bruig/models/resources.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:intl/intl.dart';
import 'package:bruig/storage_manager.dart';
import 'package:bruig/notification_service.dart';

const SCE_unknown = 0;
const SCE_sending = 1;
const SCE_sent = 2;
const SCE_received = 3;
const SCE_errored = 99;

class DateChangeEvent extends ChatEvent {
  final DateTime date;

  DateChangeEvent(this.date)
      : super("", DateFormat("EEE - d MMM").format(date));
}

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

class DayGCMessages {
  List<ChatEventModel> _msgs = [];
  UnmodifiableListView<ChatEventModel> get msgs => UnmodifiableListView(_msgs);
  void append(ChatEventModel msg) {
    _msgs.add(msg);
  }

  String date = "";
}

class ChatModel extends ChangeNotifier {
  final String id; // RemoteUID or GC ID
  final bool isGC;

  bool _isSubscribed = false;
  bool get isSubscribed => _isSubscribed;
  void set isSubscribed(bool b) {
    _isSubscribed = b;
    //notifyListeners();
  }

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

  bool _showChatListing = true; // Nick or GC name
  bool get showChatListing => _showChatListing;
  set showChatListing(bool b) {
    _showChatListing = b;
    notifyListeners();
  }

  String _userPostListID = "";
  String get userPostListID => _userPostListID;
  set userPostListID(String b) {
    _userPostListID = b;
    notifyListeners();
  }

  ImageProvider? _avatar;
  ImageProvider? get avatar => _avatar;
  void loadAvatar(Uint8List? newAvatar) async {
    if (newAvatar == null || newAvatar.isEmpty) {
      _avatar = null;
    } else {
      try {
        _avatar = MemoryImage(newAvatar);
        // Resize to a smaller size?
        notifyListeners();
      } catch (exception) {
        print("Unable to decode '$nick' avatar: $exception");
        _avatar = null;
      }
    }
  }

  List<PostListItem> _userPostList = [];
  UnmodifiableListView<PostListItem> get userPostList =>
      UnmodifiableListView(_userPostList);

  set userPostList(List<PostListItem> us) {
    _userPostList = us;
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
  void append(ChatEventModel msg, bool history) {
    if (!history) {
      if (!_active && _unreadMsgCount == 0 && _msgs.isNotEmpty) {
        msg.firstUnread = true;
      }
    }
    if (_msgs.isNotEmpty &&
        _msgs[_msgs.length - 1].source?.nick == msg.source?.nick) {
      msg.sameUser = true;
    }
    var timestamp = 0;
    var evnt = msg.event;
    if (evnt is PM) {
      timestamp =
          msg.source?.nick == null ? evnt.timestamp : evnt.timestamp * 1000;
    } else if (evnt is GCMsg) {
      timestamp =
          msg.source?.nick == null ? evnt.timestamp : evnt.timestamp * 1000;
    }
    if (timestamp != 0) {
      // Only show dateChange event if it is the first message or if the
      // previous message in the chat was from a different date.
      var dateChange =
          DateChangeEvent(DateTime.fromMillisecondsSinceEpoch(timestamp));
      if (_msgs.isEmpty) {
        _msgs.add(ChatEventModel(dateChange, null));
      } else {
        var lastTimestamp = 0;
        for (var i = _msgs.length - 1; i >= 0; i--) {
          var oldEvent = _msgs[i].event;
          if (oldEvent is PM) {
            lastTimestamp = _msgs[i].source?.nick == null
                ? oldEvent.timestamp
                : oldEvent.timestamp * 1000;
            break;
          } else if (oldEvent is GCMsg) {
            lastTimestamp = _msgs[i].source?.nick == null
                ? oldEvent.timestamp
                : oldEvent.timestamp * 1000;
            break;
          }
        }
        if (lastTimestamp != 0) {
          var lastDate = DateChangeEvent(
              DateTime.fromMillisecondsSinceEpoch(lastTimestamp));
          if (lastDate.msg != dateChange.msg) {
            _msgs.add(ChatEventModel(dateChange, null));
          }
        }
      }
    }

    _msgs.add(msg);
    if (!history) {
      if (!_active) {
        if (msg.event is PM || msg.event is GCMsg) {
          _unreadMsgCount += 1;
        } else {
          _unreadEventCount += 1;
        }
      }
    }
    if (isGC) {
      var dt = timestamp > 0
          ? DateTime.fromMillisecondsSinceEpoch(timestamp)
          : DateTime.now();
      appendDayGCMsgs(msg, dt);
    }

    notifyListeners();

    if (evnt is ProfileUpdated) {
      loadAvatar(evnt.abEntry.avatar);
    }
  }

  List<DayGCMessages> _dayGCMsgs = [];
  UnmodifiableListView<DayGCMessages> get dayGCMsgs =>
      UnmodifiableListView(_dayGCMsgs);

  // Group together message
  void appendDayGCMsgs(ChatEventModel msg, DateTime date) {
    bool dayFound = false;
    for (int i = 0; i < dayGCMsgs.length; i++) {
      if (dayGCMsgs[i].date == DateFormat("EEE - d MMM y").format(date)) {
        dayGCMsgs[i]._msgs.add(msg);
        dayFound = true;
      }
    }
    if (!dayFound) {
      var dayGCMsg = DayGCMessages();
      dayGCMsg._msgs = [msg];
      dayGCMsg.date = DateFormat("EEE - d MMM y").format(date);
      _dayGCMsgs.add(dayGCMsg);
    }
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
      var timestamp = DateTime.now().millisecondsSinceEpoch;
      var m = GCMsg(id, nick, msg, timestamp);
      var evnt = ChatEventModel(m, null);
      evnt.sentState = CMS_sending; // Track individual sending status?
      if (_msgs.isNotEmpty && _msgs[_msgs.length - 1].source == null) {
        evnt.sameUser = true;
      }
      _msgs.add(evnt);

      appendDayGCMsgs(evnt, DateTime.fromMillisecondsSinceEpoch(timestamp));

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
  bool _isSubscribing = false;
  bool get isSubscribing => _isSubscribing;
  set isSubscribing(bool b) {
    _isSubscribing = b;
    notifyListeners();
  }

  void subscribeToPosts() {
    var event = SynthChatEvent("Subscribing to user's posts");
    event.state = SCE_sending;
    isSubscribing = true;
    append(ChatEventModel(event, null), false);
    (() async {
      try {
        await Golib.subscribeToPosts(id);
        event.state = SCE_sent;
      } catch (error) {
        event.error = Exception(error);
        isSubscribing = false;
      }
    })();
  }

  Future<void> unsubscribeToPosts() {
    var event = SynthChatEvent("Unsubscribing from user's posts");
    event.state = SCE_sending;
    append(ChatEventModel(event, null), false);
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
    append(ChatEventModel(event, null), false);
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

class RescanNotifier extends ChangeNotifier {
  int _height = 0;
  int get progressHeight => _height;
  void set _progressHeight(int h) {
    _height = h;
    notifyListeners();
  }
}

class ClientModel extends ChangeNotifier {
  ClientModel() {
    _handleAcceptedInvites();
    _handleChatMsgs();
    _handleServerSessChanged();
    _handleGCListUpdates();
    _handleSSOrders();
    _handleRescanWalletProgress();
  }

  List<ChatModel> _sortedChats = [];
  UnmodifiableListView<ChatModel> get sortedChats =>
      UnmodifiableListView(_sortedChats);

  void set sortedChats(List<ChatModel> gc) {
    _sortedChats = gc;
    notifyListeners();
  }

  List<ChatModel> _filteredSearch = [];
  UnmodifiableListView<ChatModel> get filteredSearch =>
      UnmodifiableListView(_filteredSearch);

  set filteredSearch(List<ChatModel> us) {
    _filteredSearch = us;
    notifyListeners();
  }

  String _filteredSearchString = "";
  String get filteredSearchString => _filteredSearchString;
  set filteredSearchString(String b) {
    _filteredSearch = [];
    _filteredSearchString = b;
    if (b != "") {
      for (int i = 0; i < _sortedChats.length; i++) {
        if (_sortedChats[i].nick.toLowerCase().contains(b.toLowerCase())) {
          _filteredSearch.add(_sortedChats[i]);
        }
      }
      for (int i = 0; i < _hiddenChats.length; i++) {
        if (_hiddenChats[i].nick.toLowerCase().contains(b.toLowerCase())) {
          _filteredSearch.add(_hiddenChats[i]);
        }
      }
    }
    _filteredSearch
        .sort((a, b) => a._nick.toLowerCase().compareTo(b._nick.toLowerCase()));
    notifyListeners();
  }

  List<ChatModel> _hiddenChats = [];
  UnmodifiableListView<ChatModel> get hiddenChats =>
      UnmodifiableListView(_hiddenChats);

  set hiddenChats(List<ChatModel> us) {
    _hiddenChats = us;
    notifyListeners();
  }

  String _savedHiddenChats = "";
  String get savedHiddenChats => _savedHiddenChats;
  set savedHiddenChats(String b) {
    _savedHiddenChats = b;
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

  bool _showAddressBook = false;
  bool get showAddressBook => _showAddressBook;
  set showAddressBook(bool b) {
    _showAddressBook = b;
    notifyListeners();
  }

  void showAddressBookScreen() {
    showAddressBook = true;
  }

  void hideAddressBookScreen() {
    showAddressBook = false;
  }

  String _userPostListID = "";
  String get userPostListID => _userPostListID;
  set userPostListID(String b) {
    _userPostListID = b;
    notifyListeners();
  }

  List<PostListItem> _userPostList = [];
  UnmodifiableListView<PostListItem> get userPostList =>
      UnmodifiableListView(_userPostList);

  set userPostList(List<PostListItem> us) {
    _userPostList = us;
    notifyListeners();
  }

  void startChat(ChatModel chat, bool alreadyOpened) {
    if (!alreadyOpened) {
      List<ChatModel> newSortedChats = [];
      newSortedChats.add(chat);
      for (int i = 0; i < _sortedChats.length; i++) {
        newSortedChats.add(_sortedChats[i]);
      }
      _sortedChats = newSortedChats;
      _hiddenChats.remove(chat);
      if (_savedHiddenChats.contains(chat.nick)) {
        var savedHiddenChatsSplit = _savedHiddenChats.split(",");
        var newChatSplitStr = "";
        for (int i = 0; i < savedHiddenChatsSplit.length; i++) {
          if (!savedHiddenChatsSplit[i].contains(chat.nick)) {
            if (newChatSplitStr.isEmpty) {
              newChatSplitStr = chat.nick;
            } else {
              newChatSplitStr += ", ${chat.nick}";
            }
          }
        }
        _savedHiddenChats = newChatSplitStr;
        StorageManager.saveData('chatHiddenList', _savedHiddenChats);
        if (chat.isGC) {
          _subGCMenus[chat.id] = buildGCMenu(chat);
        } else {
          _subUserMenus[chat.id] = buildUserChatMenu(chat);
        }
      }
    }
    active = chat;
    notifyListeners();
  }

  final Map<String, List<ChatMenuItem>> _subGCMenus = {};
  UnmodifiableMapView<String, List<ChatMenuItem>> get subGCMenus =>
      UnmodifiableMapView(_subGCMenus);

  final Map<String, List<ChatMenuItem?>> _subUserMenus = {};
  UnmodifiableMapView<String, List<ChatMenuItem?>> get subUserMenus =>
      UnmodifiableMapView(_subUserMenus);

  List<ChatMenuItem?> _activeSubMenu = [];
  UnmodifiableListView<ChatMenuItem?> get activeSubMenu =>
      UnmodifiableListView(_activeSubMenu);

  void updateUserMenu(String id, List<ChatMenuItem?> menu) {
    _subUserMenus[id] = menu;
    notifyListeners();
  }

  void set activeSubMenu(List<ChatMenuItem?> sm) {
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

  String _activeUserPostAuthorID = "";
  String get activeUserPostAuthorID => _activeUserPostAuthorID;
  set activeUserPostAuthorID(String b) {
    _activeUserPostAuthorID = b;
    notifyListeners();
  }

  List<PostListItem> _activeUserPostList = [];
  UnmodifiableListView<PostListItem> get activeUserPostList =>
      UnmodifiableListView(_activeUserPostList);

  set activeUserPostList(List<PostListItem> us) {
    _activeUserPostList = us;
    notifyListeners();
  }

  void hideUserPostList() {
    activeUserPostList = [];
    notifyListeners();
  }

  ChatModel? _active;
  ChatModel? get active => _active;

  void set active(ChatModel? c) {
    _profile = null;
    // Remove new posts messages
    _active?.removeFirstUnread();
    _active?._setActive(false);
    _active = c;
    showAddressBook = false;
    c?._setActive(true);

    // Check for unreadMessages so we can turn off sidebar notification
    bool unreadChats = false;
    for (int i = 0; i < _sortedChats.length; i++) {
      if (_sortedChats[i].unreadMsgCount > 0) {
        unreadChats = true;
      }
    }

    hasUnreadChats = unreadChats;
    hideSubMenu();
    hideUserPostList();
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
      var c = _sortedChats.firstWhere((c) => c.nick == nick);
      active = c;
    } on StateError {
      // Ignore if chat doesn't exist.
    }
  }

  Future<void> handleSubscriptions() async {
    var newSubscriptions = await Golib.listSubscriptions();
    for (var subscription in newSubscriptions) {
      var chat = getExistingChat(subscription);
      chat?.isSubscribed = true;
    }
  }

  Future<void> newSentMsg(ChatModel? chat) async {
    if (chat != null) {
      _sortedChats.remove(chat);
      List<ChatModel> newSortedChats = [];
      newSortedChats.add(chat);
      for (int i = 0; i < _sortedChats.length; i++) {
        newSortedChats.add(_sortedChats[i]);
      }
      _sortedChats = newSortedChats;
    }
    notifyListeners();
  }

  Map<String, ChatModel> _activeChats = Map<String, ChatModel>();
  ChatModel? getExistingChat(String uid) => _activeChats[uid];
  ChatModel? getExistingChatByNick(String nick, bool isGC) {
    for (var chat in _activeChats.values) {
      if (chat.nick == nick && chat.isGC == isGC) {
        return chat;
      }
    }
    return null;
  }

  Future<ChatModel> _newChat(
      String id, String alias, bool isGC, bool startup) async {
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
    if (!isGC) {
      var subscriptions = await Golib.listSubscriptions();
      c.isSubscribed = subscriptions.contains(id);
    }
    _activeChats[id] = c;

    // Start with 500 messages and first page (0). We can load more with a scrolling
    // mechanism in the future
    List<LogEntry> chatHistory = [];
    try {
      chatHistory = await Golib.readChatHistory(id, isGC ? alias : "", 500, 0);
    } catch (exception) {
      // Ignore, as we might be opening a chat for a user that hasn't been fully
      // setup yet.
    }
    for (int i = 0; i < chatHistory.length; i++) {
      ChatEventModel evnt;
      var mine = chatHistory[i].from == _nick;
      if (isGC) {
        ChatModel? source;
        if (!mine) {
          source = getExistingChatByNick(chatHistory[i].from, false);
          print("source ${chatHistory[i].from} is ${source?.id ?? 'fooo'}");
        }

        var m = GCMsg(id, chatHistory[i].from, chatHistory[i].message,
            chatHistory[i].timestamp * (mine ? 1000 : 1));
        evnt = ChatEventModel(m, source);
      } else {
        var source = !mine ? c : null;
        var m = PM(
            id,
            chatHistory[i].message,
            mine,
            chatHistory[i].timestamp *
                (chatHistory[i].from == _nick ? 1000 : 1));
        evnt = ChatEventModel(m, source);
      }
      c.append(evnt, true);
    }

    // Sorting algo to attempt to retain order
    int sortUsedChats(ChatModel a, ChatModel b) {
      // First check if either is empty, if so prioritize the non-empty one.
      if (b._msgs.isEmpty) {
        if (a._msgs.isEmpty) {
          return 0;
        } else {
          return -1;
        }
      } else if (a._msgs.isEmpty) {
        return 1;
      }
      // If both are not empty, then check to see if unreadMsgCount is > 0 for
      // either.
      if (b.unreadMsgCount > 0 || a.unreadMsgCount > 0) {
        return b.unreadMsgCount.compareTo(a.unreadMsgCount);
      }
      // If unreadMsgCount are both 0, then check last message timestamps;
      var bTimeStamp = 0;
      var aTimeStamp = 0;
      var bLastMessage = b._msgs[b._msgs.length - 1];
      var bLastMessageEvent = b._msgs[b._msgs.length - 1].event;
      if (bLastMessageEvent is PM) {
        bTimeStamp = bLastMessage.source?.nick == null
            ? bLastMessageEvent.timestamp
            : bLastMessageEvent.timestamp * 1000;
      } else if (bLastMessageEvent is GCMsg) {
        bTimeStamp = bLastMessage.source?.nick == null
            ? bLastMessageEvent.timestamp
            : bLastMessageEvent.timestamp * 1000;
      }

      var aLastMessage = a._msgs[a._msgs.length - 1];
      var aLastMessageEvent = a._msgs[a._msgs.length - 1].event;
      if (aLastMessageEvent is PM) {
        aTimeStamp = aLastMessage.source?.nick == null
            ? aLastMessageEvent.timestamp
            : aLastMessageEvent.timestamp * 1000;
      } else if (aLastMessageEvent is GCMsg) {
        aTimeStamp = aLastMessage.source?.nick == null
            ? aLastMessageEvent.timestamp
            : aLastMessageEvent.timestamp * 1000;
      }
      return bTimeStamp.compareTo(aTimeStamp);
    }

    // TODO: this test should be superflous.
    if (_sortedChats.indexWhere((c) => c.id == id) == -1 &&
        ((c._msgs.isNotEmpty && !_savedHiddenChats.contains(c.nick)) ||
            (c._msgs.isEmpty && !startup))) {
      // Add to list of chats if not empty or the chat is empty and
      // not being create via readAddressBook call.
      _sortedChats.add(c);
      _sortedChats.sort(sortUsedChats);
      if (isGC) {
        _subGCMenus[c.id] = buildGCMenu(c);
      } else {
        _subUserMenus[c.id] = buildUserChatMenu(c);
      }
    } else if ((c._msgs.isEmpty || _savedHiddenChats.contains(c.nick)) &&
        startup) {
      // Add all empty chats on startup to hiddenGCs list.
      _hiddenChats.add(c);
      _hiddenChats.sort((a, b) => b.nick.compareTo(a.nick));
    }

    notifyListeners();

    return c;
  }

  void hideChat(ChatModel chat) {
    _active = null;
    _sortedChats.remove(chat);
    _hiddenChats.add(chat);
    _hiddenChats.sort((a, b) => b.nick.compareTo(a.nick));
    if (_savedHiddenChats.isNotEmpty) {
      _savedHiddenChats += ",${chat.nick}";
    } else {
      _savedHiddenChats = chat.nick;
    }
    StorageManager.saveData('chatHiddenList', _savedHiddenChats);
    notifyListeners();
  }

  void removeChat(ChatModel chat) {
    _sortedChats.remove(chat);
    if (chat.isGC) {
      _subGCMenus.remove(chat.id);
    } else {
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
      if (evnt is UserPostList) {
        if (evnt.posts.isNotEmpty) {
          var chat = getExistingChat(evnt.uid);
          chat?.userPostList = evnt.posts;
          activeUserPostList = evnt.posts;
          activeUserPostAuthorID = evnt.uid;
          notifyListeners();
        }
        continue;
      }
      if (evnt is FeedPostEvent) {
        if (evnt.sid == publicID) {
          // Ignore own relays.
          continue;
        }
      }
      if (evnt is FeedPostEvent) {
        if (evnt.sid == publicID) {
          // Ignore own relays.
          continue;
        }
      }
      if (evnt is PostSubscriptionResult) {
        if (evnt.wasSubRequest && evnt.error == "") {
          var chat = getExistingChat(evnt.id);
          chat!.isSubscribed = true;
          chat.isSubscribing = false;
          updateUserMenu(evnt.id, buildUserChatMenu(chat));
        } else if (evnt.error == "") {
          var chat = getExistingChat(evnt.id);
          chat!.isSubscribed = false;
          updateUserMenu(evnt.id, buildUserChatMenu(chat));
        }
      }

      var isGC =
          (evnt is GCMsg) || (evnt is GCUserEvent) || (evnt is GCAdminsChanged);

      var chat = await _newChat(evnt.sid, "", isGC, false);
      ChatModel? source;
      if (evnt is PM) {
        if (!evnt.mine) {
          source = chat;
          hasUnreadChats = true;
        }
      } else if (evnt is GCMsg) {
        source = await _newChat(evnt.senderUID, "", false, false);
        hasUnreadChats = true;
      } else if (evnt is GCUserEvent) {
        source = await _newChat(evnt.uid, "", false, false);
      } else {
        source = chat;
      }
      if (!chat.active) {
        hasUnreadChats = true;
      }
      chat.append(ChatEventModel(evnt, source), false);

      // Only do notifcications for GC messages or PMs
      if (evnt is GCMsg || evnt is PM) {
        if (source != null) {
          if (!chat.active) {
            NotificationService().showChatNotification(
                evnt.msg, source.nick, chat.isGC, chat.nick);
          }
        }
      }

      _sortedChats.remove(chat);
      List<ChatModel> newSortedChats = [];
      newSortedChats.add(chat);
      for (int i = 0; i < _sortedChats.length; i++) {
        newSortedChats.add(_sortedChats[i]);
      }
      _sortedChats = newSortedChats;

      notifyListeners();
    }
  }

  Future<void> readAddressBook() async {
    await StorageManager.readData('chatHiddenList').then((value) {
      if (value != null && value.length > 0) {
        _savedHiddenChats = value;
      }
    });
    var info = await Golib.getLocalInfo();
    _publicID = info.id;
    _nick = info.nick;
    var ab = await Golib.addressBook();
    ab.forEach((v) async {
      var c = await _newChat(v.id, v.nick, false, true);
      if (v.avatar != null) {
        c.loadAvatar(v.avatar);
      }
    });
    var gcs = await Golib.listGCs();
    gcs.forEach((v) => _newChat(v.id, v.name, true, true));

    loadingAddressBook = false;
  }

  ImageProvider? _myAvatar;
  ImageProvider? get myAvatar => _myAvatar;
  set myAvatar(ImageProvider? newAvatar) {
    _myAvatar = newAvatar;
    notifyListeners();
  }

  Future<void> fetchMyAvatar() async {
    var avatarData = await Golib.getMyAvatar();
    try {
      myAvatar = avatarData != null ? MemoryImage(avatarData) : null;
    } catch (exception) {
      print("unable to decode my avatar: $exception");
    }
  }

  void acceptInvite(Invitation invite) async {
    var user = await Golib.acceptInvite(invite);
    active = await _newChat(user.uid, user.nick, false, false);
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

  Future<void> fetchNetworkInfo() async {
    var res = await Golib.lnGetInfo();
    _network = res.chains[0].network;
  }

  void _handleAcceptedInvites() async {
    var stream = Golib.acceptedInvites();
    await for (var remoteUser in stream) {
      if (requestedMediateID(remoteUser.uid)) {
        _mediating.remove(remoteUser.uid);
      }
      var chat = await _newChat(remoteUser.uid, remoteUser.nick, false, false);
      chat.append(
          ChatEventModel(SynthChatEvent("KX Completed", SCE_received), null),
          false);
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
      _newChat(update.id, update.name, true, false);
    }
  }

  void _handleSSOrderPlaced(SSPlacedOrder po) async {
    try {
      var order = po.order;
      if (order.user == publicID) {
        print("Sample of message that would be sent to user: ${po.msg}");
        return;
      }
      var chat = getExistingChat(order.user);
      if (chat == null) {
        throw "user ${order.user} not found in placed simplestore order";
      }
      chat.sendMsg(po.msg);
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

  RescanNotifier _rescanNtf = RescanNotifier();
  RescanNotifier get rescanNotifier => _rescanNtf;
  void _handleRescanWalletProgress() async {
    var stream = Golib.rescanWalletProgress();
    await for (var h in stream) {
      _rescanNtf._progressHeight = h;
    }
  }
}
