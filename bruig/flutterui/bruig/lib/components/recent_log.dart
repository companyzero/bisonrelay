import 'dart:math';

import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/models/log.dart';
import 'package:flutter/material.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

class LogLines extends StatefulWidget {
  final LogModel log;
  final int maxLines;
  final Color? optionalTextColor;
  const LogLines(this.log,
      {super.key, this.maxLines = -1, this.optionalTextColor});

  @override
  State<LogLines> createState() => _LogLinesState();
}

class _LogLinesState extends State<LogLines> {
  LogModel get log => widget.log;
  List<String> logLines = [];
  ScrollController ctrl = ScrollController();
  TextEditingController txtCtrl = TextEditingController();

  void logUpdated() {
    setState(() {
      logLines = log.log.toList(growable: false);
      if (widget.maxLines > -1) {
        logLines = logLines.sublist(max(logLines.length - widget.maxLines, 0));
      }
      txtCtrl.text = logLines.join("\n");
      if (ctrl.hasClients) {
        ctrl.jumpTo(ctrl.position.maxScrollExtent);
      }
    });
  }

  @override
  void initState() {
    super.initState();
    log.addListener(logUpdated);
    logUpdated();

    // Perform initial scroll to bottom.
    (() async {
      while (!ctrl.hasClients) {
        await Future.delayed(const Duration(milliseconds: 10));
      }
      ctrl.jumpTo(ctrl.position.maxScrollExtent);
    })();
  }

  @override
  void didUpdateWidget(LogLines oldWidget) {
    oldWidget.log.removeListener(logUpdated);
    super.didUpdateWidget(oldWidget);
    log.addListener(logUpdated);
  }

  @override
  void dispose() {
    log.removeListener(logUpdated);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (logLines.isEmpty) {
      return const Empty();
    }

    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => TextField(
              scrollController: ctrl,
              controller: txtCtrl,
              maxLines: null,
              keyboardType: TextInputType.multiline,
              readOnly: true,
              style: theme.extraTextStyles.monospaced
                  .merge(const TextStyle(fontSize: 12)),
            ));
  }
}
