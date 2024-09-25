import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/chats.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

class ChatSearchInput extends StatefulWidget {
  final CustomInputFocusNode inputFocusNode;
  final bool createGroupChat;
  final ValueChanged<String> onChanged;
  const ChatSearchInput(
      this.inputFocusNode, this.createGroupChat, this.onChanged,
      {super.key});

  @override
  State<ChatSearchInput> createState() => _ChatSearchInputState();
}

class _ChatSearchInputState extends State<ChatSearchInput> {
  final controller = TextEditingController();

  final FocusNode node = FocusNode();

  @override
  void initState() {
    super.initState();
  }

  @override
  void dispose() {
    super.dispose();
  }

  @override
  void didUpdateWidget(ChatSearchInput oldWidget) {
    super.didUpdateWidget(oldWidget);
    widget.inputFocusNode.inputFocusNode.requestFocus();
  }

  void handleKeyPress(KeyEvent event) {
    bool modPressed = HardwareKeyboard.instance.isShiftPressed ||
        HardwareKeyboard.instance.isControlPressed;
    widget.onChanged(controller.text);
    if (event.logicalKey.keyLabel == "Enter" && !modPressed) {
      controller.value = const TextEditingValue(
          text: "", selection: TextSelection.collapsed(offset: 0));
    }
  }

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = checkIsScreenSmall(context);
    return KeyboardListener(
        focusNode: node,
        onKeyEvent: handleKeyPress,
        child: Container(
            margin: const EdgeInsets.only(bottom: 5),
            child: Row(children: [
              Expanded(
                child: TextField(
                  autofocus: isScreenSmall ? false : true,
                  focusNode: widget.inputFocusNode.inputFocusNode,
                  controller: controller,
                  minLines: 1,
                  maxLines: null,
                  keyboardType: TextInputType.multiline,
                  decoration: InputDecoration(
                    isDense: true,
                    hintText:
                        'Search name of user ${widget.createGroupChat ? "" : "or group chat"}',
                    border: const OutlineInputBorder(
                        borderRadius: BorderRadius.all(Radius.circular(30)),
                        borderSide: BorderSide(width: 1)),
                  ),
                ),
              )
            ])));
  }
}

typedef GroupChatNameInputCB = void Function(String);

class GroupChatNameInput extends StatefulWidget {
  final GroupChatNameInputCB updateGcName;
  final CustomInputFocusNode inputFocusNode;
  final String gcName;
  const GroupChatNameInput(this.updateGcName, this.inputFocusNode, this.gcName,
      {super.key});

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
    });
    super.initState();
  }

  @override
  void dispose() {
    widget.inputFocusNode.noModEnterKeyHandler = null;
    super.dispose();
  }

  @override
  void didUpdateWidget(GroupChatNameInput oldWidget) {
    super.didUpdateWidget(oldWidget);
    widget.inputFocusNode.inputFocusNode.requestFocus();
  }

  void handleKeyPress(KeyEvent event) {
    bool modPressed = HardwareKeyboard.instance.isShiftPressed ||
        HardwareKeyboard.instance.isControlPressed;
    if (event.logicalKey.keyLabel == "Enter" && !modPressed) {
      controller.value = const TextEditingValue(
          text: "", selection: TextSelection.collapsed(offset: 0));
    }
  }

  @override
  Widget build(BuildContext context) {
    return KeyboardListener(
        focusNode: node,
        onKeyEvent: handleKeyPress,
        child: Row(children: [
          Expanded(
              child: TextField(
            autofocus: true,
            focusNode: widget.inputFocusNode.inputFocusNode,
            onChanged: widget.updateGcName,
            controller: controller,
            minLines: 1,
            maxLines: null,
            keyboardType: TextInputType.multiline,
            decoration: const InputDecoration(
              isDense: true,
              hintText: "Group name (required)",
              border: OutlineInputBorder(
                  borderRadius: BorderRadius.all(Radius.circular(30)),
                  borderSide: BorderSide(width: 1)),
            ),
          )),
        ]));
  }
}
