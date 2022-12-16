import 'package:bruig/models/newconfig.dart';
import 'package:flutter/cupertino.dart';

class InitializingNewConfPage extends StatefulWidget {
  final NewConfigModel newconf;
  const InitializingNewConfPage(this.newconf, {super.key});

  @override
  State<InitializingNewConfPage> createState() =>
      _InitializingNewConfPageState();
}

class _InitializingNewConfPageState extends State<InitializingNewConfPage> {
  void checkWallet() async {
    if (await widget.newconf.hasLNWalletDB()) {
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
    var backgroundColor = const Color(0xFF19172C);
    var cardColor = const Color(0xFF05031A);
    var textColor = const Color(0xFF8E8D98);
    var secondaryTextColor = const Color(0xFFE4E3E6);
    var darkTextColor = const Color(0xFF5A5968);
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
                const SizedBox(height: 89),
                Text("Setting up Bison Relay",
                    style: TextStyle(
                        color: textColor,
                        fontSize: 34,
                        fontWeight: FontWeight.w200)),
                const SizedBox(height: 20),
              ]))
        ]));
  }
}
