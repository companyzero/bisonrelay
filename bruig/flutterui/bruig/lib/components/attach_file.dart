import 'dart:convert';
import 'dart:io';
import 'dart:math';
import 'dart:async';

import 'package:bruig/components/md_elements.dart';
import 'package:bruig/components/chat/types.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/models/uploads.dart';
import 'package:bruig/screens/compress.dart';
import 'package:bruig/screens/send_file.dart';
import 'package:bruig/theme_manager.dart';
import 'package:bruig/util.dart';
import 'package:flutter_avif/flutter_avif.dart';
import 'package:golib_plugin/golib_plugin.dart';
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
  "audio/ogg",
  "application/pdf"
];

class AttachmentEmbed {
  String? mime;
  Uint8List? data;
  SharedFileAndShares? linkedFile;
  String? alt;
  String id;
  String? filename;
  String? name;
  AttachmentEmbed(this.id,
      {this.data,
      this.linkedFile,
      this.alt,
      this.mime,
      this.filename,
      this.name});

  String displayString() {
    return "--embed[id=$id]--";
  }

  String embedString() {
    List<String> parts = [];
    if ((name ?? "") != "") {
      parts.add("name=${Uri.encodeComponent(name!)}");
    }
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

class _GalleryImg {
  final File file;
  final ImageProvider img;

  _GalleryImg(this.file, this.img);
}

class AttachFileScreen extends StatefulWidget {
  final SendMsg _send;
  final Uint8List? initialFileData;
  final String? initialMime;
  final ChatModel chat;
  final VoidCallback closeAttachScreen;
  const AttachFileScreen(this._send, this.initialFileData, this.initialMime,
      this.chat, this.closeAttachScreen,
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
  List<_GalleryImg> listImages = [];
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

  final RegExp imgExtRegexp =
      RegExp(".(gif|jpe?g|tiff?|png|webp|bmp)", caseSensitive: false);

  _fetchFiles(Directory dir) {
    List<_GalleryImg> newList = [];
    dir.list().forEach((element) {
      if (!imgExtRegexp.hasMatch(element.path)) return;
      if (element is! File) return;
      try {
        var imgProvider = Image.file(element).image;
        newList.add(_GalleryImg(element, imgProvider));
      } catch (_) {
        // Ignore invalid image.
      }
    });

    listImages = newList;

    WidgetsBinding.instance.addPostFrameCallback((_) => setState(() {
          _fetchedPath = dir;
        }));
  }

  void loadFile() async {
    var snackbar = SnackBarModel.of(context);
    var uploads = UploadsModel.of(context, listen: false);
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
        await showSendFileScreen(context,
            chat: widget.chat, file: File(filePath), uploads: uploads);
        widget.closeAttachScreen(); // File screen already does the sending.
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
    XFile? pickedFile;
    try {
      pickedFile = await _picker.pickImage(source: source);
    } catch (e) {
      snackbar.error("Unable to select file: $e");
    }

    if (pickedFile == null) {
      return;
    }

    _onImagePressed(pickedFile.path, context: context);
  }

  Future<void> _onImagePressed(
    String filePath, {
    required BuildContext context,
  }) async {
    var snackbar = SnackBarModel.of(context);
    var uploads = UploadsModel.of(context, listen: false);
    try {
      var mimeType = lookupMimeType(filePath);
      if (mimeType == null) {
        throw "Unknown image MIME type";
      }
      if (!mimeType.startsWith("image/")) {
        throw "Not an image mime type ($mimeType)";
      }

      var data = await File(filePath).readAsBytes();
      if (data.length > Golib.maxPayloadSize) {
        // Image is larger than possible to attach as message. Automatically show
        // compression screen to try and reduce the max size.
        var compressRes =
            await showCompressScreen(context, original: data, mime: mimeType);

        if (compressRes == null) {
          // User canceled compression.
          return;
        }

        if (compressRes.data.length > Golib.maxPayloadSize) {
          // Compression was insufficient to reduce size. This needs to be sent
          // as a file.
          await showSendFileScreen(context,
              chat: widget.chat, file: File(filePath), uploads: uploads);
          return;
        }

        // Replace the vars so that the setState below uses the compressed version.
        data = compressRes.data;
        mimeType = compressRes.mime;
      }

      setState(() {
        selectedAttachmentPath = filePath;
        fileData = data;
        if (mimeType != null) {
          mime = mimeType;
        }
      });
    } catch (e) {
      snackbar.error("Unable to attach file: $e");
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
      name: selectedAttachmentPath != null
          ? path.basename(selectedAttachmentPath!)
          : null,
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
                    child: mime.contains("avif")
                        ? AvifImage.memory(fileData!, errorBuilder:
                            (BuildContext context, Object error,
                                StackTrace? stackTrace) {
                            return const Center(
                                child: Text('Avif unable to be decoded'));
                          })
                        : Image.memory(fileData!, errorBuilder:
                            (BuildContext context, Object error,
                                StackTrace? stackTrace) {
                            return const Center(
                                child:
                                    Text('This image type is not supported'));
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
                  return listImages.isNotEmpty ? Text(dir.path) : const Empty();
                } else {
                  return const Text("Loading gallery");
                }
              },
            ),
            listImages.isNotEmpty
                ? Container(
                    margin: const EdgeInsets.symmetric(vertical: 8.0),
                    height: 150.0,
                    child: Scrollbar(
                        controller: scrollCtrl,
                        child: ListView.builder(
                            controller: scrollCtrl,
                            scrollDirection: Axis.horizontal,
                            primary: false,
                            padding: const EdgeInsets.all(10),
                            itemCount: listImages.length,
                            itemBuilder: (context, i) => Padding(
                                padding: const EdgeInsets.all(10),
                                child: Material(
                                    shape: const RoundedRectangleBorder(
                                        borderRadius: BorderRadius.all(
                                            Radius.circular(10))),
                                    child: InkWell(
                                        borderRadius: const BorderRadius.all(
                                            Radius.circular(10)),
                                        onTap: () => _onImagePressed(
                                            listImages[i].file.path,
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
                                                image: listImages[i].img,
                                                fit: BoxFit.contain),
                                          ),
                                        )))))))
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
