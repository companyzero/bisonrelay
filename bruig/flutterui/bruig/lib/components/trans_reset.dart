import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:bruig/components/users_dropdown.dart';

void showTransResetModalBottom(BuildContext context, ChatModel chat) {
  showModalBottomSheet(
    context: context,
    builder: (BuildContext context) => TransResetModal(chat),
  );
}

class TransResetModal extends StatefulWidget {
  final ChatModel chat;
  const TransResetModal(this.chat, {Key? key}) : super(key: key);

  @override
  State<TransResetModal> createState() => _TransResetModalState();
}

class _TransResetModalState extends State<TransResetModal> {
  ChatModel get chat => widget.chat;
  bool loading = false;
  ChatModel? userToTarget;

  void transReset(BuildContext context) async {
    if (loading) return;
    if (userToTarget == null) return;
    setState(() => loading = true);
    print("${chat.id}  ${userToTarget!.id}");
    try {
      await Golib.transReset(chat.id, userToTarget!.id);
      showSuccessSnackbar(context, 'Sent transitive reset to ${chat.nick}');
      Navigator.of(context).pop();
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to transitive reset: $exception');
      Navigator.of(context).pop();
    } finally {
      setState(() => loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(30),
      child: Row(children: [
        Text("Transitive Reset '${chat.nick}' with: ",
            style: TextStyle(color: Theme.of(context).focusColor)),
        const SizedBox(width: 10, height: 10),
        Expanded(
            child: UsersDropdown(
                cb: (ChatModel? chat) {
                  userToTarget = chat;
                },
                nick: chat.nick)),
        const SizedBox(width: 20),
        ElevatedButton(
            onPressed: !loading ? () => transReset(context) : null,
            child: const Text('Transitive Reset')),
        CancelButton(onPressed: () => Navigator.pop(context)),
      ]),
    );
  }
}
