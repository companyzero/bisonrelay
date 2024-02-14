import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

class Input extends StatefulWidget {
  final ClientModel client;
  final FocusNode inputFocusNode;
  final bool createGroupChat;
  const Input(this.client, this.inputFocusNode, this.createGroupChat,
      {Key? key})
      : super(key: key);

  @override
  State<Input> createState() => _InputState();
}

class _InputState extends State<Input> {
  final controller = TextEditingController();
  ClientModel get client => widget.client;

  final FocusNode node = FocusNode();

  @override
  void initState() {
    setState(() {
      controller.text = client.filteredSearchString;
    });
    super.initState();
  }

  @override
  void dispose() {
    super.dispose();
  }

  @override
  void didUpdateWidget(Input oldWidget) {
    super.didUpdateWidget(oldWidget);
    widget.inputFocusNode.requestFocus();
  }

  void handleKeyPress(event) {
    if (event is RawKeyUpEvent) {
      bool modPressed = event.isShiftPressed || event.isControlPressed;
      final val = controller.value;
      client.filteredSearchString = val.text;
      if (event.data.logicalKey.keyLabel == "Enter" && !modPressed) {
        controller.value = const TextEditingValue(
            text: "", selection: TextSelection.collapsed(offset: 0));
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor; // MESSAGE TEXT COLOR
    var hoverColor = theme.hoverColor;
    var hintTextColor = theme.dividerColor;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => RawKeyboardListener(
              focusNode: node,
              onKey: handleKeyPress,
              child: Container(
                margin: const EdgeInsets.only(bottom: 5),
                child: Row(
                  children: [
                    Expanded(
                        child: TextField(
                      autofocus: false,
                      focusNode: widget.inputFocusNode,
                      style: TextStyle(
                        fontSize: theme.getMediumFont(context),
                        color: textColor,
                      ),
                      controller: controller,
                      minLines: 1,
                      maxLines: null,
                      //textInputAction: TextInputAction.done,
                      //style: normalTextStyle,
                      keyboardType: TextInputType.multiline,
                      decoration: InputDecoration(
                        hoverColor: hoverColor,
                        isDense: true,
                        hintText:
                            'Search name of user ${widget.createGroupChat ? "" : "or group chat"}',
                        hintStyle: TextStyle(
                          fontSize: theme.getMediumFont(context),
                          color: hintTextColor,
                        ),
                        errorBorder: const OutlineInputBorder(
                            borderRadius: BorderRadius.all(Radius.circular(30)),
                            borderSide:
                                BorderSide(color: Colors.red, width: 2)),
                        focusedBorder: OutlineInputBorder(
                            borderRadius:
                                const BorderRadius.all(Radius.circular(30)),
                            borderSide: BorderSide(color: textColor, width: 2)),
                        border: OutlineInputBorder(
                            borderRadius:
                                const BorderRadius.all(Radius.circular(30)),
                            borderSide: BorderSide(color: textColor, width: 1)),
                      ),
                    )),
                  ],
                ),
              ),
            ));
  }
}

typedef GroupChatNameInputCB = void Function(String);

class GroupChatNameInput extends StatefulWidget {
  final GroupChatNameInputCB updateGcName;
  final FocusNode inputFocusNode;
  final String gcName;
  const GroupChatNameInput(this.updateGcName, this.inputFocusNode, this.gcName,
      {Key? key})
      : super(key: key);

  @override
  State<GroupChatNameInput> createState() => _GroupChatNameInputState();
}

class _GroupChatNameInputState extends State<GroupChatNameInput> {
  final controller = TextEditingController();

  final FocusNode node = FocusNode();

  @override
  void initState() {
    setState(() {
      controller.text = widget.gcName;
      //client.filteredSearchString;
    });
    super.initState();
  }

  @override
  void dispose() {
    super.dispose();
  }

  @override
  void didUpdateWidget(GroupChatNameInput oldWidget) {
    super.didUpdateWidget(oldWidget);
    widget.inputFocusNode.requestFocus();
  }

  void handleKeyPress(event) {
    if (event is RawKeyUpEvent) {
      bool modPressed = event.isShiftPressed || event.isControlPressed;
      final val = controller.value;
      //client.filteredSearchString = val.text;
      if (event.data.logicalKey.keyLabel == "Enter" && !modPressed) {
        controller.value = const TextEditingValue(
            text: "", selection: TextSelection.collapsed(offset: 0));
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor; // MESSAGE TEXT COLOR
    var hoverColor = theme.hoverColor;
    var hintTextColor = theme.dividerColor;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => RawKeyboardListener(
              focusNode: node,
              onKey: handleKeyPress,
              child: Container(
                margin: const EdgeInsets.only(bottom: 5),
                child: Row(
                  children: [
                    Expanded(
                        child: TextField(
                      autofocus: true,
                      focusNode: widget.inputFocusNode,
                      style: TextStyle(
                        fontSize: theme.getMediumFont(context),
                        color: textColor,
                      ),
                      onChanged: widget.updateGcName,
                      controller: controller,
                      minLines: 1,
                      maxLines: null,
                      //textInputAction: TextInputAction.done,
                      //style: normalTextStyle,
                      keyboardType: TextInputType.multiline,
                      decoration: InputDecoration(
                        hoverColor: hoverColor,
                        isDense: true,
                        hintText: 'Group name (required)',
                        hintStyle: TextStyle(
                          fontSize: theme.getMediumFont(context),
                          color: hintTextColor,
                        ),
                        errorBorder: const OutlineInputBorder(
                            borderRadius: BorderRadius.all(Radius.circular(30)),
                            borderSide:
                                BorderSide(color: Colors.red, width: 2)),
                        focusedBorder: OutlineInputBorder(
                            borderRadius:
                                const BorderRadius.all(Radius.circular(30)),
                            borderSide: BorderSide(color: textColor, width: 2)),
                        border: OutlineInputBorder(
                            borderRadius:
                                const BorderRadius.all(Radius.circular(30)),
                            borderSide: BorderSide(color: textColor, width: 1)),
                      ),
                    )),
                  ],
                ),
              ),
            ));
  }
}
