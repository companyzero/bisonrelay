import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/buttons.dart';
import 'package:bruig/main.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/screens/unlock_ln.dart';
import 'package:flutter/material.dart';

class ServerPage extends StatefulWidget {
  final NewConfigModel newconf;
  const ServerPage(this.newconf, {Key? key}) : super(key: key);

  @override
  State<ServerPage> createState() => _ServerPageState();
}

class _ServerPageState extends State<ServerPage> {
  TextEditingController serverCtrl = TextEditingController();

  @override
  void initState() {
    super.initState();

    switch (widget.newconf.netType) {
      case NetworkType.mainnet:
        serverCtrl.text = "br00.bisonrelay.org:12345";
        break;
      case NetworkType.testnet:
        serverCtrl.text = "216.128.136.239:65432";
        break;
      case NetworkType.simnet:
        serverCtrl.text = "127.0.0.1:12345";
        break;
    }
  }

  void done() async {
    try {
      widget.newconf.serverAddr = serverCtrl.text;
      var cfg = await widget.newconf.generateConfig();
      if (cfg.walletType == "internal") {
        runChainSyncDcrlnd(cfg);
      } else {
        runMainApp(cfg);
      }
    } catch (exception) {
      showErrorSnackbar(context, "Unable to generate config: $exception");
    }
  }

  @override
  Widget build(BuildContext context) {
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
                const SizedBox(height: 39),
                Text("Setting up Bison Relay",
                    style: TextStyle(
                        color: textColor,
                        fontSize: 34,
                        fontWeight: FontWeight.w200)),
                const SizedBox(height: 20),
                Text("Connect to Server",
                    style: TextStyle(
                        color: secondaryTextColor,
                        fontSize: 21,
                        fontWeight: FontWeight.w300)),
                const SizedBox(height: 34),
                Row(children: [
                  Flexible(
                      child: Align(
                          child: SizedBox(
                              width: 300,
                              child: TextField(
                                  cursorColor: secondaryTextColor,
                                  decoration: InputDecoration(
                                      border: InputBorder.none,
                                      hintText: "RPC Host",
                                      hintStyle: TextStyle(
                                          fontSize: 13, color: textColor),
                                      filled: true,
                                      fillColor: cardColor),
                                  style:
                                      TextStyle(color: textColor, fontSize: 13),
                                  controller: serverCtrl)))),
                ]),
                const SizedBox(height: 34),
                LoadingScreenButton(
                  onPressed: done,
                  text: "Connect",
                ),
              ]))
        ]));
  }
}
