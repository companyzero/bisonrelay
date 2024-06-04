import 'package:flutter/material.dart';
import 'package:path_drawing/path_drawing.dart';

class SVGClipper extends CustomClipper<Path> {
  String svgPath;
  Offset offset;

  SVGClipper(this.svgPath, {this.offset = Offset.zero});

  @override
  Path getClip(Size size) {
    var path = parseSvgPathData(svgPath);

    return path.shift(offset);
  }

  @override
  bool shouldReclip(CustomClipper oldClipper) {
    return false;
  }
}
