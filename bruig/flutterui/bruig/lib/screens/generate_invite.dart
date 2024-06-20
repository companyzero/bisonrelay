import 'dart:async';
import 'dart:io';

import 'package:bruig/components/accounts_dropdown.dart';
import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/dcr_input.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
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
    if (!hasExtraAccounts) {
      return Container(
        alignment: Alignment.center,
        width: 400,
        height: 70,
        child: const Text(
            "Cannot send funds from default account. Create a new account to fund invites."),
      );
    }

    return Card.outlined(
        child: Container(
            padding: const EdgeInsets.all(10),
            child: Row(mainAxisSize: MainAxisSize.min, children: [
              const Text("Amount:"),
              const SizedBox(width: 10),
              SizedBox(width: 110, child: dcrInput(controller: fundAmountCtrl)),
              const SizedBox(width: 20),
              const Text("Account:"),
              const SizedBox(width: 10),
              SizedBox(
                  width: 110,
                  child: AccountsDropDown(
                    excludeDefault: true,
                    onChanged: (v) => setState(() {
                      account = v;
                    }),
                  )),
            ])));
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
    var gen = generated!;
    return [
      const Txt.L("Generated invite with key"),
      const SizedBox(height: 20),
      Copyable(gen.key),
      ...(gen.funds != null
          ? [
              const SizedBox(height: 20),
              const Text(
                  "Invite funds available after the following TX is confirmed"),
              Copyable(gen.funds!.txid),
            ]
          : []),
      const SizedBox(height: 20),
      const SizedBox(
          width: 600,
          child: Text(
              "Note: invite keys are NOT public. They should ONLY be sent to the intended "
              "recipient using a secure communication channel, such as an encrypted chat system.",
              style: TextStyle(fontStyle: FontStyle.italic))),
      const SizedBox(height: 20),
      ElevatedButton(
          onPressed: () => Navigator.pop(context), child: const Text("Done"))
    ];
  }

  List<Widget> buildGeneratePanel(BuildContext context) {
    return [
      Container(
          alignment: Alignment.center,
          width: 400,
          child: path != ""
              ? Text("Path: $path")
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
          : const SizedBox(width: 400, height: 76),
      const SizedBox(height: 20),
      SizedBox(
          width: 400,
          child:
              Row(mainAxisAlignment: MainAxisAlignment.spaceBetween, children: [
            OutlinedButton(
                onPressed: !loading && path != "" ? generateInvite : null,
                child: const Text("Generate invite")),
            CancelButton(onPressed: () => Navigator.pop(context))
          ])),
    ];
  }

  @override
  Widget build(BuildContext context) {
    return StartupScreen([
      const Txt.H("Generate Invite"),
      const SizedBox(height: 20),
      ...(generated == null
          ? buildGeneratePanel(context)
          : buildGeneratedInvite(context)),
    ]);
  }
}
