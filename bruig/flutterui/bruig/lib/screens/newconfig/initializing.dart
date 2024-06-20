import 'package:bruig/components/text.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';

class InitializingNewConfPage extends StatefulWidget {
  final NewConfigModel newconf;
  const InitializingNewConfPage(this.newconf, {super.key});

  @override
  State<InitializingNewConfPage> createState() =>
      _InitializingNewConfPageState();
}

class _InitializingNewConfPageState extends State<InitializingNewConfPage> {
  void checkWallet() async {
    if (await widget.newconf.hasOldVersionWindowsWalletDB()) {
      // Has old windows version wallet that needs to be moved.
      if (mounted && context.mounted) {
        Navigator.of(context)
            .pushReplacementNamed("/newconf/moveOldWindowsWallet");
      }
    } else if (await widget.newconf.hasLNWalletDB()) {
      // No config, but LN wallet db exists. Decide what to do.
      if (mounted && context.mounted) {
        Navigator.of(context).pushReplacementNamed("/newconf/deleteOldWallet");
      }
    } else {
      if (mounted && context.mounted) {
        Navigator.of(context)
            .pushReplacementNamed("/newconf/lnChoice/internal");
      }
    }
  }

  @override
  void initState() {
    super.initState();
    checkWallet();
  }

  @override
  Widget build(BuildContext context) {
    return const StartupScreen([
      Txt.H("Setting up Bison Relay"),
    ]);
  }
}
