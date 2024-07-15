import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:bruig/components/buttons.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

class NewLNWalletSeedPage extends StatelessWidget {
  final NewConfigModel newconf;
  const NewLNWalletSeedPage(this.newconf, {Key? key}) : super(key: key);

  void copySeedToClipboard(BuildContext context) async {
    Clipboard.setData(ClipboardData(text: newconf.newWalletSeed));
    showSuccessSnackbar(context, "Copied seed to clipboard!");
  }

  @override
  Widget build(BuildContext context) {
    void done() {
      Navigator.of(context).pushNamed("/newconf/confirmseed");
    }

    var seedWords = newconf.newWalletSeed.split(' ');
    seedWords.removeWhere((w) => w == "");

    return Consumer<ThemeNotifier>(
      builder: (context, theme, _) => StartupScreen(childrenWidth: 500, [
        const Txt.H("Setting up Bison Relay"),
        const SizedBox(height: 20),
        const Txt.L("Confirm New Wallet Seed"),
        const SizedBox(height: 34),
        Wrap(
            children: seedWords
                .map((w) => Container(
                    padding:
                        const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                    color: theme.colors.surface,
                    child: Text(w)))
                .toList()),
        const SizedBox(height: 10),
        TextButton(
          onPressed: () => copySeedToClipboard(context),
          child: const Text("Copy to Clipboard"),
        ),
        const SizedBox(height: 34),
        LoadingScreenButton(onPressed: done, text: "I have copied the seed"),
      ]),
    );
  }
}
