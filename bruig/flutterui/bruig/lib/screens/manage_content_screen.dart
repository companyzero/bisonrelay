import 'dart:async';

import 'package:bruig/components/text.dart';
import 'package:bruig/screens/manage_content/manage_content.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/models/downloads.dart';
import 'package:bruig/screens/manage_content/downloads.dart';
import 'package:bruig/components/manage_bar.dart';
import 'package:bruig/screens/overview.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/models/menus.dart';
import 'package:bruig/theme_manager.dart';

class ManageContentScreenTitle extends StatelessWidget {
  const ManageContentScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer2<MainMenuModel, ThemeNotifier>(
        builder: (context, menu, theme, child) {
      var idx = LnScreenSub.indexWhere((e) => e.pageTab == menu.activePageTab);

      return Txt.L("Manage Content / ${ManageContentScreenSub[idx].label}");
    });
  }
}

class ManageContentScreen extends StatefulWidget {
  static const routeName = '/manageContent';
  final MainMenuModel menu;
  const ManageContentScreen(this.menu, {Key? key}) : super(key: key);

  @override
  State<ManageContentScreen> createState() => _ManageContentScreenState();
}

class _ManageContentScreenState extends State<ManageContentScreen> {
  int tabIndex = 0;

  Widget activeTab() {
    switch (tabIndex) {
      case 0:
        return const ManageContent(0);
      case 1:
        return const ManageContent(1);
      case 2:
        return Consumer<DownloadsModel>(
            builder: (context, downloads, child) => DownloadsScreen(downloads));
    }
    return Text("Active is $tabIndex");
  }

  void onItemChanged(int index) {
    setState(() => tabIndex = index);
    Timer(const Duration(milliseconds: 1),
        () async => widget.menu.activePageTab = index);
  }

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    if (ModalRoute.of(context)!.settings.arguments != null) {
      final args = ModalRoute.of(context)!.settings.arguments as PageTabs;
      tabIndex = args.tabIndex;
    }

    return Row(children: [
      ModalRoute.of(context)!.settings.arguments == null
          ? isScreenSmall
              ? const Empty()
              : ManageContentBar(onItemChanged, tabIndex)
          : const Empty(),
      Expanded(child: activeTab())
    ]);
  }
}
