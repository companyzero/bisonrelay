import 'dart:math';

import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

class Copyable extends StatelessWidget {
  final String text;
  final TextStyle? textStyle;
  final bool showSnackbar;
  final String? textToCopy;
  final TextOverflow? textOverflow;
  final Widget? child;
  final String? tooltip;
  const Copyable(this.text,
      {this.textStyle,
      this.showSnackbar = true,
      this.child,
      this.textToCopy,
      this.textOverflow,
      this.tooltip,
      super.key});

  Copyable.txt(Txt txt, {tooltip, key})
      : this(txt.data, child: txt, tooltip: tooltip, key: key);

  void copy(BuildContext context) {
    var toCopy = textToCopy ?? text;
    Clipboard.setData(ClipboardData(text: toCopy));

    if (!showSnackbar) {
      return;
    }

    var textMsg = toCopy.substring(0, min(toCopy.length, 36));
    if (textMsg.length < toCopy.length) {
      textMsg += "...";
    }
    showSuccessSnackbar(context, "Copied \"$textMsg\" to clipboard");
  }

  @override
  Widget build(BuildContext context) {
    var w = InkWell(
        onTap: () => copy(context),
        child: child ?? Text(text, overflow: textOverflow, style: textStyle));
    return tooltip == null ? w : Tooltip(message: tooltip, child: w);
  }
}
