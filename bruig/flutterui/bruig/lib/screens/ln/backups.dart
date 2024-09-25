import 'dart:io';

import 'package:bruig/models/snackbar.dart';
import 'package:bruig/screens/ln/components.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';

class LNBackupsPage extends StatefulWidget {
  const LNBackupsPage({super.key});

  @override
  State<LNBackupsPage> createState() => _LNBackupsPageState();
}

class _LNBackupsPageState extends State<LNBackupsPage> {
  @override
  void initState() {
    super.initState();
  }

  void saveSCB() async {
    var snackbar = SnackBarModel.of(context);
    var filePath = await FilePicker.platform.saveFile(
      dialogTitle: "Select scb file location",
      fileName: "ln-channel-backup.scb",
    );
    if (filePath == null) return;
    try {
      var data = await Golib.lnSaveMultiSCB();
      await File(filePath).writeAsBytes(data);
      snackbar.success("Saved SCB file to $filePath");
    } catch (exception) {
      snackbar.error("Unable to save SCB file: $exception");
    }
  }

  void restoreSCB() async {
    var snackbar = SnackBarModel.of(context);
    try {
      var filePickRes = await FilePicker.platform.pickFiles();
      if (filePickRes == null) return;
      var filePath = filePickRes.files.first.path;
      if (filePath == null) return;
      filePath = filePath.trim();
      if (filePath == "") return;
      var scb = await File(filePath).readAsBytes();
      await Golib.lnRestoreMultiSCB(scb);
      snackbar.success("Restored SCB file");
    } catch (exception) {
      snackbar.error("Unable to restore SCB file: $exception");
    }
  }

  @override
  Widget build(BuildContext context) {
    return Container(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const LNInfoSectionHeader("Backups"),
            const SizedBox(height: 8),
            const SizedBox(
                width: 650,
                child: Text(
                  '''
LN funds are locked in channels which cannot be recovered from the seed alone. 

Static Channel Backup (SCB) files are needed to request that remote hosts close channels with the local wallet after the wallet is restored from seed. 

This backup file needs to be updated every time a channel is opened or closed, or users are at risk of losing access to their funds.
''',
                )),
            const SizedBox(height: 8),
            Wrap(
                alignment: WrapAlignment.spaceBetween,
                spacing: 10,
                runSpacing: 20,
                children: [
                  OutlinedButton(
                      onPressed: saveSCB, child: const Text("Save SCB file")),
                  OutlinedButton(
                      onPressed: restoreSCB,
                      child: const Text("Restore SCB file")),
                ])
          ],
        ));
  }
}
