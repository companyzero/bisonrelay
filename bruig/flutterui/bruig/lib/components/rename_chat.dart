import 'package:bruig/components/buttons.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/util.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';

void showRenameModalBottom(BuildContext context, ChatModel chat) {
  showModalBottomSheet(
    context: context,
    builder: (BuildContext context) => RenameChatModal(chat),
  );
}

class RenameChatModal extends StatefulWidget {
  final ChatModel chat;
  const RenameChatModal(this.chat, {super.key});

  @override
  State<RenameChatModal> createState() => _RenameChatModalState();
}

class _RenameChatModalState extends State<RenameChatModal> {
  ChatModel get chat => widget.chat;
  TextEditingController nameCtrl = TextEditingController();

  void rename() async {
    var snackbar = SnackBarModel.of(context);
    try {
      var newName = nameCtrl.text;
      await Golib.localRename(chat.id, newName, chat.isGC);
      popNavigatorFromState(this);
      chat.nick = newName;
      snackbar.success("renamed");
    } catch (exception) {
      popNavigatorFromState(this);
      snackbar.error("Unable to rename: $exception");
    }
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(30),
      child: Wrap(
          runSpacing: 10,
          spacing: 10,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            Text("Rename '${chat.nick}' to: "),
            SizedBox(
                width: 200,
                child: TextField(
                    controller: nameCtrl,
                    autofocus: true,
                    onSubmitted: (_) => rename())),
            CancelButton(onPressed: () => Navigator.pop(context)),
            OutlinedButton(onPressed: rename, child: const Text("Rename")),
          ]),
    );
  }
}
