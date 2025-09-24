import 'package:intl/intl.dart';

double milliatomsToDCR(int atoms) => (atoms.toDouble() / 1e11);

double atomsToDCR(int atoms) => (atoms.toDouble() / 1e8);

String formatDCR(double dcr) => "${dcr.toStringAsFixed(8)} DCR";

String shortChanIDToStr(int sid) {
  var bh = sid >> 40;
  var txIndex = (sid >> 16) & 0xFFFFFF;
  var txPos = sid & 0xFFFF;
  return "$bh:$txIndex:$txPos";
}

int dcrToAtoms(double dcr) =>
    dcr < 0 ? (dcr * 1e8 - 0.5).truncate() : (dcr * 1e8 + 0.5).truncate();

typedef FromJSON<T> = T Function(Map<String, dynamic> m);

List<T> jsonToList<T>(dynamic res, FromJSON fromJson) {
  if (res == null) {
    return List.empty();
  }

  return (res as List).map<T>((v) => fromJson(v)).toList();
}

String dateTimeToGoTime(DateTime dt) {
  var y = dt.year.toString().padLeft(2, "0");
  var M = dt.month.toString().padLeft(2, "0");
  var d = dt.day.toString().padLeft(2, "0");
  var h = dt.hour.toString().padLeft(2, "0");
  var m = dt.minute.toString().padLeft(2, "0");
  var s = dt.second.toString().padLeft(2, "0");
  var ms = dt.millisecond.toString().padLeft(3, "0");
  var tzOff = dt.timeZoneOffset.inMinutes;
  var offHours = (tzOff ~/ 60).abs().toString().padLeft(2, "0");
  if (tzOff < 0) offHours = "-$offHours";
  var offMins = (tzOff % 60).toString().padLeft(2, "0");
  return "$y-$M-${d}T$h:$m:${s}.$ms$offHours:$offMins";
}
