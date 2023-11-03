import 'package:bruig/components/recent_log.dart';
import 'package:bruig/models/log.dart';
import 'package:flutter/material.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

class LogScreenTitle extends StatelessWidget {
  const LogScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Text("Bison Relay / Logs",
            style: TextStyle(
                fontSize: theme.getLargeFont(),
                color: Theme.of(context).focusColor)));
  }
}

class LogScreen extends StatelessWidget {
  static const routeName = '/log';
  final LogModel log;
  const LogScreen(this.log, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var backgroundColor = theme.backgroundColor;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Container(
            margin: const EdgeInsets.all(1),
            decoration: BoxDecoration(
              color: backgroundColor,
              borderRadius: BorderRadius.circular(3),
            ),
            padding: const EdgeInsets.all(16),
            child: Column(children: [
              const SizedBox(height: 20),
              Text("Recent Dcrlnd Log",
                  style: TextStyle(
                      color: textColor, fontSize: theme.getLargeFont())),
              const SizedBox(height: 20),
              Expanded(child: LogLines(log)),
              const SizedBox(height: 20),
            ])));
  }
}
