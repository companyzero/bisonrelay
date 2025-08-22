import 'package:bruig/components/buttons.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/util.dart';
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
  const TransResetModal(this.chat, {super.key});

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
    var snackbar = SnackBarModel.of(context);
    try {
      await Golib.transReset(chat.id, userToTarget!.id);
      snackbar.success('Sent transitive reset to ${chat.nick}');
      popNavigatorFromState(this);
    } catch (exception) {
      snackbar.error('Unable to transitive reset: $exception');
      popNavigatorFromState(this);
    } finally {
      setState(() => loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    var client = ClientModel.of(context, listen: false);
    return Container(
      padding: const EdgeInsets.all(30),
      child: Wrap(
          runSpacing: 10,
          spacing: 10,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            Text("Transitive Reset '${chat.nick}' through: "),
            SizedBox(
                width: 200,
                child: UsersDropdown(
                    client: client,
                    cb: (ChatModel? chat) {
                      userToTarget = chat;
                    },
                    excludeUIDs: [chat.id])),
            CancelButton(onPressed: () => Navigator.pop(context)),
            OutlinedButton(
                onPressed: !loading ? () => transReset(context) : null,
                child: const Text('Reset')),
          ]),
    );
  }
}
