import 'package:bruig/components/text.dart';
import 'package:bruig/components/usersearch/user_search_model.dart';
import 'package:bruig/components/usersearch/user_search_panel.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/util.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';

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
  ChatModel get target => widget.chat;
  bool loading = false;
  UserSelectionModel userSel = UserSelectionModel();

  void transReset(BuildContext context) async {
    if (loading) return;
    if (userSel.selected.isEmpty) return;

    setState(() => loading = true);
    var snackbar = SnackBarModel.of(context);
    var mediator = userSel.selected[0];

    try {
      await Golib.transReset(mediator.id, target.id);
      target.append(
          ChatEventModel(
              SynthChatEvent(
                  "Attempting transitive reset through ${mediator.nick}",
                  SCE_sent),
              null),
          false);
      snackbar.success(
          'Attempting transitive reset with ${target.nick} through mediator ${mediator.nick}');
      popNavigatorFromState(this);
    } catch (exception) {
      snackbar.error('Unable to transitive reset: $exception');
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
        Txt.L("Select transitive reset mediator"),
        const SizedBox(height: 10),
        Expanded(
            child: UserSearchPanel(
          client,
          userSelModel: userSel,
          targets: UserSearchPanelTargets.users,
          searchInputHintText: "Search for users",
          confirmLabel: "Attempt transitive reset",
          excludeUIDs: [target.id],
          onCancel: cancel,
          onConfirm: !loading ? () => transReset(context) : null,
        ))
      ]),
    );
  }
}
