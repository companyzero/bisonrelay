import 'dart:io';
import 'dart:math';
import 'dart:typed_data';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:bruig/util.dart';
import 'package:flutter/material.dart';
import 'package:image_compression_flutter/image_compression_flutter.dart';
import 'package:path_provider/path_provider.dart';
import 'package:path/path.dart' as path;

class CompressScreenResult {
  final Uint8List data;
  final String mime;

  CompressScreenResult(this.data, this.mime);
}

Future<CompressScreenResult?> showCompressScreen(BuildContext context,
    {required Uint8List original, required String mime}) async {
  return await showDialog<CompressScreenResult?>(
      useRootNavigator: true,
      context: context,
      builder: (context) {
        final Widget child;
        if (mime.startsWith("image/")) {
          child = _CompressImageScreen(original, mime);
        } else {
          child = Text("Mime type $mime does not allow compression");
        }

        return Dialog.fullscreen(child: child);
      });
}

class _CompressImageScreen extends StatefulWidget {
  final Uint8List original;
  final String mime;
  const _CompressImageScreen(this.original, this.mime);

  @override
  State<_CompressImageScreen> createState() => __CompressImageScreenState();
}

Future<String> _tempFileName() async {
  bool isMobile = Platform.isIOS || Platform.isAndroid;
  String base = isMobile
      ? (await getApplicationCacheDirectory()).path
      : (await getTemporaryDirectory()).path;
  var dir = path.join(base, "tocompress");
  if (!Directory(dir).existsSync()) {
    Directory(dir).createSync();
  }
  var fname = Random().nextInt(1 << 30).toRadixString(16);
  return path.join(dir, fname);
}

class __CompressImageScreenState extends State<_CompressImageScreen> {
  Uint8List get original => widget.original;
  String originalPath = "";
  ImageFile? originalFile;
  Uint8List? compressed;
  bool initing = true;
  bool compressing = false;

  void initOriginalFile() async {
    try {
      if (originalPath != "") {
        File(originalPath).deleteSync();
        originalPath = "";
      }
      var path = await _tempFileName();
      await File(path).writeAsBytes(original);
      originalPath = path;
      originalFile = ImageFile(filePath: originalPath, rawBytes: original);
      compress();
    } catch (exception) {
      showErrorSnackbar(this, "Unable to create original file: $exception");
    }
  }

  void compress() async {
    if (originalFile == null) {
      return;
    }

    var config = ImageFileConfiguration(
        input: originalFile!,
        config: const Configuration(
          useJpgPngNativeCompressor: false,
          outputType: ImageOutputType.jpg,
          quality: 40,
        ));

    try {
      setState(() => compressing = true);
      var output = await compressor.compress(config);
      setState(() {
        compressing = false;
        compressed = output.rawBytes;
        initing = false; // First init done.
      });
    } catch (exception) {
      showErrorSnackbar(this, "Unable to compress image: $exception");
    }
  }

  void accept() {
    if (compressed == null) {
      return;
    }

    Navigator.pop(context, CompressScreenResult(compressed!, "image/jpeg"));
  }

  @override
  void initState() {
    super.initState();
    initOriginalFile();
  }

  @override
  Widget build(BuildContext context) {
    var contextSize = MediaQuery.sizeOf(context);
    return StartupScreen(hideAboutButton: true, [
      const Txt.H("Compress Image"),
      // Image(image: MemoryImage(original)),
      Container(
          margin: const EdgeInsets.all(20),
          width: contextSize.width - 100,
          height: contextSize.height - 220,
          decoration: BoxDecoration(
              image: DecorationImage(
                  image: MemoryImage(compressed ?? original),
                  fit: BoxFit.contain))),
      SizedBox(
          width: 800,
          child: Wrap(
              alignment: WrapAlignment.spaceBetween,
              crossAxisAlignment: WrapCrossAlignment.center,
              runSpacing: 10,
              children: [
                CancelButton(onPressed: () {
                  Navigator.of(context).pop();
                }),
                Txt.S("Original File: ${humanReadableSize(original.length)}"),
                if (initing || compressing)
                  const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator()),
                if (compressed != null)
                  Txt.S(
                      "Compressed File: ${humanReadableSize(compressed!.length)}"),
                OutlinedButton(
                    onPressed: compressed == null ||
                            compressed!.length >= original.length
                        ? null
                        : accept,
                    child: const Text("Accept")),
              ])),
    ]);
  }
}
