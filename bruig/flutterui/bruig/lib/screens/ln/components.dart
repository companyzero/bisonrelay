import 'package:flutter/material.dart';

class LNInfoSectionHeader extends StatelessWidget {
  final String title;

  const LNInfoSectionHeader(this.title, {super.key});

  @override
  Widget build(BuildContext context) {
    return Row(children: [
      Text(title),
      const SizedBox(width: 8),
      const Expanded(child: Divider()),
    ]);
  }
}
