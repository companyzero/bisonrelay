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
      Key? key,
      this.allowEmpty = false,
      this.excludeUIDs = const [],
      this.limitUIDs,
      this.hintText = "Select user..."})
      : super(key: key);

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
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var backgroundColor = theme.backgroundColor;
    var highlightColor = theme.highlightColor;
    var dividerColor = theme.dividerColor;
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
          hint: Text(
            widget.hintText,
            style: TextStyle(
              fontSize: theme.getMediumFont(context),
              color: textColor,
            ),
          ),
          items: items
              .map((c) => DropdownMenuItem(
                  value: c,
                  child: Container(
                      margin: const EdgeInsets.all(0),
                      width: double.infinity,
                      alignment: Alignment.centerLeft,
                      child: Text(c,
                          style: TextStyle(
                            color: textColor,
                            fontSize: theme.getMediumFont(context),
                          )))))
              .toList(),
          selectedItemBuilder: (BuildContext context) => list
              .map(
                (ChatModel? c) => Center(
                    child: Text(
                  c != null ? c.nick : "Share globally",
                  style: TextStyle(
                      fontSize: theme.getSmallFont(context),
                      color: textColor,
                      fontStyle: FontStyle.italic,
                      fontWeight: FontWeight.bold),
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
            //height: 50,
            //width: 250,
            padding: const EdgeInsets.only(left: 14, right: 14),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(14),
              border: Border.all(
                color: dividerColor,
              ),
              color: highlightColor,
            ),
            elevation: 2,
          ),
          iconStyleData: IconStyleData(
            icon: const Icon(Icons.keyboard_arrow_down_outlined),
            iconSize: 20,
            iconEnabledColor: textColor,
            iconDisabledColor: Colors.grey,
          ),
          dropdownStyleData: const DropdownStyleData(maxHeight: 200),
          menuItemStyleData: MenuItemStyleData(
            overlayColor: MaterialStateProperty.resolveWith((states) {
              if (states.contains(MaterialState.hovered)) {
                return backgroundColor;
              }
              return highlightColor;
            }),
            height: 40,
          ),
        ),
      );
    });
  }
/*
  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var backgroundColor = theme.backgroundColor;
    return Consumer2<ClientModel, ThemeNotifier>(
        builder: (context, client, theme, child) {
      List<ChatModel?> list = client.sortedChats.cast<ChatModel?>().toList();
      list.addAll(client.hiddenChats.cast<ChatModel?>().toList());
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
      return DropdownButton<ChatModel?>(
        focusColor: backgroundColor,
        isDense: true,
        //isExpanded: true,
        icon: Icon(
          Icons.arrow_downward,
          color: textColor,
        ),
        //dropdownColor: backgroundColor,
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
        selectedItemBuilder: (BuildContext context) => list
            .map(
              (ChatModel? c) => Center(
                  child: Text(
                c != null ? c.nick : "Share globally",
                style: TextStyle(
                    fontSize: theme.getSmallFont(context),
                    color: textColor,
                    fontStyle: FontStyle.italic,
                    fontWeight: FontWeight.bold),
              )),
            )
            .toList(),
        onChanged: (ChatModel? newValue) => setState(() {
          selected = newValue;
          widget.cb != null ? widget.cb!(newValue) : null;
        }),
      );
    });
  }
}
*/
}
