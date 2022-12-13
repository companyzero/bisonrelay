import 'dart:collection';

import 'package:flutter/cupertino.dart';
import 'package:golib_plugin/golib_plugin.dart';

final globalLogModel = LogModel();

class LogModel extends ChangeNotifier {
  LogModel() {
    _handleDcrlndLogLines();
  }

  ListQueue<String> _log = ListQueue();
  Iterable<String> get log => UnmodifiableListView(_log);

  void _handleDcrlndLogLines() async {
    const maxLogLines = 500;
    var stream = Golib.logLines();
    await for (var line in stream) {
      _log.add(line.trim());
      while (_log.length > maxLogLines) {
        _log.removeFirst();
      }
      notifyListeners();
    }
  }
}
