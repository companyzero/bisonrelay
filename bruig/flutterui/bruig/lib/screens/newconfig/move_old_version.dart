import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/config.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

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
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => StartupScreen([
              unableToMove
                  ? Column(children: [
                      Text("Move old wallet",
                          style: TextStyle(
                              color: theme.getTheme().dividerColor,
                              fontSize: theme.getHugeFont(context),
                              fontWeight: FontWeight.w200)),
                      const SizedBox(height: 20),
                      SizedBox(
                          width: 377,
                          child: Text(_errorMoveMsg,
                              textAlign: TextAlign.left,
                              style: TextStyle(
                                  color: theme.getTheme().dividerColor,
                                  fontSize: theme.getMediumFont(context),
                                  fontWeight: FontWeight.w300))),
                    ])
                  : Column(children: [
                      Text("Move old wallet",
                          style: TextStyle(
                              color: theme.getTheme().dividerColor,
                              fontSize: theme.getHugeFont(context),
                              fontWeight: FontWeight.w200)),
                      const SizedBox(height: 20),
                      Column(children: [
                        SizedBox(
                            width: 377,
                            child: Column(children: [
                              Text(_warnMsg1,
                                  textAlign: TextAlign.left,
                                  style: TextStyle(
                                      color: theme.getTheme().dividerColor,
                                      fontSize: theme.getMediumFont(context),
                                      fontWeight: FontWeight.w300)),
                              Copyable(_warnMsg2,
                                  textStyle: TextStyle(
                                      color: theme.getTheme().dividerColor,
                                      fontSize: theme.getMediumFont(context),
                                      fontWeight: FontWeight.w300)),
                              Text(_warnMsg3,
                                  textAlign: TextAlign.left,
                                  style: TextStyle(
                                      color: theme.getTheme().dividerColor,
                                      fontSize: theme.getMediumFont(context),
                                      fontWeight: FontWeight.w300))
                            ])),
                        Center(
                          child: SizedBox(
                              width: 377,
                              child: CheckboxListTile(
                                title: Text(
                                    "Directory has been backed up or proceed without backing up",
                                    style: TextStyle(
                                        color: theme.getTheme().dividerColor)),
                                activeColor: theme.getTheme().dividerColor,
                                value: moveAcccepted,
                                side: BorderSide(
                                    color: theme.getTheme().dividerColor),
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
                                  LoadingScreenButton(
                                    onPressed: moveAcccepted && !moving
                                        ? () => moveOldVersion(context)
                                        : null,
                                    text: "Move Wallet",
                                  ),
                                  const SizedBox(width: 10),
                                ])))
                      ]),
                    ])
            ]));
  }
}
