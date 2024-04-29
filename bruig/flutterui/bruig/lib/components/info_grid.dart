import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:tuple/tuple.dart';

class SimpleInfoGrid extends StatelessWidget {
  final ScrollController? controller;
  final List<Tuple2<Widget, Widget>> items;
  final int colValueFlex;
  final double colLabelSize;
  final double separatorWidth;
  const SimpleInfoGrid(this.items,
      {Key? key,
      this.colLabelSize = 100,
      this.colValueFlex = 4,
      this.separatorWidth = 20,
      this.controller})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
        shrinkWrap: true,
        controller: controller,
        itemCount: items.length,
        // physics: const NeverScrollableScrollPhysics(),
        itemBuilder: (context, index) => Container(
            margin: const EdgeInsets.only(bottom: 3),
            child: Row(
              children: [
                /*
                Flexible(
                  fit: FlexFit.tight,
                  flex: colLabelFlex,
                  child: items[index].item1,
                ),
                */
                SizedBox(width: colLabelSize, child: items[index].item1),
                SizedBox(width: separatorWidth),
                Flexible(
                  flex: colValueFlex,
                  child: items[index].item2,
                ),
              ],
            )));
  }
}
