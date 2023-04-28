import 'dart:io';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:bruig/models/snackbar.dart';

class RestoreWalletPage extends StatefulWidget {
  final NewConfigModel newconf;
  final SnackBarModel snackBar;
  const RestoreWalletPage(this.newconf, this.snackBar, {super.key});

  @override
  State<RestoreWalletPage> createState() => _RestoreWalletPageState();
}

class _RestoreWalletPageState extends State<RestoreWalletPage> {
  SnackBarModel get snackBar => widget.snackBar;
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
      snackBar.error(
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
      snackBar.error("Unable to load SCB file: $exception");
    }
  }

  void goBack(BuildContext context) {
    Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    var backgroundColor = const Color(0xFF19172C);
    var cardColor = const Color(0xFF05031A);
    var textColor = const Color(0xFF8E8D98);
    var secondaryTextColor = const Color(0xFFE4E3E6);
    var darkTextColor = const Color(0xFF5A5968);

    void goToAbout() {
      Navigator.of(context).pushNamed("/about");
    }

    return Container(
        color: backgroundColor,
        child: Stack(children: [
          Container(
              decoration: const BoxDecoration(
                  image: DecorationImage(
                      fit: BoxFit.fill,
                      image: AssetImage("assets/images/loading-bg.png")))),
          Container(
              decoration: BoxDecoration(
                  gradient: LinearGradient(
                      begin: Alignment.bottomLeft,
                      end: Alignment.topRight,
                      colors: [
                    cardColor,
                    const Color(0xFF07051C),
                    backgroundColor.withOpacity(0.34),
                  ],
                      stops: const [
                    0,
                    0.17,
                    1
                  ])),
              padding: const EdgeInsets.all(10),
              child: Column(children: [
                Row(children: [
                  IconButton(
                      alignment: Alignment.topLeft,
                      tooltip: "About Bison Relay",
                      iconSize: 50,
                      onPressed: goToAbout,
                      icon: Image.asset(
                        "assets/images/icon.png",
                      )),
                ]),
                const SizedBox(height: 39),
                Text("Restoring wallet",
                    style: TextStyle(
                        color: textColor,
                        fontSize: 34,
                        fontWeight: FontWeight.w200)),
                const SizedBox(height: 20),
                SizedBox(
                    width: 577,
                    child: Text("Seed Words",
                        textAlign: TextAlign.left,
                        style: TextStyle(
                            color: darkTextColor,
                            fontSize: 13,
                            fontWeight: FontWeight.w300))),
                Center(
                    child: SizedBox(
                        width: 577,
                        child: TextField(
                            maxLines: 5,
                            cursorColor: secondaryTextColor,
                            decoration: InputDecoration(
                                border: InputBorder.none,
                                hintText: "Seed words",
                                hintStyle:
                                    TextStyle(fontSize: 21, color: textColor),
                                filled: true,
                                fillColor: cardColor),
                            style: TextStyle(
                                color: secondaryTextColor, fontSize: 21),
                            controller: seedCtrl))),
                const SizedBox(height: 10),
                Column(mainAxisAlignment: MainAxisAlignment.center, children: [
                  TextButton(
                    onPressed: selectSCB,
                    child: Text(
                      "Select optional SCB file To Restore",
                      style: TextStyle(color: textColor),
                    ),
                  ),
                  scbFilename.isNotEmpty
                      ? Text(scbFilename,
                          style: TextStyle(color: darkTextColor))
                      : const Empty(),
                ]),
                const SizedBox(height: 34),
                Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                  LoadingScreenButton(
                    onPressed: useSeed,
                    text: "Continue",
                  ),
                ]),
              ])),
          Row(mainAxisAlignment: MainAxisAlignment.center, children: [
            TextButton(
              onPressed: () => goBack(context),
              child: Text("Go Back", style: TextStyle(color: textColor)),
            )
          ])
        ]));
  }
}
