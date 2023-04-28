import 'package:bruig/components/buttons.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';

void showRenameModalBottom(BuildContext context, ChatModel chat) {
  var snackBar = Provider.of<SnackBarModel>(context);
  showModalBottomSheet(
    context: context,
    builder: (BuildContext context) => RenameChatModal(chat, snackBar),
  );
}

class RenameChatModal extends StatefulWidget {
  final ChatModel chat;
  final SnackBarModel snackBar;
  const RenameChatModal(this.chat, this.snackBar, {Key? key}) : super(key: key);

  @override
  State<RenameChatModal> createState() => _RenameChatModalState();
}

class _RenameChatModalState extends State<RenameChatModal> {
  SnackBarModel get snackBar => widget.snackBar;
  ChatModel get chat => widget.chat;
  TextEditingController nameCtrl = TextEditingController();

  void rename() async {
    try {
      var newName = nameCtrl.text;
      await Golib.localRename(chat.id, newName, chat.isGC);
      Navigator.pop(context);
      chat.nick = newName;
    } catch (exception) {
      Navigator.pop(context);
      snackBar.error("Unable to rename: $exception");
    }
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(30),
      child: Row(children: [
        Text("Rename '${chat.nick}' to: ",
            style: TextStyle(color: Theme.of(context).focusColor)),
        const SizedBox(width: 10, height: 10),
        Expanded(
            child: Container(
          margin: const EdgeInsets.only(right: 10),
          child: TextField(
            controller: nameCtrl,
            autofocus: true,
            onSubmitted: (_) {
              rename();
            },
          ),
        )),
        CancelButton(onPressed: () => Navigator.pop(context)),
        const SizedBox(width: 10, height: 10),
        ElevatedButton(onPressed: rename, child: const Text("Rename")),
      ]),
    );
  }
}
