import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:bruig/components/users_dropdown.dart';

void showSuggestKXModalBottom(BuildContext context, ChatModel chat) {
  showModalBottomSheet(
    context: context,
    builder: (BuildContext context) => SuggestKXModal(chat),
  );
}

class SuggestKXModal extends StatefulWidget {
  final ChatModel chat;
  const SuggestKXModal(this.chat, {Key? key}) : super(key: key);

  @override
  State<SuggestKXModal> createState() => _SuggestKXModalState();
}

class _SuggestKXModalState extends State<SuggestKXModal> {
  ChatModel get chat => widget.chat;
  bool loading = false;
  ChatModel? userToSuggest;

  void suggestKX(BuildContext context) async {
    if (loading) return;
    if (userToSuggest == null) return;
    setState(() => loading = true);

    try {
      await Golib.suggestKX(chat.id, userToSuggest!.id);
      showSuccessSnackbar(context, 'Sent KX suggestion to ${chat.nick}');
      Navigator.of(context).pop();
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to suggest KX: $exception');
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
        Text("Suggest KX '${chat.nick}' with: ",
            style: TextStyle(color: Theme.of(context).focusColor)),
        const SizedBox(width: 10, height: 10),
        Expanded(
            child: UsersDropdown(
                cb: (ChatModel? chat) {
                  userToSuggest = chat;
                },
                nick: chat.nick)),
        const SizedBox(width: 20),
        ElevatedButton(
            onPressed: !loading ? () => suggestKX(context) : null,
            child: const Text('Suggest KX')),
        CancelButton(onPressed: () => Navigator.pop(context)),
      ]),
    );
  }
}
