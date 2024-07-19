import 'dart:io';

import 'package:bruig/components/confirmation_dialog.dart';
import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/downloads.dart';
import 'package:bruig/screens/chats.dart';
import 'package:bruig/util.dart';
import 'package:file_icon/file_icon.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/util.dart';
import 'package:open_filex/open_filex.dart';

class _ConfirmRemoveToggle extends StatefulWidget {
  final ValueChanged<bool> onToggled;
  const _ConfirmRemoveToggle(this.onToggled);

  @override
  State<_ConfirmRemoveToggle> createState() => _ConfirmRemoveToggleState();
}

class _ConfirmRemoveToggleState extends State<_ConfirmRemoveToggle> {
  int selOpt = 0;

  @override
  Widget build(BuildContext context) {
    return ToggleButtons(
      borderRadius: const BorderRadius.all(Radius.circular(8)),
      constraints: const BoxConstraints(minHeight: 40, minWidth: 100),
      isSelected: [selOpt == 0, selOpt == 1],
      children: [
        Container(
            padding: const EdgeInsets.symmetric(horizontal: 10),
            margin: const EdgeInsets.only(right: 5),
            child: const Text("Do not remove file")),
        Container(
            padding: const EdgeInsets.symmetric(horizontal: 10),
            child: const Text("Remove file from disk")),
      ],
      onPressed: (int index) {
        setState(() {
          selOpt = index;
          widget.onToggled(selOpt == 1);
        });
      },
    );
  }
}

class _FileDownloadW extends StatefulWidget {
  final FileDownloadModel fd;
  final DownloadsModel downloads;
  final ClientModel client;
  const _FileDownloadW(this.fd, this.downloads, this.client, {Key? key})
      : super(key: key);

  @override
  State<_FileDownloadW> createState() => _FileDownloadWState();
}

class _FileDownloadWState extends State<_FileDownloadW> {
  ClientModel get client => widget.client;
  FileDownloadModel get fd => widget.fd;

  void downloadUpdated() {
    setState(() {});
  }

  void cancelDownload() async {
    confirmationDialog(context, () async {
      try {
        await widget.downloads.cancelDownload(widget.fd.uid, widget.fd.fid);
      } catch (exception) {
        showErrorSnackbar(this, "Unable to cancel download: $exception");
      }
    }, "Cancel Download?", "", "Yes", "No");
  }

  void removeDownload() async {
    bool removeFromDisk = false;

    showConfirmDialog(context, title: "Remove download?", onConfirm: () async {
      try {
        await widget.downloads.cancelDownload(widget.fd.uid, widget.fd.fid);
        if (removeFromDisk && fd.diskPath != "") {
          var f = File(fd.diskPath);
          if (f.existsSync()) {
            f.deleteSync();
          }
        }
      } catch (exception) {
        showErrorSnackbar(this, "Unable to remove download: $exception");
      }
    }, child: Builder(builder: (context) {
      if (fd.diskPath == "") return const Empty();
      return _ConfirmRemoveToggle((v) => removeFromDisk = v);
    }));
  }

  void openFile() {
    OpenFilex.open(fd.diskPath);
  }

  @override
  void initState() {
    super.initState();
    widget.fd.addListener(downloadUpdated);
  }

  @override
  void didUpdateWidget(_FileDownloadW oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.fd.removeListener(downloadUpdated);
    widget.fd.addListener(downloadUpdated);
  }

  @override
  void dispose() {
    widget.fd.removeListener(downloadUpdated);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var progress = widget.fd.progress;
    var diskPath = widget.fd.diskPath;
    var meta = widget.fd.rf.metadata;
    if (widget.fd.diskPath != "") {
      progress = 1;
    }

    String filenameTxt = meta?.filename ??
        "<metadata of file ${widget.fd.fid.substring(0, 8)}... not received yet>";

    var sender = client.getExistingChat(fd.uid);
    String fromTxt = "- from ${sender?.nick ?? fd.uid}";

    return Container(
        margin: const EdgeInsets.all(10),
        child: Row(crossAxisAlignment: CrossAxisAlignment.center, children: [
          meta?.filename != ""
              ? FileIcon(meta?.filename ?? "", size: 64)
              : const SizedBox(width: 64),
          const SizedBox(width: 10),
          Expanded(
              child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                Text(filenameTxt),
                // const SizedBox(height: 5),
                diskPath == ""
                    ? Row(children: [
                        Expanded(
                          child: LinearProgressIndicator(
                              minHeight: 8, value: progress > 1 ? 1 : progress),
                        ),
                        const SizedBox(width: 10),
                        SizedBox(
                            width: 65,
                            child: Text(
                              "${(progress * 100).toStringAsFixed(2)}%",
                              textAlign: TextAlign.right,
                            )),
                        TextButton(
                            onPressed: cancelDownload,
                            child: const Icon(Icons.cancel))
                      ])
                    : Row(children: [
                        Copyable(diskPath),
                        const SizedBox(width: 10),
                        TextButton.icon(
                            onPressed: openFile, label: const Text("Open")),
                        TextButton.icon(
                            onPressed: removeDownload,
                            label: const Text("Remove"))
                      ]),
                // const SizedBox(height: 5),
                Row(children: [
                  Text(
                      "${humanReadableSize(meta?.size ?? 0)} - ${formatDCR(atomsToDCR(meta?.cost ?? 0))}"),
                  const SizedBox(width: 5),
                  InkWell(
                      onTap: sender != null
                          ? () => ChatsScreen.gotoChatScreenFor(context, sender)
                          : null,
                      child: Text(fromTxt)),
                ]),
                // const Divider(),
                const SizedBox(height: 10),
              ])),
        ]));
  }
}

class DownloadsScreen extends StatefulWidget {
  static String routeName = "/downloads";
  final DownloadsModel downloads;
  final ClientModel client;
  const DownloadsScreen(this.downloads, this.client, {Key? key})
      : super(key: key);

  @override
  State<DownloadsScreen> createState() => _DownloadsScreenState();
}

class _DownloadsScreenState extends State<DownloadsScreen> {
  List<FileDownloadModel> files = [];

  void downloadsChanged() {
    setState(() {
      files = widget.downloads.downloads.toList();
    });
  }

  @override
  void initState() {
    super.initState();
    files = widget.downloads.downloads.toList();
    widget.downloads.addListener(downloadsChanged);
  }

  @override
  void didUpdateWidget(DownloadsScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.downloads.removeListener(downloadsChanged);
    widget.downloads.addListener(downloadsChanged);
  }

  @override
  void dispose() {
    widget.downloads.removeListener(downloadsChanged);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Column(children: [
      const Txt.L("Downloads"),
      Expanded(
          child: ListView.builder(
        shrinkWrap: true,
        itemCount: files.length,
        itemBuilder: (context, index) => Container(
            padding: const EdgeInsets.only(right: 10),
            child:
                _FileDownloadW(files[index], widget.downloads, widget.client)),
      )),
    ]);
  }
}
