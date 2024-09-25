import 'package:bruig/components/text.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:bruig/components/buttons.dart';

class NetworkChoicePage extends StatelessWidget {
  final NewConfigModel newconf;
  const NetworkChoicePage(this.newconf, {super.key});

  void setChoice(BuildContext context, NetworkType type) {
    newconf.netType = type;
    Navigator.of(context).pushNamed("/newconf/lnChoice");
  }

  void goBack(BuildContext context) {
    newconf.advancedSetup = false;
    Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    return StartupScreen([
      const Txt.H("Setting up Bison Relay"),
      const SizedBox(height: 20),
      const Text("Choose Network"),
      const SizedBox(height: 34),
      Wrap(
          spacing: 10,
          runSpacing: 10,
          alignment: WrapAlignment.center,
          children: [
            LoadingScreenButton(
                onPressed: () => setChoice(context, NetworkType.mainnet),
                text: "Mainnet",
                empty: true),
            LoadingScreenButton(
                onPressed: () => setChoice(context, NetworkType.testnet),
                text: "Testnet",
                empty: true),
            LoadingScreenButton(
                onPressed: () => setChoice(context, NetworkType.simnet),
                text: "Simnet",
                empty: true),
          ]),
      const SizedBox(height: 30),
      TextButton(onPressed: () => goBack(context), child: const Text("Go Back"))
    ]);
  }
}
