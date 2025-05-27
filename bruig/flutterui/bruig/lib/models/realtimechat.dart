import 'package:bruig/models/client.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:duration/duration.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';

class RTDTLivePeerModel extends ChangeNotifier {
  final String sessionRV;
  final int peerID;
  RTDTLivePeerModel(this.sessionRV, this.peerID);

  bool _isLive = false;
  bool get isLive => _isLive;
  void _setIsLive(bool v) {
    _isLive = v;
    notifyListeners();
  }

  bool _hasSound = false;
  bool get hasSound => _hasSound;

  bool _hasSoundStream = false;
  bool get hasSoundStream => _hasSoundStream;

  void _setHasSoundAndStream(bool newHasSound, bool newHasSoundStream) {
    _hasSound = newHasSound;
    _hasSoundStream = newHasSoundStream;
    notifyListeners();
  }

  double _gain = 0;
  double get gain => _gain;
  Future<double> modifyGain(double delta) async {
    var res =
        await Golib.rtdtModifyLivePeerVolumeGain(sessionRV, peerID, delta);
    _gain = res.gain;
    notifyListeners();
    return res.gain;
  }

  void _init() {
    modifyGain(0); // Fetch current gain.
  }

  void _removedFromSession() {
    _hasSound = false;
    _isLive = false;
    notifyListeners();
  }
}

class RTDTSessionModel extends ChangeNotifier {
  RTDTSession _info;
  RTDTSession get info => _info;
  late final ChatModel chat; // Ephemeral chat messages.

  RTDTSessionModel(this._info) {
    chat = ChatModel(_info.metadata.rv, "", false);
    const startMsg = "Note: messages in realtime chats are ephemeral and only "
        "received by people connected to the live session";
    chat.append(
        ChatEventModel(SynthChatEvent(startMsg, SCE_history), null), false);
  }

  String get sessionRV => _info.metadata.rv;
  String get sessionShortRV => _info.metadata.rv.substring(0, 16);

  void _updateInfo(RTDTSession newInfo) {
    _info = newInfo;
    notifyListeners();
  }

  bool _inLiveSession = false;
  bool get inLiveSession => _inLiveSession;
  bool _joiningLiveSession = false;
  bool get joiningLiveSession => _joiningLiveSession;
  bool _leavingLiveSession = false;
  bool get leavingLiveSession => _leavingLiveSession;

  bool get isAdmin => _info.sessionCookie != "";

  DateTime? _bannedUntil;
  DateTime? get bannedUntil => _bannedUntil;
  void _kickedAndBanned(int banSeconds) {
    if (banSeconds > 0) {
      _bannedUntil = DateTime.now().add(Duration(seconds: banSeconds));
    }
    _inLiveSession = false;
    _hasHotAudio = false;
    notifyListeners();
  }

  Future<void> _joinLiveSession() async {
    _joiningLiveSession = true;
    notifyListeners();
    try {
      await Golib.rtdtJoinSession(_info.metadata.rv);
      _inLiveSession = true;
    } finally {
      _joiningLiveSession = false;
      notifyListeners();
    }
  }

  Future<void> _leaveLiveSession() async {
    _leavingLiveSession = true;
    notifyListeners();
    try {
      await Golib.rtdtLeaveSession(_info.metadata.rv);
      _inLiveSession = false;
      _hasHotAudio = false;
    } finally {
      _leavingLiveSession = false;
      notifyListeners();
    }
  }

  bool _hasHotAudio = false;
  bool get hasHotAudio => _hasHotAudio;
  void _setHotAudio(bool v) {
    _hasHotAudio = v;
    notifyListeners();
  }

  void _removedFromSession() {
    _hasHotAudio = false;
    _inLiveSession = false;
    notifyListeners();
  }

  final Map<int, RTDTLivePeerModel> _livePeers = {};
  RTDTLivePeerModel? livePeer(int peerID) => _livePeers[peerID];
  RTDTLivePeerModel _getOrNewLivePeer(int peerID) {
    if (_livePeers.containsKey(peerID)) {
      return _livePeers[peerID]!;
    }
    var peer = RTDTLivePeerModel(_info.metadata.rv, peerID);
    peer._init();
    _livePeers[peerID] = peer;
    notifyListeners();
    return peer;
  }

  void _removeMember(int peerID, {bool notify = true}) {
    _livePeers.remove(peerID)?._removedFromSession();
    if (notify) notifyListeners();
  }

  Future<void> rotateCookies() async {
    await Golib.rtdtRotateCookies(sessionRV);
  }

  int peerIDForUID(String uid) {
    if (_info.members.isNotEmpty) {
      var idx = _info.members.indexWhere((m) => m.uid == uid);
      if (idx > -1) {
        return _info.members[idx].peerID;
      }
    } else {
      var idx =
          _info.metadata.publishers.indexWhere((m) => m.publisherID == uid);
      if (idx > -1) {
        return _info.metadata.publishers[idx].peerID;
      }
    }

    return 0;
  }

  String uidForPeerID(int peerID) {
    if (_info.members.isNotEmpty) {
      var idx = _info.members.indexWhere((m) => m.peerID == peerID);
      if (idx > -1) {
        return _info.members[idx].uid;
      }
    } else {
      var idx = _info.metadata.publishers.indexWhere((m) => m.peerID == peerID);
      if (idx > -1) {
        return _info.metadata.publishers[idx].publisherID;
      }
    }

    return "";
  }

  Future<void> kickMember(int peerID, int banSeconds) async {
    await Golib.rtdtKickFromLiveSession(sessionRV, peerID, banSeconds);
    _livePeers.remove(peerID)?._removedFromSession();
    notifyListeners();
  }

  Future<void> removeMember(String uid) async {
    await Golib.rtdtRemoveFromSession(sessionRV, uid, "");
    _livePeers.remove(peerIDForUID(uid))?._removedFromSession();
    _updateInfo(
        await Golib.rtdtGetSession(sessionRV)); // Automatically notifies.
  }

  void _refreshFromLive(LiveRTDTSession live) {
    _inLiveSession = true;
    _hasHotAudio = live.hotAudio;

    List<int> toRemove = [];
    for (var peerID in _livePeers.keys) {
      if (!live.peers.containsKey(peerID)) {
        // Member was live but is not anymore.
        toRemove.add(peerID);
      }
    }
    for (var peerID in toRemove) {
      _removeMember(peerID, notify: false);
    }

    for (var peerID in live.peers.keys) {
      if (peerID == _info.localPeerID) {
        continue;
      }

      if (!_livePeers.containsKey(peerID)) {
        // New member.
        var lp = live.peers[peerID]!;
        var peer = RTDTLivePeerModel(_info.metadata.rv, peerID);
        _livePeers[peerID] = peer;
        peer._hasSound = lp.hasSound;
        peer._hasSoundStream = lp.hasSoundStream;
        peer._isLive = true;
        peer._gain = lp.volumeGain;
      }
    }

    notifyListeners();
  }

  void _appendChatMsg(
      ClientModel client, int sourceID, String msg, int timestamp) {
    var uid = uidForPeerID(sourceID);
    var chatMsg = GCMsg(uid, sessionRV, msg, timestamp);
    var source = client.getExistingChat(uid);
    chat.append(ChatEventModel(chatMsg, source), false);
  }

  // Load messages that were stored in client.
  void _loadTrackedChatMessages(ClientModel client) async {
    try {
      var messages = await Golib.rtdtGetChatMessages(sessionRV);
      for (var msg in messages) {
        _appendChatMsg(client, msg.sourceID, msg.message, msg.timestamp);
      }
    } catch (exception) {
      debugPrint("Unable to load rtdt messages: $exception");
    }
  }

  Future<void> sendMsg(ClientModel client, String msg) async {
    var timestamp = DateTime.now().millisecondsSinceEpoch ~/ 1000;
    var chatMsg = GCMsg(client.publicID, sessionRV, msg, timestamp);
    var msgModel = ChatEventModel(chatMsg, null);
    msgModel.sentState = CMS_sending;
    chat.append(msgModel, false);

    try {
      await Golib.rtdtSendChatMsg(sessionRV, msg);
      msgModel.sentState = CMS_sent;
    } catch (exception) {
      msgModel.sendError = "$exception";
    }
  }
}

class ActiveRealTimeSessionChatModel extends ChangeNotifier {
  RTDTSessionModel? _active;
  RTDTSessionModel? get active => _active;
  set active(RTDTSessionModel? newActive) {
    _active = newActive;
    notifyListeners();
  }
}

class ActiveHotAudioSessionModel extends ChangeNotifier {
  RTDTSessionModel? _active;
  RTDTSessionModel? get active => _active;
  void _setActive(RTDTSessionModel? newActive) {
    _active = newActive;
    notifyListeners();
  }
}

class LiveRTDTSessionsModel extends ChangeNotifier {
  final Map<String, RTDTSessionModel> _sessions = {};
  List<RTDTSessionModel> get sessions => _sessions.values.toList();
  bool get hasSessions => _sessions.isNotEmpty;

  void _setLive(RTDTSessionModel live) {
    if (!_sessions.containsKey(live.sessionRV)) {
      _sessions[live.sessionRV] = live;
      notifyListeners();
    }
  }

  void _delLive(RTDTSessionModel live) {
    if (_sessions.containsKey(live.sessionRV)) {
      _sessions.remove(live.sessionRV);
      notifyListeners();
    }
  }

  void _replace(List<RTDTSessionModel> liveSessions) {
    _sessions.clear();
    for (var live in liveSessions) {
      _sessions[live.sessionRV] = live;
    }
    notifyListeners();
  }
}

class RealtimeChatRTTModel extends ChangeNotifier {
  // Assume there's a single server/connection for now.
  int _lastRTTNano = 0;
  int get lastRTTNano => _lastRTTNano;

  String get lastRTTNanoStr {
    if (_lastRTTNano <= 0) {
      return "";
    }
    if (_lastRTTNano < 1000) {
      return "${_lastRTTNano}ns";
    }
    var us = (_lastRTTNano / 1000).truncate();
    if (us < 1000) {
      return "$usµs";
    }
    var ms = (us / 1000).truncate();
    if (ms < 1000) {
      return "${ms}ms";
    }
    var s = (ms / 1000).truncate();
    return "${s}s";
  }

  RealtimeChatRTTModel() {
    _handleRTTUpdates();
  }

  void _handleRTTUpdates() async {
    await for (var update in Golib.rtdtRTTStream()) {
      _lastRTTNano = update.rttNano;
      notifyListeners();
    }
  }
}

class RealtimeChatModel extends ChangeNotifier {
  static RealtimeChatModel of(BuildContext context, {bool listen = true}) =>
      Provider.of<RealtimeChatModel>(context, listen: listen);

  final Map<String, RTDTSessionModel> _sessions = {};
  List<RTDTSessionModel> get sessions => _sessions.values.toList();
  final Map<String, RTDTSessionModel> _gcSessions = {};
  RTDTSessionModel? gcSession(String gcID) => _gcSessions[gcID];

  final LiveRTDTSessionsModel liveSessions = LiveRTDTSessionsModel();

  final ActiveRealTimeSessionChatModel active =
      ActiveRealTimeSessionChatModel();

  final ClientModel client;
  final SnackBarModel snackbar;

  RealtimeChatModel(this.client, this.snackbar) {
    _handleSessEvents();
    _handlePeerUpdates();
  }

  void _handleSessEvents() async {
    var stream = Golib.rtdtSessionEvents();
    await for (var updt in stream) {
      if (updt is RTDTSessionUpdate) {
        var isNew = await _updateSess(updt.update.sessionRV);
        if (isNew) notifyListeners();
      }

      if (updt is RTDTKickedFromLive) {
        var sess = _sessions[updt.sessionRV];
        if (sess == null) {
          continue;
        }
        sess._kickedAndBanned(updt.banSeconds);
        if (hotAudioSession.active == sess) {
          hotAudioSession._setActive(null);
        }
        liveSessions._delLive(sess);
        var msg =
            "Kicked from realtime chat ${updt.sessionRV.substring(0, 16)}";
        if (updt.banSeconds > 0) {
          var durStr = prettyDuration(Duration(seconds: updt.banSeconds),
              abbreviated: true);
          msg += "\nTemporarily banned for $durStr";
        }
        snackbar.error(msg);
      }

      if (updt is RTDTRemovedFromSession) {
        _removeSess(updt.sessionRV, notifyRemoved: true);
        var chat = client.getExistingChat(updt.uid);
        var nick = chat?.nick ?? updt.uid;
        var msg =
            "$nick removed local client from realtime chat session ${updt.sessionRV.substring(0, 16)}.";
        if (updt.reason != "") {
          msg += "\nReason given: '${updt.reason}'";
        }
        if (chat != null) {
          chat.append(
              ChatEventModel(SynthChatEvent(msg, SCE_received), null), false);
        }
        snackbar.error(msg);
      }

      if (updt is RTDTRotatedCookies) {
        var chat = client.getExistingChat(updt.uid);
        if (chat != null) {
          var msg =
              "${chat.nick} rotated cookies of realtime chat session ${updt.sessionRV}";
          chat.append(
              ChatEventModel(SynthChatEvent(msg, SCE_received), null), false);
        }
      }

      if (updt is RTDTSessionDissolved) {
        _removeSess(updt.sessionRV, notifyRemoved: true);
        var chat = client.getExistingChat(updt.uid);
        var nick = chat?.nick ?? updt.uid;
        var msg = "$nick dissolved realtime chat session ${updt.sessionRV}";
        if (chat != null) {
          chat.append(
              ChatEventModel(SynthChatEvent(msg, SCE_received), null), false);
        }
        snackbar.error(msg);
      }

      if (updt is RTDTPeerExited) {
        _sessions[updt.sessionRV]?._removeMember(updt.peerID);
        _updateSess(updt.sessionRV);
        var chat = client.getExistingChat(updt.uid);
        if (chat != null) {
          var msg = "User left realtime chat session ${updt.sessionRV}";
          chat.append(
              ChatEventModel(SynthChatEvent(msg, SCE_received), null), false);
        }
      }

      if (updt is RTDTRemadeLiveSessionHot) {
        var sess = _sessions[updt.sessionRV];
        if (sess == null) {
          continue;
        }
        sess._setHotAudio(true);
        if (hotAudioSession.active != sess) {
          hotAudioSession.active?._setHotAudio(false);
          hotAudioSession._setActive(sess);
        }
        liveSessions._setLive(sess);
      }

      if (updt is RTDTChatMessage) {
        var timestamp = DateTime.now().millisecondsSinceEpoch ~/ 1000;
        _sessions[updt.sessionRV]?._appendChatMsg(
            client, updt.publisher.peerID, updt.message, timestamp);
      }
    }
  }

  void _handlePeerUpdates() async {
    var stream = Golib.rtdtLivePeerUpdates();
    await for (var updt in stream) {
      var sess = _sessions[updt.update.sessionRV];
      if (sess == null) {
        continue;
      }

      var peer = sess._getOrNewLivePeer(updt.update.peerId);
      switch (updt.ntfType) {
        case NTRTDTLivePeerJoined:
          peer._setIsLive(true);
          break;

        case NTRTDTLivePeerStalled:
          peer._setIsLive(false);
          break;

        case NTRTDTPeerSoundChanged:
          peer._setHasSoundAndStream(
              updt.update.hasSound, updt.update.hasSoundStream);
          break;
      }
    }
  }

  // Returns true if this is a new session.
  Future<bool> _updateSess(String rv) async {
    var sess = await Golib.rtdtGetSession(rv);
    var contains = _sessions.containsKey(rv);
    if (contains) {
      if (_sessions[rv]?.info.gc != sess.gc) {
        // This should not happen (changing the "GC" field).
        _gcSessions.remove(_sessions[rv]?.info.gc);
      }
      _sessions[rv]?._updateInfo(sess);
    } else {
      _sessions[rv] = RTDTSessionModel(sess);
    }

    if (sess.gc != "") {
      _gcSessions[sess.gc] = _sessions[rv]!;
    }

    if (!contains) {
      notifyListeners();
    }

    return !contains;
  }

  // Removes the session.
  void _removeSess(String rv, {notifyRemoved = false}) async {
    var sess = _sessions[rv];
    if (sess == null) {
      return;
    }

    if (active.active == sess) {
      active.active = null;
    }

    if (hotAudioSession.active == sess) {
      hotAudioSession._setActive(null);
    }

    _sessions.remove(rv);
    if (notifyRemoved) {
      sess._removedFromSession();
    }

    if (sess.info.gc != "") {
      _gcSessions.remove(sess.info.gc);
    }

    liveSessions._delLive(sess);

    notifyListeners();
  }

  Future<void> refreshSessions() async {
    var list = await Golib.rtdtListSessions();
    var listRVs = {};
    List<RTDTSessionModel> liveSessionsList = [];
    for (var rv in list) {
      var isNew = await _updateSess(rv);
      var sess = _sessions[rv]!;
      listRVs[rv] = sess;

      var liveSess = await Golib.rtdtGetLiveSession(rv);
      if (liveSess == null) {
        if (sess.inLiveSession) {
          sess._removedFromSession();
        }

        if (hotAudioSession.active == sess) {
          hotAudioSession._setActive(null);
        }
      } else {
        sess._refreshFromLive(liveSess);
        liveSessionsList.add(sess);
        if (isNew) {
          // Only reload from history of tracked messages if this is the first
          // time this is created. This handles cases when the UI was detached
          // and then reattached on mobile.
          sess._loadTrackedChatMessages(client);
        }
        if (liveSess.hotAudio && hotAudioSession.active != sess) {
          hotAudioSession.active?._setHotAudio(false);
          hotAudioSession._setActive(sess);
        }
      }
    }

    // Remove any missing sessions.
    var toDelete = [];
    for (var rv in _sessions.keys) {
      if (listRVs[rv] != null) {
        continue;
      }

      toDelete.add(rv);
    }
    for (var rv in toDelete) {
      _removeSess(rv, notifyRemoved: true);
    }

    liveSessions._replace(liveSessionsList);
  }

  // Unique key per invite.
  String _inviteKey(InvitedToRTDTSess invite) =>
      invite.inviter + invite.invite.rv + invite.invite.tag.toString();

  final List<String> _canceledInvites = [];
  void cancelInvite(InvitedToRTDTSess invite) =>
      _canceledInvites.add(_inviteKey(invite));
  bool isInviteCanceled(InvitedToRTDTSess invite) =>
      _canceledInvites.contains(_inviteKey(invite));

  final List<String> _acceptedInvites = [];
  bool isInviteAccepted(InvitedToRTDTSess invite) =>
      _acceptedInvites.contains(_inviteKey(invite));
  Future<void> acceptInvite(InvitedToRTDTSess invite) async {
    _acceptedInvites.add(_inviteKey(invite));
    await Golib.rtdtAcceptInvite(AcceptRTDTInviteArgs(
        invite.inviter, invite.invite, invite.invite.allowedAsPublisher));
  }

  Future<void> joinLiveSession(RTDTSessionModel sess) async {
    await sess._joinLiveSession();
    liveSessions._setLive(sess);
  }

  Future<void> leaveLiveSession(RTDTSessionModel sess) async {
    await sess._leaveLiveSession();
    liveSessions._delLive(sess);
  }

  final ActiveHotAudioSessionModel hotAudioSession =
      ActiveHotAudioSessionModel();

  Future<void> switchHotAudio(RTDTSessionModel targetSession) async {
    await Golib.rtdtSwitchHotAudio(targetSession._info.metadata.rv);
    if (hotAudioSession.active != targetSession) {
      hotAudioSession.active?._setHotAudio(false);
      hotAudioSession._setActive(targetSession);
    }
    targetSession._setHotAudio(true);
  }

  Future<void> disableHotAudio() async {
    await Golib.rtdtSwitchHotAudio("");
    if (hotAudioSession.active != null) {
      hotAudioSession.active?._setHotAudio(false);
      hotAudioSession._setActive(null);
    }
  }

  Future<void> createSession(
      int size, String descr, List<String> toInvite) async {
    String rv =
        await Golib.rtdtCreateSession(CreateRTDTSessArgs(size, descr, null));
    await _updateSess(rv);
    for (var uid in toInvite) {
      inviteToSession(rv, uid);
    }
  }

  Future<void> createSessionFromGC(String gc, int extraSize) async {
    String rv =
        await Golib.rtdtCreateSession(CreateRTDTSessArgs(extraSize, "", gc));
    await _updateSess(rv);
  }

  Future<void> exitSession(String sessionRV) async {
    await Golib.rtdtExitSession(sessionRV);
    _removeSess(sessionRV);
  }

  Future<void> dissolveSession(String sessionRV) async {
    await Golib.rtdtDissolveSession(sessionRV);
    _removeSess(sessionRV);
  }

  Future<void> inviteToSession(String sessionRV, String uid) async {
    await Golib.rtdtInviteToSession(InviteToRTDTSessArgs(sessionRV, uid, true));
    var cm = client.getExistingChat(uid);
    if (cm != null) {
      cm.append(
          ChatEventModel(
              SynthChatEvent(
                  "Invited to realtime chat session ${sessionRV.substring(0, 16)}",
                  SCE_sent),
              null),
          false);
    }
  }
}
