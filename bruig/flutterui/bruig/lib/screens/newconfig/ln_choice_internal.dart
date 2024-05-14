import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/buttons.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/screens/config_network.dart';
import 'package:bruig/screens/startupscreen.dart';
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
    if (passCtrl.text == "") {
      showErrorSnackbar(context, "Password cannot be empty");
      return;
    }
    if (passCtrl.text != passRepeatCtrl.text) {
      showErrorSnackbar(context, "Passwords are different");
      return;
    }
    if (passCtrl.text.length < 8) {
      showErrorSnackbar(context, "Password cannot have less then 8 chars");
      return;
    }
    setState(() {
      loading = true;
    });
    try {
      await widget.newconf
          .createNewWallet(passCtrl.text, newconf.seedToRestore);
      Navigator.of(context).pushNamed("/newconf/seed");
    } catch (exception) {
      showErrorSnackbar(context, "Unable to create new LN wallet: $exception");
      if (newconf.seedToRestore.isNotEmpty) {
        // This assumes if there was a previously existing wallet, the user was
        // already given the choice to delete it.
        newconf.deleteLNWalletDir();
        Navigator.of(context).pushNamed("/newconf/restore");
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
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => StartupScreen([
              Text("Setting up Bison Relay",
                  style: TextStyle(
                      color: theme.getTheme().dividerColor,
                      fontSize: theme.getHugeFont(context),
                      fontWeight: FontWeight.w200)),
              SizedBox(height: isScreenSmall ? 8 : 20),
              Text(
                  newconf.seedToRestore.isEmpty
                      ? "Creating New Wallet"
                      : "Restoring Wallet",
                  style: TextStyle(
                      color: theme.getTheme().focusColor,
                      fontSize: theme.getLargeFont(context),
                      fontWeight: FontWeight.w300)),
              SizedBox(height: isScreenSmall ? 8 : 34),
              Column(children: [
                SizedBox(
                    width: 377,
                    child: Text("Wallet Password",
                        textAlign: TextAlign.left,
                        style: TextStyle(
                            color: theme.getTheme().indicatorColor,
                            fontSize: theme.getMediumFont(context),
                            fontWeight: FontWeight.w300))),
                Center(
                    child: SizedBox(
                        width: 377,
                        child: TextField(
                            cursorColor: theme.getTheme().focusColor,
                            decoration: InputDecoration(
                                border: InputBorder.none,
                                hintText: "Password",
                                hintStyle: TextStyle(
                                    fontSize: theme.getLargeFont(context),
                                    color: theme.getTheme().dividerColor),
                                filled: true,
                                fillColor: theme.getTheme().cardColor),
                            style: TextStyle(
                                color: theme.getTheme().focusColor,
                                fontSize: theme.getLargeFont(context)),
                            controller: passCtrl,
                            obscureText: true))),
                const SizedBox(height: 13),
                SizedBox(
                    width: 377,
                    child: Text("Repeat Password",
                        textAlign: TextAlign.left,
                        style: TextStyle(
                            color: theme.getTheme().indicatorColor,
                            fontSize: theme.getMediumFont(context),
                            fontWeight: FontWeight.w300))),
                Center(
                  child: SizedBox(
                      width: 377,
                      child: TextField(
                          cursorColor: theme.getTheme().focusColor,
                          decoration: InputDecoration(
                              border: InputBorder.none,
                              hintText: "Confirm",
                              hintStyle: TextStyle(
                                  fontSize: theme.getLargeFont(context),
                                  color: theme.getTheme().dividerColor),
                              filled: true,
                              fillColor: theme.getTheme().cardColor),
                          //decoration: InputDecoration(),
                          style: TextStyle(
                              color: theme.getTheme().focusColor,
                              fontSize: theme.getLargeFont(context)),
                          controller: passRepeatCtrl,
                          obscureText: true)),
                ),
                SizedBox(height: isScreenSmall ? 12 : 35),
                Center(
                    child: SizedBox(
                        width: 278,
                        child: Row(children: [
                          const SizedBox(width: 35),
                          LoadingScreenButton(
                            onPressed: !loading ? createWallet : null,
                            text: "Create Wallet",
                          ),
                          const SizedBox(width: 10),
                          loading
                              ? SizedBox(
                                  height: 25,
                                  width: 25,
                                  child: CircularProgressIndicator(
                                      value: null,
                                      backgroundColor:
                                          theme.getTheme().backgroundColor,
                                      color: theme.getTheme().dividerColor,
                                      strokeWidth: 2),
                                )
                              : const SizedBox(width: 25),
                        ]))),
              ]),
              const SizedBox(height: 10),
              Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                !newconf.advancedSetup
                    ? TextButton(
                        onPressed: startAdvancedSetup,
                        child: Text("Advanced Setup",
                            style: TextStyle(
                                color: theme.getTheme().dividerColor)),
                      )
                    : const Empty(),
                newconf.seedToRestore.isEmpty
                    ? TextButton(
                        onPressed: startSeedRestore,
                        child: Text("Restore from Seed",
                            style: TextStyle(
                                color: theme.getTheme().dividerColor)),
                      )
                    : const Empty(),
                TextButton(
                    onPressed: () {
                      Navigator.of(context)
                          .pushNamed(ConfigNetworkScreen.routeName);
                    },
                    child: Text("Network Config",
                        style: TextStyle(color: theme.getTheme().dividerColor)))
              ])
            ]));
  }
}
