import 'package:bruig/components/usersearch/user_search_panel.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/widgets.dart';

class NewMessageScreen extends StatefulWidget {
  static const routeName = "/chat/newMessage";

  final ClientModel client;
  const NewMessageScreen(this.client, {super.key});

  @override
  State<NewMessageScreen> createState() => _NewMessageScreenState();
}

class _NewMessageScreenState extends State<NewMessageScreen> {
  ClientModel get client => widget.client;

  void goBack() {
    Navigator.of(context).pop();
  }

  void chatTapped(ChatModel chat) {
    client.makeTopActive(chat);
    Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    return Container(
        padding: const EdgeInsets.all(10),
        child: UserSearchPanel(
          client,
          confirmLabel: "",
          targets: UserSearchPanelTargets.usersAndGCs,
          onCancel: goBack,
          onChatTapped: chatTapped,
        ));
  }
}
