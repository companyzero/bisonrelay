import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

typedef ChatModelCB = Function(ChatModel? c);

class UsersDropdown extends StatefulWidget {
  final ChatModelCB? cb;
  final bool allowEmpty;
  final List<String> excludeUIDs;
  final List<String>? limitUIDs;
  const UsersDropdown(
      {this.cb,
      Key? key,
      this.allowEmpty = false,
      this.excludeUIDs = const [],
      this.limitUIDs})
      : super(key: key);

  @override
  State<UsersDropdown> createState() => _UsersDropdownState();
}

class _UsersDropdownState extends State<UsersDropdown> {
  ChatModel? selected;

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var backgroundColor = theme.backgroundColor;
    return Consumer2<ClientModel, ThemeNotifier>(
        builder: (context, client, theme, child) {
      List<ChatModel?> list = client.userChats.cast<ChatModel?>().toList();
      list.addAll(client.hiddenUsers.cast<ChatModel?>().toList());
      if (widget.limitUIDs != null) {
        list.removeWhere((c) => !(widget.limitUIDs!.contains(c?.id)));
      }
      list.sort(
          (a, b) => a!.nick.toLowerCase().compareTo(b!.nick.toLowerCase()));
      if (widget.allowEmpty) {
        list.insert(0, null);
      }
      // Only use chats that aren't in the exclude UID list
      if (widget.excludeUIDs.isNotEmpty && list.isNotEmpty) {
        list = list.where((e) => !widget.excludeUIDs.contains(e!.id)).toList();
      }
      return DropdownButton<ChatModel?>(
        focusColor: Colors.red,
        isDense: true,
        isExpanded: true,
        icon: Icon(
          Icons.arrow_downward,
          color: textColor,
        ),
        dropdownColor: backgroundColor,
        underline: Container(),
        value: selected,
        items: (list.map<DropdownMenuItem<ChatModel?>>(
            (ChatModel? c) => DropdownMenuItem(
                value: c,
                child: Container(
                    margin: const EdgeInsets.all(0),
                    width: double.infinity,
                    alignment: Alignment.centerLeft,
                    child: Text(c != null ? c.nick : "Share globally",
                        style: TextStyle(
                          color: textColor,
                          fontSize: theme.getSmallFont(context),
                        )))))).toList(),
        selectedItemBuilder: (BuildContext context) => (list.map(
          (ChatModel? c) => Text(
            c != null ? c.nick : "Share globally",
            style: TextStyle(
                fontSize: theme.getSmallFont(context),
                color: textColor,
                fontStyle: FontStyle.italic,
                fontWeight: FontWeight.bold),
          ),
        )).toList(),
        onChanged: (ChatModel? newValue) => setState(() {
          selected = newValue;
          widget.cb != null ? widget.cb!(newValue) : null;
        }),
      );
    });
  }
}
