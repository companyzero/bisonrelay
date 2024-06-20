import 'package:bruig/components/text.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/models/snackbar.dart';
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
    var snackbar = SnackBarModel.of(context);
    setState(() {
      deleting = true;
    });
    try {
      await newconf.deleteLNWalletDir();
    } catch (exception) {
      snackbar.error("Unable to delete wallet dir: $exception");
      return;
    }
    if (mounted && context.mounted) {
      Navigator.of(context).pushReplacementNamed("/newconf/lnChoice/internal");
    }
  }

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => StartupScreen([
              const Txt.H("Remove old wallet"),
              const SizedBox(height: 20),
              const SizedBox(
                  width: 377, child: Text(_warnMsg, textAlign: TextAlign.left)),
              Center(
                child: SizedBox(
                    width: 377,
                    child: CheckboxListTile(
                      title: const Text("Wallet does not have any funds"),
                      value: deleteAccepted,
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
                        OutlinedButton(
                          onPressed: deleteAccepted && !deleting
                              ? () => deleteWalletDir(context)
                              : null,
                          child: const Text("Delete Wallet"),
                        ),
                        const SizedBox(width: 10),
                      ])))
            ]));
  }
}
