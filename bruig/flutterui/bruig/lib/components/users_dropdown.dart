import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

typedef ChatModelCB = Function(ChatModel? c);

class UsersDropdown extends StatefulWidget {
  final ChatModelCB? cb;
  final bool allowEmpty;
  const UsersDropdown({this.cb, Key? key, this.allowEmpty = false})
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
    return Consumer<ClientModel>(builder: (context, client, child) {
      List<ChatModel?> list = client.userChats.cast<ChatModel?>().toList();
      if (widget.allowEmpty) {
        list.insert(0, null);
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
                          fontSize: 11,
                        )))))).toList(),
        selectedItemBuilder: (BuildContext context) => (list.map(
          (ChatModel? c) => Text(
            c != null ? c.nick : "Share globally",
            style: TextStyle(
                fontSize: 11,
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
