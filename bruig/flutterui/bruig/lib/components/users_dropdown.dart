import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';
import 'package:dropdown_button2/dropdown_button2.dart';

typedef ChatModelCB = Function(ChatModel? c);

class UsersDropdown extends StatefulWidget {
  final ChatModelCB? cb;
  final bool allowEmpty;
  final List<String> excludeUIDs;
  final List<String>? limitUIDs;
  final String hintText;
  const UsersDropdown(
      {this.cb,
      super.key,
      this.allowEmpty = false,
      this.excludeUIDs = const [],
      this.limitUIDs,
      this.hintText = "Select user..."});

  @override
  State<UsersDropdown> createState() => _UsersDropdownState();
}

class _UsersDropdownState extends State<UsersDropdown> {
  ChatModel? selected;
  String? selectedValue;
  final TextEditingController textEditingController = TextEditingController();

  @override
  void dispose() {
    textEditingController.dispose();
    super.dispose();
  }

  final List<String> items = [
    'A_Item1',
    'A_Item2',
    'A_Item3',
    'A_Item4',
    'B_Item1',
    'B_Item2',
    'B_Item3',
    'B_Item4',
  ];
  @override
  Widget build(BuildContext context) {
    return Consumer2<ClientModel, ThemeNotifier>(
        builder: (context, client, theme, child) {
      List<ChatModel?> list =
          client.activeChats.sorted.cast<ChatModel?>().toList();
      list.addAll(client.hiddenChats.sorted.cast<ChatModel?>().toList());
      list.removeWhere((c) => c != null ? c.isGC : false);
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
      List<String> items = list.map((c) => c != null ? c.nick : "").toList();
      return DropdownButtonHideUnderline(
        child: DropdownButton2<String>(
          autofocus: true,
          isExpanded: true,
          isDense: false,
          hint: Txt(widget.hintText),
          items: items
              .map((c) => DropdownMenuItem(
                  value: c,
                  child: Container(
                      margin: const EdgeInsets.all(0),
                      width: double.infinity,
                      alignment: Alignment.centerLeft,
                      child: Text(c))))
              .toList(),
          selectedItemBuilder: (BuildContext context) => list
              .map(
                (ChatModel? c) => Center(
                    child: Txt.S(
                  c != null ? c.nick : "Share globally",
                )),
              )
              .toList(),
          value: selectedValue,
          onChanged: (value) {
            setState(() {
              selected = list.firstWhere((c) => c?.nick == value);
              widget.cb != null ? widget.cb!(selected) : null;
              selectedValue = value;
            });
          },
          buttonStyleData: ButtonStyleData(
            padding: const EdgeInsets.only(left: 14, right: 14),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(14),
              border: Border.all(
                color: theme.colors.outline,
              ),
            ),
            elevation: 2,
          ),
          iconStyleData: const IconStyleData(
            icon: Icon(Icons.keyboard_arrow_down_outlined),
            iconSize: 20,
          ),
          dropdownStyleData: const DropdownStyleData(maxHeight: 200),
        ),
      );
    });
  }
}
