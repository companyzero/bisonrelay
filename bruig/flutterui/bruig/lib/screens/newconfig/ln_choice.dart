import 'package:bruig/components/text.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:bruig/components/buttons.dart';

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
    return StartupScreen([
      const Txt.H("Setting up Bison Relay"),
      const SizedBox(height: 20),
      const Txt.L("Choose Network Mode"),
      const SizedBox(height: 34),
      Wrap(spacing: 10, runSpacing: 10, children: [
        LoadingScreenButton(
            onPressed: () => setChoice(context, LNNodeType.internal),
            text: "Internal",
            empty: true),
        LoadingScreenButton(
            onPressed: () => setChoice(context, LNNodeType.external),
            text: "External",
            empty: true),
      ]),
      const SizedBox(height: 30),
      TextButton(
          onPressed: () => goBack(context), child: const Text("Go Back")),
    ]);
  }
}
