import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:bruig/components/buttons.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

class LNChoicePage extends StatelessWidget {
  final NewConfigModel newconf;
  const LNChoicePage(this.newconf, {Key? key}) : super(key: key);

  void setChoice(BuildContext context, LNNodeType type) {
    newconf.nodeType = type;
    switch (type) {
      case LNNodeType.internal:
        Navigator.of(context).pushNamed("/newconf/lnChoice/internal");
        break;
      case LNNodeType.external:
        Navigator.of(context).pushNamed("/newconf/lnChoice/external");
        break;
    }
  }

  void goBack(BuildContext context) {
    Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => StartupScreen(Column(children: [
              Text("Setting up Bison Relay",
                  style: TextStyle(
                      color: theme.getTheme().dividerColor,
                      fontSize: theme.getHugeFont(context),
                      fontWeight: FontWeight.w200)),
              const SizedBox(height: 20),
              Text("Choose Network Mode",
                  style: TextStyle(
                      color: theme.getTheme().focusColor,
                      fontSize: theme.getLargeFont(context),
                      fontWeight: FontWeight.w300)),
              const SizedBox(height: 34),
              Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                LoadingScreenButton(
                    onPressed: () => setChoice(context, LNNodeType.internal),
                    text: "Internal",
                    empty: true),
                const SizedBox(width: 13),
                LoadingScreenButton(
                    onPressed: () => setChoice(context, LNNodeType.external),
                    text: "External",
                    empty: true),
              ]),
              const Expanded(child: Empty()),
              Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                TextButton(
                  onPressed: () => goBack(context),
                  child: Text("Go Back",
                      style: TextStyle(color: theme.getTheme().dividerColor)),
                )
              ])
            ])));
  }
}
