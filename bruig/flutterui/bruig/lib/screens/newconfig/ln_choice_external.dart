import 'dart:io';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/util.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';
import 'package:bruig/screens/startupscreen.dart';

class LNExternalWalletPage extends StatefulWidget {
  final NewConfigModel newconf;
  const LNExternalWalletPage(this.newconf, {Key? key}) : super(key: key);

  @override
  State<LNExternalWalletPage> createState() => _LNExternalWalletPageState();
}

class _LNExternalWalletPageState extends State<LNExternalWalletPage> {
  final TextEditingController hostCtrl = TextEditingController();
  final TextEditingController tlsCtrl = TextEditingController();
  final TextEditingController macaroonCtrl = TextEditingController();
  bool loading = false;

  void done() async {
    var snackbar = SnackBarModel.of(context);
    var host = hostCtrl.text.trim();
    var tls = tlsCtrl.text.trim();
    var macaroon = macaroonCtrl.text.trim();
    if (host == "") {
      snackbar.error("Host cannot be empty");
      return;
    }
    if (!File(tls).existsSync()) {
      snackbar.error("TLS path $tls does not exist");
      return;
    }
    if (!File(macaroon).existsSync()) {
      snackbar.error("Macaroon path $macaroon does not exist");
      return;
    }
    setState(() {
      loading = true;
    });
    try {
      var res = await widget.newconf.tryExternalDcrlnd(host, tls, macaroon);
      if (res.chains.length != 1) {
        snackbar.error("Wrong number of chains ($res.chains.length)");
        return;
      }
      String wantNetwork = "";
      switch (widget.newconf.netType) {
        case NetworkType.mainnet:
          wantNetwork = "mainnet";
          break;
        case NetworkType.testnet:
          wantNetwork = "testnet";
          break;
        case NetworkType.simnet:
          wantNetwork = "simnet";
          break;
      }
      if (res.chains[0].network != wantNetwork) {
        snackbar.error(
            "LN running in the wrong network (${res.chains[0].network} vs $wantNetwork)");
        return;
      }
      pushNavigatorFromState(this, "/newconf/server");
    } catch (exception) {
      snackbar.error("Unable to connect to external dcrlnd: $exception");
    } finally {
      setState(() {
        loading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => StartupScreen(childrenWidth: 400, [
              const Txt.H("Setting up Bison Relay"),
              const SizedBox(height: 20),
              const Txt.L("External dcrlnd Config"),
              const SizedBox(height: 34),
              const Align(
                  alignment: Alignment.centerLeft, child: Text("RPC Host")),
              const SizedBox(height: 5),
              TextField(
                  decoration: InputDecoration(
                      border: InputBorder.none,
                      hintText: "RPC Host",
                      filled: true,
                      fillColor: theme.colors.surface),
                  controller: hostCtrl),
              const Align(
                  alignment: Alignment.centerLeft,
                  child: Text("TLS Cert Path")),
              TextField(
                  decoration: InputDecoration(
                      border: InputBorder.none,
                      hintText: "TLS Cert Path",
                      filled: true,
                      fillColor: theme.colors.surface),
                  controller: tlsCtrl),
              const Align(
                  alignment: Alignment.centerLeft,
                  child: Text("Macaroon Path")),
              TextField(
                  decoration: InputDecoration(
                      border: InputBorder.none,
                      hintText: "Macaroon Path",
                      filled: true,
                      fillColor: theme.colors.surface),
                  controller: macaroonCtrl),
              const SizedBox(height: 20),
              !loading
                  ? LoadingScreenButton(
                      onPressed: !loading ? done : null,
                      text: "Connect Wallet",
                    )
                  : const CircularProgressIndicator(
                      value: null, strokeWidth: 2),
            ]));
  }
}
