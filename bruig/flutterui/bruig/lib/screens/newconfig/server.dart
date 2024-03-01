import 'dart:io';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/buttons.dart';
import 'package:bruig/main.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:bruig/screens/unlock_ln.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

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
        serverCtrl.text = Platform.isAndroid ? "10.0.2.2:12345" : "127.0.0.1:12345";
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
              const SizedBox(height: 39),
              Text("Setting up Bison Relay",
                  style: TextStyle(
                      color: theme.getTheme().dividerColor,
                      fontSize: theme.getHugeFont(context),
                      fontWeight: FontWeight.w200)),
              const SizedBox(height: 20),
              Text("Connect to Server",
                  style: TextStyle(
                      color: theme.getTheme().focusColor,
                      fontSize: theme.getLargeFont(context),
                      fontWeight: FontWeight.w300)),
              const SizedBox(height: 34),
              Row(children: [
                Flexible(
                    child: Align(
                        child: SizedBox(
                            width: 300,
                            child: TextField(
                                cursorColor: theme.getTheme().focusColor,
                                decoration: InputDecoration(
                                    border: InputBorder.none,
                                    hintText: "RPC Host",
                                    hintStyle: TextStyle(
                                        fontSize: theme.getMediumFont(context),
                                        color: theme.getTheme().dividerColor),
                                    filled: true,
                                    fillColor: theme.getTheme().cardColor),
                                style: TextStyle(
                                    color: theme.getTheme().dividerColor,
                                    fontSize: theme.getMediumFont(context)),
                                controller: serverCtrl)))),
              ]),
              const SizedBox(height: 34),
              LoadingScreenButton(
                onPressed: done,
                text: "Connect",
              ),
            ])));
  }
}
