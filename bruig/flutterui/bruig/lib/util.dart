import 'dart:math';
import 'dart:convert';

import 'package:crypto/crypto.dart';
import 'package:flutter/material.dart';

// return a consistent color for each nick. Pretty dumb so far.
Color colorFromNick(String nick) {
  var buff = md5.convert(utf8.encode(nick)).bytes;
  var i = (buff[0] << 16) + (buff[1] << 8) + buff[2];
  // var h = (i / 0xffffff) * 360;
  var c = HSVColor.fromAHSV(1, (i / 0xffffff) * 360, 0.5, 1);
  return c.toColor();
}

String generateRandomString(int len) {
  var r = Random();
  const chars =
      'AaBbCcDdEeFfGgHhIiJjKkLlMmNnOoPpQqRrSsTtUuVvWwXxYyZz1234567890';
  return List.generate(len, (index) => chars[r.nextInt(chars.length)]).join();
}

String humanReadableSize(int size) {
  size = size.abs();
  var sizes = [
    [1e12, "TB"],
    [1e9, "GB"],
    [1e6, "MB"],
    [1e3, "KB"],
  ];
  for (int i = 0; i < sizes.length; i++) {
    var div = sizes[i][0] as double;
    var lbl = sizes[i][1] as String;
    if (size < div) {
      continue;
    }
    return "${(size.toDouble() / div).toStringAsFixed(2)} $lbl";
  }
  return "$size B";
}
