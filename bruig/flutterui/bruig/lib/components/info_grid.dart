import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:tuple/tuple.dart';

class SimpleInfoGrid extends StatelessWidget {
  final ScrollController? controller;
  final List<Tuple2<Widget, Widget>> items;
  const SimpleInfoGrid(this.items, {Key? key, this.controller})
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
                Flexible(
                  fit: FlexFit.tight,
                  child: items[index].item1,
                ),
                const SizedBox(width: 20),
                Flexible(
                  flex: 4,
                  child: items[index].item2,
                ),
              ],
            )));
  }
}
