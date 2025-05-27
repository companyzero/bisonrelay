import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/components/usersearch/selected_users_panel.dart';
import 'package:bruig/components/usersearch/user_search_model.dart';
import 'package:bruig/components/usersearch/user_search_panel.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/realtimechat.dart';
import 'package:flutter/material.dart';

class InviteToRealtimeChatScreen extends StatefulWidget {
  static const routeName = "/inviteToRealtimeChatSession";

  final RealtimeChatModel rtc;
  const InviteToRealtimeChatScreen(this.rtc, {super.key});

  @override
  State<InviteToRealtimeChatScreen> createState() =>
      _InviteToRealtimeChatScreenState();
}

class _InviteToRealtimeChatScreenState
    extends State<InviteToRealtimeChatScreen> {
  final UserSelectionModel userSelModel =
      UserSelectionModel(allowMultiple: true);
  bool inviting = false;

  @override
  void initState() {
    super.initState();
  }

  void invite() async {
    String sessionRV;
    var routeArgs = ModalRoute.of(context)!.settings.arguments;
    if (routeArgs is RTDTSessionModel) {
      sessionRV = routeArgs.sessionRV;
    } else {
      return;
    }

    setState(() => inviting = true);
    try {
      for (var cm in userSelModel.selected) {
        await widget.rtc.inviteToSession(sessionRV, cm.id);
      }
      showSuccessSnackbar(this, "Invited users to realtime chat session");
      if (mounted) {
        Navigator.of(context).pop();
      }
    } catch (exception) {
      showErrorSnackbar(this, "Unable to invite users to session: $exception");
    } finally {
      setState(() => inviting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    ClientModel client = ClientModel.of(context, listen: false);

    return Scaffold(
        body: Container(
      padding: const EdgeInsets.all(10),
      child: Column(children: [
        const Txt.H("Invite to Realtime Chat Session"),
        const SizedBox(height: 20),
        Expanded(
            child: UserSearchPanel(
          client,
          userSelModel: userSelModel,
          showButtonsRow: false,
        )),
        const SizedBox(height: 10),
        Container(
            padding: const EdgeInsets.all(10),
            height: 60,
            width: 500,
            child:
                SingleChildScrollView(child: SelectedUsersPanel(userSelModel))),
        const SizedBox(height: 10),
        Row(mainAxisAlignment: MainAxisAlignment.spaceEvenly, children: [
          CancelButton(onPressed: () {
            Navigator.of(context).pop();
          }),
          ElevatedButton(
              onPressed: inviting ? null : invite, child: const Text("Invite")),
        ]),
      ]),
    ));
  }
}
