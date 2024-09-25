import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/downloads.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/util.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/util.dart';

class ConfirmFileDownloadScreen extends StatelessWidget {
  final ClientModel client;
  final DownloadsModel downloads;
  const ConfirmFileDownloadScreen(this.client, this.downloads, {super.key});

  @override
  Widget build(BuildContext context) {
    var fd = ModalRoute.of(context)!.settings.arguments as ConfirmFileDownload;
    var cm = client.getExistingChat(fd.uid);
    var sender = cm?.nick ?? fd.uid;
    var cost = formatDCR(atomsToDCR(fd.metadata.cost));
    var size = humanReadableSize(fd.metadata.size);

    reply(bool res) async {
      var snackbar = SnackBarModel.of(context);
      try {
        if (res) {
          downloads.ensureDownloadExists(fd.uid, fd.fid, fd.metadata);
        }
        await downloads.confirmFileDownload(fd.uid, fd.fid, res);
      } catch (exception) {
        snackbar.error("Unable to confirm file download: $exception");
      } finally {
        if (context.mounted) Navigator.pop(context);
      }
    }

    return Scaffold(
        body: Container(
            padding: const EdgeInsets.all(10),
            child: Center(
                child: Column(children: [
              const Txt.H("Confirm File Download"),
              const SizedBox(height: 20),
              Text("Sender: $sender"),
              const SizedBox(height: 20),
              Text("FID: ${fd.uid}"),
              const SizedBox(height: 20),
              Text("File Name: ${fd.metadata.filename}"),
              const SizedBox(height: 20),
              Text("Size: $size"),
              const SizedBox(height: 20),
              Text("Cost: $cost"),
              const Expanded(child: Empty()),
              SizedBox(
                  width: 600,
                  child: Wrap(
                      alignment: WrapAlignment.spaceBetween,
                      runSpacing: 10,
                      children: [
                        ElevatedButton(
                            onPressed: () => reply(true),
                            child: const Text("Pay & Download")),
                        const SizedBox(width: 10),
                        CancelButton(onPressed: () => reply(false)),
                      ])),
            ]))));
  }
}
