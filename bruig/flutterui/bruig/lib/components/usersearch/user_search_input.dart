import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/chats.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

class ChatSearchInput extends StatefulWidget {
  final CustomInputFocusNode inputFocusNode;
  final bool onlyUsers;
  final ValueChanged<String> onChanged;
  final String? hintText;
  const ChatSearchInput(this.inputFocusNode, this.onlyUsers, this.onChanged,
      {this.hintText, super.key});

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
    if (oldWidget.onlyUsers != widget.onlyUsers ||
        oldWidget.hintText != widget.hintText) {
      controller.text = "";
    }
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
                    hintText: widget.hintText ??
                        'Search name of user ${widget.onlyUsers ? "" : "or group chat"}',
                    border: const OutlineInputBorder(
                        borderRadius: BorderRadius.all(Radius.circular(30)),
                        borderSide: BorderSide(width: 1)),
                  ),
                ),
              )
            ])));
  }
}
