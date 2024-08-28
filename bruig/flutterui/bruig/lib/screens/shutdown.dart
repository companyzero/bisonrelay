import 'dart:async';

import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/recent_log.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/log.dart';
import 'package:bruig/models/shutdown.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';

class ShutdownScreen extends StatefulWidget {
  static const routeName = "/shutdown";

  static void startShutdown(BuildContext context, {bool restart = false}) {
    Timer(const Duration(milliseconds: 100), () {
      // Remove everything from nav history because user shouldn't navigate back after
      // shutdown starts.
      Navigator.of(context, rootNavigator: true).pushNamedAndRemoveUntil(
        routeName,
        (_) => false,
        arguments: restart,
      );
    });
  }

  static void startShutdownFromNavKey(GlobalKey<NavigatorState> navkey) {
    navkey.currentState!
        .pushNamedAndRemoveUntil(ShutdownScreen.routeName, (_) => false);
  }

  final LogModel log;
  final ShutdownModel shutdown;
  const ShutdownScreen(this.log, this.shutdown, {Key? key}) : super(key: key);

  @override
  State<ShutdownScreen> createState() => _ShutdownScreenState();
}

class _ShutdownScreenState extends State<ShutdownScreen> {
  String? clientStopErr;
  bool clientStopped = false;
  bool dcrlndStopped = false;

  void updated() {
    setState(() {
      clientStopped = widget.shutdown.clientStopped;
      dcrlndStopped = widget.shutdown.dcrlndStopped;
      clientStopErr = widget.shutdown.clientStopErr;
    });
  }

  @override
  void initState() {
    super.initState();
    widget.shutdown.addListener(updated);

    // Start shutdown.
    Timer(const Duration(milliseconds: 100), () {
      bool restart = false;
      if (ModalRoute.of(context)!.settings.arguments != null) {
        restart = ModalRoute.of(context)!.settings.arguments as bool;
      }
      widget.shutdown.startShutdown(restart: restart);
    });
  }

  @override
  void dispose() {
    widget.shutdown.removeListener(updated);
    super.dispose();
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
