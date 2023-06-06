import 'dart:async';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/users_dropdown.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:file_picker/file_picker.dart';
import 'package:provider/provider.dart';

typedef RemoveContentCB = Future<void> Function(String fid, String? uid);

class SharedContentFile extends StatefulWidget {
  final SharedFileAndShares file;
  final RemoveContentCB removeContentCB;
  final ClientModel client;
  const SharedContentFile(this.file, this.removeContentCB, this.client,
      {Key? key})
      : super(key: key);

  @override
  State<SharedContentFile> createState() => _SharedContentFileState();
}

class _SharedContentFileState extends State<SharedContentFile> {
  bool loading = false;

  removeContent(BuildContext context, String? uid) async {
    setState(() => loading = true);
    try {
      await widget.removeContentCB(widget.file.sf.fid, uid);
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to unshare content: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var file = widget.file;
    return Column(
      children: [
        Row(
          children: [
            Expanded(
                flex: 100,
                child: Text(file.sf.filename,
                    style: TextStyle(color: textColor, fontSize: 11))),
            Expanded(
                flex: 50,
                child: Text((file.cost / 1e8).toString(),
                    style: TextStyle(color: textColor, fontSize: 11))),
            Expanded(
                flex: 40,
                child: Text(file.sf.fid,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(color: textColor, fontSize: 11))),
            Expanded(
                flex: 5,
                child: !widget.file.global
                    ? const Text("")
                    : IconButton(
                        iconSize: 18,
                        icon: Icon(
                            loading ? Icons.hourglass_bottom : Icons.delete),
                        onPressed:
                            loading ? null : () => removeContent(context, null),
                      ))
          ],
        ),
        ListView.builder(
          scrollDirection: Axis.vertical,
          shrinkWrap: true,
          itemCount: widget.file.shares.length,
          itemBuilder: (BuildContext context, int index) {
            return Row(children: [
              const SizedBox(width: 40),
              Text(widget.file.shares[index],
                  style: TextStyle(color: textColor, fontSize: 11)),
              const SizedBox(width: 20),
              Text(
                  widget.client
                          .getExistingChat(widget.file.shares[index])
                          ?.nick ??
                      "",
                  style: TextStyle(color: textColor, fontSize: 11)),
              IconButton(
                iconSize: 18,
                icon: Icon(loading ? Icons.hourglass_bottom : Icons.delete),
                onPressed: loading
                    ? null
                    : () => removeContent(context, widget.file.shares[index]),
              )
            ]);
          },
        ),
      ],
    );
  }
}

typedef FileSelectedCB = Function(SharedFile);

class SharedContent extends StatelessWidget {
  final List<SharedFileAndShares> files;
  final RemoveContentCB removeContent;
  final ClientModel client;
  final FileSelectedCB? fileSelectedCB;
  const SharedContent(
      this.client, this.files, this.removeContent, this.fileSelectedCB,
      {super.key});

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var backgroundColor = theme.backgroundColor;
    var otherBackgroundColor = theme.indicatorColor;
    return Container(
      padding: const EdgeInsets.all(30),
      child: ListView.builder(
          itemCount: files.length,
          itemBuilder: (BuildContext context, int index) {
            return ListTile(
                contentPadding: const EdgeInsets.all(0),
                title: Container(
                    margin: const EdgeInsets.only(top: 0, bottom: 0),
                    padding: const EdgeInsets.only(
                        left: 8, top: 0, right: 8, bottom: 0),
                    color: index.isOdd ? backgroundColor : otherBackgroundColor,
                    child:
                        SharedContentFile(files[index], removeContent, client)),
                onTap: fileSelectedCB != null
                    ? () => fileSelectedCB!(files[index].sf)
                    : null);
          }),
    );
  }
}

typedef AddContentCB = Future<void> Function(
    String filename, String nick, double cost);

class AddContentPanel extends StatefulWidget {
  final AddContentCB addContentCB;
  const AddContentPanel(this.addContentCB, {Key? key}) : super(key: key);

  @override
  State<AddContentPanel> createState() => _AddContentPanelState();
}

class _AddContentPanelState extends State<AddContentPanel> {
  bool loading = false;
  TextEditingController fnameCtrl = TextEditingController();
  TextEditingController toCtrl = TextEditingController();
  TextEditingController costCtrl = TextEditingController();
  Timer? _debounce;

  @override
  dispose() {
    _debounce?.cancel();
    super.dispose();
  }

  void pickFile() {
    String? filePath;
    if (_debounce?.isActive ?? false) _debounce!.cancel();
    _debounce = Timer(const Duration(milliseconds: 500), () async {
      var filePickRes = await FilePicker.platform.pickFiles();
      if (filePickRes == null) return;
      var fPath = filePickRes.files.first.path;
      if (fPath == null) return;
      filePath = fPath.trim();
    });
    if (filePath == null) return;
    setState(() {
      fnameCtrl.text = filePath!;
    });
  }

  void addContent() async {
    var fname = fnameCtrl.text.trim();
    if (fname == "") return;
    double cost = 0;
    if (costCtrl.text.isNotEmpty) {
      cost = double.parse(costCtrl.text);
    }
    var nick = toCtrl.text.trim();
    setState(() => loading = true);
    try {
      await widget.addContentCB(fname, nick, cost);
      setState(() {
        fnameCtrl.clear();
        costCtrl.clear();
        toCtrl.clear();
      });
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to share content: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    var inputBackground = theme.hoverColor;
    var secondaryTextColor = theme.focusColor;
    return Column(children: [
      Row(children: [
        SizedBox(
            width: 100,
            child: OutlinedButton(
              onPressed: loading ? null : pickFile,
              style: OutlinedButton.styleFrom(
                textStyle: TextStyle(color: textColor, fontSize: 11),
              ),
              child: Text("Select File",
                  style: TextStyle(color: textColor, fontSize: 11)),
            )),
        const SizedBox(width: 15),
        Container(
            padding:
                const EdgeInsets.only(left: 8, top: 4, bottom: 4, right: 8),
            decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(3), color: inputBackground),
            width: 250,
            height: 34,
            child: Center(
                child: Text(
              fnameCtrl.text,
              softWrap: true,
              style: TextStyle(color: textColor, fontSize: 11),
              //overflow: TextOverflow.ellipsis,
            ))),
      ]),
      const SizedBox(height: 20),
      Row(children: [
        SizedBox(
          width: 115,
          child: Text("Sharing Preference:",
              style: TextStyle(color: textColor, fontSize: 11)),
        ),
        Container(
            alignment: Alignment.centerRight,
            padding:
                const EdgeInsets.only(left: 8, top: 4, bottom: 4, right: 8),
            decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(3), color: inputBackground),
            width: 200,
            height: 34,
            child: UsersDropdown(
              allowEmpty: true,
              cb: (c) => setState(() {
                toCtrl.text = c?.id ?? "";
              }),
            ))
      ]),
      const SizedBox(height: 20),
      Row(children: [
        SizedBox(
          width: 115,
          child: Text("Cost for user:",
              style: TextStyle(color: textColor, fontSize: 11)),
        ),
        Container(
          width: 100,
          child: TextField(
            enabled: !loading,
            cursorColor: secondaryTextColor,
            decoration: InputDecoration(
                border: InputBorder.none,
                hintText: "Cost DCR/kb",
                hintStyle: TextStyle(fontSize: 11, color: textColor),
                filled: true,
                fillColor: inputBackground),
            style: TextStyle(color: secondaryTextColor, fontSize: 11),
            controller: costCtrl,
            keyboardType: const TextInputType.numberWithOptions(decimal: true),
            inputFormatters: [
              FilteringTextInputFormatter.allow(RegExp(r'[0-9]+\.?[0-9]*'))
            ],
          ),
        ),
        const SizedBox(width: 10),
        Tooltip(
          message: "How much others will pay for this content",
          child: Icon(
            Icons.help,
            size: 16,
            color: textColor,
          ),
        ),
      ]),
      const SizedBox(height: 20),
      Row(children: [
        SizedBox(
            width: 100,
            child: OutlinedButton(
              //style: ElevatedButton.styleFrom(primary: Colors.transparent),
              onPressed: loading ? null : addContent,
              child: Text("Share",
                  style: TextStyle(color: textColor, fontSize: 11)),
            ))
      ]),
    ]);
  }
}

class ManageContentScreenTitle extends StatelessWidget {
  const ManageContentScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return Text("Bison Relay / Manage Content",
        style: TextStyle(color: Theme.of(context).focusColor));
  }
}

class ManageContentScreenArgs {
  final bool selectFile;

  ManageContentScreenArgs(this.selectFile);
}

class ManageContent extends StatefulWidget {
  static String routeName = "/manageContent";
  final int view;
  const ManageContent(this.view, {Key? key}) : super(key: key);

  @override
  State<ManageContent> createState() => _ManageContentState();
}

class _ManageContentState extends State<ManageContent> {
  List<SharedFileAndShares> files = [];

  Future<void> loadSharedContent() async {
    var newfiles = await Golib.listSharedFiles();
    newfiles.sort((SharedFileAndShares a, SharedFileAndShares b) {
      // Sort by dir, then filename.
      return a.sf.filename.compareTo(b.sf.filename);
    });
    setState(() {
      files = newfiles;
    });
  }

  Future<void> addContent(String filename, String nick, double cost) async {
    await Golib.shareFile(filename, nick, cost, "");
    await loadSharedContent();
  }

  Future<void> removeContent(String fid, String? uid) async {
    await Golib.unshareFile(fid, uid);
    await loadSharedContent();
  }

  @override
  void initState() {
    super.initState();
    loadSharedContent();
  }

  void fileSelected(SharedFile sf) {
    Navigator.of(context).pop(sf);
  }

  @override
  Widget build(BuildContext context) {
    FileSelectedCB? fileSelCB;
    if (ModalRoute.of(context)!.settings.arguments is ManageContentScreenArgs) {
      var args =
          ModalRoute.of(context)!.settings.arguments as ManageContentScreenArgs;
      if (args.selectFile) {
        fileSelCB = fileSelected;
      }
    }
    var theme = Theme.of(context);
    var backgroundColor = theme.backgroundColor;
    return Consumer<ClientModel>(
        builder: (context, client, child) => Container(
              margin: const EdgeInsets.all(1),
              decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(3),
                  color: backgroundColor),
              padding: const EdgeInsets.all(16),
              child: widget.view == 1
                  ? SharedContent(
                      client,
                      files,
                      removeContent,
                      fileSelCB,
                    )
                  : AddContentPanel(addContent),
            ));
  }
}
