import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/models/newconfig.dart';
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

    void goToAbout() {
      Navigator.of(context).pushNamed("/about");
    }

    var backgroundColor = const Color(0xFF19172C);
    var cardColor = const Color(0xFF05031A);
    var textColor = const Color(0xFF8E8D98);
    var secondaryTextColor = const Color(0xFFE4E3E6);
    var seedWords = newconf.newWalletSeed.split(' ');

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Container(
            color: backgroundColor,
            child: Stack(children: [
              Container(
                  decoration: const BoxDecoration(
                      image: DecorationImage(
                          fit: BoxFit.fill,
                          image: AssetImage("assets/images/loading-bg.png")))),
              Container(
                decoration: BoxDecoration(
                    gradient: LinearGradient(
                        begin: Alignment.bottomLeft,
                        end: Alignment.topRight,
                        colors: [
                      cardColor,
                      const Color(0xFF07051C),
                      backgroundColor.withOpacity(0.34),
                    ],
                        stops: const [
                      0,
                      0.17,
                      1
                    ])),
                padding: const EdgeInsets.all(10),
                child: Column(children: [
                  Row(children: [
                    IconButton(
                        alignment: Alignment.topLeft,
                        tooltip: "About Bison Relay",
                        iconSize: 50,
                        onPressed: goToAbout,
                        icon: Image.asset(
                          "assets/images/icon.png",
                        )),
                  ]),
                  const SizedBox(height: 39),
                  Text("Setting up Bison Relay",
                      style: TextStyle(
                          color: textColor,
                          fontSize: theme.getHugeFont(),
                          fontWeight: FontWeight.w200)),
                  const SizedBox(height: 20),
                  Text("Confirm New Wallet Seed",
                      style: TextStyle(
                          color: secondaryTextColor,
                          fontSize: theme.getLargeFont(),
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
                                    color: backgroundColor,
                                    child: Text(i,
                                        style: TextStyle(
                                            color: textColor,
                                            fontSize: theme.getMediumFont(),
                                            fontWeight: FontWeight.w300)))
                                : const Empty()
                        ])),
                  ),
                  const SizedBox(height: 10),
                  TextButton(
                    onPressed: () => copySeedToClipboard(context),
                    child: Text("Copy to Clipboard",
                        style: TextStyle(color: textColor)),
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
                    color: backgroundColor,
                    child: Text(seedWords[index],
                        style: TextStyle(
                            color: textColor,
                            fontSize: theme.getMediumFont(),
                            fontWeight: FontWeight.w300))),
              )),
              */
                  const SizedBox(height: 34),
                  LoadingScreenButton(
                    onPressed: done,
                    text: "I have copied the seed",
                  ),
                ]),
              )
            ])));
  }
}
