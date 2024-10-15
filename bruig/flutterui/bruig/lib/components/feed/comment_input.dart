import 'dart:math';

import 'package:bruig/components/attach_file.dart';
import 'package:bruig/models/emoji.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/chats.dart';
import 'package:emoji_picker_flutter/emoji_picker_flutter.dart';
import 'package:flutter/material.dart';
import 'package:bruig/components/chat/types.dart';
import 'package:bruig/theme_manager.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import 'package:super_clipboard/super_clipboard.dart';

class CommentInput extends StatefulWidget {
  final SendMsg commentReply;
  final String label;
  final String hintText;
  final CustomInputFocusNode inputFocusNode;
  const CommentInput(
      this.commentReply, this.label, this.hintText, this.inputFocusNode,
      {super.key});

  @override
  State<CommentInput> createState() => _CommentInputState();
}

class _CommentInputState extends State<CommentInput> {
  final controller = TextEditingController();

  List<AttachmentEmbed> embeds = [];
  bool isAttaching = false;
  Uint8List? initialAttachData;
  String? initialAttachMime;

  void replaceTextSelection(String s) {
    var sel = controller.selection.copyWith();
    if (controller.selection.start == -1 && controller.selection.end == -1) {
      controller.text = controller.text + s;
    } else if (sel.isCollapsed) {
      controller.text = controller.text.substring(0, sel.start) +
          s +
          controller.text.substring(min(controller.text.length, sel.start));
      var newPos = sel.baseOffset + s.length;
      controller.selection =
          sel.copyWith(baseOffset: newPos, extentOffset: newPos);
    } else {
      controller.text =
          controller.text.substring(0, controller.selection.start) +
              s +
              controller.text.substring(controller.selection.end);
      var newPos = sel.baseOffset + s.length;
      controller.selection =
          sel.copyWith(baseOffset: newPos, extentOffset: newPos);
    }
  }

  Future<void> pasteEvent() async {
    final clip = SystemClipboard.instance;
    if (clip == null) {
      // Clipboard API is not supported on this platform. Use the standard.
      replaceTextSelection(Clipboard.kTextPlain);
      return;
    }
    final reader = await clip.read();

    /// Binary formats need to be read as streams
    if (reader.canProvide(Formats.png)) {
      reader.getFile(Formats.png, (file) async {
        final stream = await file.readAll();
        setState(() {
          initialAttachData = stream;
          initialAttachMime = "image/png";
          isAttaching = true;
        });
      });
      return;
    }

    // Automatically convert to markdown?
    // if (reader.canProvide(Formats.htmlText)) {
    //   final html = await reader.readValue(Formats.htmlText);
    //   print("XXXX clip is html $html");
    // }

    if (reader.canProvide(Formats.plainText)) {
      final text = await reader.readValue(Formats.plainText);
      replaceTextSelection(text ?? "");
      return;
    }
  }

  @override
  void initState() {
    super.initState();
    widget.inputFocusNode.noModEnterKeyHandler = sendMsg;
    widget.inputFocusNode.pasteEventHandler = pasteEvent;
    widget.inputFocusNode.addEmojiHandler = addEmoji;
  }

  @override
  void didUpdateWidget(CommentInput oldWidget) {
    super.didUpdateWidget(oldWidget);
    var workingMsg = controller.text;
    if (workingMsg != controller.text) {
      isAttaching = false;
      controller.text = workingMsg;
      controller.selection = TextSelection(
          baseOffset: workingMsg.length, extentOffset: workingMsg.length);
      widget.inputFocusNode.inputFocusNode.requestFocus();
    }
    oldWidget.inputFocusNode.pasteEventHandler = null;
    widget.inputFocusNode.pasteEventHandler = pasteEvent;
    oldWidget.inputFocusNode.addEmojiHandler = null;
    widget.inputFocusNode.addEmojiHandler = addEmoji;
  }

  @override
  void dispose() {
    widget.inputFocusNode.noModEnterKeyHandler = null;
    widget.inputFocusNode.pasteEventHandler = null;
    widget.inputFocusNode.addEmojiHandler = null;
    super.dispose();
  }

  void sendAttachment(String msg) {
    setState(() {
      isAttaching = false;
      initialAttachData = null;
      initialAttachMime = null;
    });
    widget.commentReply(msg);
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
      widget.commentReply(withEmbeds);
      setState(() {
        embeds = [];
      });
    }

    Provider.of<TypingEmojiSelModel>(context, listen: false).clearSelection();
  }

  void addEmoji(Emoji? e) {
    if (e != null) {
      // Selected emoji from panel eidget.
      var oldPos = controller.selection.start;
      var newText = controller.selection.textBefore(controller.text) +
          e.emoji +
          controller.selection.textAfter(controller.text);
      controller.value = TextEditingValue(
          text: newText,
          selection: TextSelection.collapsed(offset: oldPos + e.emoji.length));
      return;
    }

    // Selected emoji from typing panel.
    var typingEmoji = Provider.of<TypingEmojiSelModel>(context, listen: false);
    var newText = typingEmoji.replaceTypedEmojiCode(controller);
    if (newText == "") return;

    var oldPos =
        controller.selection.start - typingEmoji.lastEmojiCode.length + 1;
    controller.value = TextEditingValue(
        text: newText, selection: TextSelection.collapsed(offset: oldPos));
  }

  void attachFile() {
    setState(() {
      isAttaching = true;
    });
  }

  void cancelAttach() {
    setState(() {
      isAttaching = false;
      initialAttachData = null;
      initialAttachMime = null;
      widget.inputFocusNode.inputFocusNode.requestFocus();
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
                AttachFileScreen(
                    sendAttachment, initialAttachData, initialAttachMime)
              ])
            : Row(children: [
                IconButton(
                    padding: const EdgeInsets.all(0),
                    iconSize: 25,
                    onPressed: attachFile,
                    icon: const Icon(Icons.add_outlined)),
                const SizedBox(width: 5),
                IconButton(
                    padding: const EdgeInsets.all(0),
                    iconSize: 25,
                    onPressed: () {
                      var emojiModel =
                          TypingEmojiSelModel.of(context, listen: false);
                      emojiModel.showAddEmojiPanel.value =
                          !emojiModel.showAddEmojiPanel.value;
                    },
                    icon: const Icon(Icons.emoji_emotions_outlined)),
                const SizedBox(width: 5),
                Expanded(
                    child: TextField(
                  onChanged: (value) {
                    // Check if user is typing an emoji code (:foo:).
                    TypingEmojiSelModel.of(context, listen: false)
                        .maybeSelectEmojis(controller);
                  },
                  autofocus: isScreenSmall ? false : true,
                  focusNode: widget.inputFocusNode.inputFocusNode,
                  controller: controller,
                  minLines: 1,
                  maxLines: null,
                  contextMenuBuilder: (BuildContext context,
                          EditableTextState editableTextState) =>
                      AdaptiveTextSelectionToolbar.editable(
                    anchors: editableTextState.contextMenuAnchors,
                    clipboardStatus: ClipboardStatus.pasteable,
                    onCopy: null,
                    onCut: null,
                    onLiveTextInput: null,
                    onLookUp: null,
                    onSearchWeb: null,
                    onSelectAll: null,
                    onShare: null,
                    onPaste: pasteEvent,
                  ),
                  style: theme.textStyleFor(context, TextSize.medium, null),
                  keyboardType: TextInputType.multiline,
                  decoration: InputDecoration(
                    isDense: true,
                    border: const OutlineInputBorder(
                      borderRadius: BorderRadius.all(Radius.circular(30.0)),
                      borderSide: BorderSide(width: 2.0),
                    ),
                    hintText: widget.hintText,
                    suffixIcon: Row(
                        mainAxisSize: MainAxisSize.min,
                        mainAxisAlignment: MainAxisAlignment.end,
                        children: [
                          IconButton(
                              tooltip: widget.label,
                              padding: const EdgeInsets.all(0),
                              iconSize: 20,
                              onPressed: sendMsg,
                              icon: const Icon(Icons.send))
                        ]),
                  ),
                )),
              ]));
  }
}
