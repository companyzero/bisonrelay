import 'package:bruig/components/containers.dart';
import 'package:bruig/components/text.dart';
import 'package:flutter/material.dart';

class LNManagementBar extends StatelessWidget {
  final int selectedIndex;
  final Function tabChange;
  const LNManagementBar(this.tabChange, this.selectedIndex, {Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    return SecondarySideMenuList(width: 110, items: [
      ListTile(
          selected: selectedIndex == 0,
          title: const Txt.S("Overview"),
          onTap: () => tabChange(0)),
      ListTile(
          selected: selectedIndex == 1,
          title: const Txt.S("Accounts"),
          onTap: () => tabChange(1)),
      ListTile(
          selected: selectedIndex == 2,
          title: const Txt.S("On-Chain"),
          onTap: () => tabChange(2)),
      ListTile(
          selected: selectedIndex == 3,
          title: const Txt.S("Channels"),
          onTap: () => tabChange(3)),
      ListTile(
          selected: selectedIndex == 4,
          title: const Txt.S("Payments"),
          onTap: () => tabChange(4)),
      ListTile(
          selected: selectedIndex == 5,
          title: const Txt.S("Network"),
          onTap: () => tabChange(5)),
      ListTile(
          selected: selectedIndex == 6,
          title: const Txt.S("Backups"),
          onTap: () => tabChange(6)),
    ]);
  }
}
