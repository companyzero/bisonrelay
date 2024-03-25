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

List<String> allowedMimeTypes = [
  "text/plain",
  "image/avif",
  "image/bmp",
  "image/gif",
  "image/jpeg",
  "image/jxl",
  "image/png",
  "image/webp"
];

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
  ScrollController scrollCtrl = ScrollController();
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
      var directory = await getExternalStorageDirectory();
      if (directory != null) {
        return Directory("${directory.path}/Downloads");
      }
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
    if (_debounce?.isActive ?? false) _debounce!.cancel();
    _debounce = Timer(const Duration(milliseconds: 500), () async {
      try {
        var filePickRes = await FilePicker.platform.pickFiles(
          type: FileType.any,
        );
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

        var mimeType = lookupMimeType(filePath);
        if (mimeType == null) {
          throw "Unable to lookup file type";
        }
        if (!allowedMimeTypes.contains(mimeType)) {
          throw "Selected file ($filePath) type not allowed, only $allowedMimeTypes currently allowed";
        }
        setState(() {
          this.filePath = filePath!;
          fileData = data;
          mime = mimeType;
        });
      } on Exception catch (exception) {
        showErrorSnackbar(context, "Unable to attach file: $exception");
      } catch (exception) {
        showErrorSnackbar(context, "Unable to attach file: $exception");
      }
    });
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
        return;
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
        builder: (context, theme, _) => selectedAttachmentPath != null
            ? Column(children: [
                mime.startsWith('image/')
                    ? Container(
                        margin: const EdgeInsets.symmetric(vertical: 8.0),
                        height: 200.0,
                        child: Image.file(File(selectedAttachmentPath!),
                            errorBuilder: (BuildContext context, Object error,
                                StackTrace? stackTrace) {
                          return const Center(
                              child: Text('This image type is not supported'));
                        }))
                    : mime.startsWith('text/')
                        ? Container(
                            margin: const EdgeInsets.symmetric(vertical: 8.0),
                            height: 100.0,
                            child: Text(selectedAttachmentPath!,
                                style: TextStyle(
                                    fontSize: theme.getLargeFont(context),
                                    color: textColor)))
                        : const Empty(),
                fileData != null && fileData!.isNotEmpty
                    ? Row(mainAxisAlignment: MainAxisAlignment.end, children: [
                        IconButton(
                            tooltip: "Send Attachment",
                            padding: const EdgeInsets.all(0),
                            iconSize: 25,
                            onPressed: fileData != null && fileData!.isNotEmpty
                                ? attach
                                : null,
                            icon: const Icon(Icons.send_outlined),
                            color: textColor)
                      ])
                    : const Empty(),
              ])
            : Column(mainAxisAlignment: MainAxisAlignment.start, children: [
                FutureBuilder(
                  future: _futureGetPath,
                  builder: (BuildContext context, AsyncSnapshot snapshot) {
                    if (snapshot.hasData) {
                      var dir = snapshot.data;
                      if (_permissionStatus && _fetchedPath != dir) {
                        _fetchFiles(dir);
                      }
                      return Text(dir.path,
                          style: TextStyle(
                              fontSize: theme.getMediumFont(context),
                              color: textColor));
                    } else {
                      return Text("Loading gallery",
                          style: TextStyle(
                              fontSize: theme.getLargeFont(context),
                              color: textColor));
                    }
                  },
                ),
                listImagePath.isNotEmpty
                    ? Container(
                        margin: const EdgeInsets.symmetric(vertical: 8.0),
                        height: 150.0,
                        child: ListView(
                            controller: scrollCtrl,
                            scrollDirection: Axis.horizontal,
                            primary: false,
                            padding: const EdgeInsets.all(10),
                            children: [
                              for (var i = 0; i < listImagePath.length; i++)
                                Padding(
                                    padding: const EdgeInsets.all(10),
                                    child: Material(
                                        shape: const RoundedRectangleBorder(
                                            borderRadius: BorderRadius.all(
                                                Radius.circular(10))),
                                        child: InkWell(
                                            borderRadius:
                                                const BorderRadius.all(
                                                    Radius.circular(10)),
                                            hoverColor:
                                                Theme.of(context).hoverColor,
                                            onTap: () => _onImagePressed(
                                                listImagePath[i],
                                                context: context),
                                            child: Container(
                                              width: 100,
                                              margin:
                                                  const EdgeInsets.symmetric(
                                                      horizontal: 2,
                                                      vertical: 2),
                                              decoration: BoxDecoration(
                                                borderRadius:
                                                    const BorderRadius.all(
                                                        Radius.circular(8.0)),
                                                image: DecorationImage(
                                                    image: Image.file(
                                                            listImagePath[i])
                                                        .image,
                                                    fit: BoxFit.contain),
                                              ),
                                            ))))
                            ]))
                    : const Empty(),
                Row(children: [
                  const SizedBox(width: 10),
                  Expanded(
                      child: Material(
                          shape: const RoundedRectangleBorder(
                              borderRadius:
                                  BorderRadius.all(Radius.circular(10))),
                          child: InkWell(
                              onTap: () {
                                _onImageButtonPressed(ImageSource.gallery,
                                    context: context);
                              },
                              child: SizedBox(
                                  height: 100,
                                  child: Column(
                                      mainAxisAlignment:
                                          MainAxisAlignment.center,
                                      children: [
                                        Icon(Icons.photo,
                                            size: 40, color: textColor),
                                        Text("Gallery",
                                            style: TextStyle(
                                                fontSize: theme
                                                    .getMediumFont(context),
                                                color: textColor))
                                      ]))))),
                  const SizedBox(width: 20),
                  Expanded(
                    child: Material(
                        shape: const RoundedRectangleBorder(
                            borderRadius:
                                BorderRadius.all(Radius.circular(10))),
                        child: InkWell(
                            onTap: loadFile,
                            child: SizedBox(
                                height: 100,
                                child: Column(
                                    mainAxisAlignment: MainAxisAlignment.center,
                                    children: [
                                      Icon(Icons.insert_drive_file_outlined,
                                          size: 40, color: textColor),
                                      Text("File",
                                          style: TextStyle(
                                              fontSize:
                                                  theme.getMediumFont(context),
                                              color: textColor))
                                    ])))),
                  ),
                  const SizedBox(width: 10),
                ])
              ]));
  }
}
