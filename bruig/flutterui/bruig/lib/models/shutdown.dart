import 'dart:async';
import 'dart:io';

import 'package:bruig/util.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter/services.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:restart_app/restart_app.dart';
import 'package:window_manager/window_manager.dart';

late final ShutdownModel globalShutdownModel;

// forceQuitApp bypasses the standard shutdown procedure.
void forceQuitApp() {
  if (Platform.isAndroid || Platform.isIOS) {
    SystemNavigator.pop();
  } else {
    windowManager.destroy();
  }
}

void initGlobalShutdownModel() {
  globalShutdownModel = ShutdownModel();
  try {
    globalShutdownModel._handleNotifications();
  } catch (exception) {
    throw "Unable to listen to shutdown notifications: $exception";
  }
}

class ShutdownModel extends ChangeNotifier {
  bool _dcrlndStopped = false;
  bool get dcrlndStopped => _dcrlndStopped;

  bool _clientStopped = false;
  bool get clientStopped => _clientStopped;

  String? _clientStopErr;
  String? get clientStopErr => _clientStopErr;

  bool _shutdownStarted = false;

  bool _restart = false;

  Future<void> startShutdown({restart = false}) async {
    if (_shutdownStarted) {
      return;
    }

    _restart = restart;
    if (Platform.isAndroid) {
      Golib.setNtfnsEnabled(false);
      Golib.stopForegroundSvc();
    }

    _shutdownStarted = true;

    try {
      await Golib.stopClient();
    } catch (exception) {
      if (!exception.toString().contains("unknown client handle")) {
        debugPrint("Error while stopping client: $exception");
      }

      _clientStopped = true;
      _stopDcrlnd();
    }
  }

  void _stopDcrlnd() async {
    try {
      await Golib.lnStopDcrlnd();
    } catch (exception) {
      if (!exception.toString().contains("dcrlnd not running")) {
        debugPrint("Unable to stop dcrlnd: $exception");
      }

      _dcrlndStopped = true;
      notifyListeners();
      _quitApp();
    }
  }

  void _quitApp() async {
    // When shutdown was not explicitly requested via startShutdown(), expect
    // the app gets sent into the /fatalError page (to give the user a chance
    // to see any error messages) and from there forceQuit() is called.
    if (!_shutdownStarted) {
      return;
    }

    // Give a chance for any final updates to the screen before closing app window.
    await sleep(const Duration(seconds: 1));

    // Actually close or restart app.
    if (Platform.isAndroid || Platform.isIOS) {
      if (_restart) {
        Restart.restartApp();
      } else {
        SystemNavigator.pop();
      }
    } else {
      // TODO: support restart.
      windowManager.destroy();
    }
  }

  void _handleNotifications() async {
    var stream = Golib.shutdownEvents();
    await for (var event in stream) {
      switch (event.type) {
        case NTLNDcrlndStopped:
          _dcrlndStopped = true;
          notifyListeners();
          _quitApp();
          break;

        case NTClientStopped:
          _clientStopped = true;
          if (event.payload != null) {
            _clientStopErr = "${event.payload}";
          }
          notifyListeners();

          _stopDcrlnd();
          break;
      }
    }
  }
}
