import 'package:bruig/models/newconfig.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:bruig/components/buttons.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

class NetworkChoicePage extends StatelessWidget {
  final NewConfigModel newconf;
  const NetworkChoicePage(this.newconf, {Key? key}) : super(key: key);

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
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    void goToAbout() {
      Navigator.of(context).pushNamed("/about");
    }

    return StartupScreen(Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Column(children: [
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
              //const SizedBox(height: 208),
              Text("Setting up Bison Relay",
                  style: TextStyle(
                      color: theme.getTheme().dividerColor,
                      fontSize: theme.getHugeFont(context),
                      fontWeight: FontWeight.w200)),
              const SizedBox(height: 20),
              Text("Choose Network",
                  style: TextStyle(
                      color: theme.getTheme().focusColor,
                      fontSize: theme.getMediumFont(context),
                      fontWeight: FontWeight.w300)),
              const SizedBox(height: 34),
              Flex(
                  direction: isScreenSmall ? Axis.vertical : Axis.horizontal,
                  crossAxisAlignment: CrossAxisAlignment.center,
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    LoadingScreenButton(
                        onPressed: () =>
                            setChoice(context, NetworkType.mainnet),
                        text: "Mainnet",
                        empty: true),
                    isScreenSmall
                        ? const SizedBox(height: 13)
                        : const SizedBox(width: 13),
                    LoadingScreenButton(
                        onPressed: () =>
                            setChoice(context, NetworkType.testnet),
                        text: "Testnet",
                        empty: true),
                    isScreenSmall
                        ? const SizedBox(height: 13)
                        : const SizedBox(width: 13),
                    LoadingScreenButton(
                        onPressed: () => setChoice(context, NetworkType.simnet),
                        text: "Simnet",
                        empty: true),
                  ]),
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
