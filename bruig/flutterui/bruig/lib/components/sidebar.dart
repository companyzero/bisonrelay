import 'package:bruig/components/app_notifications.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/feed.dart';
import 'package:bruig/models/menus.dart';
import 'package:bruig/models/notifications.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:provider/provider.dart';
import 'package:sidebarx/sidebarx.dart';

class Sidebar extends StatefulWidget {
  final ClientModel client;
  final MainMenuModel mainMenu;
  final AppNotifications ntfns;
  final GlobalKey<NavigatorState> navKey;
  final FeedModel feed;

  const Sidebar(this.client, this.mainMenu, this.ntfns, this.navKey, this.feed,
      {Key? key})
      : super(key: key);

  @override
  State<Sidebar> createState() => _SidebarState();
}

class _SidebarState extends State<Sidebar> {
  ClientModel get client => widget.client;
  MainMenuModel get mainMenu => widget.mainMenu;
  ServerSessionState connState = ServerSessionState.empty();
  SidebarXController ctrl =
      SidebarXController(selectedIndex: 0, extended: true);
  FeedModel get feed => widget.feed;

  void feedUpdated() async {
    setState(() {});
  }

  void clientUpdated() async {
    setState(() {
      connState = client.connState;
    });
  }

  void switchScreen(String route) {
    widget.navKey.currentState!.pushReplacementNamed(route);
  }

  void menuUpdated() {
    setState(() {
      ctrl.selectIndex(mainMenu.activeIndex);
    });
  }

  @override
  void initState() {
    super.initState();
    clientUpdated();
    feed.addListener(feedUpdated);
    client.addListener(clientUpdated);
    mainMenu.addListener(menuUpdated);
  }

  @override
  void didUpdateWidget(Sidebar oldWidget) {
    oldWidget.feed.removeListener(feedUpdated);
    oldWidget.client.removeListener(clientUpdated);
    oldWidget.mainMenu.removeListener(menuUpdated);
    super.didUpdateWidget(oldWidget);
    feed.addListener(feedUpdated);
    client.addListener(clientUpdated);
    mainMenu.addListener(menuUpdated);
  }

  @override
  void dispose() {
    feed.removeListener(feedUpdated);
    client.removeListener(clientUpdated);
    mainMenu.removeListener(menuUpdated);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    // Check current screen size.  If over 1000px and NOT extended, then extend
    // If NOT over 1000px and extended, then collapse sidebar.
    MediaQueryData queryData;
    queryData = MediaQuery.of(context);
    if (queryData.size.width < 1000 && ctrl.extended == true) {
      ctrl.setExtended(false);
    } else if (queryData.size.width > 1000 && ctrl.extended == false) {
      ctrl.setExtended(true);
    }

    var theme = Theme.of(context);
    var textColor = theme.focusColor; // MESSAGE TEXT COLOR
    var selectedColor = theme.highlightColor;
    var unselectedTextColor = theme.dividerColor;
    var sidebarBackground = theme.backgroundColor;
    var scaffoldBackgroundColor = theme.canvasColor;
    var hoverColor = theme.hoverColor;

    final divider = Divider(color: scaffoldBackgroundColor, height: 2);
    return Consumer<ClientModel>(builder: (context, client, child) {
      return SidebarX(
        theme: SidebarXTheme(
          margin: const EdgeInsets.all(1),
          padding: const EdgeInsets.all(2),
          width: 63,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(3),
            gradient: LinearGradient(
                begin: Alignment.centerRight,
                end: Alignment.centerLeft,
                colors: [
                  hoverColor,
                  sidebarBackground,
                  sidebarBackground,
                ],
                stops: const [
                  0,
                  0.51,
                  1
                ]),
          ),
          hoverColor: scaffoldBackgroundColor,
          textStyle: TextStyle(color: unselectedTextColor),
          selectedTextStyle: TextStyle(color: textColor),
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
            color: selectedColor,
          ),
          iconTheme: IconThemeData(
            color: unselectedTextColor,
            size: 21,
          ),
          selectedIconTheme: IconThemeData(
            color: textColor,
            size: 21,
          ),
        ),
        extendedTheme: SidebarXTheme(
          margin: const EdgeInsets.all(1),
          width: 200,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(5),
            gradient: LinearGradient(
                begin: Alignment.centerRight,
                end: Alignment.centerLeft,
                colors: [
                  hoverColor,
                  sidebarBackground,
                  sidebarBackground,
                ],
                stops: const [
                  0,
                  0.51,
                  1
                ]),
          ),
        ),
        footerDivider: divider,
        footerBuilder: (context, something) => Container(
            margin: const EdgeInsets.all(5),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(5),
            ),
            child: NotificationsDrawerHeader(widget.ntfns)),
        controller: ctrl,
        items: mainMenu.menus
            .map((e) => SidebarXItem(
                  label: e.label,
                  iconWidget: (e.label == "Chats" && client.hasUnreadChats) ||
                          (e.label == "News Feed" &&
                              feed.hasUnreadPostsComments)
                      ? e.iconNotification
                      : e.icon,
                  onTap: () => switchScreen(e.routeName),
                ))
            .toList(),
      );
    });
  }
}
