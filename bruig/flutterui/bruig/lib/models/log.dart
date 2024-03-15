import 'dart:collection';

import 'package:flutter/cupertino.dart';
import 'package:golib_plugin/golib_plugin.dart';

final globalLogModel = LogModel();

class LogModel extends ChangeNotifier {
  LogModel() {
    _handleDcrlndLogLines();
  }

  bool _compactingDb = false;
  get compactingDb => _compactingDb;

  bool _compactingDbErrored = false;
  get compactingDbErrored => _compactingDbErrored;

  bool _migratingDb = false;
  get migratingDb => _migratingDb;

  final ListQueue<String> _log = ListQueue();
  Iterable<String> get log => UnmodifiableListView(_log);

  void _handleDcrlndLogLines() async {
    const maxLogLines = 500;
    var stream = Golib.logLines();
    await for (var line in stream) {
      _log.add(line.trim());
      while (_log.length > maxLogLines) {
        _log.removeFirst();
      }

      if (line.contains("Compacting database file at")) _compactingDb = true;
      if (line.contains("error during compact")) _compactingDbErrored = true;
      if (line.contains("Performing database schema migration")) {
        _migratingDb = true;
      }

      notifyListeners();
    }
  }
}
