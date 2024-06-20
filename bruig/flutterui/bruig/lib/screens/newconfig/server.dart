import 'dart:io';
import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/main.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/models/snackbar.dart';
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
        serverCtrl.text =
            Platform.isAndroid ? "10.0.2.2:12345" : "127.0.0.1:12345";
        break;
    }
  }

  void done() async {
    var snackbar = SnackBarModel.of(context);
    try {
      widget.newconf.serverAddr = serverCtrl.text;
      var cfg = await widget.newconf.generateConfig();
      if (cfg.walletType == "internal") {
        runChainSyncDcrlnd(cfg);
      } else {
        runMainApp(cfg);
      }
    } catch (exception) {
      snackbar.error("Unable to generate config: $exception");
    }
  }

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => StartupScreen(childrenWidth: 400, [
              const Txt.H("Setting up Bison Relay"),
              const SizedBox(height: 20),
              const Txt.L("Connect to Server"),
              const SizedBox(height: 34),
              TextField(
                  decoration: InputDecoration(
                      border: InputBorder.none,
                      hintText: "Server Address",
                      fillColor: theme.colors.surface,
                      filled: true),
                  controller: serverCtrl),
              const SizedBox(height: 34),
              LoadingScreenButton(
                onPressed: done,
                text: "Connect",
              ),
            ]));
  }
}
