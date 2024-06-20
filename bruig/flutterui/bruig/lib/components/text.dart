// ignore_for_file: non_constant_identifier_names

import 'package:bruig/theme_manager.dart';
import 'package:flutter/material.dart';
import 'package:flutter/material.dart' as material;
import 'package:provider/provider.dart';

class Txt extends StatelessWidget {
  final String data;
  final TextSize? size;
  final TextColor? color;
  final TextOverflow? overflow;

  const Txt(
    this.data, {
    this.size,
    this.color,
    this.overflow,
    super.key,
  });

  const Txt.S(String data, {color, key, overflow})
      : this(
          data,
          size: TextSize.small,
          color: color,
          overflow: overflow,
          key: key,
        );

  const Txt.M(String data, {color, key, overflow})
      : this(
          data,
          size: TextSize.medium,
          color: color,
          overflow: overflow,
          key: key,
        );

  const Txt.L(String data, {color, key, overflow})
      : this(
          data,
          size: TextSize.large,
          color: color,
          overflow: overflow,
          key: key,
        );

  const Txt.H(String data, {color, key, overflow})
      : this(
          data,
          size: TextSize.huge,
          color: color,
          overflow: overflow,
          key: key,
        );

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => material.Text(data,
            overflow: overflow,
            style: theme.textStyleFor(context, size, color)));
  }
}
