import 'package:flutter/foundation.dart';
import 'dart:collection';

import 'package:flutter/material.dart';

class SnackBarMessage {
  final String msg;
  final bool error;
  final DateTime timestamp;

  SnackBarMessage(this.msg, this.error, this.timestamp);
  factory SnackBarMessage.empty() => SnackBarMessage("", false, DateTime.now());
}

class SnackBarModel extends ChangeNotifier {
  List<SnackBarMessage> _snackBars = [];
  UnmodifiableListView<SnackBarMessage> get snackBars =>
      UnmodifiableListView(_snackBars);
  void success(String msg) {
    _snackBars.add(SnackBarMessage(msg, false, DateTime.now()));
    notifyListeners();
  }

  void error(String msg) {
    _snackBars.add(SnackBarMessage(msg, true, DateTime.now()));
    notifyListeners();
  }
}
