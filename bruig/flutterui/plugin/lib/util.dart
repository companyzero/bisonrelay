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
