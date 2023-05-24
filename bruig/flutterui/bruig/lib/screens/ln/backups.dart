import 'dart:io';

import 'package:bruig/components/snackbars.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';

class LNBackupsPage extends StatefulWidget {
  const LNBackupsPage({Key? key}) : super(key: key);

  @override
  State<LNBackupsPage> createState() => _LNBackupsPageState();
}

class _LNBackupsPageState extends State<LNBackupsPage> {
  @override
  void initState() {
    super.initState();
  }

  void saveSCB() async {
    var filePath = await FilePicker.platform.saveFile(
      dialogTitle: "Select scb file location",
      fileName: "ln-channel-backup.scb",
    );
    if (filePath == null) return;
    try {
      var data = await Golib.lnSaveMultiSCB();
      await File(filePath).writeAsBytes(data);
      showSuccessSnackbar(context, "Saved SCB file to $filePath");
    } catch (exception) {
      showErrorSnackbar(context, "Unable to save SCB file: $exception");
    }
  }

  void restoreSCB() async {
    try {
      var filePickRes = await FilePicker.platform.pickFiles();
      if (filePickRes == null) return;
      var filePath = filePickRes.files.first.path;
      if (filePath == null) return;
      filePath = filePath.trim();
      if (filePath == "") return;
      var scb = await File(filePath).readAsBytes();
      await Golib.lnRestoreMultiSCB(scb);
      showSuccessSnackbar(context, "Restored SCB file");
    } catch (exception) {
      showErrorSnackbar(context, "Unable to restore SCB file: $exception");
    }
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var darkTextColor = theme.indicatorColor;
    var dividerColor = theme.highlightColor;
    var backgroundColor = theme.backgroundColor;

    return Container(
        margin: const EdgeInsets.all(1),
        decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(3), color: backgroundColor),
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(children: [
              Text("Backups",
                  textAlign: TextAlign.left,
                  style: TextStyle(color: darkTextColor, fontSize: 15)),
              Expanded(
                  child: Divider(
                color: dividerColor, //color of divider
                height: 10, //height spacing of divider
                thickness: 1, //thickness of divier line
                indent: 8, //spacing at the start of divider
                endIndent: 5, //spacing at the end of divider
              )),
            ]),
            const SizedBox(height: 30),
            Text(
              '''LN funds are locked in channels which cannot be recovered from the seed alone.
Static Channel Backup (SCB) files are needed to request that remote hosts 
close channels with the local wallet after the wallet is restored from
seed. This backup file needs to be updated every time a channel is opened or
closed, or users are at risk of losing access to their funds.
''',
              style: TextStyle(color: darkTextColor),
            ),
            const SizedBox(height: 30),
            ElevatedButton(
                onPressed: saveSCB,
                child: Text("Save SCB file",
                    style: TextStyle(fontSize: 11, color: textColor))),
            const SizedBox(height: 30),
            ElevatedButton(
                onPressed: restoreSCB,
                child: Text("Restore SCB file",
                    style: TextStyle(fontSize: 11, color: textColor))),
          ],
        ));
  }
}
