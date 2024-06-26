import 'dart:async';
import 'dart:io';

import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/recent_log.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/log.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:window_manager/window_manager.dart';

void quitApp() {
  if (Platform.isAndroid || Platform.isIOS) {
    SystemChannels.platform.invokeMethod('SystemNavigator.pop');
  } else {
    windowManager.destroy();
  }
}

class ShutdownScreen extends StatefulWidget {
  final bool internalDcrlnd;
  final Stream<ConfNotification> ntfs;
  final LogModel log;
  const ShutdownScreen(this.internalDcrlnd, this.ntfs, this.log, {Key? key})
      : super(key: key);

  @override
  State<ShutdownScreen> createState() => _ShutdownScreenState();
}

class _ShutdownScreenState extends State<ShutdownScreen> {
  String? clientStopErr;

  void handleNotifications() async {
    await for (var ntf in widget.ntfs) {
      switch (ntf.type) {
        case NTClientStopped:
          if (ntf.payload != null) {
            setState(() {
              clientStopErr = "${ntf.payload}";
            });
          }

          if (widget.internalDcrlnd) {
            Golib.lnStopDcrlnd();
          } else {
            quitApp();
          }
          break;

        case NTLNDcrlndStopped:
          Timer(const Duration(seconds: 1), quitApp);
          break;

        default:
          debugPrint("XXXX got ntf ${ntf.type}");
      }
    }
  }

  void initShutdown() async {
    handleNotifications();

    // Ensure we ask for the shutdown only after hooking into the
    // confirmations() stream.
    if (Platform.isAndroid) {
      Golib.setNtfnsEnabled(false);
      Golib.stopForegroundSvc();
    }
    Timer(const Duration(milliseconds: 100), Golib.stopClient);
  }

  @override
  void initState() {
    super.initState();
    initShutdown();
  }

  @override
  Widget build(BuildContext context) {
    return StartupScreen([
      const Txt.H("Shutting Down Bison Relay"),
      clientStopErr != null ? Text(clientStopErr!) : const Empty(),
      const SizedBox(height: 20),
      const Divider(),
      const SizedBox(height: 20),
      LogLines(widget.log)
    ]);
  }
}
