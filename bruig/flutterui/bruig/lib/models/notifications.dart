import 'package:flutter/cupertino.dart';

enum AppNtfnType {
  walletNeedsFunds,
  walletNeedsChannels,
  walletNeedsInChannels,
  error,
  walletCheckFailed,
  invoiceGenFailed,
  serverUnwelcomeError,
}

class AppNtfn {
  final AppNtfnType type;
  final String? msg;

  AppNtfn(this.type, {this.msg});
}

class AppNotifications extends ChangeNotifier {
  final List<AppNtfn> _ntfns = [];
  Iterable<AppNtfn> get ntfns => _ntfns.toList(growable: false);
  int get count => _ntfns.length;

  addNtfn(AppNtfn ntf) {
    _ntfns.add(ntf);
    notifyListeners();
  }

  delNtfn(AppNtfn ntf) {
    _ntfns.remove(ntf);
    notifyListeners();
  }

  delType(AppNtfnType type) {
    _ntfns.removeWhere((v) => v.type == type);
    notifyListeners();
  }
}
