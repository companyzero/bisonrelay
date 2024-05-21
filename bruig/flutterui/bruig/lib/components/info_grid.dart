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
  const SimpleInfoGrid(
    this.items, {
    Key? key,
    this.colLabelSize = 100,
    this.colValueFlex = 4,
    this.separatorWidth = 20,
    this.controller,
    this.useListBuilder = true,
  }) : super(key: key);

  Widget buildChild(Tuple2<Widget, Widget> child) => Container(
      margin: const EdgeInsets.only(bottom: 3),
      child: Row(
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
