import 'package:bruig/components/app_notifications.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/indicator.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/feed.dart';
import 'package:bruig/models/menus.dart';
import 'package:bruig/models/notifications.dart';
import 'package:bruig/theme_manager.dart';

import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:sidebarx/sidebarx.dart';
import 'package:window_manager/window_manager.dart';

class Sidebar extends StatefulWidget {
  final ClientModel client;
  final MainMenuModel mainMenu;
  final AppNotifications ntfns;
  final GlobalKey<NavigatorState> navKey;
  final FeedModel feed;

  const Sidebar(this.client, this.mainMenu, this.ntfns, this.navKey, this.feed,
      {super.key});

  @override
  State<Sidebar> createState() => _SidebarState();
}

class _SidebarState extends State<Sidebar> with WindowListener {
  ClientModel get client => widget.client;
  MainMenuModel get mainMenu => widget.mainMenu;
  SidebarXController ctrl =
      SidebarXController(selectedIndex: 0, extended: true);
  FeedModel get feed => widget.feed;
  bool hasUnreadMsgs = false;
  double prevWindowSize = -1;

  void feedUpdated() async {
    setState(() {});
  }

  void connStateChanged() async {
    // Needed because the list of menus changes depending on the connstate.
    setState(() {});
  }

  void switchScreen(String route) {
    // Do not change screen if already there.
    String currentPath = "";
    widget.navKey.currentState?.popUntil((route) {
      currentPath = route.settings.name ?? "";
      return true;
    });

    if (currentPath == route) {
      return;
    }

    widget.navKey.currentState!.pushReplacementNamed(route);
  }

  void menuUpdated() {
    setState(() {
      ctrl.selectIndex(mainMenu.activeIndex);
    });
  }

  void hasUnreadMsgsChanged() {
    setState(() {
      hasUnreadMsgs = client.hasUnreadChats.val;
    });
  }

  @override
  void onWindowResize() {
    var size = MediaQuery.sizeOf(context);
    if (prevWindowSize < 0) {
      prevWindowSize = size.width;
      return;
    }

    // Check current screen size.  If over 1000px and NOT extended, then extend
    // If NOT over 1000px and extended, then collapse sidebar.
    var newSize = size.width;

    if (newSize < prevWindowSize && newSize < 1000 && ctrl.extended == true) {
      ctrl.setExtended(false);
    } else if (newSize > prevWindowSize &&
        newSize > 1000 &&
        ctrl.extended == false) {
      ctrl.setExtended(true);
    }

    prevWindowSize = size.width;
  }

  @override
  void initState() {
    super.initState();
    feed.addListener(feedUpdated);
    client.connState.addListener(connStateChanged);
    mainMenu.addListener(menuUpdated);
    client.hasUnreadChats.addListener(hasUnreadMsgsChanged);
    windowManager.addListener(this);
  }

  @override
  void didUpdateWidget(Sidebar oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.client != widget.client) {
      oldWidget.feed.removeListener(feedUpdated);
      oldWidget.client.connState.removeListener(connStateChanged);
      oldWidget.mainMenu.removeListener(menuUpdated);
      oldWidget.client.hasUnreadChats.removeListener(hasUnreadMsgsChanged);
      feed.addListener(feedUpdated);
      client.connState.addListener(connStateChanged);
      mainMenu.addListener(menuUpdated);
      client.hasUnreadChats.addListener(hasUnreadMsgsChanged);
    }
  }

  @override
  void dispose() {
    feed.removeListener(feedUpdated);
    client.connState.removeListener(connStateChanged);
    mainMenu.removeListener(menuUpdated);
    client.hasUnreadChats.removeListener(hasUnreadMsgsChanged);
    windowManager.removeListener(this);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var selectedColor = theme.highlightColor;

    return Consumer2<ClientModel, ThemeNotifier>(
        builder: (context, client, theme, child) {
      return SidebarX(
        theme: SidebarXTheme(
          margin: const EdgeInsets.all(1),
          padding: const EdgeInsets.all(2),
          width: 70,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(3),
            color: theme.colors.surfaceContainerLow,
            border: Border(
                right: BorderSide(color: theme.extraColors.sidebarDivider)),
          ),
          hoverTextStyle:
              theme.textStyleFor(context, null, TextColor.onSurfaceVariant),
          textStyle:
              theme.textStyleFor(context, null, TextColor.onSurfaceVariant),
          selectedTextStyle:
              theme.textStyleFor(context, null, TextColor.onSurface),
          itemPadding:
              const EdgeInsets.only(top: 7, bottom: 6, left: 12, right: 12),
          itemMargin:
              const EdgeInsets.only(top: 5, bottom: 0, left: 5, right: 5),
          selectedItemMargin:
              const EdgeInsets.only(top: 5, bottom: 0, left: 5, right: 5),
          selectedItemPadding:
              const EdgeInsets.only(top: 7, bottom: 6, left: 12, right: 12),
          selectedItemTextPadding: const EdgeInsets.only(left: 7),
          itemTextPadding: const EdgeInsets.only(left: 7),
          itemDecoration: BoxDecoration(
            borderRadius: BorderRadius.circular(5),
          ),
          selectedItemDecoration: BoxDecoration(
            borderRadius: BorderRadius.circular(5),
            color: theme.surfaceColor(SurfaceColor.surfaceContainerHighest),
          ),
          iconTheme: IconThemeData(
            color: theme.colors.onSurfaceVariant,
            size: 21,
          ),
          selectedIconTheme: IconThemeData(
            color: selectedColor,
            size: 21,
          ),
        ),
        extendedTheme: const SidebarXTheme(width: 200),
        footerDivider:
            Divider(height: 2, color: theme.extraColors.sidebarDivider),
        footerBuilder: (context, something) => Container(
            margin: const EdgeInsets.all(5),
            child: NotificationsDrawerHeader(widget.ntfns)),
        controller: ctrl,
        items: mainMenu.menus
            .map((e) => SidebarXItem(
                  label: e.label,
                  iconBuilder: (selected, hovered) =>
                      (e.label == "Chat" && hasUnreadMsgs) ||
                              (e.label == "Feed" && feed.hasUnreadPostsComments)
                          ? Stack(children: [
                              Container(
                                  padding: const EdgeInsets.all(3),
                                  child: e.icon ?? const Empty()),
                              const Positioned(
                                  top: 1, right: 1, child: RedDotIndicator()),
                            ])
                          : Container(
                              padding: const EdgeInsets.all(3), child: e.icon),
                  onTap: () => switchScreen(e.routeName),
                ))
            .toList(),
      );
    });
  }
}
