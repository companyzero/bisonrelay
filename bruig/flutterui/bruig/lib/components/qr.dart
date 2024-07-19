import 'dart:typed_data';
import 'dart:ui' as ui;
import 'package:flutter/material.dart';

class QrCodePainter extends CustomPainter {
  final double margin;
  final ui.Image qrImage;
  late Paint _paintBg;
  late Paint _paintFg;

  QrCodePainter({required this.qrImage, this.margin = 10}) {
    _paintBg = Paint()
      ..color = Colors.white
      ..style = ui.PaintingStyle.fill;
    _paintFg = Paint()
      ..color = Colors.black
      ..style = ui.PaintingStyle.fill;
  }

  @override
  void paint(Canvas canvas, Size size) {
    // Draw everything in white.
    final rect = Rect.fromPoints(Offset.zero, Offset(size.width, size.height));
    canvas.drawRect(rect, _paintBg);

    // Draw the image in the center.
    canvas.drawImage(qrImage, Offset(margin, margin), _paintFg);
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;

  ui.Picture toPicture(double size) {
    final recorder = ui.PictureRecorder();
    final canvas = Canvas(recorder);
    paint(canvas, Size(size, size));
    canvas.drawRect(
        Rect.fromCenter(center: const Offset(100, 100), width: 30, height: 30),
        _paintFg);
    return recorder.endRecording();
  }

  Future<ui.Image> toImage(double size,
      {ui.ImageByteFormat format = ui.ImageByteFormat.png}) async {
    var pic = toPicture(size);
    return await pic.toImage(size.toInt(), size.toInt());
  }

  Future<ByteData?> toImageData(double originalSize,
      {ui.ImageByteFormat format = ui.ImageByteFormat.png}) async {
    final image = await toImage(originalSize + margin * 2, format: format);
    return await image.toByteData(format: format);
  }
}
