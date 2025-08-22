import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';

typedef ChatModelCB = Function(ChatModel? c);

class UsersDropdown extends StatefulWidget {
  final ChatModelCB? cb;
  final List<String> excludeUIDs;
  final String hintText;
  final ClientModel client;
  final FocusNode? focusNode;
  const UsersDropdown(
      {this.cb,
      required this.client,
      super.key,
      this.focusNode,
      this.excludeUIDs = const [],
      this.hintText = "Select user..."});

  @override
  State<UsersDropdown> createState() => _UsersDropdownState();
}

class _UsersDropdownState extends State<UsersDropdown> {
  ClientModel get client => widget.client;
  ChatModel? selected;
  String? selectedValue;

  final TextEditingController txtCtrl = TextEditingController();

  List<DropdownMenuEntry<ChatModel>> entries = [];

  List<DropdownMenuEntry<ChatModel>> recalcList() {
    var list = client.hiddenChats.sorted + client.activeChats.sorted;
    list.removeWhere((c) {
      if (c.isGC) return true;
      if (widget.excludeUIDs.isNotEmpty) {
        return widget.excludeUIDs.contains(c.id);
      }
      return false;
    });

    list.sort((a, b) => a.nick.toLowerCase().compareTo(b.nick.toLowerCase()));
    return list
        .map((c) => DropdownMenuEntry(
            value: c, label: c.nick, leadingIcon: ChatAvatar(c)))
        .toList();
  }

  @override
  void initState() {
    super.initState();
    entries = recalcList();
    print("calculated list");
  }

  @override
  void didUpdateWidget(covariant UsersDropdown oldWidget) {
    super.didUpdateWidget(oldWidget);
    entries = recalcList();
  }

  @override
  void dispose() {
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return DropdownMenu<ChatModel>(
      dropdownMenuEntries: entries,
      controller: txtCtrl,
      focusNode: widget.focusNode,
      onSelected: (value) {
        if (widget.cb != null) widget.cb!(value);
      },
    );
  }
}
