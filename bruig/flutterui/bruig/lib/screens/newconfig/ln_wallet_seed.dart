import 'package:bruig/components/empty_widget.dart';
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
    // showSuccessSnackbar(context, "Copied seed to clipboard!");
  }

  @override
  Widget build(BuildContext context) {
    void done() {
      Navigator.of(context).pushNamed("/newconf/confirmseed");
    }

    var seedWords = newconf.newWalletSeed.split(' ');

    return Consumer<ThemeNotifier>(
      builder: (context, theme, _) => StartupScreen([
        Text("Setting up Bison Relay",
            style: TextStyle(
                color: theme.getTheme().dividerColor,
                fontSize: theme.getHugeFont(context),
                fontWeight: FontWeight.w200)),
        const SizedBox(height: 20),
        Text("Confirm New Wallet Seed",
            style: TextStyle(
                color: theme.getTheme().focusColor,
                fontSize: theme.getLargeFont(context),
                fontWeight: FontWeight.w300)),
        const SizedBox(height: 34),
        Center(
          child: SizedBox(
              width: 519,
              child: Wrap(spacing: 5, runSpacing: 5, children: [
                for (var i in seedWords)
                  i != ""
                      ? Container(
                          padding: const EdgeInsets.only(
                              left: 8, top: 3, right: 8, bottom: 3),
                          color: theme.getTheme().backgroundColor,
                          child: Text(i,
                              style: TextStyle(
                                  color: theme.getTheme().dividerColor,
                                  fontSize: theme.getMediumFont(context),
                                  fontWeight: FontWeight.w300)))
                      : const Empty()
              ])),
        ),
        const SizedBox(height: 10),
        TextButton(
          onPressed: () => copySeedToClipboard(context),
          child: Text("Copy to Clipboard",
              style: TextStyle(color: theme.getTheme().dividerColor)),
        ),
        /*   XXX NEED TO FIGURE OUT LISTVIEW within a row FOR SEED WORD BUBBLES
              Expanded(
                  child: ListView.builder(
                shrinkWrap: true,
                itemCount: seedWords.length,
                itemBuilder: (context, index) => Container(
                    margin: EdgeInsets.all(5),
                    padding:
                        EdgeInsets.only(left: 8, top: 3, right: 8, bottom: 3),
                    color: theme.getTheme().backgroundColor,
                    child: Text(seedWords[index],
                        style: TextStyle(
                            color: theme.getTheme().dividerColor,
                            fontSize: theme.getMediumFont(context),
                            fontWeight: FontWeight.w300))),
              )),
              */
        const SizedBox(height: 34),
        LoadingScreenButton(
          onPressed: done,
          text: "I have copied the seed",
        ),
        const Expanded(child: Empty()),
      ]),
    );
  }
}
