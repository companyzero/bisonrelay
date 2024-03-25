import 'dart:io';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/snackbars.dart';
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
        builder: (context, theme, _) => StartupScreen(Column(children: [
              Text("Restoring wallet",
                  style: TextStyle(
                      color: theme.getTheme().dividerColor,
                      fontSize: theme.getHugeFont(context),
                      fontWeight: FontWeight.w200)),
              const SizedBox(height: 20),
              SizedBox(
                  width: 577,
                  child: Text("Seed Words",
                      textAlign: TextAlign.left,
                      style: TextStyle(
                          color: theme.getTheme().indicatorColor,
                          fontSize: theme.getMediumFont(context),
                          fontWeight: FontWeight.w300))),
              Center(
                  child: SizedBox(
                      width: 577,
                      child: TextField(
                          maxLines: 5,
                          cursorColor: theme.getTheme().focusColor,
                          decoration: InputDecoration(
                              border: InputBorder.none,
                              hintText: "Seed words",
                              hintStyle: TextStyle(
                                  fontSize: theme.getLargeFont(context),
                                  color: theme.getTheme().dividerColor),
                              filled: true,
                              fillColor: theme.getTheme().cardColor),
                          style: TextStyle(
                              color: theme.getTheme().focusColor,
                              fontSize: theme.getLargeFont(context)),
                          controller: seedCtrl))),
              const SizedBox(height: 10),
              Column(mainAxisAlignment: MainAxisAlignment.center, children: [
                TextButton(
                  onPressed: selectSCB,
                  child: Text(
                    "Select optional SCB file To Restore",
                    style: TextStyle(color: theme.getTheme().dividerColor),
                  ),
                ),
                scbFilename.isNotEmpty
                    ? Text(scbFilename,
                        style:
                            TextStyle(color: theme.getTheme().indicatorColor))
                    : const Empty(),
              ]),
              const SizedBox(height: 34),
              Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                LoadingScreenButton(
                  onPressed: useSeed,
                  text: "Continue",
                ),
              ]),
              const SizedBox(height: 34),
              Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                TextButton(
                  onPressed: () => goBack(context),
                  child: Text("Go Back",
                      style: TextStyle(color: theme.getTheme().dividerColor)),
                )
              ])
            ])));
  }
}
