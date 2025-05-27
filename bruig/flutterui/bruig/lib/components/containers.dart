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
  final BorderRadiusGeometry? borderRadius;
  const Box(
      {this.color = SurfaceColor.surface,
      this.child,
      this.padding,
      this.constraints,
      this.margin,
      this.width,
      this.height,
      this.borderRadius,
      super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Container(
              margin: margin,
              padding: padding,
              constraints: constraints,
              color: borderRadius == null ? theme.surfaceColor(color) : null,
              decoration: borderRadius == null
                  ? null
                  : BoxDecoration(
                      borderRadius: borderRadius,
                      color: theme.surfaceColor(color)),
              width: width,
              height: height,
              child: DefaultTextStyle.merge(
                  style: theme.textStyleFor(context, null,
                      textColorForSurfaceColor[color] ?? TextColor.onSurface),
                  child: child ?? const Empty()),
            ));
  }
}

// SecondarySideMenuItem is an individual item (ListTile or similar) of a
// SecondarySideMenu. This is needed to fix
// https://github.com/flutter/flutter/issues/59511.
class SecondarySideMenuItem extends StatelessWidget {
  final Widget child;
  const SecondarySideMenuItem(this.child, {super.key});

  @override
  Widget build(BuildContext context) {
    return Material(child: child);
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
  final List<ListTile>? items;
  final ListView? list;
  final Widget? footer;
  const SecondarySideMenuList(
      {this.width, this.items, this.list, this.footer, super.key});

  Widget _child() {
    if (list != null) {
      return list!;
    }

    if (items != null) {
      return ListView(
          shrinkWrap: true,
          children: items!.map((e) => SecondarySideMenuItem(e)).toList());
    }

    return const Empty();
  }

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => SecondarySideMenu(
              width: width,
              child: Column(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Expanded(
                        child: ListTileTheme.merge(
                            tileColor: theme.colors.surfaceContainerLowest,
                            // tileColor: Colors.amber,
                            child: _child())),
                    ...(footer != null ? [footer!] : []),
                  ]),
            ));
  }
}
