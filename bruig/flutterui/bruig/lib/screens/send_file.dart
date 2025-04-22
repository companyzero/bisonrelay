import 'dart:io';

import 'package:bruig/components/attach_file.dart';
import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:bruig/util.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:mime/mime.dart';
import 'package:path/path.dart' as path;

class SendFileScreenResult {
  const SendFileScreenResult();
}

// Mime types which allow embedding vs sending with FT.
final _embedMimes = RegExp("(image|audio|video)/");

Future<SendFileScreenResult?> showSendFileScreen(
  BuildContext context, {
  required ChatModel chat,
  required File file,
}) async {
  if (chat.isGC) {
    var mimeType = lookupMimeType(file.path) ?? "binary/octet-stream";
    if (!_embedMimes.hasMatch(mimeType)) {
      showErrorSnackbar(context, "Cannot send file of type $mimeType to GC");
      return null;
    }
  }
  return await showDialog<SendFileScreenResult?>(
      useRootNavigator: true,
      context: context,
      builder: (context) {
        final Widget child = SendFileScreen(file, chat);
        return Dialog.fullscreen(child: child);
      });
}

class SendFileScreen extends StatefulWidget {
  final File file;
  final ChatModel chat;
  const SendFileScreen(this.file, this.chat, {super.key});

  @override
  State<SendFileScreen> createState() => _SendFileScreenState();
}

class _SendFileScreenState extends State<SendFileScreen> {
  File get file => widget.file;
  ChatModel get chat => widget.chat;
  String filename = "";
  int fileSize = 0;
  bool sending = false;

  void cancel() {
    Navigator.of(context).pop();
  }

  Future<void> sendAsEmbed(String mimeType) async {
    var id = generateRandomString(10);
    var embed = AttachmentEmbed(
      id,
      data: file.readAsBytesSync(),
      alt: "",
      mime: mimeType,
      filename: filename,
    );
    chat.sendMsg(embed.embedString());
  }

  Future<void> sendAsFileTransfer() async {
    var chatMsg =
        SynthChatEvent("Sending file \"$filename\" to user", SCE_sending);
    chat.append(ChatEventModel(chatMsg, null), false);

    try {
      await Golib.sendFile(chat.id, file.absolute.path);
      chatMsg.state = SCE_sent;
      showSuccessSnackbar(this, "Sent file \"$filename\" to ${chat.nick}");
    } catch (exception) {
      chatMsg.error = Exception(exception);
      showErrorSnackbar(this, "Unable to send file: $exception");
    }
  }

  void send() async {
    setState(() => sending = true);

    await sleep(Duration(milliseconds: 250)); // Wait UI to disable send button.

    // Check for files that can be embedded in a message as opposed to sent with
    // the FT subsystem. Prefer to embed those (e.g. images, audio, etc).
    var mimeType = lookupMimeType(file.path) ?? "binary/octet-stream";
    var size = await file.length();
    if (_embedMimes.hasMatch(mimeType) && size <= Golib.maxPayloadSize) {
      // File can be embedded.
      await sendAsEmbed(mimeType);
    } else if (chat.isGC) {
      // File transfers are not supported in GCs at the moment.
      showErrorSnackbar(
          this, "Cannot send file of type $mimeType and size $size to GC");
    } else {
      // File needs to be sent with filetransfer.
      await sendAsFileTransfer();
    }

    if (mounted) Navigator.pop(context);
  }

  @override
  void initState() {
    super.initState();
    filename = path.basename(file.path);
    (() async {
      var size = file.lengthSync();
      setState(() {
        fileSize = size;
      });
    })();
  }

  @override
  Widget build(BuildContext context) {
    return StartupScreen(hideAboutButton: true, [
      const Txt.H("Send File"),
      const SizedBox(height: 20),
      Txt.M("Filename: $filename"),
      const SizedBox(height: 5),
      Txt.M("Size: ${humanReadableSize(fileSize)}"),
      // const Expanded(child: Empty()),
      const SizedBox(height: 20),
      Wrap(spacing: 5, children: [
        ElevatedButton(onPressed: sending ? null : send, child: Text("Send")),
        CancelButton(onPressed: cancel),
      ]),
    ]);
  }
}
