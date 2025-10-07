import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/inputs.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/components/usersearch/selected_users_panel.dart';
import 'package:bruig/components/usersearch/user_search_model.dart';
import 'package:bruig/components/usersearch/user_search_panel.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/realtimechat.dart';
import 'package:bruig/screens/overview.dart';
import 'package:bruig/screens/realtimechat/rtclist.dart';
import 'package:flutter/material.dart';

class CreateRealtimeChatScreenArgs {
  final bool isInstant;
  final ChatModel? initial;

  CreateRealtimeChatScreenArgs({this.isInstant = false, this.initial});
}

class CreateRealtimeChatScreen extends StatefulWidget {
  static const routeName = "/createRealtimeChatSession";

  final RealtimeChatModel rtc;
  const CreateRealtimeChatScreen(this.rtc, {super.key});

  @override
  State<CreateRealtimeChatScreen> createState() =>
      _CreateRealtimeChatScreenState();
}

class _CreateRealtimeChatScreenState extends State<CreateRealtimeChatScreen> {
  IntEditingController sizeCtrl = IntEditingController();
  TextEditingController descrCtrl = TextEditingController();
  bool creating = false;
  bool isInstant = false;
  final UserSelectionModel userSelModel =
      UserSelectionModel(allowMultiple: true);

  @override
  void initState() {
    super.initState();
    sizeCtrl.intvalue = 2;
    // sizeCtrl.text = "2";
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    var modalArgs = ModalRoute.of(context)?.settings.arguments;
    if (modalArgs is CreateRealtimeChatScreenArgs) {
      isInstant = modalArgs.isInstant;
      if (modalArgs.initial != null) {
        userSelModel.add(modalArgs.initial!);
      }
    }
  }

  void create() async {
    setState(() => creating = true);
    if (sizeCtrl.intvalue < 2 || sizeCtrl.intvalue > 1 << 16) {
      showErrorSnackbar(context, "Invalid session size");
      setState(() => creating = false);
      return;
    }

    try {
      List<String> toInvite = userSelModel.selected.map((c) => c.id).toList();
      if (isInstant) {
        await widget.rtc.createInstantSession(toInvite);
      } else {
        await widget.rtc
            .createSession(sizeCtrl.intvalue, descrCtrl.text, toInvite);
        showSuccessSnackbar(this, "Created realtime chat session!");
      }
      if (mounted) {
        Navigator.of(context).pop();
        if (isInstant) {
          Navigator.of(context).pushReplacementNamed(
              OverviewScreen.subRoute(RealtimeChatScreen.routeName));
        }
      }
    } catch (exception) {
      showErrorSnackbar(this, "Unable to create session: $exception");
    } finally {
      setState(() => creating = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    ClientModel client = ClientModel.of(context, listen: false);

    return Scaffold(
        body: Container(
      padding: const EdgeInsets.all(10),
      child: Column(children: [
        (!isInstant
            ? const Txt.H("Create Realtime Chat Session")
            : const Txt.H("Instant Realtime Call")),
        const SizedBox(height: 20),
        if (!isInstant) ...[
          SizedBox(
              width: 100,
              child: Row(children: [
                const Text("Size:"),
                const SizedBox(width: 10),
                Expanded(child: intInput(controller: sizeCtrl)),
              ])),
          SizedBox(
              width: 400,
              child: Row(children: [
                const Text("Description:"),
                const SizedBox(width: 10),
                Expanded(child: TextField(controller: descrCtrl)),
              ])),
          const SizedBox(height: 10),
        ],
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
              onPressed: !creating ? create : null,
              child: !isInstant ? const Text("Create") : const Text("Call")),
        ]),
      ]),
    ));
  }
}
