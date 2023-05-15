import 'package:flutter/material.dart';
import 'dart:typed_data';

class ImageDialog extends StatelessWidget {
  final Uint8List imgContent;
  const ImageDialog(this.imgContent, {super.key});

  @override
  Widget build(BuildContext context) {
    return Dialog(
      child: Container(
        constraints: const BoxConstraints(maxHeight: 1000, maxWidth: 1000),
        decoration: BoxDecoration(
          image: DecorationImage(
            image: MemoryImage(imgContent),
          ),
        ),
      ),
    );
  }
}
