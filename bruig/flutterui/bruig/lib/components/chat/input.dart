import 'package:bruig/components/attach_file.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:bruig/components/chat/types.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

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

  void handleKeyPress(event) {
    if (event is RawKeyUpEvent) {
      bool modPressed = event.isShiftPressed || event.isControlPressed;
      final val = controller.value;
      if (event.data.logicalKey.keyLabel == "Enter" && !modPressed) {
        sendMsg();
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
      controller.text = "${controller.text}${embed.displayString()} ";
      widget.chat.workingMsg = controller.text;
      widget.inputFocusNode.requestFocus();
    });
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    var cardColor = theme.cardColor;
    var secondaryTextColor = theme.focusColor;
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => RawKeyboardListener(
              focusNode: node,
              onKey: handleKeyPress,
              child: Container(
                  margin: const EdgeInsets.only(bottom: 5),
                  child: TextField(
                    autofocus: isScreenSmall ? false : true,
                    focusNode: widget.inputFocusNode,
                    style: TextStyle(
                      fontSize: theme.getMediumFont(context),
                      color: secondaryTextColor,
                    ),
                    controller: controller,
                    minLines: 1,
                    maxLines: null,
                    //textInputAction: TextInputAction.done,
                    //style: normalTextStyle,
                    keyboardType: TextInputType.multiline,
                    decoration: InputDecoration(
                      errorBorder: const OutlineInputBorder(
                        borderRadius: BorderRadius.all(Radius.circular(30.0)),
                        borderSide: BorderSide(color: Colors.red, width: 2.0),
                      ),
                      focusedBorder: OutlineInputBorder(
                        borderRadius:
                            const BorderRadius.all(Radius.circular(30.0)),
                        borderSide:
                            BorderSide(color: secondaryTextColor, width: 2.0),
                      ),
                      border: OutlineInputBorder(
                        borderRadius:
                            const BorderRadius.all(Radius.circular(30.0)),
                        borderSide: BorderSide(color: cardColor, width: 2.0),
                      ),
                      hintText: "Start a message",
                      hintStyle: TextStyle(
                          fontSize: theme.getMediumFont(context),
                          letterSpacing: 0.5,
                          fontWeight: FontWeight.w300,
                          color: textColor),
                      filled: true,
                      fillColor: cardColor,
                      prefixIcon: IconButton(
                          padding: const EdgeInsets.all(0),
                          iconSize: 25,
                          onPressed: attachFile,
                          icon: const Icon(Icons.attach_file)),
                      prefixIconColor: textColor,
                      suffixIcon: IconButton(
                          padding: const EdgeInsets.all(0),
                          iconSize: 25,
                          onPressed: sendMsg,
                          icon: const Icon(Icons.send)),
                      suffixIconColor: textColor,
                    ),
                  )),
            ));
  }
}
