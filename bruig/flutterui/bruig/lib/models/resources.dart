import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/cupertino.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';

class RequestedResource extends ChangeNotifier {
  final String uid;
  final ResourceTag tag;
  RMFetchResource? request;
  RMFetchResourceReply? reply;

  RequestedResource(this.uid, this.tag);
}

final sectionStartRegexp = RegExp(r'--section id=([\w]+) --');
final sectionEndRegexp = RegExp(r'--/section--');

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

  String pageData() {
    var utfData = currentPage?.response.data ?? Uint8List(0);
    var data = utf8.decode(utfData);

    // Remove --section-- strings (these are handled internally, not at the
    // markdown rendering level.
    data = data.replaceAll(sectionStartRegexp, "");
    data = data.replaceAll(sectionEndRegexp, "");
    data += "\n";

    return data;
  }

  void replaceAsyncTargetWithLoading(String asyncTargetID) {
    if (currentPage?.response.data == null) {
      return;
    }

    var data = utf8.decode(currentPage!.response.data!);
    try {
      var reStartPattern = r'--section id=' + asyncTargetID + r' --\n';
      var reStart = RegExp(reStartPattern);
      var startPos = reStart.firstMatch(data);
      if (startPos == null) {
        // Did not find the target location.
        return;
      }

      var endPos = sectionEndRegexp.firstMatch(data.substring(startPos.end));
      if (endPos == null) {
        // Unterminated section.
        return;
      }

      var endPosStart =
          endPos.start + startPos.end; // Convert to absolute index

      // Create the new buffer, replacing the contents inside the section with
      // the new data.
      data =
          "${data.substring(0, startPos.end)}(⏳ Loading response)\n${data.substring(endPosStart)}";
    } catch (exception) {
      // Ignore any errors when trying to replace this target.
      debugPrint(
          "Unable to set target $asyncTargetID in page as loading: $exception");
    }

    var utfData = utf8.encode(data);
    currentPage = currentPage!
        .copyWith(response: currentPage!.response.copyWith(data: utfData));
  }

  void replaceAsyncTargets(List<FetchedResource> history) {
    if (currentPage?.response.data == null) {
      return;
    }

    var data = utf8.decode(currentPage!.response.data!);
    for (var fr in history) {
      try {
        if (fr.response.data == null) {
          continue;
        }

        var reStartPattern = r'--section id=' + fr.asyncTargetID + r' --\n';
        var reStart = RegExp(reStartPattern);
        var startPos = reStart.firstMatch(data);
        if (startPos == null) {
          // Did not find the target location.
          continue;
        }

        var endPos = sectionEndRegexp.firstMatch(data.substring(startPos.end));
        if (endPos == null) {
          // Unterminated section.
          continue;
        }

        var endPosStart =
            endPos.start + startPos.end; // Convert to absolute index

        // Create the new buffer, replacing the contents inside the section with
        // the new data.
        data = data.substring(0, startPos.end) +
            utf8.decode(fr.response.data!) +
            data.substring(endPosStart);
      } catch (exception) {
        // Ignore any errors when trying to replace this target.
        debugPrint(
            "Unable to replace target ${fr.asyncTargetID} in page: $exception");
      }
    }

    var utfData = utf8.encode(data);
    currentPage = currentPage!
        .copyWith(response: currentPage!.response.copyWith(data: utfData));
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
      int parentPage, dynamic data, String asyncTargetID) async {
    sessionID = await Golib.fetchResource(
        uid, path, null, sessionID, parentPage, data, asyncTargetID);

    var sess = session(sessionID);
    if (asyncTargetID == "") {
      sess._setLoading(true);
    } else {
      sess.replaceAsyncTargetWithLoading(asyncTargetID);
    }
    return sess;
  }

  void _handleFetchedResources() async {
    var stream = Golib.fetchedResources();
    await for (var fr in stream) {
      var sess = session(fr.sessionID);

      if (fr.asyncTargetID != "") {
        List<FetchedResource> targets;
        if (sess.currentPage == null) {
          // Received async response without having full page, load page and
          // prior history.
          try {
            var history = await Golib.loadFetchedResource(
                fr.uid, fr.sessionID, fr.pageID);
            sess.currentPage = history[0];
            targets = history.sublist(1);
          } catch (exception) {
            debugPrint("Exception handling fetched resource: $exception");
            continue;
          }
        } else {
          targets = [fr];
        }

        // Replace the async target contents.
        sess.replaceAsyncTargets(targets);
      } else {
        // Full page reload.
        sess.currentPage = fr;
      }
    }
  }
}
