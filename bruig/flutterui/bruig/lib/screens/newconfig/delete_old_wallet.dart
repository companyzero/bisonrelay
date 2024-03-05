import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

const _warnMsg = "There is an existing, incomplete install of"
    "the wallet. If this really was an unusued wallet, delete "
    "the wallet below and restart the setup process.\n\n"
    "THIS ACTION CANNOT BE REVERSED";

class DeleteOldWalletPage extends StatefulWidget {
  final NewConfigModel newconf;
  const DeleteOldWalletPage(this.newconf, {super.key});

  @override
  State<DeleteOldWalletPage> createState() => _DeleteOldWalletPageState();
}

class _DeleteOldWalletPageState extends State<DeleteOldWalletPage> {
  NewConfigModel get newconf => widget.newconf;
  bool deleteAccepted = false;
  bool deleting = false;

  void deleteWalletDir(BuildContext context) async {
    setState(() {
      deleting = true;
    });
    try {
      await newconf.deleteLNWalletDir();
    } catch (exception) {
      showErrorSnackbar(context, "Unable to delete wallet dir: $exception");
      return;
    }
    Navigator.of(context).pushReplacementNamed("/newconf/lnChoice/internal");
  }

  @override
  Widget build(BuildContext context) {
    return StartupScreen(Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Column(children: [
              const SetupScreenAbountButton(),
              const SizedBox(height: 39),
              Text("Remove old wallet",
                  style: TextStyle(
                      color: theme.getTheme().dividerColor,
                      fontSize: theme.getHugeFont(context),
                      fontWeight: FontWeight.w200)),
              const SizedBox(height: 20),
              Column(children: [
                SizedBox(
                    width: 377,
                    child: Text(_warnMsg,
                        textAlign: TextAlign.left,
                        style: TextStyle(
                            color: theme.getTheme().dividerColor,
                            fontSize: theme.getMediumFont(context),
                            fontWeight: FontWeight.w300))),
                Center(
                  child: SizedBox(
                      width: 377,
                      child: CheckboxListTile(
                        title: Text("Wallet does not have any funds",
                            style: TextStyle(
                                color: theme.getTheme().dividerColor)),
                        activeColor: theme.getTheme().dividerColor,
                        value: deleteAccepted,
                        side: BorderSide(color: theme.getTheme().dividerColor),
                        onChanged: (val) {
                          setState(() {
                            deleteAccepted = val ?? false;
                          });
                        },
                      )),
                ),
                const SizedBox(height: 34),
                Center(
                    child: SizedBox(
                        width: 278,
                        child: Row(children: [
                          const SizedBox(width: 35),
                          LoadingScreenButton(
                            onPressed: deleteAccepted && !deleting
                                ? () => deleteWalletDir(context)
                                : null,
                            text: "Delete Wallet",
                          ),
                          const SizedBox(width: 10),
                        ])))
              ]),
            ])));
  }
}
