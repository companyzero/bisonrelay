import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/theme_manager.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:tuple/tuple.dart';

class SimpleInfoGrid extends StatelessWidget {
  final ScrollController? controller;
  final List<Tuple2<Widget, Widget>> items;
  final int colValueFlex;
  final double colLabelSize;
  final double separatorWidth;
  final bool useListBuilder;
  final MainAxisAlignment rowAlignment;

  const SimpleInfoGrid(
    this.items, {
    Key? key,
    this.colLabelSize = 100,
    this.colValueFlex = 4,
    this.separatorWidth = 20,
    this.controller,
    this.useListBuilder = true,
    this.rowAlignment = MainAxisAlignment.start,
  }) : super(key: key);

  Widget buildChild(Tuple2<Widget, Widget> child) => Container(
      margin: const EdgeInsets.only(bottom: 3),
      child: Row(
        mainAxisAlignment: rowAlignment,
        children: [
          SizedBox(width: colLabelSize, child: child.item1),
          SizedBox(width: separatorWidth),
          Flexible(
            flex: colValueFlex,
            child: child.item2,
          ),
        ],
      ));

  @override
  Widget build(BuildContext context) {
    if (useListBuilder) {
      return ListView.builder(
          shrinkWrap: true,
          controller: controller,
          itemCount: items.length,
          // physics: const NeverScrollableScrollPhysics(),
          itemBuilder: (context, index) => buildChild(items[index]));
    }

    return Column(children: items.map(buildChild).toList());
  }
}

class SimpleInfoGridCopyableVal {
  final String label;
  final String value;

  SimpleInfoGridCopyableVal(this.label, this.value);
}

// SimpleInfoGridAdv with a more generic API: items can be Tuple2<String>, List<string>,
// and values can be Copyable.
class SimpleInfoGridAdv extends StatelessWidget {
  final ScrollController? controller;
  final List<dynamic> items;
  final int colValueFlex;
  final double colLabelSize;
  final double separatorWidth;
  final bool useListBuilder;
  final MainAxisAlignment rowAlignment;
  final TextSize textSize;

  const SimpleInfoGridAdv({
    Key? key,
    required this.items,
    this.colLabelSize = 100,
    this.colValueFlex = 4,
    this.separatorWidth = 20,
    this.controller,
    this.useListBuilder = true,
    this.rowAlignment = MainAxisAlignment.start,
    this.textSize = TextSize.small,
  }) : super(key: key);

  Widget buildChild(dynamic child) {
    late Widget label;
    late Widget value;
    if (child is Tuple2<String, String>) {
      label = Txt(child.item1, size: textSize);
      value = Txt(child.item2, size: textSize);
    } else if (child is List<String>) {
      label = Txt(child[0], size: textSize);
      value = Txt(child[1], size: textSize);
    } else if (child is List<dynamic>) {
      label = Txt(child[0], size: textSize);
      if (child[1] is Copyable) {
        var c = child[1] as Copyable;
        value = Copyable.txt(Txt(c.text, size: textSize), tooltip: c.tooltip);
      } else {
        value = Txt(child[1], size: textSize);
      }
    } else if (child is SimpleInfoGridCopyableVal) {
      label = Txt(child.label);
      value = Copyable.txt(Txt(child.value, size: textSize));
    } else {
      label = const Text("error");
      value = const Text("unknown item type");
    }

    return Container(
        margin: const EdgeInsets.only(bottom: 3),
        child: Row(
          mainAxisAlignment: rowAlignment,
          children: [
            SizedBox(width: colLabelSize, child: label),
            SizedBox(width: separatorWidth),
            Flexible(
              flex: colValueFlex,
              child: value,
            ),
          ],
        ));
  }

  @override
  Widget build(BuildContext context) {
    if (useListBuilder) {
      return ListView.builder(
          shrinkWrap: true,
          controller: controller,
          itemCount: items.length,
          // physics: const NeverScrollableScrollPhysics(),
          itemBuilder: (context, index) => buildChild(items[index]));
    }

    return Column(children: items.map(buildChild).toList());
  }
}
