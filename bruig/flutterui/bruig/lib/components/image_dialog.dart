import 'package:flutter/material.dart';
import 'dart:typed_data';

class ImageDialog extends StatelessWidget {
  final Uint8List imgContent;
  const ImageDialog(this.imgContent, {super.key});

  @override
  Widget build(BuildContext context) {
    return Dialog(
      child: Image.memory(
        imgContent,
        width: 1000,
        height: 1000,
        fit: BoxFit.cover,
        filterQuality: FilterQuality.high,
      ),
    );
  }
}
