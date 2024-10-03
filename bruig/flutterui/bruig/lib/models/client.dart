// ignore_for_file: constant_identifier_names, unnecessary_new, deprecated_colon_for_default_value

import 'dart:async';
import 'dart:collection';

import 'package:bruig/models/resources.dart';
import 'package:bruig/models/uistate.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:intl/intl.dart';
import 'package:bruig/storage_manager.dart';
import 'package:provider/provider.dart';

const SCE_unknown = 0;
const SCE_sending = 1;
const SCE_sent = 2;
const SCE_received = 3;
const SCE_history = 98;
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
  set state(int v) {
    _state = v;
    notifyListeners();
  }

  Exception? _error;
  Exception? get error => _error;
  set error(Exception? e) {
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

class RequestedUsersPostListEvent extends ChatEvent {
  const RequestedUsersPostListEvent(String uid)
      : super(uid, "Listing user's posts");
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
  set sentState(int v) {
    _sentState = v;
    notifyListeners();
  }

  String? _sendError;
  String? get sendError => _sendError;
  set sendError(String? err) {
    _sendError = err;
    _sentState = CMS_errored;
    notifyListeners();
  }

  bool _firstUnread = false;
  bool get firstUnread => _firstUnread;
  set firstUnread(bool b) {
    _firstUnread = b;
    notifyListeners();
  }

  bool _sameUser = false;
  bool get sameUser => _sameUser;
  set sameUser(bool b) {
    _sameUser = b;
    notifyListeners();
  }

  bool _showAvatar = false;
  bool get showAvatar => _showAvatar;
  set showAvatar(bool b) {
    _showAvatar = b;
    notifyListeners();
  }

  // isMessage is true if this event is a PM or GCM.
  bool get isMessage => (event is PM) || (event is GCMsg);
}

class DayGCMessages {
  List<ChatEventModel> _msgs = [];
  UnmodifiableListView<ChatEventModel> get msgs => UnmodifiableListView(_msgs);
  void append(ChatEventModel msg) {
    _msgs.insert(0, msg);
  }

  String date = "";
}

class AvatarModel extends ChangeNotifier {
  ImageProvider? _image;
  ImageProvider? get image => _image;
  void loadAvatar(Uint8List? newAvatar) async {
    if (newAvatar == null || newAvatar.isEmpty) {
      _image = null;
    } else {
      try {
        _image = MemoryImage(newAvatar);
        // Resize to a smaller size?
      } catch (exception) {
        debugPrint("Unable to decode avatar: $exception");
        _image = null;
      }
    }
    notifyListeners();
  }
}

class PostsListModel extends ChangeNotifier {
  List<PostListItem> _posts = [];
  List<PostListItem> get posts => _posts;
  bool get isNotEmpty => _posts.isNotEmpty;
  void _populatePosts(List<PostListItem> items) {
    _posts = items;
    notifyListeners();
  }
}

final ChatModel emptyChatModel = ChatModel.empty();

class ChatModel extends ChangeNotifier {
  final String id; // RemoteUID or GC ID
  final bool isGC;

  String _nick; // Nick or GC name
  String get nick => _nick;
  set nick(String nn) {
    _nick = nn;
    notifyListeners();
  }

  ChatModel(this.id, this._nick, this.isGC);
  factory ChatModel.empty() => ChatModel("", "", false);

  bool isSubscribed = false;

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

  int _scrollPosition = 0;
  int get scrollPosition => _scrollPosition;
  set scrollPosition(int s) {
    _scrollPosition = s;
    notifyListeners();
  }

  final AvatarModel _avatar = AvatarModel();
  AvatarModel get avatar => _avatar;

  // List of posts from this user.
  late final PostsListModel userPostsList = PostsListModel();

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

  final List<ChatEventModel> _msgs = [];
  UnmodifiableListView<ChatEventModel> get msgs => UnmodifiableListView(_msgs);
  void append(ChatEventModel msg, bool history, {doNotifyListeners = true}) {
    if (!history) {
      if (!_active && _unreadMsgCount == 0 && _msgs.isNotEmpty) {
        msg.firstUnread = true;
      }
    }
    if (_msgs.isNotEmpty && _msgs[0].source?.nick == msg.source?.nick) {
      msg.sameUser = true;
    }

    // Logic to show avatar on left of message.  Should only show on the bottom
    // message if multiple messages from a user.
    if (_msgs.isEmpty) {
      // If there are no messages yet, just show avatar on the new message
      msg.showAvatar = true;
    } else if (_msgs[0].source?.id == msg.source?.id && msg.isMessage) {
      // If there are messages then check to see if the previous message has the
      // same or different nick; if same remove avatar from previous and add
      // to new message. If different then just showAvatar on the new message
      // and keep previous message set to true.
      _msgs[0].showAvatar = false;
      msg.showAvatar = true;
    } else if (_msgs[_msgs.length - 1].source?.id != msg.source?.id) {
      msg.showAvatar = true;
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
        for (var i = 0; i < _msgs.length; i++) {
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
            _msgs.insert(0, (ChatEventModel(dateChange, null)));
          }
        }
      }
    }
    _msgs.insert(0, msg);
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

    if (doNotifyListeners) {
      notifyListeners();
    }

    if (evnt is ProfileUpdated) {
      avatar.loadAvatar(evnt.abEntry.avatar);
    }
  }

  final List<DayGCMessages> _dayGCMsgs = [];
  UnmodifiableListView<DayGCMessages> get dayGCMsgs =>
      UnmodifiableListView(_dayGCMsgs);

  // Group together message
  void appendDayGCMsgs(ChatEventModel msg, DateTime date) {
    bool dayFound = false;
    for (int i = 0; i < dayGCMsgs.length; i++) {
      if (dayGCMsgs[i].date == DateFormat("EEE - d MMM y").format(date)) {
        dayGCMsgs[i]._msgs.insert(0, (msg));
        dayFound = true;
      }
    }
    if (!dayFound) {
      var dayGCMsg = DayGCMessages();
      dayGCMsg._msgs = [msg];
      dayGCMsg.date = DateFormat("EEE - d MMM y").format(date);
      _dayGCMsgs.insert(0, dayGCMsg);
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
    var tip = Golib.payTip(id, amount);
    _msgs.insert(0, ChatEventModel(tip, this));
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
      _msgs.insert(0, (evnt));

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
      _msgs.insert(0, evnt);
      notifyListeners();

      try {
        await Golib.pm(m);
        evnt.sentState = CMS_sent;
      } catch (exception) {
        evnt.sendError = "$exception";
      }
    }
  }

  String workingMsg = "";
  bool _isSubscribing = false;
  bool get isSubscribing => _isSubscribing;
  set isSubscribing(bool b) {
    _isSubscribing = b;
    notifyListeners();
  }

  Future<void> subscribeToPosts() async {
    var event = SynthChatEvent("Subscribing to user's posts");
    event.state = SCE_sending;
    _isSubscribing = true;
    append(ChatEventModel(event, null), false);
    try {
      await Golib.subscribeToPosts(id);
      event.state = SCE_sent;
    } catch (error) {
      event.error = Exception(error);
      isSubscribing = false;
    }
  }

  Future<void> unsubscribeToPosts() async {
    var event = SynthChatEvent("Unsubscribing from user's posts");
    event.state = SCE_sending;
    append(ChatEventModel(event, null), false);
    try {
      await Golib.unsubscribeToPosts(id);
      isSubscribed = false;
      _isSubscribing = false;
      event.state = SCE_sent;
      notifyListeners();
    } catch (error) {
      event.error = Exception(error);
    }
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

  // mostRecentTimestamp returns the most recent timestamp for a message or zero.
  int mostRecentTimestamp() {
    int res = 0;
    for (var m in _msgs) {
      var event = m.event;
      if (event is PM) {
        res = m.source?.nick == null ? event.timestamp : event.timestamp * 1000;
        break;
      } else if (event is GCMsg) {
        res = m.source?.nick == null ? event.timestamp : event.timestamp * 1000;
        break;
      }
    }
    return res;
  }

  final ValueNotifier<List<String>?> _unkxdMembers = ValueNotifier(null);
  ValueNotifier<List<String>?> get unkxdMembers {
    if (isGC && _unkxdMembers.value == null) {
      (() async {
        _unkxdMembers.value = await Golib.gcListUnkxdMembers(id);
      })();
    }

    return _unkxdMembers;
  }

  void _removeUnkxdMember(String id) {
    if (_unkxdMembers.value == null) {
      // Handles case where chat is not a GC as well.
      return;
    }

    if (!_unkxdMembers.value!.contains(id)) {
      return;
    }

    var newList = _unkxdMembers.value!.toList();
    newList.remove(id);
    _unkxdMembers.value = newList;
  }
}

class ConnStateModel extends ChangeNotifier {
  ServerSessionState _state = ServerSessionState.empty();
  ServerSessionState get state => _state;

  bool get isOnline => _state.state == connStateOnline;
  bool get isCheckingWallet => _state.state == connStateCheckingWallet;
  String? get checkWalletErr => _state.checkWalletErr;

  _setState(ServerSessionState v) {
    _state = v;
    notifyListeners();
  }
}

class ActiveChatModel extends ChangeNotifier {
  ChatModel? _chat;
  ChatModel? get chat => _chat;

  void _setActiveCchat(ChatModel? v) {
    _chat = v;
    notifyListeners();
  }

  bool get empty => _chat == null;
}

// ChatsListModel tracks a list of chats. These could be either active or inactive
// ("hiden") chats.
class ChatsListModel extends ChangeNotifier {
  final List<ChatModel> _sorted = [];
  UnmodifiableListView<ChatModel> get sorted => UnmodifiableListView(_sorted);

  bool get isNotEmpty => _sorted.isNotEmpty;

  bool contains(ChatModel? c) => _sorted.contains(c);

  // hasUnreadMsgs is true if any chats have unread messages.
  bool get hasUnreadMsgs => _sorted.any((c) => c.unreadMsgCount > 0);

  // firstByNick returns the first chatModel with the given nick (if one exists).
  ChatModel? firstByNick(String nick) {
    var idx = _sorted.indexWhere((c) => c.nick == nick);
    return idx == -1 ? null : _sorted[idx];
  }

  // firstByNick returns the first chatModel with the given uid (if one exists).
  ChatModel? firstByUID(String id, {bool? isGC}) {
    var idx = _sorted
        .indexWhere((c) => c.id == id && (isGC == null || c.isGC == isGC));
    return idx == -1 ? null : _sorted[idx];
  }

  // Sorting algo to attempt to retain order
  static int _compareFunc(ChatModel a, ChatModel b) {
    // If both are empty, sort by nick.
    if (a._msgs.isEmpty && b._msgs.isEmpty) {
      return a.nick.compareTo(b.nick);
    }

    // If only one is empty, prioritize the non-empty one.
    if (a._msgs.isNotEmpty && b._msgs.isEmpty) {
      return -1;
    } else if (a._msgs.isEmpty && b._msgs.isNotEmpty) {
      return 1;
    }

    // If any have unread msgs, prioritize the one with a higher unread msg count.
    if (b.unreadMsgCount > 0 || a.unreadMsgCount > 0) {
      return b.unreadMsgCount.compareTo(a.unreadMsgCount);
    }

    // If unreadMsgCount are both 0, then check last message timestamps (most
    // recently communicated first).
    var ats = a.mostRecentTimestamp();
    var bts = b.mostRecentTimestamp();
    return bts.compareTo(ats);
  }

  void _sort({bool notify = true}) {
    _sorted.sort(_compareFunc);
    if (notify) {
      notifyListeners();
    }
  }

  // Add the chat to the list as the most recent chat.
  void _addActive(ChatModel chat) {
    if (_sorted.isNotEmpty && _sorted[0] == chat) {
      // Already the most recent chat.
      return;
    }

    _sorted.remove(chat);
    _sorted.insert(0, chat);
    notifyListeners();
  }

  // Adds to the list of chats but does not make it the most recent one.
  void _addInactive(ChatModel chat) {
    if (_sorted.contains(chat)) {
      return;
    }
    _sorted.add(chat);
  }

  void _remove(ChatModel chat) {
    if (_sorted.remove(chat)) {
      notifyListeners();
    }
  }
}

// BoolFlagModel is a model that holds a single bool value.
class BoolFlagModel extends ChangeNotifier {
  bool _val;
  bool get val => _val;
  set val(bool v) {
    if (v != _val) {
      _val = v;
      notifyListeners();
    }
  }

  void setWithoutNotification(bool v) {
    _val = v;
  }

  BoolFlagModel({initial = false}) : _val = initial;
}

class ClientModel extends ChangeNotifier {
  late final UIStateModel ui;

  ClientModel() {
    ui = UIStateModel();

    _handleServerSessChanged();
  }

  static ClientModel of(BuildContext context, {bool listen = true}) =>
      Provider.of<ClientModel>(context, listen: listen);

  // activeChats are chats that have messages in them, hiddenChats are the ones
  // that have no message or have been explicitly hidden from the chats list.
  final ChatsListModel activeChats = ChatsListModel();
  final ChatsListModel hiddenChats = ChatsListModel();

  // searchChats searches all chats that match the given string (both actice and
  // hidden).
  UnmodifiableListView<ChatModel> searchChats(String b,
      {bool ignoreGC = false}) {
    if (b == "") {
      return UnmodifiableListView([]);
    }

    b = b.toLowerCase();

    List<ChatModel> res = [];
    for (var list in [activeChats.sorted, hiddenChats.sorted]) {
      for (var chat in list) {
        if (ignoreGC && chat.isGC) {
          continue;
        }

        if (!chat.nick.toLowerCase().contains(b)) {
          continue;
        }

        res.add(chat);
      }
    }

    return UnmodifiableListView(res);
  }

  Future<void> createNewGCAndInvite(
      String gcName, List<ChatModel> usersToInvite) async {
    if (gcName == "") return;

    // Create the GC, create the chat model and make it the active and most
    // recent chat.
    var gcid = await Golib.createGC(gcName);
    var newChat = await _newChat(gcid, gcName, true);
    _makeActive(newChat, moveToMostRecent: true);

    // Invite users to chat.
    for (var user in usersToInvite) {
      var userChat = getExistingChat(user.id);
      if (userChat == null) {
        // Shouldn't happen.
        continue;
      }

      await Golib.inviteToGC(InviteToGC(newChat.id, user.id));
      var event = ChatEventModel(
          SynthChatEvent("Inviting ${user.nick} to ${newChat.nick}", SCE_sent),
          userChat);
      newChat.append(event, false);
    }
  }

  String _savedHiddenChats = "";
  String get savedHiddenChats => _savedHiddenChats;
  set savedHiddenChats(String b) {
    _savedHiddenChats = b;
    notifyListeners();
  }

  bool _loadingAddressBook = true;
  bool get loadingAddressBook => _loadingAddressBook;
  set loadingAddressBook(bool b) {
    _loadingAddressBook = b;
    notifyListeners();
  }

  final BoolFlagModel hasUnreadChats = BoolFlagModel();

  String _publicID = "";
  String get publicID => _publicID;

  String _nick = "";
  String get nick => _nick;

  final ConnStateModel connState = ConnStateModel();

  String _network = "";
  String get network => _network;

  final ActiveChatModel activeChat = ActiveChatModel();
  ChatModel? get active => activeChat.chat;
  void _makeActive(ChatModel? c, {bool moveToMostRecent = false}) {
    // Nothing to do if this is already the active chat.
    if (c == active) {
      return;
    }

    // De-active previously active chat.
    active?.removeFirstUnread();
    active?._setActive(false);

    // If there are no more active chats, nothing else to do.
    if (c == null) {
      activeChat._setActiveCchat(null);
      return;
    }
    ChatModel chat = c;

    // Activate new chat.
    chat._setActive(true);
    activeChat._setActiveCchat(chat);

    // Update list of chats.
    moveToMostRecent
        ? activeChats._addActive(chat)
        : activeChats._addInactive(chat);
    hiddenChats._remove(chat);

    // Check if this cleared the indicator for unread messages.
    hasUnreadChats.val = activeChats.hasUnreadMsgs;

    // Rework this?
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
    }
  }

  set active(ChatModel? c) => _makeActive(c);

  void setActiveByNick(String nick, bool isGC) {
    var c = activeChats.firstByNick(nick);
    if (c != null) {
      active = c;
    }
  }

  void setActiveByUID(String uid, {bool? isGC}) {
    var c = activeChats.firstByUID(uid, isGC: isGC);
    if (c != null) {
      active = c;
    }
  }

  Future<void> handleSubscriptions() async {
    var newSubscriptions = await Golib.listSubscriptions();
    for (var subscription in newSubscriptions) {
      var chat = getExistingChat(subscription);
      chat?.isSubscribed = true;
    }
  }

  // newSentMsg marks the chat as having sent a message and reorders the list
  // of chats.
  Future<void> newSentMsg(ChatModel chat) async {
    activeChats._addActive(chat);
    hiddenChats._remove(chat);
  }

  final Map<String, ChatModel> _activeChats = {};
  ChatModel? getExistingChat(String uid) => _activeChats[uid];
  ChatModel? getExistingChatByNick(String nick, bool isGC) {
    for (var chat in _activeChats.values) {
      if (chat.nick == nick && chat.isGC == isGC) {
        return chat;
      }
    }
    return null;
  }

  bool get hasChats => _activeChats.isNotEmpty;

  Future<ChatModel> _newChat(String id, String alias, bool isGC,
      {loadHistory = true}) async {
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
      // TODO: replace with a specific isSubscribed? call instead of always
      // fetching the list.
      var subscriptions = await Golib.listSubscriptions();
      c.isSubscribed = subscriptions.contains(id);
    }
    _activeChats[id] = c;

    // Start with 500 messages and first page (0). We can load more with a scrolling
    // mechanism in the future
    List<LogEntry> chatHistory = [];
    if (loadHistory) {
      try {
        chatHistory = await Golib.readChatHistory(id, isGC, 500, 0);
      } catch (exception) {
        // Ignore, as we might be opening a chat for a user that hasn't been fully
        // setup yet.
      }
    }
    for (int i = 0; i < chatHistory.length; i++) {
      ChatEventModel evnt;
      var mine = chatHistory[i].from == _nick;
      if (isGC) {
        ChatModel? source;
        ChatEvent m;
        if (chatHistory[i].internal) {
          m = SynthChatEvent(chatHistory[i].message, SCE_history);
        } else {
          if (!mine) {
            source = getExistingChatByNick(chatHistory[i].from, false);
          }

          m = GCMsg(id, chatHistory[i].from, chatHistory[i].message,
              chatHistory[i].timestamp * (mine ? 1000 : 1));
        }
        evnt = ChatEventModel(m, source);
      } else {
        ChatEvent m;
        var source = !mine ? c : null;
        if (chatHistory[i].internal) {
          m = SynthChatEvent(chatHistory[i].message, SCE_history);
        } else {
          m = PM(
              id,
              chatHistory[i].message,
              mine,
              chatHistory[i].timestamp *
                  (chatHistory[i].from == _nick ? 1000 : 1));
        }
        evnt = ChatEventModel(m, source);
      }
      c.append(evnt, true);
    }

    // Add the new chat to the list of chats (either hidden or active).
    if (c._msgs.isEmpty || _savedHiddenChats.contains(c.nick)) {
      hiddenChats._addInactive(c);
    } else {
      activeChats._addInactive(c);
    }

    notifyListeners();

    return c;
  }

  void hideChat(ChatModel chat) {
    if (chat == active) {
      active = null;
    }

    activeChats._remove(chat);
    hiddenChats._addActive(chat);

    // Save this chat as explicitly hidden.
    if (_savedHiddenChats.isNotEmpty) {
      _savedHiddenChats += ",${chat.nick}";
    } else {
      _savedHiddenChats = chat.nick;
    }
    StorageManager.saveData('chatHiddenList', _savedHiddenChats);
  }

  void removeChat(ChatModel chat) {
    activeChats._remove(chat);
    hiddenChats._remove(chat);

    if (active == chat) {
      active = null;
    }
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
          chat?.userPostsList._populatePosts(evnt.posts);
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
        var chat = getExistingChat(evnt.id);
        chat?.isSubscribing = false;
        if (evnt.wasSubRequest && evnt.error == "") {
          chat?.isSubscribed = true;
        } else if (evnt.error == "") {
          chat?.isSubscribed = false;
        } else if (evnt.error.contains("already subscribed")) {
          chat?.isSubscribed = true;
        } else if (evnt.error.contains("not subscribed")) {
          chat?.isSubscribed = false;
        }
      }

      var isGC = (evnt is GCMsg) ||
          (evnt is GCUserEvent) ||
          (evnt is GCAdminsChanged) ||
          (evnt is GCAddedMembers) ||
          (evnt is GCMemberParted) ||
          (evnt is GCUpgradedVersion);

      var chat = await _newChat(evnt.sid, "", isGC);
      ChatModel? source;
      String? sourceId;
      if (evnt is PM) {
        if (!evnt.mine) {
          source = chat;
        }
      } else if (evnt is GCMsg) {
        sourceId = evnt.senderUID;
      } else if (evnt is GCUserEvent) {
        sourceId = evnt.uid;
      } else if (evnt is GCAdminsChanged) {
        sourceId = evnt.source;
      } else if (evnt is GCAddedMembers) {
        sourceId = evnt.sid;
      } else if (evnt is GCMemberParted) {
        sourceId = evnt.sid;
      } else if (evnt is GCUpgradedVersion) {
        sourceId = evnt.sid;
      } else {
        source = chat;
      }
      if (sourceId != null) {
        source = getExistingChat(sourceId);
      }
      chat.append(ChatEventModel(evnt, source), false);

      // Make this the most recent chat.
      activeChats._addActive(chat);
      hiddenChats._remove(chat);

      // Track that there are unread messages.
      if (chat.unreadMsgCount > 0) {
        hasUnreadChats.val = true;
      }

      chat.notifyListeners();
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
    for (var v in ab) {
      var c = await _newChat(v.id, v.nick, false);
      if (v.avatar != null) {
        c.avatar.loadAvatar(v.avatar);
      }
    }
    var gcs = await Golib.listGCs();
    for (var v in gcs) {
      await _newChat(v.id, v.name, true);
    }

    // Re-sort list of chats.
    activeChats._sort();
    hiddenChats._sort();

    if (gcs.isEmpty && ab.length == 1) {
      // On newly setup clients, add the first contact to the list of contacts to
      // avoid confusing users before they send their first message.
      var firstChat = getExistingChat(ab[0].id)!;
      active = firstChat;
    }

    loadingAddressBook = false;

    // Start processing events.
    _handleAcceptedInvites();
    _handleChatMsgs();
    _handleGCListUpdates();
    _handleSSOrders();
  }

  AvatarModel myAvatar = AvatarModel();

  Future<void> fetchMyAvatar() async {
    var avatarData = await Golib.getMyAvatar();
    try {
      myAvatar.loadAvatar(avatarData);
    } catch (exception) {
      debugPrint("unable to decode my avatar: $exception");
    }
  }

  void acceptInvite(Invitation invite) async {
    var user = await Golib.acceptInvite(invite);
    active = await _newChat(user.uid, user.nick, false);
  }

  final List<String> _mediating = [];
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

      // Do not load history to avoid a duplicated "KX completed" message (if
      // that message is in the history).
      var chat = await _newChat(remoteUser.uid, remoteUser.nick, false,
          loadHistory: false);
      chat.append(
          ChatEventModel(SynthChatEvent("Completed KX", SCE_received), null),
          false);

      // Load user's avatar (async).
      (() async {
        var abEntry = await Golib.addressBookEntry(remoteUser.uid);
        chat.avatar.loadAvatar(abEntry.avatar);
      })();

      // Go through list of GCs, if any were missing this user, mark them as KXd.
      (() async {
        for (var c in _activeChats.values) {
          c._removeUnkxdMember(remoteUser.uid);
        }
      })();
    }
  }

  void _handleServerSessChanged() async {
    var stream = Golib.serverSessionChanged();
    await for (var state in stream) {
      connState._setState(state);
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
      if (order.user == publicID) {
        debugPrint("Sample of message that would be sent to user: ${po.msg}");
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
}
