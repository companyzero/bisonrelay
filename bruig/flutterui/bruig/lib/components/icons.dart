import 'package:bruig/theme_manager.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

class ColoredIcon extends StatelessWidget {
  final TextColor? color;
  final double? size;
  final IconData icon;
  const ColoredIcon(this.icon, {this.color, this.size, super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Icon(
              icon,
              color: color != null ? theme.textColor(color!) : null,
              size: size,
            ));
  }
}
