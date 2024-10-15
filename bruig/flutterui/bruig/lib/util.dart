import 'dart:async';
import 'dart:math';
import 'dart:convert';

import 'package:crypto/crypto.dart';
import 'package:duration/duration.dart';
import 'package:flutter/material.dart';

// return a consistent color for each nick. Pretty dumb so far.
Color colorFromNick(String nick, Brightness brightness) {
  var buff = md5.convert(utf8.encode(nick)).bytes;
  var i = (buff[0] << 16) + (buff[1] << 8) + buff[2];
  // var h = (i / 0xffffff) * 360;
  var c = HSVColor.fromAHSV(
      1, (i / 0xffffff) * 360, 0.5, brightness == Brightness.dark ? 1.0 : 0.65);
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

// Parse a go-like string duration to nanoseconds. Supports day and week.
int parseDuration(String s) {
  // [-+]?([0-9]*(\.[0-9]*)?[a-z]+)+
  var orig = s;
  int d = 0; // In Nanoseconds.
  bool isNeg = false;
  if (s != "") {
    if (s[0] == "+" || s[0] == "-") {
      isNeg = s[0] == "-";
      s = s.substring(1);
    }
  }

  if (s == "0") {
    return 0;
  }

  if (s == "") {
    throw "invalid duration: '$orig'";
  }

  var fullMatch = RegExp(r'^([0-9]*(\.[0-9]*)?([a-z]+))+$').allMatches(s);
  if (fullMatch.isEmpty) {
    throw "invalid duration: '$orig'";
  }

  var matches = RegExp(r'([0-9]*(?:\.[0-9]*)?)([a-z]+)').allMatches(s);

  for (var el in matches) {
    /*
    for (var i = 0; i <= el.groupCount; i++) {
      print("$i ${el.group(i)}");
    }
    */

    if (el.groupCount != 2) {
      throw "invalid duration: '$orig'";
    }
    double v = double.parse(el.group(1)!);
    String label = el.group(2)!;
    int unit;
    switch (label) {
      case "ns":
        unit = 1;
        break;
      case "us":
        unit = 1e3.toInt();
        break;
      case "µs":
      case "μs":
        unit = 1e6.toInt();
        break;
      case "ms":
        unit = 1e9.toInt();
        break;
      case "s":
        unit = 1e12.toInt();
        break;
      case "m":
        unit = 1e12.toInt() * 60;
        break;
      case "h":
        unit = 1e12.toInt() * 60 * 60;
        break;
      case "d":
        unit = 1e12.toInt() * 60 * 60 * 24;
        break;
      case "w":
        unit = 1e12.toInt() * 60 * 60 * 24 * 7;
        break;
      default:
        throw "invalid duration: '$orig'";
    }

    if (v > ((1 << 63) - 1) / unit) {
      throw ("invalid duration (overflow): '$orig'");
    }

    v *= unit;
    d += v.truncate();
  }

  if (isNeg) {
    d *= -1;
  }
  return d;
}

int parseDurationSeconds(String s) => (parseDuration(s) / 1e12).truncate();

String formatTerseTime(DateTime d) {
  var diff = DateTime.now().difference(d);
  return prettyDuration(diff, tersity: DurationTersity.hour, abbreviated: true);
}

// Call Navigator.of(context).pop() ensuring the state is still mounted to avoid
// exception.
void popNavigatorFromState(State s, {bool rootNavigator = false}) => s.mounted
    ? Navigator.of(s.context, rootNavigator: rootNavigator).pop()
    : null;

// Push the given named route to the navigator if the state is still mounted.
//
// NOTE: this does nothing if the state is unmounted.
void pushNavigatorFromState(State s, String name,
        {bool rootNavigator = false, Object? arguments}) =>
    s.mounted
        ? Navigator.of(s.context, rootNavigator: rootNavigator)
            .pushNamed(name, arguments: arguments)
        : null;

void replaceNavigatorFromState(State s, String name,
        {bool rootNavigator = false, Object? arguments}) =>
    s.mounted
        ? Navigator.of(s.context, rootNavigator: rootNavigator)
            .pushReplacementNamed(name, arguments: arguments)
        : null;

// Convenience function to sleep in async functions.
Future<void> sleep(Duration d) {
  var p = Completer<void>();
  Timer(d, p.complete);
  return p.future;
}

String formatSmallDuration(Duration d) {
  String res = "";
  if (d.inHours > 0) {
    res = "${d.inHours.toString().padLeft(2, '0')}:";
  }
  var mins = (d.inMinutes - (d.inHours * 60)).toString().padLeft(2, '0');
  var secs = (d.inSeconds - (d.inMinutes * 60)).toString().padLeft(2, '0');
  return "$res$mins:$secs";
}
