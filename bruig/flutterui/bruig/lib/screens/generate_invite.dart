import 'dart:async';
import 'dart:io';

import 'package:bruig/components/accounts_dropdown.dart';
import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/dcr_input.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/components/qr.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:path/path.dart' as path;
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import 'package:path_provider/path_provider.dart';
import 'package:permission_handler/permission_handler.dart';
import 'package:qr_flutter/qr_flutter.dart';
import 'package:share_plus/share_plus.dart';

class GenerateInviteScreen extends StatefulWidget {
  const GenerateInviteScreen({super.key});

  @override
  State<GenerateInviteScreen> createState() => _GenerateInviteScreenState();
}

class _GenerateInviteScreenState extends State<GenerateInviteScreen> {
  String invitePath = "";
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

    // Set the default export destination on mobile.
    Platform.isAndroid || Platform.isIOS
        ? (() async {
            var nowStr = DateTime.now().toIso8601String().replaceAll(":", "_");
            var fname = path.join(
                (await getApplicationDocumentsDirectory()).path,
                "invites",
                "br-invite-$nowStr.bin");
            var dir = File(fname).parent;
            if (!await dir.exists()) {
              await dir.create();
            }
            setState(() {
              invitePath = fname;
            });
          })()
        : null;
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
            invitePath = "$filePath/invite.bin";
          });
        }
      } else {
        var filePath = await FilePicker.platform.saveFile(
          dialogTitle: "Select invitation file location",
          fileName: "invite.bin",
        );
        if (filePath == null) return;
        setState(() {
          invitePath = filePath;
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
    var snackbar = SnackBarModel.of(context);
    int amount = 0;
    if (sendFunds) {
      if (fundAmountCtrl.amount <= 0) {
        snackbar.error("Amount to fund in invite cannot be <= 0");
        return;
      }
      if (account == "") {
        snackbar.error("Account cannot be empty");
        return;
      }
      amount = dcrToAtoms(fundAmountCtrl.amount);
    }

    setState(() {
      loading = true;
    });
    try {
      var res = await Golib.generateInvite(invitePath, amount, account, null);
      setState(() {
        generated = res;
      });
    } catch (exception) {
      snackbar.error("Unable to generate invitation: $exception");
    }
    setState(() {
      loading = false;
    });
  }

  void exportInviteQRCode() async {
    // Disabled on linux due to commit 8e418d1818 (flutter version 3.21.0-0.0.pre)
    // and later not correctly rendering the QR code.
    if (Platform.isLinux) {
      SnackBarModel.of(context).error("Disabled on linux due to flutter bug");
      return;
    }

    var qr = QrCode.fromData(
        data: generated!.key, errorCorrectLevel: QrErrorCorrectLevel.L);
    var painter = QrPainter.withQr(
      qr: qr,
      gapless: true,
      embeddedImageStyle: null,
    );

    var picData = await QrCodePainter(
      margin: 30,
      qrImage: await painter.toImage(512),
    ).toImageData(512);

    try {
      if (Platform.isAndroid || Platform.isIOS) {
        // Share with contacts.
        var fname = path.join(
            (await getApplicationCacheDirectory()).path, "br-invite.png");
        await File(fname).writeAsBytes(picData!.buffer.asUint8List());
        await Share.shareXFiles(
            [XFile(fname, name: "br-invite.png", mimeType: "image/png")],
            text: "bruig logs");
      } else {
        // Save to file.
        var fname = await FilePicker.platform.saveFile(
          dialogTitle: "Save Invite QR Code",
          fileName: "br-invite.png",
          type: FileType.image,
        );
        if (fname == null) {
          return;
        }
        await File(fname)
            .writeAsBytes(picData!.buffer.asUint8List(), flush: true);
      }
    } catch (exception) {
      showErrorSnackbar(this, "Unable to export QR code: $exception");
    }
  }

  List<Widget> buildGeneratedInvite(BuildContext context) {
    var gen = generated!;
    return [
      InkWell(
          onTap: !Platform.isLinux ? exportInviteQRCode : null,
          child: Container(
              color: Colors.white,
              child: QrImageView(
                  data: gen.key, version: QrVersions.auto, size: 200.0))),
      const SizedBox(height: 20),
      Copyable(gen.key),
      const SizedBox(height: 20),
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
          child: invitePath != ""
              ? Text("Path: $invitePath")
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
          width: 300,
          child: Wrap(
              alignment: WrapAlignment.spaceBetween,
              runSpacing: 10,
              children: [
                OutlinedButton(
                    onPressed:
                        !loading && invitePath != "" ? generateInvite : null,
                    child: const Text("Generate invite")),
                CancelButton(onPressed: () => Navigator.pop(context))
              ])),
    ];
  }

  @override
  Widget build(BuildContext context) {
    return StartupScreen(childrenWidth: 600, [
      generated == null
          ? const Txt.H("Generate Invite")
          : const Txt.H("Generated Invite"),
      const SizedBox(height: 20),
      ...(generated == null
          ? buildGeneratePanel(context)
          : buildGeneratedInvite(context)),
    ]);
  }
}
