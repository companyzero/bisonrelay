import 'dart:convert';
import 'dart:async';

import 'package:bruig/models/feed.dart';
import 'package:bruig/screens/feed.dart';
import 'package:bruig/util.dart';
import 'package:flutter/material.dart';
import 'package:file_picker/file_picker.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:bruig/components/snackbars.dart';

void showAltTextModal(BuildContext context, String mime, String id,
    TextEditingController contentCtrl) {
  showModalBottomSheet(
    context: context,
    builder: (BuildContext context) => AddAltText(mime, id, contentCtrl),
  );
}

class AddAltText extends StatefulWidget {
  final String mime;
  final String id;
  final TextEditingController contentCtrl;
  const AddAltText(this.mime, this.id, this.contentCtrl, {super.key});

  @override
  State<AddAltText> createState() => _AddAltTextState();
}

class _AddAltTextState extends State<AddAltText> {
  TextEditingController embedAlt = TextEditingController();

  String get mime => widget.mime;
  String get id => widget.id;
  TextEditingController get contentCtrl => widget.contentCtrl;

  void _addEmbed() {
    List<String> embed = [];
    if (mime != "") {
      embed.add("type=$mime");
    }
    if (embedAlt.text != "") {
      embed.add("alt=${Uri.encodeComponent(embedAlt.text)}");
    }
    if (id != "") {
      embed.add("data=[content $id]");
    }
    var embedText = "\n--embed[${embed.join(",")}]--\n";
    contentCtrl.text += embedText;
    Navigator.pop(context);
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;

    return Container(
      padding: const EdgeInsets.all(30),
      child: Row(
        children: [
          Text("Add Alt Text: ",
              style: TextStyle(color: textColor, fontSize: 15)),
          Expanded(
            child: TextField(
              onSubmitted: (_) {
                _addEmbed();
              },
              controller: embedAlt,
              autofocus: true,
            ),
          ),
          const SizedBox(width: 30),
          ElevatedButton(
            onPressed: () => _addEmbed(),
            style: ElevatedButton.styleFrom(backgroundColor: Colors.grey),
            child: const Text("No, thanks"),
          ),
          const SizedBox(width: 10),
          ElevatedButton(onPressed: _addEmbed, child: const Text("Add")),
        ],
      ),
    );
  }
}

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
  int estimatedSize = 0;
  Timer? _debounce;
  Timer? _debounceSizeCalc;

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
    if (_debounceSizeCalc?.isActive ?? false) _debounceSizeCalc!.cancel();
    _debounceSizeCalc = Timer(const Duration(milliseconds: 500), () async {
      try {
        var estSize = await Golib.estimatePostSize(getFullContent());
        setState(() {
          estimatedSize = estSize;
        });
      } catch (exception) {
        showErrorSnackbar(context, "Unable to estimate post size: $exception");
      }
    });
  }

  void pickFile(BuildContext context) async {
    if (_debounce?.isActive ?? false) _debounce!.cancel();
    _debounce = Timer(const Duration(milliseconds: 500), () async {
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
        showErrorSnackbar(
            context, "File size is too large ${f.size} > ${1024 * 1024}");
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

      showAltTextModal(context, mime, id, contentCtrl);

      setState(() {
        embedContents[id] = data;
        embedMime = mime;
        embedData = id;
      });
      recalcEstimatedSize();
    });
  }

  // TODO: Implement together with link to content button
  // void linkToFile() async {
  //   var args = ManageContentScreenArgs(true);
  //   var fid = await Navigator.of(context, rootNavigator: true)
  //       .pushNamed("/manageContent", arguments: args);
  //   if (fid == null) {
  //     return;
  //   }
  //   setState(() {
  //     embedLink = fid as SharedFile;
  //   });
  // }

  @override
  dispose() {
    _debounce?.cancel();
    _debounceSizeCalc?.cancel();
    super.dispose();
  }

  @override
  void initState() {
    super.initState();
    contentCtrl.addListener(recalcEstimatedSize);
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
              ),
            ),
          ),
          const Divider(thickness: 2),
          Row(children: [
            Text("Add embedded",
                style: TextStyle(color: textColor, fontSize: 15)),
            const SizedBox(width: 10),
            OutlinedButton(
              onPressed: () {
                pickFile(context);
              },
              child: const Text("Load File"),
            ),
            /*  XXX Need to figure out Link to Content button
            const SizedBox(width: 10),
            OutlinedButton(
                onPressed: linkToFile, child: const Text("Link to Content")),
            const SizedBox(width: 10),
            Flexible(
              flex: 3,
              fit: FlexFit.tight,
              child: Text(embedLink?.filename ?? ""),
            ),
            */
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
