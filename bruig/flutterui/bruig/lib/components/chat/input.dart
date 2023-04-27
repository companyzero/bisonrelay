import 'package:bruig/components/attach_file.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:bruig/components/chat/types.dart';
import 'package:bruig/models/client.dart';

class Input extends StatefulWidget {
  final SendMsg _send;
  final ChatModel chat;
  final FocusNode inputFocusNode;
  const Input(this._send, this.chat, this.inputFocusNode, {Key? key})
      : super(key: key);

  @override
  State<Input> createState() => _InputState();
}

class _InputState extends State<Input> {
  final controller = TextEditingController();

  final FocusNode node = FocusNode();
  List<AttachmentEmbed> embeds = [];

  @override
  void initState() {
    super.initState();
    controller.text = widget.chat.workingMsg;
  }

  @override
  void didUpdateWidget(Input oldWidget) {
    super.didUpdateWidget(oldWidget);
    var workingMsg = widget.chat.workingMsg;
    if (workingMsg != controller.text) {
      controller.text = workingMsg;
      controller.selection = TextSelection(
          baseOffset: workingMsg.length, extentOffset: workingMsg.length);
      widget.inputFocusNode.requestFocus();
    }
  }

  void handleKeyPress(event) {
    if (event is RawKeyUpEvent) {
      bool modPressed = event.isShiftPressed || event.isControlPressed;
      final val = controller.value;
      if (event.data.logicalKey.keyLabel == "Enter" && !modPressed) {
        final messageWithoutNewLine =
            controller.text.substring(0, val.selection.start - 1) +
                controller.text.substring(val.selection.start);
        controller.value = const TextEditingValue(
            text: "", selection: TextSelection.collapsed(offset: 0));
        final String withEmbeds = embeds.fold(
            messageWithoutNewLine.trim(), (s, e) => e.replaceInString(s));
        /*
          if (withEmbeds.length > 1024 * 1024) {
            showErrorSnackbar(context,
                "Message is larger than maximum allowed (limit: 1MiB)");
            return;
          }
          */
        if (withEmbeds != "") {
          widget._send(withEmbeds);
          widget.chat.workingMsg = "";
          setState(() {
            embeds = [];
          });
        }
      } else {
        widget.chat.workingMsg = val.text.trim();
      }
    }
  }

  void attachFile() async {
    var res = await Navigator.of(context, rootNavigator: true)
        .pushNamed(AttachFileScreen.routeName);
    if (res == null) {
      return;
    }
    var embed = res as AttachmentEmbed;
    embeds.add(embed);
    setState(() {
      controller.text = controller.text + embed.displayString() + " ";
      widget.chat.workingMsg = controller.text;
      widget.inputFocusNode.requestFocus();
    });
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor; // MESSAGE TEXT COLOR
    var hoverColor = theme.hoverColor;
    var backgroundColor = theme.highlightColor;
    var hintTextColor = theme.dividerColor;
    return RawKeyboardListener(
      focusNode: node,
      onKey: handleKeyPress,
      child: Container(
        margin: const EdgeInsets.only(bottom: 5),
        child: Row(
          children: [
            IconButton(onPressed: attachFile, icon: Icon(Icons.attach_file)),
            Expanded(
                child: TextField(
              autofocus: true,
              focusNode: widget.inputFocusNode,
              style: TextStyle(
                fontSize: 11,
                color: textColor,
              ),
              controller: controller,
              minLines: 1,
              maxLines: null,
              //textInputAction: TextInputAction.done,
              //style: normalTextStyle,
              keyboardType: TextInputType.multiline,
              decoration: InputDecoration(
                filled: true,
                fillColor: backgroundColor,
                hoverColor: hoverColor,
                isDense: true,
                hintText: 'Type a message',
                hintStyle: TextStyle(
                  fontSize: 11,
                  color: hintTextColor,
                ),
                border: InputBorder.none,
              ),
            )),
          ],
        ),
      ),
    );
  }
}
