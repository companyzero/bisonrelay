import 'package:bruig/components/text.dart';
import 'package:bruig/components/usersearch/user_search_model.dart';
import 'package:bruig/components/usersearch/user_search_panel.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/util.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';

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
  UserSelectionModel userSel = UserSelectionModel();

  @override
  void initState() {
    super.initState();
  }

  void suggestKX(BuildContext context) async {
    if (loading) return;
    if (userSel.selected.isEmpty) return;

    setState(() => loading = true);
    var snackbar = SnackBarModel.of(context);
    var target = userSel.selected[0];

    try {
      await Golib.suggestKX(chat.id, target.id);
      chat.append(
          ChatEventModel(
              SynthChatEvent("Sent KX suggestion ${target.nick}", SCE_sent),
              null),
          false);
      snackbar.success('Sent ${target.nick} as KX suggestion to ${chat.nick}');
      popNavigatorFromState(this);
    } catch (exception) {
      snackbar.error('Unable to suggest KX: $exception');
      popNavigatorFromState(this);
    } finally {
      setState(() => loading = false);
    }
  }

  void cancel() {
    Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    var client = ClientModel.of(context, listen: false);
    return Container(
      padding: const EdgeInsets.all(10),
      child: Column(children: [
        Txt.L("Suggest ${chat.nick} KX with"),
        const SizedBox(height: 10),
        Expanded(
            child: UserSearchPanel(
          client,
          userSelModel: userSel,
          targets: UserSearchPanelTargets.users,
          searchInputHintText: "Search for users",
          confirmLabel: "Send Suggestion",
          excludeUIDs: [chat.id],
          onCancel: cancel,
          onConfirm: !loading ? () => suggestKX(context) : null,
        ))
      ]),
    );
  }
}
