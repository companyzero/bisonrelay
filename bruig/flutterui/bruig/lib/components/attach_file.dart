import 'dart:convert';
import 'dart:io';
import 'dart:math';
import 'dart:async';

import 'package:bruig/components/chat/types.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:image_picker/image_picker.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';
import 'package:path_provider/path_provider.dart';
import 'package:permission_handler/permission_handler.dart';
import 'package:mime/mime.dart';

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
  final SendMsg _send;
  const AttachFileScreen(this._send, {super.key});

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
  late Future<Directory?> _futureGetPath;
  List<dynamic> listImagePath = [];
  Directory _fetchedPath = Directory.systemTemp;
  bool _permissionStatus = false;

  dynamic _pickImageError;
  final ImagePicker _picker = ImagePicker();
  String? selectedAttachmentPath;

  @override
  void initState() {
    super.initState();
    _listenForPermissionStatus();
    _futureGetPath = _getPath();
  }

  @override
  dispose() {
    _debounce?.cancel();
    super.dispose();
  }

  // Check for storage permission
  void _listenForPermissionStatus() async {
    if (Platform.isAndroid || Platform.isIOS) {
      final status = await Permission.storage.request().isGranted;
      // setState() triggers build again
      setState(() => _permissionStatus = status);
    } else {
      setState(() => _permissionStatus = true);
    }
  }

  Future<Directory?> _getPath() async {
    if (Platform.isAndroid) {
      return await getExternalStorageDirectory();
    }
    return await getDownloadsDirectory();
  }

  _fetchFiles(Directory dir) {
    List<dynamic> listImage = [];
    dir.list().forEach((element) {
      RegExp regExp =
          RegExp(".(gif|jpe?g|tiff?|png|webp|bmp)", caseSensitive: false);
      // Only add in List if path is an image
      if (regExp.hasMatch('$element')) listImage.add(element);
      setState(() {
        listImagePath = listImage;
      });
    });
    WidgetsBinding.instance.addPostFrameCallback((_) => setState(() {
          _fetchedPath = dir;
        }));
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

  Future<void> _onImageButtonPressed(
    ImageSource source, {
    required BuildContext context,
  }) async {
    try {
      final XFile? pickedFile = await _picker.pickImage(
        source: source,
      );
      if (pickedFile == null) {
        throw "Unable to load chosen image";
      }

      var data = await pickedFile.readAsBytes();
      if (data.length > 1024 * 1024) {
        throw "File is too large to attach (limit: 1MiB)";
      }
      var mimeType = lookupMimeType(pickedFile.path);
      setState(() {
        selectedAttachmentPath = pickedFile.path;
        fileData = data;
        if (mimeType != null) {
          mime = mimeType;
        }
      });
    } catch (e) {
      showErrorSnackbar(context, "Unable to attach file: $e");
      setState(() {
        _pickImageError = e;
      });
    }
  }

  Future<void> _onImagePressed(
    File image, {
    required BuildContext context,
  }) async {
    try {
      var data = await image.readAsBytes();
      if (data.length > 1024 * 1024) {
        throw "File is too large to attach (limit: 1MiB)";
      }
      var mimeType = lookupMimeType(image.path);
      setState(() {
        selectedAttachmentPath = image.path;
        fileData = data;
        if (mimeType != null) {
          mime = mimeType;
        }
      });
    } catch (e) {
      showErrorSnackbar(context, "Unable to select image: $e");
      setState(() {
        _pickImageError = e;
      });
    }
  }

  void attach() {
    var id = (Random().nextInt(1 << 30)).toRadixString(16);
    String? alt;
    if (altTxtCtrl.text != "") {
      alt = altTxtCtrl.text;
    }
    var embed = AttachmentEmbed(id,
        data: fileData, linkedFile: linkedFile, alt: alt, mime: mime);
    widget._send(embed.embedString());

    setState(() {
      mime = "";
      fileData = null;
      selectedAttachmentPath = "";
    });
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => selectedAttachmentPath != null &&
                mime.startsWith('image/')
            ? Container(
                margin: const EdgeInsets.symmetric(vertical: 8.0),
                height: 200.0,
                child: Image.file(
                  File(selectedAttachmentPath!),
                  errorBuilder: (BuildContext context, Object error,
                      StackTrace? stackTrace) {
                    return const Center(
                        child: Text('This image type is not supported'));
                  },
                ))
            : Column(mainAxisAlignment: MainAxisAlignment.start, children: [
                FutureBuilder(
                  future: _futureGetPath,
                  builder: (BuildContext context, AsyncSnapshot snapshot) {
                    if (snapshot.hasData) {
                      var dir = snapshot.data;
                      if (_permissionStatus && _fetchedPath != dir) {
                        _fetchFiles(dir);
                      }
                      return const Empty();
                    } else {
                      return const Text("Loading gallery");
                    }
                  },
                ),
                Container(
                    margin: const EdgeInsets.symmetric(vertical: 8.0),
                    height: 200.0,
                    child: ListView(
                        scrollDirection: Axis.horizontal,
                        primary: false,
                        padding: const EdgeInsets.all(20),
                        children: [
                          for (var i = 0; i < listImagePath.length; i++)
                            InkWell(
                                borderRadius:
                                    const BorderRadius.all(Radius.circular(30)),
                                hoverColor: Theme.of(context).hoverColor,
                                onTap: () => _onImagePressed(listImagePath[i],
                                    context: context),
                                child: Container(
                                  height: 100,
                                  width: 100,
                                  margin: const EdgeInsets.symmetric(
                                      horizontal: 2, vertical: 2),
                                  decoration: BoxDecoration(
                                    borderRadius: const BorderRadius.all(
                                        Radius.circular(8.0)),
                                    image: DecorationImage(
                                        image:
                                            Image.file(listImagePath[i]).image,
                                        fit: BoxFit.contain),
                                  ),
                                )),
                          IconButton(
                            splashRadius: 100,
                            padding: const EdgeInsets.all(20),
                            onPressed: () {
                              _onImageButtonPressed(ImageSource.gallery,
                                  context: context);
                            },
                            tooltip: 'Pick Image from gallery',
                            icon: Icon(Icons.photo, size: 40, color: textColor),
                          )
                        ])),
                fileData != null && fileData!.isNotEmpty
                    ? Row(
                        mainAxisAlignment: MainAxisAlignment.start,
                        children: [
                            IconButton(
                                padding: const EdgeInsets.all(0),
                                iconSize: 25,
                                onPressed:
                                    fileData != null && fileData!.isNotEmpty
                                        ? attach
                                        : null,
                                icon: const Icon(Icons.attach_file_outlined),
                                color: textColor)
                          ])
                    : const Empty(),
                //const ImageSelection()
              ])
        /*
              body: Container(
                padding: const EdgeInsets.all(40),
                child: Center(
                  child: Column(children: [
                    Text("Attach File",
                        style: TextStyle(
                            fontSize: theme.getLargeFont(context),
                            color: textColor)),
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
                      ElevatedButton(
                          onPressed: attach, child: const Text("Attach")),
                      const SizedBox(width: 10),
                      CancelButton(onPressed: () {
                        Navigator.of(context).pop();
                      }),
                    ]),
                  ]),
                ),
              ),
            ));
            */
        );
  }
}
