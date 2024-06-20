import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/config_network.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:bruig/util.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

class LNInternalWalletPage extends StatefulWidget {
  final NewConfigModel newconf;
  const LNInternalWalletPage(this.newconf, {Key? key}) : super(key: key);

  @override
  State<LNInternalWalletPage> createState() => _LNInternalWalletPageState();
}

class _LNInternalWalletPageState extends State<LNInternalWalletPage> {
  NewConfigModel get newconf => widget.newconf;
  TextEditingController passCtrl = TextEditingController();
  TextEditingController passRepeatCtrl = TextEditingController();
  bool loading = false;

  Future<void> createWallet() async {
    var snackbar = SnackBarModel.of(context);
    if (passCtrl.text == "") {
      snackbar.error("Password cannot be empty");
      return;
    }
    if (passCtrl.text != passRepeatCtrl.text) {
      snackbar.error("Passwords are different");
      return;
    }
    if (passCtrl.text.length < 8) {
      snackbar.error("Password cannot have less then 8 chars");
      return;
    }
    setState(() {
      loading = true;
    });
    try {
      await widget.newconf
          .createNewWallet(passCtrl.text, newconf.seedToRestore);
      pushNavigatorFromState(this, "/newconf/seed");
    } catch (exception) {
      snackbar.error("Unable to create new LN wallet: $exception");
      if (newconf.seedToRestore.isNotEmpty) {
        // This assumes if there was a previously existing wallet, the user was
        // already given the choice to delete it.
        newconf.deleteLNWalletDir();
        pushNavigatorFromState(this, "/newconf/restore");
      }
    } finally {
      setState(() {
        loading = false;
      });
    }
  }

  void startAdvancedSetup() {
    newconf.advancedSetup = true;
    Navigator.of(context).pushNamed("/newconf/networkChoice");
  }

  void startSeedRestore() {
    Navigator.of(context).pushNamed("/newconf/restore");
  }

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = checkIsScreenSmall(context);

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => StartupScreen(childrenWidth: 400, [
              const Txt.H("Setting up Bison Relay"),
              SizedBox(height: isScreenSmall ? 8 : 20),
              Txt.L(newconf.seedToRestore.isEmpty
                  ? "Creating New Wallet"
                  : "Restoring Wallet"),
              SizedBox(height: isScreenSmall ? 8 : 34),
              const Align(
                  alignment: Alignment.centerLeft,
                  child: Text("Wallet Password")),
              TextField(
                  decoration: InputDecoration(
                    border: InputBorder.none,
                    hintText: "Password",
                    filled: true,
                    fillColor: theme.colors.surface,
                  ),
                  controller: passCtrl,
                  obscureText: true),
              const Align(
                  alignment: Alignment.centerLeft,
                  child: Text("Repeat Password")),
              TextField(
                  decoration: InputDecoration(
                    border: InputBorder.none,
                    hintText: "Confirm",
                    filled: true,
                    fillColor: theme.colors.surface,
                  ),
                  controller: passRepeatCtrl,
                  obscureText: true),
              const SizedBox(height: 10),
              !loading
                  ? LoadingScreenButton(
                      onPressed: !loading ? createWallet : null,
                      text: "Create Wallet",
                    )
                  : const SizedBox(
                      height: 25,
                      width: 25,
                      child: CircularProgressIndicator(
                          value: null, strokeWidth: 2),
                    ),
              const SizedBox(height: 30),
              Wrap(alignment: WrapAlignment.center, runSpacing: 10, children: [
                !newconf.advancedSetup
                    ? TextButton(
                        onPressed: startAdvancedSetup,
                        child: const Txt("Advanced Setup",
                            color: TextColor.onSurfaceVariant))
                    : const Empty(),
                newconf.seedToRestore.isEmpty
                    ? TextButton(
                        onPressed: startSeedRestore,
                        child: const Txt("Restore from Seed",
                            color: TextColor.onSurfaceVariant))
                    : const Empty(),
                TextButton(
                    onPressed: () {
                      Navigator.of(context)
                          .pushNamed(ConfigNetworkScreen.routeName);
                    },
                    child: const Txt("Network Config",
                        color: TextColor.onSurfaceVariant)),
              ])
            ]));
  }
}
