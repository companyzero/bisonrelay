import 'dart:math';

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
