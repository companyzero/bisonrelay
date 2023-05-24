import 'package:flutter/cupertino.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';

class RequestedResource extends ChangeNotifier {
  final String uid;
  final ResourceTag tag;
  RMFetchResource? request;
  RMFetchResourceReply? reply;

  RequestedResource(this.uid, this.tag);

  void _replyReceived(RMFetchResource req, RMFetchResourceReply res) {
    request = req;
    reply = res;
    notifyListeners();
  }
}

class PagesSession extends ChangeNotifier {
  final int id;
  PagesSession(this.id);

  FetchedResource? _current;
  FetchedResource? get currentPage => _current;
  set currentPage(FetchedResource? v) {
    _current = v;
    notifyListeners();
    _loading = false;
  }

  bool _loading = false;
  bool get loading => _loading;
  void _setLoading(bool v) {
    _loading = v;
    notifyListeners();
  }
}

class ResourcesModel extends ChangeNotifier {
  ResourcesModel() {
    _handleFetchedResources();
  }

  final Map<int, PagesSession> _sessions = {};
  PagesSession session(int id) {
    if (!_sessions.containsKey(id)) {
      var sess = PagesSession(id);
      _sessions[id] = sess;
      notifyListeners();
    }
    return _sessions[id]!;
  }

  List<PagesSession> get sessions => _sessions.values.toList(growable: false);

  PagesSession? _mostRecent;
  PagesSession? get mostRecent => _mostRecent;
  set mostRecent(PagesSession? v) {
    if (_mostRecent != v) {
      _mostRecent = v;
      notifyListeners();
    }
  }

  Future<PagesSession> fetchPage(String uid, List<String> path, int sessionID,
      int parentPage, dynamic data) async {
    sessionID =
        await Golib.fetchResource(uid, path, null, sessionID, parentPage, data);

    var sess = session(sessionID);
    sess._setLoading(true);
    return sess;
  }

  void _handleFetchedResources() async {
    var stream = Golib.fetchedResources();
    await for (var fr in stream) {
      var sess = session(fr.sessionID);
      sess.currentPage = fr;
    }
  }
}
