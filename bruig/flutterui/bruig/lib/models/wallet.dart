import 'dart:collection';

import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';

class RescanState extends ChangeNotifier {
  bool _rescanning = false;
  bool get rescanning => _rescanning;

  int _targetHeight = -1;
  int get targetHeight => _targetHeight;

  int _progressHeight = 0;
  int get progressHeight => _progressHeight;

  double get progress =>
      _targetHeight > 0 ? _progressHeight / _targetHeight : 0;

  void _setProgress(int p) {
    _progressHeight = p;
    notifyListeners();
  }

  void _startRescan(int tipHeight) {
    _rescanning = true;
    _progressHeight = 0;
    _targetHeight = tipHeight;
    notifyListeners();
  }

  void _stopRescan() {
    _rescanning = false;
    notifyListeners();
  }
}

class TransactionsState extends ChangeNotifier {
  List<Transaction> _transactions = [];
  UnmodifiableListView<Transaction> get transactions =>
      UnmodifiableListView(_transactions);

  bool get isEmpty => _transactions.isEmpty;

  bool _listing = false;
  bool get listing => _listing;

  Future<void> list() async {
    _listing = true;
    notifyListeners();

    try {
      var res = await Golib.listTransactions(0, 0);
      _transactions = res;
      _listing = false;
      notifyListeners();
    } catch (exception) {
      _listing = false;
      rethrow;
    }
  }
}

class WalletModel extends ChangeNotifier {
  WalletModel() {
    _handleRescanWalletProgress();
  }

  final RescanState rescanState = RescanState();

  void _handleRescanWalletProgress() async {
    var stream = Golib.rescanWalletProgress();
    await for (var h in stream) {
      rescanState._setProgress(h);
    }
  }

  Future<void> rescan() async {
    int tipHeight = (await Golib.lnGetInfo()).blockHeight;
    try {
      rescanState._startRescan(tipHeight);
      await Golib.rescanWallet(0);
      rescanState._stopRescan();
    } catch (exception) {
      rescanState._stopRescan();
      rethrow;
    }
  }

  final TransactionsState transactions = TransactionsState();
}
