import 'package:flutter/foundation.dart';
import 'dart:collection';

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
  void append(SnackBarMessage snackBarMessage) {
    _snackBars.add(snackBarMessage);
    notifyListeners();
  }
}
