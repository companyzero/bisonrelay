import 'package:bruig/models/downloads.dart';
import 'package:flutter/material.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

class _FileDownloadW extends StatefulWidget {
  final FileDownloadModel fd;
  const _FileDownloadW(this.fd, {Key? key}) : super(key: key);

  @override
  State<_FileDownloadW> createState() => _FileDownloadWState();
}

class _FileDownloadWState extends State<_FileDownloadW> {
  void downloadUpdated() {
    setState(() {});
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
    if (widget.fd.diskPath != "") {
      progress = 1;
    }

    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var cardColor = theme.cardColor;

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Container(
              margin: const EdgeInsets.all(10),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Container(
                    margin: const EdgeInsets.only(right: 10),
                    width: 200,
                    child: LinearProgressIndicator(
                        minHeight: 8,
                        value: progress > 1 ? 1 : progress,
                        color: cardColor,
                        backgroundColor: cardColor,
                        valueColor: AlwaysStoppedAnimation<Color>(textColor)),
                  ),
                  Expanded(
                      child: Text(widget.fd.rf.metadata.filename,
                          style: TextStyle(
                              color: textColor,
                              fontSize: theme.getSmallFont()))),
                  Expanded(
                      child: Text(diskPath,
                          style: TextStyle(
                              color: textColor,
                              fontStyle: FontStyle.italic,
                              fontSize: theme.getSmallFont()))),
                ],
              ),
            ));
  }
}

class DownloadsScreenTitle extends StatelessWidget {
  const DownloadsScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return Text("Bison Relay / Downloads",
        style: TextStyle(color: Theme.of(context).focusColor));
  }
}

class DownloadsScreen extends StatefulWidget {
  static String routeName = "/downloads";
  final DownloadsModel downloads;
  const DownloadsScreen(this.downloads, {Key? key}) : super(key: key);

  @override
  State<DownloadsScreen> createState() => _DownloadsScreenState();
}

class _DownloadsScreenState extends State<DownloadsScreen> {
  List<FileDownloadModel> files = [];

  @override
  void initState() {
    super.initState();
    files = widget.downloads.downloads.toList();
  }

  @override
  void didUpdateWidget(DownloadsScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
  }

  @override
  void dispose() {
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var backgroundColor = theme.backgroundColor;

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Container(
            margin: const EdgeInsets.all(1),
            decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(3), color: backgroundColor),
            padding: const EdgeInsets.all(16),
            child: Column(children: [
              Text("Downloads",
                  style: TextStyle(color: textColor, fontSize: 20)),
              Expanded(
                  child: ListView.builder(
                shrinkWrap: true,
                itemCount: files.length,
                itemBuilder: (context, index) => _FileDownloadW(files[index]),
              )),
            ])));
  }
}
