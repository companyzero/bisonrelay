import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/theme_manager.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

class Collapsable extends StatefulWidget {
  final String title;
  final TextStyle? titleStyle;
  final Widget? child;
  const Collapsable(this.title, {super.key, this.child, this.titleStyle});

  @override
  State<Collapsable> createState() => _CollapsableState();
}

class _CollapsableState extends State<Collapsable> {
  bool showChild = false;

  @override
  Widget build(BuildContext context) {
    var themeNtf = Provider.of<ThemeNotifier>(context);
    Widget child = const Empty();
    if (showChild && widget.child != null) {
      child = widget.child!;
    }

    TextStyle titleStyle = widget.titleStyle ??
        themeNtf.textStyleFor(context, TextSize.medium, null)!;

    return Column(children: [
      InkWell(
          onTap: () {
            setState(() {
              showChild = !showChild;
            });
          },
          child: Row(mainAxisAlignment: MainAxisAlignment.center, children: [
            Icon(showChild
                ? Icons.arrow_drop_down_outlined
                : Icons.arrow_drop_up_outlined),
            Text(widget.title, style: titleStyle),
          ])),
      AnimatedCrossFade(
          firstChild: child,
          secondChild: const Empty(),
          sizeCurve: Curves.easeIn,
          crossFadeState: CrossFadeState.showFirst,
          duration: const Duration(milliseconds: 200))
    ]);
  }
}
