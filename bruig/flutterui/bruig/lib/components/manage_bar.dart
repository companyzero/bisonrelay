import 'package:bruig/components/containers.dart';
import 'package:bruig/components/text.dart';
import 'package:flutter/material.dart';

class ManageContentBar extends StatefulWidget {
  final int selectedIndex;
  final Function tabChange;
  const ManageContentBar(this.tabChange, this.selectedIndex, {super.key});

  @override
  State<ManageContentBar> createState() => _ManageContentBarState();
}

class _ManageContentBarState extends State<ManageContentBar> {
  int get selectedIndex => widget.selectedIndex;
  Function get tabChange => widget.tabChange;

  @override
  Widget build(BuildContext context) {
    return SecondarySideMenuList(items: [
      ListTile(
        title: const Txt.S("Add"),
        onTap: () => tabChange(0),
      ),
      ListTile(
        title: const Txt.S("Shared"),
        onTap: () => tabChange(1),
      ),
      ListTile(
        title: const Txt.S("Downloads"),
        onTap: () => tabChange(2),
      ),
    ]);
  }
}
