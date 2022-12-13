import 'package:bruig/models/newconfig.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:bruig/components/buttons.dart';

class NetworkChoicePage extends StatelessWidget {
  final NewConfigModel newconf;
  const NetworkChoicePage(this.newconf, {Key? key}) : super(key: key);

  void setChoice(BuildContext context, NetworkType type) {
    newconf.netType = type;
    Navigator.of(context).pushNamed("/newconf/lnChoice");
  }

  @override
  Widget build(BuildContext context) {
    var backgroundColor = const Color(0xFF19172C);
    var cardColor = const Color(0xFF05031A);
    var textColor = const Color(0xFF8E8D98);
    var secondaryTextColor = const Color(0xFFE4E3E6);
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
                const SizedBox(height: 258),
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
                Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                  LoadingScreenButton(
                      onPressed: () => setChoice(context, NetworkType.mainnet),
                      text: "Mainnet",
                      empty: true),
                  const SizedBox(width: 13),
                  LoadingScreenButton(
                      onPressed: () => setChoice(context, NetworkType.testnet),
                      text: "Testnet",
                      empty: true),
                  const SizedBox(width: 13),
                  LoadingScreenButton(
                      onPressed: () => setChoice(context, NetworkType.simnet),
                      text: "Simnet",
                      empty: true),
                ])
              ]))
        ]));
  }
}
