import 'package:bruig/models/newconfig.dart';
import 'package:flutter/material.dart';
import 'package:bruig/components/buttons.dart';

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
    var backgroundColor = const Color(0xFF19172C);
    var cardColor = const Color(0xFF05031A);
    var textColor = const Color(0xFF8E8D98);
    var secondaryTextColor = const Color(0xFFE4E3E6);

    void goToAbout() {
      Navigator.of(context).pushNamed("/about");
    }

    return Container(
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
                //const SizedBox(height: 208),
                Text("Setting up Bison Relay",
                    style: TextStyle(
                        color: textColor,
                        fontSize: 34,
                        fontWeight: FontWeight.w200)),
                const SizedBox(height: 20),
                Text("Choose Network",
                    style: TextStyle(
                        color: secondaryTextColor,
                        fontSize: 21,
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
                          onPressed: () =>
                              setChoice(context, NetworkType.simnet),
                          text: "Simnet",
                          empty: true),
                    ])
              ])),
          Row(mainAxisAlignment: MainAxisAlignment.center, children: [
            TextButton(
              onPressed: () => goBack(context),
              child: Text("Go Back", style: TextStyle(color: textColor)),
            )
          ])
        ]));
  }
}
