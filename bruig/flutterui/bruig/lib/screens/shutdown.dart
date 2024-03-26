import 'dart:async';

import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/recent_log.dart';
import 'package:bruig/models/log.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:window_manager/window_manager.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

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
            windowManager.destroy();
          }
          break;

        case NTLNDcrlndStopped:
          Timer(const Duration(seconds: 1), windowManager.destroy);
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
    Timer(const Duration(milliseconds: 100), Golib.stopClient);
  }

  @override
  void initState() {
    super.initState();
    initShutdown();
  }

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => StartupScreen([
              Text("Shutting Down Bison Relay",
                  style: TextStyle(
                      color: theme.getTheme().dividerColor,
                      fontSize: theme.getHugeFont(context),
                      fontWeight: FontWeight.w200)),
              clientStopErr != null ? Text(clientStopErr!) : const Empty(),
              const SizedBox(height: 20),
              const Divider(),
              const SizedBox(height: 20),
              Expanded(child: LogLines(widget.log))
            ]));
  }
}
