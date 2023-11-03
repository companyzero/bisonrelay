import 'dart:async';

import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/recent_log.dart';
import 'package:bruig/models/log.dart';
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
    var backgroundColor = const Color(0xFF19172C);
    var cardColor = const Color(0xFF05031A);
    var textColor = const Color(0xFF8E8D98);
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Scaffold(
            body: Container(
                color: backgroundColor,
                child: Stack(children: [
                  Container(
                      decoration: const BoxDecoration(
                          image: DecorationImage(
                              fit: BoxFit.fill,
                              image:
                                  AssetImage("assets/images/loading-bg.png")))),
                  Container(
                    decoration: BoxDecoration(
                        gradient: LinearGradient(
                            begin: Alignment.bottomLeft,
                            end: Alignment.topRight,
                            colors: [
                          cardColor,
                          const Color(0xFF07051C),
                          backgroundColor.withOpacity(0.34),
                        ],
                            stops: const [
                          0,
                          0.17,
                          1
                        ])),
                  ),
                  Container(
                      padding: const EdgeInsets.all(10),
                      child: Column(children: [
                        const SizedBox(height: 89),
                        Text("Shutting Down Bison Relay",
                            style: TextStyle(
                                color: textColor,
                                fontSize: theme.getHugeFont(),
                                fontWeight: FontWeight.w200)),
                        clientStopErr != null
                            ? Text(clientStopErr!)
                            : const Empty(),
                        const SizedBox(height: 20),
                        const Divider(),
                        const SizedBox(height: 20),
                        Expanded(child: LogLines(widget.log))
                      ]))
                ]))));
  }
}
