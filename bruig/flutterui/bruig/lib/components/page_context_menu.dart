import 'dart:async';

import 'package:flutter/material.dart';
import 'package:bruig/components/context_menu.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/models/menus.dart';
import 'package:bruig/models/client.dart';
import 'package:provider/provider.dart';
import 'package:bruig/screens/overview.dart';

class PageContextMenu extends StatelessWidget {
  const PageContextMenu(
      {super.key,
      required this.menuItem,
      required this.subMenu,
      required this.contextMenu,
      required this.navKey});
  final MainMenuItem menuItem;
  final List<SubMenuInfo> subMenu;
  final List<ChatMenuItem?> contextMenu;
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

  void executeContextMenuItem(BuildContext context, ClientModel client,
      List<ChatMenuItem> contextMenuList, String result) {
    for (var contextItem in contextMenuList) {
      if (contextItem.label == result) {
        contextItem.onSelected(context, client);
        return;
      }
    }
    //Navigator.pop(context);
  }

  bool isMenuItemEnabled(BuildContext context, ClientModel client,
      List<ChatMenuItem> contextMenuList, String label) {
    for (var contextItem in contextMenuList) {
      if (contextItem.label == label) {
        return contextItem.onSelected(context, client) != null;
      }
    }
    return false;
  }

  @override
  Widget build(BuildContext context) {
    if (contextMenu.isNotEmpty) {
      List<ChatMenuItem> contextMenuFill =
          contextMenu.whereType<ChatMenuItem>().toList();
      return Consumer2<MainMenuModel, ClientModel>(
          builder: (context, mainMenu, client, child) => ContextMenu(
                handleItemTap: (result) => result != null
                    ? executeContextMenuItem(
                        context, client, contextMenuFill, result)
                    : {},
                items: contextMenuFill
                    .map((e) => PopupMenuItem(
                        enabled: isMenuItemEnabled(
                            context, client, contextMenuFill, e.label),
                        value: e.label,
                        child: Text(e.label)))
                    .toList(),
                pageContextMenu: true,
                child: const Empty(),
              ));
    }
    return Consumer<MainMenuModel>(
        builder: (context, mainMenu, child) => ContextMenu(
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
