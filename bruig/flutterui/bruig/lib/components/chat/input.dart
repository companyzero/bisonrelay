import 'dart:math';

import 'package:bruig/components/attach_file.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/emoji.dart';
import 'package:bruig/components/icons.dart';
import 'package:bruig/components/chat/record_audio.dart';
import 'package:bruig/models/audio.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/chats.dart';
import 'package:emoji_picker_flutter/emoji_picker_flutter.dart';
import 'package:flutter/material.dart';
import 'package:bruig/components/chat/types.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/theme_manager.dart';
import 'package:flutter/services.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';
import 'package:super_clipboard/super_clipboard.dart';

final _crToLfRegexp = RegExp(r'\r\n|\r');

class ChatInput extends StatefulWidget {
  final SendMsg _send;
  final ChatModel chat;
  final CustomInputFocusNode inputFocusNode;
  final bool allowAudio;
  const ChatInput(this._send, this.chat, this.inputFocusNode,
      {this.allowAudio = true, super.key});

  @override
  State<ChatInput> createState() => _ChatInputState();
}

class _ChatInputState extends State<ChatInput> {
  final controller = TextEditingController();

  late AudioModel audio;
  List<AttachmentEmbed> embeds = [];
  bool isAttaching = false;
  bool isRecordingAudio = false;
  Uint8List? initialAttachData;
  String? initialAttachMime;
  bool wasEmptyText = true;

  void replaceTextSelection(String s) {
    s = s.replaceAll(_crToLfRegexp, '\n'); // Switch CRLF to LF.
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

  void controllerUpdated() {
    bool changedEmpty = (wasEmptyText && controller.text != "") ||
        (!wasEmptyText && controller.text == "");
    if (changedEmpty) {
      setState(() {
        wasEmptyText = controller.text == "";
      });
    }
  }

  bool containsUnkxdMembers = false;

  void containsUnxkdChanged() async {
    setState(() {
      containsUnkxdMembers =
          widget.chat.unkxdMembers.value?.isNotEmpty ?? false;
    });
  }

  @override
  void initState() {
    super.initState();
    controller.addListener(() {
      // Snapshot last good caret while user is typing/moving it.
      widget.inputFocusNode.saveSelection();
    });
    widget.inputFocusNode.controller = controller;
    controller.text = widget.chat.workingMsg;
    widget.inputFocusNode.noModEnterKeyHandler = sendMsg;
    widget.inputFocusNode.pasteEventHandler = pasteEvent;
    widget.inputFocusNode.addEmojiHandler = addEmoji;
    widget.chat.unkxdMembers.addListener(containsUnxkdChanged);
    containsUnkxdMembers = widget.chat.unkxdMembers.value?.isNotEmpty ?? false;
    controller.addListener(controllerUpdated);
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    audio = Provider.of<AudioModel>(context);
  }

  @override
  void didUpdateWidget(ChatInput oldWidget) {
    super.didUpdateWidget(oldWidget);
    var workingMsg = widget.chat.workingMsg;
    if (workingMsg != controller.text) {
      cancelAttach(callSetState: false);
      controller.text = workingMsg;
      controller.selection = TextSelection(
          baseOffset: workingMsg.length, extentOffset: workingMsg.length);
      widget.inputFocusNode.inputFocusNode.requestFocus();
    }
    oldWidget.inputFocusNode.pasteEventHandler = null;
    widget.inputFocusNode.pasteEventHandler = pasteEvent;
    oldWidget.inputFocusNode.addEmojiHandler = null;
    widget.inputFocusNode.addEmojiHandler = addEmoji;
    if (oldWidget.chat != widget.chat) {
      oldWidget.chat.unkxdMembers.removeListener(containsUnxkdChanged);
      widget.chat.unkxdMembers.addListener(containsUnxkdChanged);
      containsUnkxdMembers =
          widget.chat.unkxdMembers.value?.isNotEmpty ?? false;
      cancelAttach(callSetState: false);
    }
  }

  @override
  void dispose() {
    widget.inputFocusNode.controller = null;
    widget.inputFocusNode.noModEnterKeyHandler = null;
    widget.inputFocusNode.pasteEventHandler = null;
    widget.inputFocusNode.addEmojiHandler = null;
    widget.chat.unkxdMembers.removeListener(containsUnxkdChanged);
    super.dispose();
  }

  void sendAttachment(String msg) {
    cancelAttach();
    widget._send(msg);
  }

  void sendMsg() {
    final messageWithoutNewLine = controller.text.trim();
    controller.value = const TextEditingValue(
        text: "", selection: TextSelection.collapsed(offset: 0));
    final String withEmbeds =
        embeds.fold(messageWithoutNewLine, (s, e) => e.replaceInString(s));
    if (withEmbeds.length > Golib.maxPayloadSize) {
      showErrorSnackbar(context,
          "Message is larger than maximum allowed (limit: ${Golib.maxPayloadSizeStr})");
      return;
    }
    if (withEmbeds != "") {
      widget._send(withEmbeds);
      widget.chat.workingMsg = "";
      setState(() {
        embeds = [];
      });
    }

    Provider.of<TypingEmojiSelModel>(context, listen: false).clearSelection();
  }

  void addEmoji(Emoji? e) {
    if (e != null) {
      // Insert emoji at current caret/selection; move caret after it.
      var sel = controller.selection;
      if (!sel.isValid) {
        // Fallback to last saved caret, or end of text.
        sel = widget.inputFocusNode.takeSavedSelection() ??
            TextSelection.collapsed(offset: controller.text.length);
      }

      final text = controller.text;
      final len = text.length;
      final start = sel.start.clamp(0, len);
      final end = sel.end.clamp(0, len);

      final before = text.substring(0, start);
      final after = text.substring(end);
      final newText = before + e.emoji + after;
      final newOff = before.length + e.emoji.length; // caret after emoji

      widget.chat.workingMsg = newText;
      controller.value = TextEditingValue(
        text: newText,
        selection: TextSelection.collapsed(offset: newOff),
        composing: TextRange.empty,
      );

      widget.inputFocusNode.inputFocusNode.requestFocus();
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        widget.inputFocusNode
            .saveSelection(); // so closing picker restores here
      });
      return;
    }

    widget.inputFocusNode.inputFocusNode.requestFocus();
    // Selected emoji from typing panel.
    final typingEmoji =
        Provider.of<TypingEmojiSelModel>(context, listen: false);
    final oldText = controller.text;
    final caret = controller.selection.start;
    final codeLen =
        typingEmoji.lastEmojiCode.length; // length of shortcut including ':'

    // Replace the shortcode; this returns the full updated text.
    final newText = typingEmoji.replaceTypedEmojiCode(controller);
    if (newText == "") return;

    // Emoji length = newText - (oldText - codeLen) in UTF-16 units.
    final emojiLen = newText.length - (oldText.length - codeLen);
    final newCaret = caret + (emojiLen - codeLen); // move caret after the emoji

    widget.chat.workingMsg = newText;
    controller.value = TextEditingValue(
      text: newText,
      selection: TextSelection.collapsed(offset: newCaret),
      composing: TextRange.empty,
    );
  }

  void attachFile() {
    setState(() {
      isAttaching = true;
    });
  }

  void cancelAttach({callSetState = true}) {
    void doCancel() {
      isAttaching = false;
      initialAttachData = null;
      initialAttachMime = null;
      widget.inputFocusNode.inputFocusNode.requestFocus();
    }

    if (callSetState) {
      setState(doCancel);
    } else {
      doCancel();
    }
  }

  void recordAudioNote() {
    setState(() => isRecordingAudio = true);
  }

  void cancelAudioNote() {
    setState(() => isRecordingAudio = false);
  }

  void _toggleEmojiPanel() {
    final emojiModel = TypingEmojiSelModel.of(context, listen: false);

    final wasOpen = emojiModel.showAddEmojiPanel.value;

    emojiModel.showAddEmojiPanel.value = !wasOpen;

    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;

      final saved = widget.inputFocusNode.takeSavedSelection();
      if (saved != null) {
        final len = controller.text.length;
        final clamped = TextSelection(
          baseOffset: saved.start.clamp(0, len),
          extentOffset: saved.end.clamp(0, len),
          affinity: saved.affinity,
          isDirectional: saved.isDirectional,
        );
        controller.value = controller.value.copyWith(
          selection: clamped,
          composing: TextRange.empty,
        );
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = checkIsScreenSmall(context);

    if (isAttaching) {
      return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        IconButton(
            padding: const EdgeInsets.all(0),
            iconSize: 25,
            onPressed: cancelAttach,
            icon: const Icon(Icons.keyboard_arrow_left_outlined)),
        AttachFileScreen(sendAttachment, initialAttachData, initialAttachMime,
            widget.chat, cancelAttach)
      ]);
    }

    var theme = Provider.of<ThemeNotifier>(context, listen: false);

    if (audio.recording || audio.hasRecord) {
      return Row(children: [
        Expanded(
            child:
                RecordAudioInputPanel(audio: audio, sendMsg: sendAttachment)),
        const RecordAudioInputButton(),
      ]);
    }

    return Row(children: [
      Expanded(
        child: TextField(
          onChanged: (value) {
            widget.chat.workingMsg = value;

            // Check if user is typing an emoji code (:foo:).
            TypingEmojiSelModel.of(context, listen: false)
                .maybeSelectEmojis(controller);
          },
          autofocus: isScreenSmall ? false : true,
          focusNode: widget.inputFocusNode.inputFocusNode,
          controller: controller,
          minLines: 1,
          maxLines: null,
          contextMenuBuilder:
              (BuildContext context, EditableTextState editableTextState) =>
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
            hintText: "Start a message",
            prefixIcon: IconButton(
              focusNode: FocusNode(canRequestFocus: false, skipTraversal: true),
              onPressed: _toggleEmojiPanel,
              icon: const Icon(Icons.emoji_emotions_outlined),
            ),
            suffixIcon: Row(
                mainAxisSize: MainAxisSize.min,
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  if (!isScreenSmall || controller.text == "")
                    IconButton(
                        onPressed: attachFile,
                        icon: const Icon(Icons.attach_file)),
                  if (containsUnkxdMembers &&
                      (!isScreenSmall || controller.text == ""))
                    const Tooltip(
                        message: "There are un-kx'd members in this GC.\n"
                            "These members won't receive messages from you until the KX "
                            "process completes.\nThis usually happens automatically, after "
                            "they come back online.",
                        child: ColoredIcon(Icons.warning_amber_outlined,
                            color: TextColor.error)),
                  IconButton(
                      padding: const EdgeInsets.all(0),
                      iconSize: 20,
                      onPressed: sendMsg,
                      icon: const Icon(Icons.send))
                ]),
          ),
        ),
      ),
      if ((!isScreenSmall || controller.text == "") && widget.allowAudio) ...[
        const SizedBox(width: 5),
        const RecordAudioInputButton(),
      ],
    ]);
  }
}
