import 'package:bruig/components/buttons.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

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
      Navigator.of(context)
          .pushReplacementNamed("/newconf/moveOldWindowsWallet");
    } else if (await widget.newconf.hasLNWalletDB()) {
      // No config, but LN wallet db exists. Decide what to do.
      Navigator.of(context).pushReplacementNamed("/newconf/deleteOldWallet");
    } else {
      Navigator.of(context).pushReplacementNamed("/newconf/lnChoice/internal");
    }
  }

  @override
  void initState() {
    super.initState();
    checkWallet();
  }

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => StartupScreen([
              Text("Setting up Bison Relay",
                  style: TextStyle(
                      color: theme.getTheme().dividerColor,
                      fontSize: theme.getHugeFont(context),
                      fontWeight: FontWeight.w200)),
              const SizedBox(height: 20),
            ]));
  }
}
