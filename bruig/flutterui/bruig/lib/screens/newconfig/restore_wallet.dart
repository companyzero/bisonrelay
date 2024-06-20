import 'dart:io';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

class RestoreWalletPage extends StatefulWidget {
  final NewConfigModel newconf;
  const RestoreWalletPage(this.newconf, {super.key});

  @override
  State<RestoreWalletPage> createState() => _RestoreWalletPageState();
}

class _RestoreWalletPageState extends State<RestoreWalletPage> {
  NewConfigModel get newconf => widget.newconf;
  TextEditingController seedCtrl = TextEditingController();
  String scbFilename = "";

  @override
  void initState() {
    super.initState();
    seedCtrl.text = newconf.seedToRestore.join(" ");
  }

  void useSeed() {
    var split = seedCtrl.text.split(' ').map((s) => s.trim()).toList();
    split.removeWhere((s) => s.isEmpty);
    if (split.length != 24) {
      showErrorSnackbar(context,
          "Seed contains ${split.length} words instead of the required 24");
      return;
    }

    // TODO: prevalidate seed words?

    newconf.seedToRestore = split;
    Navigator.of(context).pushNamed("/newconf/lnChoice/internal");
  }

  void selectSCB() async {
    try {
      var filePickRes = await FilePicker.platform.pickFiles();
      if (filePickRes == null) return;
      var filePath = filePickRes.files.first.path;
      if (filePath == null) return;
      filePath = filePath.trim();
      if (filePath == "") return;
      var scb = await File(filePath).readAsBytes();
      setState(() {
        scbFilename = filePath!;
      });
      newconf.multichanBackupRestore = scb;
    } catch (exception) {
      showErrorSnackbar(context, "Unable to load SCB file: $exception");
    }
  }

  void goBack(BuildContext context) {
    Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => StartupScreen(childrenWidth: 600, [
              const Txt.H("Restoring wallet"),
              const SizedBox(height: 20),
              TextField(
                  maxLines: 5,
                  decoration: InputDecoration(
                      border: InputBorder.none,
                      hintText: "abscond ablate ...",
                      labelText: "Seed Words",
                      filled: true,
                      alignLabelWithHint: true,
                      fillColor: theme.colors.surface),
                  controller: seedCtrl),
              const SizedBox(height: 10),
              TextButton(
                  onPressed: selectSCB,
                  child: const Text("Select optional SCB file To Restore")),
              scbFilename.isNotEmpty ? Txt.S(scbFilename) : const Empty(),
              const SizedBox(height: 34),
              LoadingScreenButton(onPressed: useSeed, text: "Continue"),
              const SizedBox(height: 34),
              TextButton(
                onPressed: () => goBack(context),
                child: const Text("Go Back"),
              )
            ]));
  }
}
