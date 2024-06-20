import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/config.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';

const _errorMoveMsg = "We were unable to move you existing BR wallet/db due "
    "to there already being a wallet/db at the following location:\n"
    "%LOCALAPPDATA%/bruig\n"
    "Please resolve this conflict and then restart Bison Relay";

const _warnMsg1 = "There is an existing version of "
    "the wallet. To use that old version you need to move it "
    "to a new location.  It is advised to make a backup of "
    "the following directory before proceeding with the move:";
const _warnMsg2 =
    "%LOCALAPPDATA%/Packages/com.flutter.bruig_ywj3797wkq8tj/LocalCache/Local/bruig";
const _warnMsg3 = "THIS ACTION CANNOT BE REVERSED";

class MoveOldVersionWalletPage extends StatefulWidget {
  final NewConfigModel newconf;
  const MoveOldVersionWalletPage(this.newconf, {super.key});

  @override
  State<MoveOldVersionWalletPage> createState() =>
      _MoveOldVersionWalletPageState();
}

class _MoveOldVersionWalletPageState extends State<MoveOldVersionWalletPage> {
  NewConfigModel get newconf => widget.newconf;
  bool moveAcccepted = false;
  bool moving = false;
  bool unableToMove = false;

  void moveOldVersion(BuildContext context) async {
    setState(() {
      moving = true;
    });
    try {
      await newconf.moveOldWalletVersion();
    } catch (exception) {
      if (exception == unableToMoveOldWallet) {
        showErrorSnackbar(context,
            "Unable to move wallet dir because of existing wallet in new location: $exception");
        unableToMove = true;
        return;
      } else {
        showErrorSnackbar(context, "Unable to move wallet dir: $exception");
      }
      return;
    }
    Navigator.of(context).pushReplacementNamed("/newconf/lnChoice/internal");
  }

  @override
  Widget build(BuildContext context) {
    return StartupScreen([
      unableToMove
          ? const Column(children: [
              Txt.H("Move old wallet"),
              SizedBox(height: 20),
              SizedBox(
                  width: 377,
                  child: Text(_errorMoveMsg, textAlign: TextAlign.left)),
            ])
          : Column(children: [
              const Txt.H("Move old wallet"),
              const SizedBox(height: 20),
              Column(children: [
                const SizedBox(
                    width: 377,
                    child: Column(children: [
                      Text(_warnMsg1, textAlign: TextAlign.left),
                      Copyable(_warnMsg2),
                      Text(_warnMsg3, textAlign: TextAlign.left),
                    ])),
                Center(
                  child: SizedBox(
                      width: 377,
                      child: CheckboxListTile(
                        title: const Text(
                            "Directory has been backed up or proceed without backing up"),
                        value: moveAcccepted,
                        onChanged: (val) {
                          setState(() {
                            moveAcccepted = val ?? false;
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
                            onPressed: moveAcccepted && !moving
                                ? () => moveOldVersion(context)
                                : null,
                            child: const Text("Move Wallet"),
                          ),
                          const SizedBox(width: 10),
                        ])))
              ]),
            ])
    ]);
  }
}
