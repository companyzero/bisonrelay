import 'package:bruig/components/attach_file.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/chats.dart';
import 'package:flutter/material.dart';
import 'package:bruig/components/chat/types.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

class ChatInput extends StatefulWidget {
  final SendMsg _send;
  final ChatModel chat;
  final CustomInputFocusNode inputFocusNode;
  const ChatInput(this._send, this.chat, this.inputFocusNode, {Key? key})
      : super(key: key);

  @override
  State<ChatInput> createState() => _ChatInputState();
}

class _ChatInputState extends State<ChatInput> {
  final controller = TextEditingController();

  final FocusNode node = FocusNode();
  List<AttachmentEmbed> embeds = [];
  bool isAttaching = false;

  @override
  void initState() {
    super.initState();
    controller.text = widget.chat.workingMsg;
    widget.inputFocusNode.noModEnterKeyHandler = sendMsg;
  }

  @override
  void didUpdateWidget(ChatInput oldWidget) {
    super.didUpdateWidget(oldWidget);
    var workingMsg = widget.chat.workingMsg;
    if (workingMsg != controller.text) {
      isAttaching = false;
      controller.text = workingMsg;
      controller.selection = TextSelection(
          baseOffset: workingMsg.length, extentOffset: workingMsg.length);
      widget.inputFocusNode.inputFocusNode.requestFocus();
    }
  }

  @override
  void dispose() {
    widget.inputFocusNode.noModEnterKeyHandler = null;
    super.dispose();
  }

  void sendMsg() {
    final messageWithoutNewLine = controller.text.trim();
    controller.value = const TextEditingValue(
        text: "", selection: TextSelection.collapsed(offset: 0));
    final String withEmbeds =
        embeds.fold(messageWithoutNewLine, (s, e) => e.replaceInString(s));
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
  }

  void attachFile() {
    setState(() {
      isAttaching = true;
    });
  }

  void cancelAttach() {
    setState(() {
      isAttaching = false;
    });
  }

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = checkIsScreenSmall(context);
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => isAttaching
            ? Column(children: [
                Row(mainAxisAlignment: MainAxisAlignment.start, children: [
                  IconButton(
                      padding: const EdgeInsets.all(0),
                      iconSize: 25,
                      onPressed: cancelAttach,
                      icon: const Icon(Icons.keyboard_arrow_left_outlined))
                ]),
                AttachFileScreen(widget._send)
              ])
            : Row(children: [
                IconButton(
                    padding: const EdgeInsets.all(0),
                    iconSize: 25,
                    onPressed: attachFile,
                    icon: const Icon(Icons.add_outlined)),
                const SizedBox(width: 5),
                Expanded(
                    child: TextField(
                  onChanged: (value) {
                    widget.chat.workingMsg = value;
                  },
                  autofocus: isScreenSmall ? false : true,
                  focusNode: widget.inputFocusNode.inputFocusNode,
                  controller: controller,
                  minLines: 1,
                  maxLines: null,
                  style: theme.textStyleFor(context, TextSize.medium, null),
                  keyboardType: TextInputType.multiline,
                  decoration: InputDecoration(
                    isDense: true,
                    border: const OutlineInputBorder(
                      borderRadius: BorderRadius.all(Radius.circular(30.0)),
                      borderSide: BorderSide(width: 2.0),
                    ),
                    hintText: "Start a message",
                    suffixIcon: IconButton(
                        padding: const EdgeInsets.all(0),
                        iconSize: 20,
                        onPressed: sendMsg,
                        icon: const Icon(Icons.send)),
                  ),
                )),
              ]));
  }
}
