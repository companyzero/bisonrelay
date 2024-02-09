import 'dart:async';

import 'package:flutter/material.dart';
import 'package:bruig/components/context_menu.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/models/menus.dart';
import 'package:provider/provider.dart';
import 'package:bruig/screens/overview.dart';

class PageContextMenu extends StatelessWidget {
  const PageContextMenu(
      {super.key,
      required this.menuItem,
      required this.subMenu,
      required this.navKey});
  final MainMenuItem menuItem;
  final List<SubMenuInfo> subMenu;
  final GlobalKey<NavigatorState> navKey;

  void goToSubMenuPage(
      BuildContext context, MainMenuModel mainMenu, String route, int pageTab) {
    if (pageTab == mainMenu.activePageTab) return;
    navKey.currentState!
        .pushReplacementNamed(route, arguments: PageTabs(pageTab, [], null));
    Timer(const Duration(milliseconds: 1),
        () async => mainMenu.activePageTab = pageTab);
    //Navigator.pop(context);
  }

  @override
  Widget build(BuildContext context) {
    return Consumer<MainMenuModel>(
        builder: (context, mainMenu, chil) => ContextMenu(
              handleItemTap: (result) => result != null
                  ? goToSubMenuPage(
                      context, mainMenu, menuItem.routeName, result)
                  : {},
              items: subMenu
                  .map((e) =>
                      PopupMenuItem(value: e.pageTab, child: Text(e.label)))
                  .toList(),
              pageContextMenu: true,
              child: const Empty(),
            ));
  }
}
