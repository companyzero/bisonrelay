import 'dart:async';

import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/screens/config_network.dart';
import 'package:bruig/screens/newconfig/delete_old_wallet.dart';
import 'package:bruig/screens/newconfig/initializing.dart';
import 'package:bruig/screens/newconfig/ln_choice.dart';
import 'package:bruig/screens/newconfig/ln_choice_external.dart';
import 'package:bruig/screens/newconfig/ln_choice_internal.dart';
import 'package:bruig/screens/newconfig/ln_wallet_seed.dart';
import 'package:bruig/screens/newconfig/ln_wallet_seed_confirm.dart';
import 'package:bruig/screens/newconfig/move_old_version.dart';
import 'package:bruig/screens/newconfig/network_choice.dart';
import 'package:bruig/screens/newconfig/restore_wallet.dart';
import 'package:bruig/screens/newconfig/server.dart';
import 'package:bruig/screens/about.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

Future<void> runNewConfigApp(List<String> args) async {
  runApp(MultiProvider(
    providers: [
      ChangeNotifierProvider(create: (c) => NewConfigModel(args)),
      ChangeNotifierProvider(create: (c) => SnackBarModel()),
      ChangeNotifierProvider(create: (c) => ThemeNotifier()),
    ],
    child: NewConfigScreen(args),
  ));
}

class NewConfigScreen extends StatefulWidget {
  final List<String> args;
  const NewConfigScreen(this.args, {Key? key}) : super(key: key);

  @override
  State<NewConfigScreen> createState() => _NewConfigScreenState();
}

class _NewConfigScreenState extends State<NewConfigScreen> {
  @override
  void initState() {
    super.initState();
  }

  @override
  Widget build(BuildContext context) {
    return Consumer2<NewConfigModel, SnackBarModel>(
        builder: (context, newconf, snackBar, child) {
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
          "/newconf/moveOldWindowsWallet": (context) =>
              MoveOldVersionWalletPage(newconf),
          "/newconf/deleteOldWallet": (context) => DeleteOldWalletPage(newconf),
          "/newconf/networkChoice": (context) => NetworkChoicePage(newconf),
          "/newconf/lnChoice": (context) => LNChoicePage(newconf),
          "/newconf/lnChoice/internal": (context) =>
              LNInternalWalletPage(newconf),
          "/newconf/lnChoice/external": (context) =>
              LNExternalWalletPage(newconf),
          "/newconf/server": (context) => ServerPage(newconf),
          "/newconf/seed": (context) => NewLNWalletSeedPage(newconf),
          "/newconf/restore": (context) => RestoreWalletPage(newconf),
          ConfigNetworkScreen.routeName: (context) => ConfigNetworkScreen(
                newConf: newconf,
              ),
        },
        builder: (BuildContext context, Widget? child) => Scaffold(
          body: SnackbarDisplayer(snackBar, Center(child: child)),
        ),
      );
    });
  }
}
