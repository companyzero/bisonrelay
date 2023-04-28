import 'dart:async';

import 'package:bruig/models/newconfig.dart';
import 'package:bruig/screens/newconfig/delete_old_wallet.dart';
import 'package:bruig/screens/newconfig/initializing.dart';
import 'package:bruig/screens/newconfig/ln_choice.dart';
import 'package:bruig/screens/newconfig/ln_choice_external.dart';
import 'package:bruig/screens/newconfig/ln_choice_internal.dart';
import 'package:bruig/screens/newconfig/ln_wallet_seed.dart';
import 'package:bruig/screens/newconfig/ln_wallet_seed_confirm.dart';
import 'package:bruig/screens/newconfig/network_choice.dart';
import 'package:bruig/screens/newconfig/restore_wallet.dart';
import 'package:bruig/screens/newconfig/server.dart';
import 'package:bruig/screens/about.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/components/copyable.dart';

Future<void> runNewConfigApp(List<String> args) async {
  runApp(MultiProvider(
    providers: [
      ChangeNotifierProvider(create: (c) => NewConfigModel(args)),
      ChangeNotifierProvider(create: (c) => SnackBarModel()),
    ],
    child: Consumer<SnackBarModel>(
        builder: (context, snackBar, child) => NewConfigScreen(args, snackBar)),
  ));
}

class NewConfigScreen extends StatefulWidget {
  final List<String> args;
  final SnackBarModel snackBar;
  const NewConfigScreen(this.args, this.snackBar, {Key? key}) : super(key: key);

  @override
  State<NewConfigScreen> createState() => _NewConfigScreenState();
}

class _NewConfigScreenState extends State<NewConfigScreen> {
  SnackBarModel get snackBar => widget.snackBar;
  SnackBarMessage snackBarMsg = SnackBarMessage.empty();

  @override
  void initState() {
    super.initState();
    widget.snackBar.addListener(snackBarChanged);
  }

  @override
  void didUpdateWidget(NewConfigScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.snackBar.removeListener(snackBarChanged);
    widget.snackBar.addListener(snackBarChanged);
  }

  @override
  void dispose() {
    widget.snackBar.removeListener(snackBarChanged);
    super.dispose();
  }

  void snackBarChanged() {
    if (snackBar.snackBars.isNotEmpty) {
      var newSnackbarMessage =
          snackBar.snackBars[snackBar.snackBars.length - 1];
      if (newSnackbarMessage.msg != snackBarMsg.msg ||
          newSnackbarMessage.error != snackBarMsg.error ||
          newSnackbarMessage.timestamp != snackBarMsg.timestamp) {
        setState(() {
          snackBarMsg = newSnackbarMessage;
        });
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
            backgroundColor:
                snackBarMsg.error ? Colors.red[300] : Colors.green[300],
            content: Copyable(
                snackBarMsg.msg, const TextStyle(color: Color(0xFFE4E3E6)),
                showSnackbar: false)));
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Consumer<NewConfigModel>(builder: (context, newconf, child) {
      return MaterialApp(
        title: "Setup New Config",
        theme: ThemeData(
          primarySwatch: Colors.green, // XXX THEMEDATA HERE??
        ),
        initialRoute: "/newconf/initializing",
        routes: {
          '/about': (context) => const AboutScreen(),
          "/newconf/initializing": (context) =>
              InitializingNewConfPage(newconf),
          "/newconf/confirmseed": (context) => ConfirmLNWalletSeedPage(newconf),
          "/newconf/deleteOldWallet": (context) =>
              DeleteOldWalletPage(newconf, snackBar),
          "/newconf/networkChoice": (context) => NetworkChoicePage(newconf),
          "/newconf/lnChoice": (context) => LNChoicePage(newconf),
          "/newconf/lnChoice/internal": (context) =>
              LNInternalWalletPage(newconf, snackBar),
          "/newconf/lnChoice/external": (context) =>
              LNExternalWalletPage(newconf, snackBar),
          "/newconf/server": (context) => ServerPage(newconf, snackBar),
          "/newconf/seed": (context) => NewLNWalletSeedPage(newconf),
          "/newconf/restore": (context) => RestoreWalletPage(newconf, snackBar),
        },
        builder: (BuildContext context, Widget? child) => Scaffold(
          body: Center(child: child),
        ),
      );
    });
  }
}
