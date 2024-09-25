import 'package:bruig/components/buttons.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/util.dart';
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
  const SuggestKXModal(this.chat, {super.key});

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
    var snackbar = SnackBarModel.of(context);

    try {
      await Golib.suggestKX(chat.id, userToSuggest!.id);
      snackbar.success('Sent KX suggestion to ${chat.nick}');
      popNavigatorFromState(this);
    } catch (exception) {
      snackbar.error('Unable to suggest KX: $exception');
      popNavigatorFromState(this);
    } finally {
      setState(() => loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(20),
      child: Wrap(
          runSpacing: 10,
          spacing: 10,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            Text("Suggest '${chat.nick}' KX with: "),
            SizedBox(
                width: 200,
                child: UsersDropdown(
                    cb: (ChatModel? chat) {
                      userToSuggest = chat;
                    },
                    excludeUIDs: [chat.id])),
            CancelButton(onPressed: () => Navigator.pop(context)),
            OutlinedButton(
                onPressed: !loading ? () => suggestKX(context) : null,
                child: const Text('Suggest')),
          ]),
    );
  }
}
