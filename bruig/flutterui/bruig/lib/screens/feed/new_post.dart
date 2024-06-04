import 'dart:convert';
import 'dart:async';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/models/feed.dart';
import 'package:bruig/screens/feed.dart';
import 'package:bruig/util.dart';
import 'package:flutter/material.dart';
import 'package:file_picker/file_picker.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

void showAltTextModal(BuildContext context, String mime, String data,
    NewPostModel post, TextEditingController contentCtrl) {
  showModalBottomSheet(
    context: context,
    builder: (BuildContext context) =>
        AddAltText(mime, data, post, contentCtrl),
  );
}

class AddAltText extends StatefulWidget {
  final String mime;
  final String data;
  final TextEditingController contentCtrl;
  final NewPostModel post;
  const AddAltText(this.mime, this.data, this.post, this.contentCtrl,
      {super.key});

  @override
  State<AddAltText> createState() => _AddAltTextState();
}

class _AddAltTextState extends State<AddAltText> {
  TextEditingController embedAlt = TextEditingController();

  String get mime => widget.mime;
  TextEditingController get contentCtrl => widget.contentCtrl;

  void _addEmbed() {
    List<String> embed = [];
    if (mime != "") {
      embed.add("type=$mime");
    }
    if (embedAlt.text != "") {
      embed.add("alt=${Uri.encodeComponent(embedAlt.text)}");
    }

    var id = widget.post.trackEmbed(widget.data);
    if (id != "") {
      embed.add("data=[content $id]");
    }
    var embedText = "--embed[${embed.join(",")}]--";

    var insertPos = contentCtrl.selection.start;
    if (insertPos > -1 && insertPos < contentCtrl.text.length) {
      contentCtrl.text = contentCtrl.text.substring(0, insertPos) +
          embedText +
          contentCtrl.text.substring(insertPos);
    } else {
      contentCtrl.text += "\n$embedText\n";
    }

    Navigator.pop(context);
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;

    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Container(
              padding: const EdgeInsets.all(30),
              child: Row(
                children: [
                  Text("Add Alt Text: ",
                      style: TextStyle(
                          color: textColor,
                          fontSize: theme.getMediumFont(context))),
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
                    style:
                        ElevatedButton.styleFrom(backgroundColor: Colors.grey),
                    child: const Text("No, thanks"),
                  ),
                  const SizedBox(width: 10),
                  ElevatedButton(
                      onPressed: _addEmbed, child: const Text("Add")),
                ],
              ),
            ));
  }
}

class NewPostScreen extends StatefulWidget {
  final FeedModel feed;
  const NewPostScreen(this.feed, {Key? key}) : super(key: key);

  @override
  State<NewPostScreen> createState() => _NewPostScreenState();
}

class _NewPostScreenState extends State<NewPostScreen> {
  NewPostModel get post => widget.feed.newPost;
  TextEditingController contentCtrl = TextEditingController();
  bool loading = false;

  // Add embed fields.
  SharedFile? embedLink;
  int estimatedSize = 0;
  Timer? _debounce;
  Timer? _debounceSizeCalc;

  void goBack() {
    Navigator.pop(context);
  }

  void createPost() async {
    setState(() {
      loading = true;
    });
    try {
      await widget.feed.createPost(post.getFullContent());
      setState(() {
        post.clear();
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
        var estSize = await Golib.estimatePostSize(post.getFullContent());
        setState(() {
          estimatedSize = estSize;
        });
      } catch (exception) {
        showErrorSnackbar(context, "Unable to estimate post size: $exception");
      }
    });
  }

  void contentChanged() async {
    post.content = contentCtrl.text;
    recalcEstimatedSize();
  }

  void pickFile(BuildContext context) async {
    if (_debounce?.isActive ?? false) _debounce!.cancel();
    _debounce = Timer(const Duration(milliseconds: 500), () async {
      var filePickRes = await FilePicker.platform.pickFiles(
        allowedExtensions: [
          "avif",
          "bmp",
          "gif",
          "jpg",
          "jpeg",
          "jxl",
          "png",
          "webp",
          "txt"
        ],
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
        case "avif":
          mime = "image/avif";
          break;
        case "bmp":
          mime = "image/bmp";
          break;
        case "gif":
          mime = "image/gif";
          break;
        case "jpg":
        case "jpeg":
          mime = "image/jpeg";
          break;
        case "jxl":
          mime = "image/jxl";
          break;
        case "png":
          mime = "image/png";
          break;
        case "webp":
          mime = "image/webp";
          break;
        default:
          showErrorSnackbar(context, "Unable to recognize type of embed");
          return;
      }

      var data = const Base64Encoder().convert(f.bytes!);

      showAltTextModal(context, mime, data, post, contentCtrl);
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

  void clearPost() {
    post.clear();
    contentCtrl.text = "";
  }

  @override
  dispose() {
    _debounce?.cancel();
    _debounceSizeCalc?.cancel();
    super.dispose();
  }

  @override
  void initState() {
    super.initState();
    contentCtrl.text = post.content;
    contentCtrl.addListener(contentChanged);
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

    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Container(
              margin: const EdgeInsets.all(1),
              decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(3),
                  color: backgroundColor),
              padding: const EdgeInsets.all(16),
              child: Column(
                children: [
                  Text("New Post",
                      style: TextStyle(
                          color: textColor,
                          fontSize: theme.getLargeFont(context))),
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
                        style: TextStyle(
                            color: textColor,
                            fontSize: theme.getMediumFont(context))),
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
                  Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        ElevatedButton(
                            onPressed:
                                !loading && validSize ? createPost : null,
                            child: const Text("Create Post")),
                        CancelButton(onPressed: clearPost, label: "Clear Post"),
                      ])
                ],
              ),
            ));
  }
}
