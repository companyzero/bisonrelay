import 'package:flutter/material.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

class InteractiveAvatar extends StatelessWidget {
  const InteractiveAvatar({
    super.key,
    required this.chatNick,
    this.bgColor,
    this.avatarColor,
    this.avatarTextColor,
    this.onTap,
    this.onSecondaryTap,
  });

  final String chatNick;
  final Color? bgColor;
  final Color? avatarColor;
  final Color? avatarTextColor;
  final VoidCallback? onTap;
  final VoidCallback? onSecondaryTap;

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Material(
              color: bgColor?.withOpacity(0),
              child: MouseRegion(
                cursor: SystemMouseCursors.click,
                child: GestureDetector(
                  onTap: onTap,
                  onSecondaryTap: onSecondaryTap,
                  child: CircleAvatar(
                      backgroundColor: avatarColor,
                      child: Text(chatNick[0].toUpperCase(),
                          style: TextStyle(
                              color: avatarTextColor,
                              fontSize: theme.getLargeFont(context)))),
                ),
              ),
            ));
  }
}
