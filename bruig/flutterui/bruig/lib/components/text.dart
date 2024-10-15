// ignore_for_file: non_constant_identifier_names

import 'dart:io';

import 'package:bruig/theme_manager.dart';
import 'package:flutter/material.dart';
import 'package:flutter/material.dart' as material;
import 'package:provider/provider.dart';

class Txt extends StatelessWidget {
  final String data;
  final TextSize? size;
  final TextColor? color;
  final TextOverflow? overflow;
  final TextAlign? textAlign;
  final TextStyle? style; // To be merged with default style.

  const Txt(
    this.data, {
    this.size,
    this.color,
    this.overflow,
    this.textAlign,
    this.style,
    super.key,
  });

  const Txt.S(String data, {color, key, overflow, textAlign, style})
      : this(
          data,
          size: TextSize.small,
          color: color,
          overflow: overflow,
          textAlign: textAlign,
          style: style,
          key: key,
        );

  const Txt.M(String data, {color, key, overflow, textAlign, style})
      : this(
          data,
          size: TextSize.medium,
          color: color,
          overflow: overflow,
          textAlign: textAlign,
          style: style,
          key: key,
        );

  const Txt.L(String data, {color, key, overflow, textAlign, style})
      : this(
          data,
          size: TextSize.large,
          color: color,
          overflow: overflow,
          textAlign: textAlign,
          style: style,
          key: key,
        );

  const Txt.H(String data, {color, key, overflow, textAlign, style})
      : this(
          data,
          size: TextSize.huge,
          color: color,
          overflow: overflow,
          textAlign: textAlign,
          style: style,
          key: key,
        );

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(builder: (context, theme, child) {
      var parentStyle = theme.textStyleFor(context, size, color);

      // Merge style and parentStyle when both are defined, otherwise use only
      // the non-null one or null to rely on theme defaults.
      var mergedStyle = parentStyle != null && style != null
          ? style!.merge(parentStyle)
          : style ?? parentStyle;

      return material.Text(data,
          textAlign: textAlign, overflow: overflow, style: mergedStyle);
    });
  }
}

// Show tooltip except on mobile platforms.
class TooltipExcludingMobile extends StatelessWidget {
  final Widget child;
  final String? message;
  final InlineSpan? richMessage;
  const TooltipExcludingMobile(
      {super.key, required this.child, this.message, this.richMessage});

  @override
  Widget build(BuildContext context) {
    if (Platform.isAndroid || Platform.isIOS) {
      return child;
    }

    return Tooltip(message: message, richMessage: richMessage, child: child);
  }
}
