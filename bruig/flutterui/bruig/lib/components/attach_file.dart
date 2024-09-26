import 'dart:convert';
import 'dart:io';
import 'dart:math';
import 'dart:async';

import 'package:bruig/components/md_elements.dart';
import 'package:bruig/components/chat/types.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/screens/compress.dart';
import 'package:bruig/theme_manager.dart';
import 'package:bruig/util.dart';
import 'package:image_picker/image_picker.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:path_provider/path_provider.dart';
import 'package:permission_handler/permission_handler.dart';
import 'package:mime/mime.dart';
import 'package:path/path.dart' as path;

List<String> allowedMimeTypes = [
  "text/plain",
  "image/avif",
  "image/bmp",
  "image/gif",
  "image/jpeg",
  "image/jxl",
  "image/png",
  "image/webp",
  "application/pdf"
];

class AttachmentEmbed {
  String? mime;
  Uint8List? data;
  SharedFileAndShares? linkedFile;
  String? alt;
  String id;
  String? filename;
  AttachmentEmbed(this.id,
      {this.data, this.linkedFile, this.alt, this.mime, this.filename});

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
    } else {
      if (filename != null && filename?.trim() != "") {
        parts.add("filename=${filename!.trim()}");
      }
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
  final Uint8List? initialFileData;
  final String? initialMime;
  const AttachFileScreen(this._send, this.initialFileData, this.initialMime,
      {super.key});

  @override
  State<AttachFileScreen> createState() => _AttachFileScreenState();
}

class _AttachFileScreenState extends State<AttachFileScreen> {
  String filePath = "";
  Uint8List? fileData;
  String? fileName;
  String mime = "";
  TextEditingController altTxtCtrl = TextEditingController();
  SharedFileAndShares? linkedFile;
  Timer? _debounce;
  late Future<Directory?> _futureGetPath;
  List<dynamic> listImagePath = [];
  Directory _fetchedPath = Directory.systemTemp;
  bool _permissionStatus = false;

  final ImagePicker _picker = ImagePicker();
  String? selectedAttachmentPath;
  ScrollController scrollCtrl = ScrollController();
  @override
  void initState() {
    super.initState();
    _listenForPermissionStatus();
    _futureGetPath = _getPath();
    fileData = widget.initialFileData;
    mime = widget.initialMime ?? "";
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
      Directory? directory = Directory('/storage/emulated/0/Download');
      // Put file in global download folder, if for an unknown reason it didn't exist, we fallback
      // ignore: avoid_slow_async_io
      if (!await directory.exists()) {
        directory = await getExternalStorageDirectory();
      }
      return directory;
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
    var snackbar = SnackBarModel.of(context);
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
          selectedAttachmentPath = filePath;
          this.filePath = filePath!;
          fileName = path.basename(filePath);
          fileData = data;
          mime = mimeType;
        });
      } on Exception catch (exception) {
        snackbar.error("Unable to attach file: $exception");
      } catch (exception) {
        snackbar.error("Unable to attach file: $exception");
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
    var snackbar = SnackBarModel.of(context);
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
      snackbar.error("Unable to attach file: $e");
    }
  }

  Future<void> _onImagePressed(
    File image, {
    required BuildContext context,
  }) async {
    var snackbar = SnackBarModel.of(context);
    try {
      var data = await image.readAsBytes();
      if (data.length > 1024 * 1024) {
        throw "File is too large to attach (limit: 1MiB)";
      }
      var mimeType = lookupMimeType(image.path);
      setState(() {
        selectedAttachmentPath = image.path;
        fileData = data;
        fileName = path.basename(image.path);
        if (mimeType != null) {
          mime = mimeType;
        }
      });
    } catch (e) {
      snackbar.error("Unable to select image: $e");
    }
  }

  void attach() {
    var id = (Random().nextInt(1 << 30)).toRadixString(16);
    String? alt;
    if (altTxtCtrl.text != "") {
      alt = altTxtCtrl.text;
    }
    var embed = AttachmentEmbed(
      id,
      data: fileData,
      linkedFile: linkedFile,
      alt: alt,
      mime: mime,
      filename: fileName,
    );
    widget._send(embed.embedString());

    setState(() {
      mime = "";
      fileData = null;
      fileName = null;
      selectedAttachmentPath = "";
    });
  }

  @override
  Widget build(BuildContext context) {
    bool canCompress = (mime.startsWith('image/'));
    return fileData != null || selectedAttachmentPath != null
        ? Column(children: [
            mime.startsWith('image/')
                ? Container(
                    margin: const EdgeInsets.symmetric(vertical: 8.0),
                    height: 200.0,
                    child: Image.memory(fileData!, errorBuilder:
                        (BuildContext context, Object error,
                            StackTrace? stackTrace) {
                      return const Center(
                          child: Text('This image type is not supported'));
                    }))
                : mime.startsWith('text/')
                    ? Container(
                        margin: const EdgeInsets.symmetric(vertical: 8.0),
                        height: 100.0,
                        child: Text(selectedAttachmentPath!))
                    : mime.contains('pdf')
                        ? MarkdownArea(
                            AttachmentEmbed('pdf',
                                    data: fileData,
                                    linkedFile: linkedFile,
                                    alt: "",
                                    mime: mime)
                                .embedString(),
                            false)
                        : const Empty(),
            fileData != null && fileData!.isNotEmpty
                ? Row(
                    // mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                        const SizedBox(width: 10),
                        Txt.S(
                          "Size: ${humanReadableSize(fileData?.length ?? 0)}",
                          color: TextColor.onSurfaceVariant,
                        ),
                        if (canCompress)
                          TextButton.icon(
                            icon: const Icon(Icons.compress),
                            label: const Txt.S("Compress"),
                            onPressed: () async {
                              var res = await showCompressScreen(context,
                                  original: fileData!, mime: mime);
                              if (res == null) {
                                return;
                              }
                              setState(() {
                                fileData = res.data;
                                mime = res.mime;
                              });
                            },
                          ),
                        const Expanded(child: Empty()),
                        IconButton(
                            tooltip: "Send Attachment",
                            padding: const EdgeInsets.all(0),
                            iconSize: 25,
                            onPressed: fileData != null && fileData!.isNotEmpty
                                ? attach
                                : null,
                            icon: const Icon(Icons.send_outlined))
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
                  return listImagePath.isNotEmpty
                      ? Text(dir.path)
                      : const Empty();
                } else {
                  return const Text("Loading gallery");
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
                                        borderRadius: const BorderRadius.all(
                                            Radius.circular(10)),
                                        onTap: () => _onImagePressed(
                                            listImagePath[i],
                                            context: context),
                                        child: Container(
                                          width: 100,
                                          margin: const EdgeInsets.symmetric(
                                              horizontal: 2, vertical: 2),
                                          decoration: BoxDecoration(
                                            borderRadius:
                                                const BorderRadius.all(
                                                    Radius.circular(8.0)),
                                            image: DecorationImage(
                                                image:
                                                    Image.file(listImagePath[i])
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
                          borderRadius: BorderRadius.all(Radius.circular(10))),
                      child: InkWell(
                          onTap: () {
                            _onImageButtonPressed(ImageSource.gallery,
                                context: context);
                          },
                          child: const SizedBox(
                              height: 100,
                              child: Column(
                                  mainAxisAlignment: MainAxisAlignment.center,
                                  children: [
                                    Icon(Icons.photo, size: 40),
                                    Text("Gallery")
                                  ]))))),
              const SizedBox(width: 20),
              Expanded(
                  child: Material(
                      shape: const RoundedRectangleBorder(
                          borderRadius: BorderRadius.all(Radius.circular(10))),
                      child: InkWell(
                          onTap: loadFile,
                          child: const SizedBox(
                              height: 100,
                              child: Column(
                                  mainAxisAlignment: MainAxisAlignment.center,
                                  children: [
                                    Icon(Icons.insert_drive_file_outlined,
                                        size: 40),
                                    Text("File"),
                                  ]))))),
              const SizedBox(width: 10),
            ])
          ]);
  }
}
