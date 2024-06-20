import 'package:flutter/foundation.dart';
import 'dart:collection';

import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

class SnackBarMessage {
  final String msg;
  final bool error;
  final DateTime timestamp;

  SnackBarMessage(this.msg, this.error, this.timestamp);
  factory SnackBarMessage.empty() => SnackBarMessage("", false, DateTime.now());
}

class SnackBarModel extends ChangeNotifier {
  // Return the closest SnackBarModel in the stack. Note that this does NOT
  // listen by default, because this is usually called on what will soon become
  // an async call.
  static SnackBarModel of(BuildContext context, {bool listen = false}) =>
      Provider.of<SnackBarModel>(context, listen: listen);

  final List<SnackBarMessage> _snackBars = [];
  UnmodifiableListView<SnackBarMessage> get snackBars =>
      UnmodifiableListView(_snackBars);
  void success(String snackBarMessage) {
    _snackBars.add(SnackBarMessage(snackBarMessage, false, DateTime.now()));
    notifyListeners();
  }

  void error(String snackBarMessage) {
    _snackBars.add(SnackBarMessage(snackBarMessage, true, DateTime.now()));
    notifyListeners();
  }
}
