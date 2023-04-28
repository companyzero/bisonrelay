import 'dart:math';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import 'package:bruig/models/snackbar.dart';

class Copyable extends StatelessWidget {
  final String text;
  final TextStyle textStyle;
  final bool showSnackbar;
  final String? textToCopy;
  const Copyable(this.text, this.textStyle,
      {this.showSnackbar = true, this.textToCopy, Key? key})
      : super(key: key);

  void copy(BuildContext context) {
    var snackBar = Provider.of<SnackBarModel>(context);
    var toCopy = textToCopy ?? text;
    Clipboard.setData(ClipboardData(text: toCopy));

    if (!showSnackbar) {
      return;
    }

    var textMsg = toCopy.substring(0, min(toCopy.length, 36));
    if (textMsg.length < toCopy.length) {
      textMsg += "...";
    }
    snackBar.success("Copied \"$textMsg\" to clipboard");
  }

  @override
  Widget build(BuildContext context) {
    return InkWell(
        onTap: () => copy(context), child: Text(text, style: textStyle));
  }
}
