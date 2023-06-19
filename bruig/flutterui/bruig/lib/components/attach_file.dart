import 'dart:convert';
import 'dart:io';
import 'dart:math';
import 'dart:async';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/local_content_dropdown.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:golib_plugin/definitions.dart';

class AttachmentEmbed {
  String? mime;
  Uint8List? data;
  SharedFileAndShares? linkedFile;
  String? alt;
  String id;
  AttachmentEmbed(this.id, {this.data, this.linkedFile, this.alt, this.mime});

  String displayString() {
    return "--embed[id=$id]--";
  }

  String embedString() {
    List<String> parts = [];
    if ((alt ?? "") != "") {
      parts.add("alt=${Uri.encodeComponent(alt!)}");
    }
    if (linkedFile != null) {
      parts.add("download=${linkedFile!.sf.fid}");
      parts.add("filename=${linkedFile!.sf.filename}");
      parts.add("cost=${linkedFile!.cost}");
      parts.add("size=${linkedFile!.size}");
    }
    if ((mime ?? "") != "") {
      parts.add("type=$mime");
    }
    if (data != null) {
      var b64Data = const Base64Encoder().convert(data!);
      parts.add("data=$b64Data");
    }
    var allParts = parts.join(",");
    return "--embed[$allParts]--";
  }

  String replaceInString(String s) {
    final pattern = RegExp("--embed\\[id=$id\\]--");
    return s.replaceAllMapped(pattern, (match) {
      return embedString();
    });
  }
}

class AttachFileScreen extends StatefulWidget {
  static String routeName = "/attachFile";
  const AttachFileScreen({super.key});

  @override
  State<AttachFileScreen> createState() => _AttachFileScreenState();
}

class _AttachFileScreenState extends State<AttachFileScreen> {
  String filePath = "";
  Uint8List? fileData;
  String mime = "";
  TextEditingController altTxtCtrl = TextEditingController();
  SharedFileAndShares? linkedFile;
  Timer? _debounce;

  @override
  dispose() {
    _debounce?.cancel();
    super.dispose();
  }

  void loadFile() async {
    try {
      if (_debounce?.isActive ?? false) _debounce!.cancel();
      _debounce = Timer(const Duration(milliseconds: 500), () async {
        var filePickRes = await FilePicker.platform.pickFiles();
        if (filePickRes == null) return;
        var firstFile = filePickRes.files.first;
        var filePath = firstFile.path;
        if (filePath == null) return;
        filePath = filePath.trim();
        if (filePath == "") return;
        var data = await File(filePath).readAsBytes();

        if (data.length > 1024 * 1024) {
          throw "File is too large to attach (limit: 1MiB)";
        }

        var mime = "";
        switch (firstFile.extension) {
          case "txt":
            mime = "text/plain";
            break;
          case "jpg":
            mime = "image/jpeg";
            break;
          case "jpeg":
            mime = "image/jpeg";
            break;
          case "png":
            mime = "image/png";
            break;
          default:
            throw "Unable to recognize type of embed";
        }

        setState(() {
          this.filePath = filePath!;
          fileData = data;
          this.mime = mime;
        });
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to attach file: $exception");
    }
  }

  void onLinkChanged(SharedFileAndShares? newLink) {
    setState(() {
      linkedFile = newLink;
    });
  }

  void attach() {
    var id = (Random().nextInt(1 << 30)).toRadixString(16);
    String? alt;
    if (altTxtCtrl.text != "") {
      alt = altTxtCtrl.text;
    }
    var embed = AttachmentEmbed(id,
        data: fileData, linkedFile: linkedFile, alt: alt, mime: mime);
    Navigator.of(context).pop(embed);
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;

    return Scaffold(
      body: Container(
        padding: const EdgeInsets.all(40),
        child: Center(
          child: Column(children: [
            Text("Attach File",
                style: TextStyle(fontSize: 20, color: textColor)),
            const SizedBox(height: 20),
            Row(children: [
              filePath != ""
                  ? Expanded(
                      child: Text(
                        "File: $filePath",
                        style: TextStyle(color: textColor),
                      ),
                    )
                  : const Empty(),
              ElevatedButton(
                  onPressed: loadFile, child: const Text("Load File")),
            ]),
            const SizedBox(height: 20),
            Row(children: [
              Text("Alt Text:", style: TextStyle(color: textColor)),
              const SizedBox(width: 41),
              Flexible(child: TextField(controller: altTxtCtrl)),
            ]),
            const SizedBox(height: 20),
            Row(children: [
              Text("Linking to: ", style: TextStyle(color: textColor)),
              const SizedBox(width: 20),
              Flexible(
                  child: LocalContentDropDown(
                true,
                onChanged: onLinkChanged,
              )),
            ]),
            const Expanded(child: Empty()),
            Row(mainAxisAlignment: MainAxisAlignment.center, children: [
              ElevatedButton(onPressed: attach, child: const Text("Attach")),
              const SizedBox(width: 10),
              CancelButton(onPressed: () {
                Navigator.of(context).pop();
              }),
            ]),
          ]),
        ),
      ),
    );
  }
}
