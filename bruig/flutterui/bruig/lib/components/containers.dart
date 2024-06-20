import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/theme_manager.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

// Standard container that is painted with a material 3 color and changes
// all text rendered inside to (by default) use the corresponding onXXX color.
class Box extends StatelessWidget {
  final SurfaceColor color;
  final Widget? child;
  final EdgeInsetsGeometry? padding;
  final BoxConstraints? constraints;
  final EdgeInsetsGeometry? margin;
  final double? width;
  final double? height;
  const Box(
      {this.color = SurfaceColor.surface,
      this.child,
      this.padding,
      this.constraints,
      this.margin,
      this.width,
      this.height,
      super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Container(
              margin: margin,
              padding: padding,
              constraints: constraints,
              color: theme.surfaceColor(color),
              width: width,
              height: height,
              child: DefaultTextStyle.merge(
                  style: theme.textStyleFor(context, null,
                      textColorForSurfaceColor[color] ?? TextColor.onSurface),
                  child: child ?? const Empty()),
            ));
  }
}

// Used on pages that have a secondary side menu when window has desktop size.
class SecondarySideMenu extends StatelessWidget {
  final Widget? child;
  final double? width;
  const SecondarySideMenu({this.child, this.width, super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Container(
              margin: const EdgeInsets.all(1),
              width: width ?? 120,
              // color: theme.colors.surfaceContainer,
              decoration: BoxDecoration(
                  border: Border(
                      right:
                          BorderSide(color: theme.extraColors.sidebarDivider))),
              child: child,
            ));
  }
}

class SecondarySideMenuList extends StatelessWidget {
  final double? width;
  final List<ListTile> children;
  const SecondarySideMenuList({this.width, required this.children, super.key});

  @override
  Widget build(BuildContext context) {
    // The column is needed to work around an issue where setting a background color
    // to a listView makes hover color not work.
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => SecondarySideMenu(
              width: width,
              child: Column(children: [
                ListTileTheme.merge(
                    tileColor: theme.colors.surfaceContainerLowest,
                    child: ListView(shrinkWrap: true, children: children)),
                Expanded(
                    child:
                        Container(color: theme.colors.surfaceContainerLowest)),
              ]),
            ));
  }
}
