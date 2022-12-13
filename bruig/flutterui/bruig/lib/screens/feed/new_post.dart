import 'dart:convert';

import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/feed.dart';
import 'package:bruig/screens/manage_content/manage_content.dart';
import 'package:bruig/screens/feed.dart';
import 'package:bruig/util.dart';
import 'package:flutter/material.dart';
import 'package:file_picker/file_picker.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';

class NewPostScreen extends StatefulWidget {
  final FeedModel feed;
  const NewPostScreen(this.feed, {Key? key}) : super(key: key);

  @override
  State<NewPostScreen> createState() => _NewPostScreenState();
}

class _NewPostScreenState extends State<NewPostScreen> {
  TextEditingController contentCtrl = TextEditingController();
  bool loading = false;

  // Add embed fields.
  Map<String, String> embedContents = {};
  String embedData = "";
  String embedMime = "";
  SharedFile? embedLink;
  TextEditingController embedAlt = TextEditingController();
  int estimatedSize = 0;

  void goBack() {
    Navigator.pop(context);
  }

  // Returns the actual full content that will be included in the post.
  String getFullContent() {
    // Replace embedded content with actual content.
    var content = contentCtrl.text;
    final pattern = RegExp(r"(--embed\[.*data=)\[content ([a-zA-Z0-9]{12})]");
    content = content.replaceAllMapped(pattern, (match) {
      var content = embedContents[match.group(2)];
      if (content == null) {
        throw "Content not found: ${match.group(2)}";
      }
      return match.group(1)! + content;
    });
    return content;
  }

  void createPost() async {
    setState(() {
      loading = true;
    });
    try {
      await widget.feed.createPost(getFullContent());
      setState(() {
        contentCtrl.clear();
        estimatedSize = 0;
      });
      showSuccessSnackbar(context, "Created new post");
      Navigator.of(context).pushNamed(FeedScreen.routeName);
    } catch (exception) {
      showErrorSnackbar(context, "Unable to create post: $exception");
    } finally {
      setState(() {
        loading = false;
      });
    }
  }

  void recalcEstimatedSize() async {
    try {
      var estSize = await Golib.estimatePostSize(getFullContent());
      setState(() {
        estimatedSize = estSize;
      });
    } catch (exception) {
      showSuccessSnackbar(context, "Unable to estimate post size: $exception");
    }
  }

  void pickFile() async {
    var filePickRes = await FilePicker.platform.pickFiles(
      allowedExtensions: ["png", "jpg", "jpeg", "txt"],
      withData: true,
    );
    if (filePickRes == null) return;
    var f = filePickRes.files.first;
    var filePath = f.path;
    if (filePath == null) return;
    filePath = filePath.trim();
    if (filePath == "") return;

    if (f.size > 1024 * 1024) {
      showErrorSnackbar(context, "File size is too large");
      return;
    }

    var mime = "";
    switch (f.extension) {
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
        showErrorSnackbar(context, "Unable to recognize type of embed");
        return;
    }

    var data = const Base64Encoder().convert(f.bytes!);
    var id = generateRandomString(12);
    while (embedContents.containsKey(id)) {
      id = generateRandomString(12);
    }

    setState(() {
      embedContents[id] = data;
      embedMime = mime;
      embedData = id;
    });
    recalcEstimatedSize();
  }

  void addEmbed() {
    List<String> parms = [];
    if (embedMime != "") {
      parms.add("type=$embedMime");
    }
    if (embedLink != null) {
      parms.add("download=${embedLink!.fid}");
    }
    if (embedAlt.text != "") {
      parms.add("alt=${Uri.encodeComponent(embedAlt.text)}");
    }
    if (embedData != "") {
      parms.add("data=[content $embedData]");
    }
    var embed = "\n--embed[${parms.join(",")}]--\n";
    contentCtrl.text += embed;
    clearEmbed();
    recalcEstimatedSize();
  }

  void clearEmbed() {
    setState(() {
      embedMime = "";
      embedData = "";
      embedAlt.clear();
      embedLink = null;
    });
  }

  void linkToFile() async {
    var args = ManageContentScreenArgs(true);
    var fid = await Navigator.of(context, rootNavigator: true)
        .pushNamed("/manageContent", arguments: args);
    if (fid == null) {
      return;
    }
    setState(() {
      embedLink = fid as SharedFile;
    });
  }

  @override
  void initState() {
    super.initState();
    contentCtrl.addListener(recalcEstimatedSize); // TODO: debounce.
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor; // MESSAGE TEXT COLOR
    var backgroundColor = theme.backgroundColor;

    var validSize = estimatedSize <= 1024 * 1024;
    if (!validSize) {
      textColor = theme.errorColor;
    }

    return Container(
      margin: const EdgeInsets.all(1),
      decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(3), color: backgroundColor),
      padding: const EdgeInsets.all(16),
      child: Column(
        children: [
          Text("New Post", style: TextStyle(color: textColor, fontSize: 20)),
          Expanded(
              child: Container(
                  margin: const EdgeInsets.only(bottom: 15),
                  child: TextField(
                    controller: contentCtrl,
                    keyboardType: TextInputType.multiline,
                    maxLines: null,
                  ))),
          const Divider(thickness: 2),
          Row(children: [
            Text("Add embedded",
                style: TextStyle(color: textColor, fontSize: 15)),
            const SizedBox(width: 10),
            OutlinedButton(onPressed: pickFile, child: const Text("Load File")),
            const SizedBox(width: 10),
            Flexible(
              flex: 5,
              fit: FlexFit.tight,
              child: Text(embedMime,
                  style: TextStyle(color: textColor, fontSize: 15)),
            ),
            const SizedBox(width: 10),
            /*  XXX Need to figure out Link to Content button
            OutlinedButton(
                onPressed: linkToFile, child: const Text("Link to Content")),
            const SizedBox(width: 10),
            */
            Flexible(
              flex: 3,
              fit: FlexFit.tight,
              child: Text(embedLink?.filename ?? ""),
            ),
            const SizedBox(width: 10),
            Text("Alt Text: ",
                style: TextStyle(color: textColor, fontSize: 15)),
            Flexible(
              flex: 5,
              child: TextField(controller: embedAlt),
            ),
            IconButton(onPressed: addEmbed, icon: const Icon(Icons.add)),
            IconButton(onPressed: clearEmbed, icon: const Icon(Icons.delete)),
          ]),
          const SizedBox(height: 20),
          const Divider(thickness: 2),
          Text(
            "Estimated Size: ${humanReadableSize(estimatedSize)}",
            style: TextStyle(color: textColor),
          ),
          const SizedBox(height: 10),
          Row(mainAxisAlignment: MainAxisAlignment.center, children: [
            const SizedBox(width: 20),
            ElevatedButton(
                onPressed: !loading && validSize ? createPost : null,
                child: const Text("Create Post")),
            const SizedBox(width: 20),
          ])
        ],
      ),
    );
  }
}
