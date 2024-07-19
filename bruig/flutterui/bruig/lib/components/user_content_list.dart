import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/downloads.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/util.dart';
import 'package:open_filex/open_filex.dart';

typedef DownloadContentCB = Future<FileDownloadModel> Function(
    ReceivedFile file);

class SharedContentFile extends StatefulWidget {
  final ReceivedFile file;
  final ChatModel chat;
  final DownloadContentCB downloadContentCB;
  final FileDownloadModel? fd;
  const SharedContentFile(this.file, this.chat, this.fd, this.downloadContentCB,
      {Key? key})
      : super(key: key);

  @override
  State<SharedContentFile> createState() => _SharedContentFileState();
}

class _SharedContentFileState extends State<SharedContentFile> {
  bool loading = false;
  FileDownloadModel? fd;

  downloadContent(BuildContext context) async {
    setState(() => loading = true);
    var snackbar = SnackBarModel.of(context);
    try {
      fd = await widget.downloadContentCB(widget.file);
      fd!.addListener(fdUpdated);
    } catch (exception) {
      snackbar.error('Unable to download content: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  showContent(BuildContext context) {
    if (widget.file.metadata == null) {
      return;
    }

    if (widget.file.metadata!.filename.endsWith(".md")) {
      // FIXME: display content?
      /*
      Navigator.pushNamed(context, "/downloadedMdContent",
          arguments:
              PostContentScreenArgs(widget.chat.nick, widget.file.diskPath));
      */
    } else {
      OpenFilex.open(widget.fd?.diskPath);
    }
  }

  void fdUpdated() {
    setState(() {});
    if ((fd?.diskPath ?? "") != "") {
      showSuccessSnackbar(this, "Download ${fd!.diskPath} completed!");
    }
  }

  @override
  void initState() {
    super.initState();
    fd = widget.fd;
    if (fd != null) {
      fd!.addListener(fdUpdated);
    }
  }

  @override
  void didUpdateWidget(SharedContentFile oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (fd != null) {
      fd!.removeListener(fdUpdated);
    }
    fd = widget.fd;
    if (fd != null) {
      fd!.addListener(fdUpdated);
    }
  }

  @override
  void dispose() {
    super.dispose();
    if (fd != null) {
      fd!.removeListener(fdUpdated);
    }
  }

  @override
  Widget build(BuildContext context) {
    var file = widget.file;
    var isDownloading = fd != null && fd!.diskPath == "";
    var progress = fd?.progress ?? 0;
    if (isDownloading && progress < 0.1) {
      progress = 0.1;
    }
    var diskPath = file.diskPath;
    if (fd != null && diskPath == "") {
      diskPath = fd!.diskPath;
    }
    return Row(
      children: [
        Expanded(flex: 30, child: Text("${file.metadata?.directory}")),
        Expanded(flex: 100, child: Text("${file.metadata?.filename}")),
        Expanded(
            flex: 50,
            child: Text((formatDCR(atomsToDCR(file.metadata?.cost ?? 0))))),
        Expanded(
            flex: 40, child: Text(file.fid, overflow: TextOverflow.ellipsis)),
        isDownloading
            ? CircularProgressIndicator(value: progress)
            : IconButton(
                iconSize: 18,
                icon: Icon(loading || isDownloading
                    ? Icons.hourglass_bottom
                    : diskPath != ""
                        ? Icons.open_in_new
                        : Icons.download),
                onPressed: loading
                    ? null
                    : diskPath != ""
                        ? () => showContent(context)
                        : () => downloadContent(context),
              )
      ],
    );
  }
}

class UserContentListW extends StatelessWidget {
  final UserContentList content;
  final ChatModel chat;
  final DownloadsModel downloads;
  const UserContentListW(this.chat, this.downloads, this.content, {Key? key})
      : super(key: key);

  Future<FileDownloadModel> downloadContent(ReceivedFile file) async {
    return await downloads.getUserFile(file);
  }

  @override
  Widget build(BuildContext context) {
    return Column(
        children: content.files
            .map((e) => Container(
                margin: const EdgeInsets.only(bottom: 5),
                child: SharedContentFile(e, chat,
                    downloads.getDownload(e.uid, e.fid), downloadContent)))
            .toList());
  }
}
