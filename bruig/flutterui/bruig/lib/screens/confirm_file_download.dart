import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/downloads.dart';
import 'package:bruig/util.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/util.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

class ConfirmFileDownloadScreen extends StatelessWidget {
  final ClientModel client;
  final DownloadsModel downloads;
  const ConfirmFileDownloadScreen(this.client, this.downloads, {Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    var fd = ModalRoute.of(context)!.settings.arguments as ConfirmFileDownload;
    var cm = client.getExistingChat(fd.uid);
    var sender = cm?.nick ?? fd.uid;
    var cost = formatDCR(atomsToDCR(fd.metadata.cost));
    var size = humanReadableSize(fd.metadata.size);
    var theme = Theme.of(context);
    var textColor = theme.focusColor;

    reply(bool res) async {
      try {
        if (res) {
          downloads.ensureDownloadExists(fd.uid, fd.fid, fd.metadata);
        }
        await downloads.confirmFileDownload(fd.uid, fd.fid, res);
      } catch (exception) {
        showErrorSnackbar(
            context, "Unable to confirm file download: $exception");
      } finally {
        Navigator.pop(context);
      }
    }

    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Scaffold(
            body: Container(
                padding: const EdgeInsets.all(10),
                child: Center(
                    child: Column(children: [
                  Text("Confirm File Download",
                      style: TextStyle(
                          fontSize: theme.getLargeFont(context),
                          color: textColor)),
                  const SizedBox(height: 20),
                  Text("Sender: $sender",
                      style: TextStyle(
                          fontSize: theme.getMediumFont(context),
                          color: textColor)),
                  const SizedBox(height: 20),
                  Text("FID: ${fd.uid}",
                      style: TextStyle(
                          fontSize: theme.getMediumFont(context),
                          color: textColor)),
                  const SizedBox(height: 20),
                  Text("File Name: ${fd.metadata.filename}",
                      style: TextStyle(
                          fontSize: theme.getMediumFont(context),
                          color: textColor)),
                  const SizedBox(height: 20),
                  Text("Size: $size",
                      style: TextStyle(
                          fontSize: theme.getMediumFont(context),
                          color: textColor)),
                  const SizedBox(height: 20),
                  Text("Cost: $cost",
                      style: TextStyle(
                          fontSize: theme.getMediumFont(context),
                          color: textColor)),
                  const SizedBox(height: 20),
                  Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                    ElevatedButton(
                        onPressed: () => reply(true),
                        child: const Text("Pay & Download")),
                    const SizedBox(width: 10),
                    CancelButton(onPressed: () => reply(false)),
                  ]),
                ])))));
  }
}
