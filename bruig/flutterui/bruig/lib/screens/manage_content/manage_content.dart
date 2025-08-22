import 'dart:async';
import 'package:bruig/components/dcr_input.dart';
import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/components/usersearch/user_search_model.dart';
import 'package:bruig/components/usersearch/user_search_panel.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:file_picker/file_picker.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

typedef RemoveContentCB = Future<void> Function(String fid, String? uid);

class SharedContentFile extends StatefulWidget {
  final SharedFileAndShares file;
  final RemoveContentCB removeContentCB;
  final ClientModel client;
  const SharedContentFile(this.file, this.removeContentCB, this.client,
      {super.key});

  @override
  State<SharedContentFile> createState() => _SharedContentFileState();
}

class _SharedContentFileState extends State<SharedContentFile> {
  bool loading = false;

  removeContent(BuildContext context, String? uid) async {
    var snackbar = SnackBarModel.of(context);
    setState(() => loading = true);
    try {
      await widget.removeContentCB(widget.file.sf.fid, uid);
    } catch (exception) {
      snackbar.error('Unable to unshare content: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    var file = widget.file;
    return Column(
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Txt.S(file.sf.filename),
            /*
            Txt.S((file.cost / 1e8).toString()),
            Txt.S(file.sf.fid, overflow: TextOverflow.ellipsis),
            */
            !widget.file.global
                ? const Text("")
                : IconButton(
                    iconSize: 18,
                    icon: Icon(loading ? Icons.hourglass_bottom : Icons.delete),
                    onPressed:
                        loading ? null : () => removeContent(context, null),
                  )
          ],
        ),
        /*
        ListView.builder(
          scrollDirection: Axis.vertical,
          shrinkWrap: true,
          itemCount: widget.file.shares.length,
          itemBuilder: (BuildContext context, int index) {
            return Row(children: [
              const SizedBox(width: 40),
              Txt.S(widget.file.shares[index]),
              const SizedBox(width: 20),
              Txt.S(
                  widget.client
                          .getExistingChat(widget.file.shares[index])
                          ?.nick ??
                      ""),
              IconButton(
                iconSize: 18,
                icon: Icon(loading ? Icons.hourglass_bottom : Icons.delete),
                onPressed: loading
                    ? null
                    : () => removeContent(context, widget.file.shares[index]),
              )
            ]);
            
          },
        ),*/
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
    var theme = ThemeNotifier.of(context);
    return ListView.builder(
      itemCount: files.length,
      itemBuilder: (BuildContext context, int index) {
        return ListTile(
            contentPadding: const EdgeInsets.all(0),
            title: Container(
                margin: const EdgeInsets.only(left: 8, right: 8),
                padding:
                    const EdgeInsets.only(bottom: 5, left: 5, right: 5, top: 5),
                decoration: BoxDecoration(
                    borderRadius: BorderRadius.circular(10),
                    border: Border.all(color: theme.colors.outline)),
                child: SharedContentFile(files[index], removeContent, client)),
            onTap: fileSelectedCB != null
                ? () => fileSelectedCB!(files[index].sf)
                : null);
      },
    );
  }
}

typedef AddContentCB = Future<void> Function(
    String filename, String uid, double cost);

class AddContentPanel extends StatefulWidget {
  final AddContentCB addContentCB;
  const AddContentPanel(this.addContentCB, {super.key});

  @override
  State<AddContentPanel> createState() => _AddContentPanelState();
}

class _AddContentPanelState extends State<AddContentPanel> {
  bool loading = false;
  TextEditingController fnameCtrl = TextEditingController();
  AmountEditingController costCtrl = AmountEditingController();
  ChatModel? limitToUser;
  Timer? _debounce;
  bool selectingTargetUser = false;
  UserSelectionModel userSel = UserSelectionModel();

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
      if (filePath == null) {
        return;
      }
      setState(() {
        fnameCtrl.text = filePath!;
      });
    });
  }

  void addContent() async {
    var snackbar = SnackBarModel.of(context);
    var fname = fnameCtrl.text.trim();
    if (fname == "") return;
    double cost = 0;
    if (costCtrl.text.isNotEmpty) {
      cost = double.parse(costCtrl.text);
    }
    var uid = limitToUser?.id ?? "";
    setState(() => loading = true);
    try {
      await widget.addContentCB(fname, uid, cost);
      setState(() {
        fnameCtrl.clear();
        costCtrl.clear();
        limitToUser = null;
        userSel.clear();
      });
    } catch (exception) {
      snackbar.error('Unable to share content: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  Widget buildSelectTargetUser(BuildContext context) {
    var client = ClientModel.of(context, listen: false);
    return Container(
      padding: const EdgeInsets.all(10),
      child: Column(children: [
        Expanded(
            child: UserSearchPanel(
          client,
          userSelModel: userSel,
          targets: UserSearchPanelTargets.users,
          searchInputHintText: "Search for users",
          confirmLabel: "Select as target user",
          onCancel: () {
            setState(() => selectingTargetUser = false);
          },
          onConfirm: () {
            setState(() {
              limitToUser =
                  userSel.selected.isNotEmpty ? userSel.selected[0] : null;
              selectingTargetUser = false;
            });
          },
        ))
      ]),
    );
  }

  @override
  Widget build(BuildContext context) {
    if (selectingTargetUser) {
      return buildSelectTargetUser(context);
    }

    return Column(children: [
      Row(children: [
        SizedBox(
            width: 130,
            child: OutlinedButton(
              onPressed: loading ? null : pickFile,
              child: const Txt.S("Select File"),
            )),
        const SizedBox(width: 15),
        Expanded(child: Txt.S(fnameCtrl.text, overflow: TextOverflow.ellipsis)),
      ]),
      const SizedBox(height: 20),
      Row(children: [
        const SizedBox(
          width: 130,
          child: Txt.S("Sharing Preference:"),
        ),
        Container(
          alignment: Alignment.centerRight,
          padding: const EdgeInsets.only(left: 8, top: 4, bottom: 4, right: 8),
          decoration: BoxDecoration(borderRadius: BorderRadius.circular(3)),
          // width: 200,
          height: 34,
          child: Wrap(crossAxisAlignment: WrapCrossAlignment.center, children: [
            if (limitToUser != null) ...[
              ChatAvatar(limitToUser!),
              const SizedBox(width: 5),
              Txt.S(limitToUser!.nick),
              const SizedBox(width: 10),
            ],
            TextButton(
                onPressed: () {
                  setState(() => selectingTargetUser = true);
                },
                child: Txt.S("Select Target user")),
          ]),
        ),
      ]),
      const SizedBox(height: 20),
      Row(children: [
        const SizedBox(
          width: 130,
          child: Txt.S("Cost for user:"),
        ),
        SizedBox(
          width: 100,
          child: dcrInput(controller: costCtrl),
        ),
        const SizedBox(width: 10),
        const Tooltip(
          message: "How much others will pay for this content",
          child: Icon(
            Icons.help_outline,
            size: 16,
          ),
        ),
      ]),
      const SizedBox(height: 20),
      Row(children: [
        SizedBox(
            width: 100,
            child: OutlinedButton(
              onPressed: loading ? null : addContent,
              child: const Txt.S("Share"),
            ))
      ]),
    ]);
  }
}

class ManageContentScreenArgs {
  final bool selectFile;

  ManageContentScreenArgs(this.selectFile);
}

class ManageContent extends StatefulWidget {
  static String routeName = "/manageContent";
  final int view;
  const ManageContent(this.view, {super.key});

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

  Future<void> addContent(String filename, String uid, double cost) async {
    await Golib.shareFile(filename, uid, cost, "");
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
    return Consumer<ClientModel>(
        builder: (context, client, child) => Container(
              margin: const EdgeInsets.all(1),
              padding: const EdgeInsets.all(10),
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
