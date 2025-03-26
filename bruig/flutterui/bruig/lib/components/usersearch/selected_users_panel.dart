import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/usersearch/user_search_model.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/cupertino.dart';

class SelectedUsersPanel extends StatefulWidget {
  final UserSelectionModel userSelModel;
  const SelectedUsersPanel(this.userSelModel, {super.key});

  @override
  State<SelectedUsersPanel> createState() => _SelectedUsersPanelState();
}

class _SelectedUsersPanelState extends State<SelectedUsersPanel> {
  UserSelectionModel get userSelModel => widget.userSelModel;

  void userSelectionChanged() {
    setState(() {});
  }

  @override
  void initState() {
    super.initState();
    userSelModel.addListener(userSelectionChanged);
  }

  @override
  void didUpdateWidget(SelectedUsersPanel oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.userSelModel.removeListener(userSelectionChanged);
    userSelModel.addListener(userSelectionChanged);
  }

  @override
  void dispose() {
    userSelModel.removeListener(userSelectionChanged);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var client = ClientModel.of(context, listen: false);
    return Wrap(
      spacing: 10,
      runSpacing: 10,
      children: userSelModel.selected
          .map((chat) => UserMenuAvatar(client, chat))
          .toList(),
    );
  }
}
