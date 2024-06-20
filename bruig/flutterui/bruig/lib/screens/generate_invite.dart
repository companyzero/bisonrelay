import 'dart:async';
import 'dart:io';

import 'package:bruig/components/accounts_dropdown.dart';
import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/dcr_input.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';
import 'package:permission_handler/permission_handler.dart';

class GenerateInviteScreen extends StatefulWidget {
  const GenerateInviteScreen({super.key});

  @override
  State<GenerateInviteScreen> createState() => _GenerateInviteScreenState();
}

class _GenerateInviteScreenState extends State<GenerateInviteScreen> {
  String path = "";
  List<bool> selFunding = [true, false];
  bool get sendFunds => selFunding[1];
  AmountEditingController fundAmountCtrl = AmountEditingController();
  String account = "";
  bool hasExtraAccounts = false;
  bool loading = false;
  GeneratedKXInvite? generated;
  Timer? _debounce;

  void checkExtraAccounts() async {
    var accts = await Golib.listAccounts();
    setState(() {
      hasExtraAccounts = accts.length > 1;
    });
  }

  @override
  void initState() {
    super.initState();
    checkExtraAccounts();
  }

  @override
  void dispose() {
    super.dispose();
    _debounce?.cancel();
  }

  void selectPath() async {
    if (_debounce?.isActive ?? false) _debounce!.cancel();
    _debounce = Timer(const Duration(milliseconds: 500), () async {
      if (Platform.isAndroid) {
        if (await Permission.manageExternalStorage.request().isGranted) {
          var filePath = await FilePicker.platform.getDirectoryPath(
            dialogTitle: "Select invitation file location",
          );
          if (filePath == null) return;
          setState(() {
            path = "$filePath/invite.bin";
          });
        }
      } else {
        var filePath = await FilePicker.platform.saveFile(
          dialogTitle: "Select invitation file location",
          fileName: "invite.bin",
        );
        if (filePath == null) return;
        setState(() {
          path = filePath;
        });
      }
    });
  }

  Widget buildSendFundsWidget(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    var darkTextColor = theme.indicatorColor;

    if (!hasExtraAccounts) {
      return SizedBox(
          width: 400,
          height: 70,
          child: Center(
            child: Text(
                "Cannot send funds from default account. Create a new account to fund invites.",
                style: TextStyle(color: textColor)),
          ));
    }

    return Container(
        padding: const EdgeInsets.all(10),
        decoration: BoxDecoration(
            border: Border.all(color: darkTextColor),
            borderRadius: BorderRadius.circular(8)),
        child: Row(mainAxisSize: MainAxisSize.min, children: [
          Text("Amount:", style: TextStyle(color: textColor)),
          const SizedBox(width: 10),
          SizedBox(width: 110, child: dcrInput(controller: fundAmountCtrl)),
          const SizedBox(width: 20),
          Text("Account:", style: TextStyle(color: textColor)),
          const SizedBox(width: 10),
          SizedBox(
              width: 110,
              child: AccountsDropDown(
                excludeDefault: true,
                onChanged: (v) => setState(() {
                  account = v;
                }),
              )),
        ]));
  }

  void generateInvite() async {
    int amount = 0;
    if (sendFunds) {
      if (fundAmountCtrl.amount <= 0) {
        showErrorSnackbar(context, "Amount to fund in invite cannot be <= 0");
        return;
      }
      if (account == "") {
        showErrorSnackbar(context, "Account cannot be empty");
        return;
      }
      amount = dcrToAtoms(fundAmountCtrl.amount);
    }

    setState(() {
      loading = true;
    });
    try {
      var res = await Golib.generateInvite(path, amount, account, null);
      setState(() {
        generated = res;
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to generate invitation: $exception");
    }
    setState(() {
      loading = false;
    });
  }

  List<Widget> buildGeneratedInvite(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    var ts = TextStyle(color: textColor);
    var gen = generated!;
    return [
      Text("Generated invite with key", style: ts),
      const SizedBox(height: 20),
      Copyable(gen.key, textStyle: ts),
      ...(gen.funds != null
          ? [
              const SizedBox(height: 20),
              Text("Invite funds available after the following TX is confirmed",
                  style: ts),
              Copyable(gen.funds!.txid, textStyle: ts),
            ]
          : []),
      const SizedBox(height: 20),
      SizedBox(
          width: 600,
          child: Text(
              "Note: invite keys are NOT public. They should ONLY be sent to the intended " +
                  "recipient using a secure communication channel, such as an encrypted chat system.",
              style: TextStyle(color: textColor, fontStyle: FontStyle.italic))),
      const SizedBox(height: 20),
      ElevatedButton(
          onPressed: () => Navigator.pop(context), child: const Text("Done"))
    ];
  }

  List<Widget> buildGeneratePanel(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    return [
      SizedBox(
          width: 400,
          child: path != ""
              ? Center(
                  child: Text(
                  "Path: $path",
                  style: TextStyle(color: textColor),
                ))
              : ElevatedButton(
                  onPressed: selectPath, child: const Text("Select Path"))),
      const SizedBox(height: 20),
      ToggleButtons(
          borderRadius: const BorderRadius.all(Radius.circular(8)),
          constraints: const BoxConstraints(minHeight: 40, minWidth: 100),
          isSelected: selFunding,
          onPressed: (int index) {
            setState(() {
              for (int i = 0; i < selFunding.length; i++) {
                selFunding[i] = i == index;
              }
            });
          },
          children: const [
            Text("No Funds"),
            Text("Send Funds"),
          ]),
      const SizedBox(height: 20),
      sendFunds
          ? buildSendFundsWidget(context)
          : const SizedBox(width: 400, height: 70),
      const SizedBox(height: 20),
      ElevatedButton(
          onPressed: !loading && path != "" ? generateInvite : null,
          child: const Text("Generate invite")),
      const SizedBox(height: 20),
      ElevatedButton(
          style: ElevatedButton.styleFrom(backgroundColor: theme.errorColor),
          onPressed: () => Navigator.pop(context),
          child: const Text("Cancel"))
    ];
  }

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => StartupScreen([
              Text("Generate Invite",
                  style: TextStyle(
                      color: theme.getTheme().dividerColor,
                      fontSize: theme.getHugeFont(context),
                      fontWeight: FontWeight.w200)),
              const SizedBox(height: 20),
              ...(generated == null
                  ? buildGeneratePanel(context)
                  : buildGeneratedInvite(context)),
            ]));
  }
}
