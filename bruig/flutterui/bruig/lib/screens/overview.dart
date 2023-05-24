import 'dart:async';

import 'package:bruig/components/route_error.dart';
import 'package:bruig/components/sidebar.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/downloads.dart';
import 'package:bruig/models/feed.dart';
import 'package:bruig/models/menus.dart';
import 'package:bruig/models/notifications.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/screens/feed.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';

class _OverviewScreenTitle extends StatefulWidget {
  final MainMenuModel mainMenu;

  const _OverviewScreenTitle(this.mainMenu);

  @override
  State<_OverviewScreenTitle> createState() => _OverviewScreenTitleState();
}

class _OverviewScreenTitleState extends State<_OverviewScreenTitle> {
  MainMenuModel get mainMenu => widget.mainMenu;

  void mainMenuUpdated() => setState(() {});

  @override
  void initState() {
    super.initState();
    mainMenu.addListener(mainMenuUpdated);
  }

  @override
  void didUpdateWidget(_OverviewScreenTitle oldWidget) {
    oldWidget.mainMenu.removeListener(mainMenuUpdated);
    super.didUpdateWidget(oldWidget);
    mainMenu.addListener(mainMenuUpdated);
  }

  @override
  void dispose() {
    mainMenu.removeListener(mainMenuUpdated);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return mainMenu.activeMenu.titleBuilder(context);
  }
}

class PageTabs {
  final int tabIndex;

  PageTabs(this.tabIndex);
}

class OverviewScreen extends StatefulWidget {
  static const routeName = '/overview';
  static String subRoute(String route) => route.isNotEmpty && route[0] == "/"
      ? "$routeName$route"
      : "$routeName/$route";
  final ClientModel client;
  final AppNotifications ntfns;
  final DownloadsModel down;
  final String initialRoute;
  final MainMenuModel mainMenu;
  final FeedModel feed;
  final SnackBarModel snackBar;
  const OverviewScreen(this.down, this.client, this.ntfns, this.initialRoute,
      this.mainMenu, this.feed, this.snackBar,
      {Key? key})
      : super(key: key);

  @override
  State<OverviewScreen> createState() => _OverviewScreenState();
}

class _OverviewScreenState extends State<OverviewScreen> {
  ClientModel get client => widget.client;
  AppNotifications get ntfns => widget.ntfns;
  DownloadsModel get down => widget.down;
  ServerSessionState connState = ServerSessionState.empty();
  GlobalKey<NavigatorState> navKey = GlobalKey(debugLabel: "overview nav key");

  void clientChanged() {
    var newConnState = client.connState;
    if (newConnState.state != connState.state ||
        newConnState.checkWalletErr != connState.checkWalletErr) {
      setState(() {
        connState = newConnState;
      });
      ntfns.delType(AppNtfnType.walletCheckFailed);
      if (newConnState.state == connStateCheckingWallet &&
          newConnState.checkWalletErr != null) {
        var msg = "LN wallet check failed: ${newConnState.checkWalletErr}";
        ntfns.addNtfn(AppNtfn(AppNtfnType.walletCheckFailed, msg: msg));
      }
    }
  }

  void goToSubMenuPage(String route, int pageTab) {
    navKey.currentState!
        .pushReplacementNamed(route, arguments: PageTabs(pageTab));
    Timer(const Duration(milliseconds: 1),
        () async => widget.mainMenu.activePageTab = pageTab);
    Navigator.pop(context);
  }

  void goToNewPost() {
    navKey.currentState!.pushReplacementNamed('/feed', arguments: PageTabs(3));
  }

  void goToAbout() {
    Navigator.of(context).pushNamed("/about");
  }

  void goOnline() async {
    try {
      await Golib.goOnline();
      showSuccessSnackbar(context, "Going online...");
    } catch (exception) {
      showErrorSnackbar(context, "Unable to go online: $exception");
    }
  }

  void remainOffline() async {
    try {
      await Golib.remainOffline();
      showSuccessSnackbar(context, "Going offline...");
    } catch (exception) {
      showErrorSnackbar(context, "Unable to go offline: $exception");
    }
  }

  void skipWalletCheck() async {
    try {
      await Golib.skipWalletCheck();
      showSuccessSnackbar(context, "Skipping next wallet check...");
    } catch (exception) {
      showErrorSnackbar(context, "Unable to skip wallet check: $exception");
    }
  }

  @override
  void initState() {
    super.initState();
    connState = widget.client.connState;
    widget.client.addListener(clientChanged);
  }

  @override
  void didUpdateWidget(OverviewScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.client.removeListener(clientChanged);
    widget.client.addListener(clientChanged);
  }

  @override
  void dispose() {
    widget.client.removeListener(clientChanged);
    super.dispose();
  }

  void switchScreen(String route) {
    navKey.currentState!.pushReplacementNamed(route);
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var selectedColor = theme.dividerColor;
    var unselectedTextColor = theme.focusColor;
    var sidebarBackground = theme.backgroundColor;
    var scaffoldBackgroundColor = theme.canvasColor;
    var hoverColor = theme.hoverColor;

    var connectedIcon = Icons.cloud;
    String connStateLabel;
    GestureTapCallback connStateAction;

    switch (connState.state) {
      case connStateCheckingWallet:
        connectedIcon = Icons.cloud_off;
        connStateLabel = "Skip Wallet Check";
        connStateAction = skipWalletCheck;
        break;
      case connStateOffline:
        connectedIcon = Icons.cloud_off;
        connStateLabel = "Go Online";
        connStateAction = goOnline;
        break;
      default:
        connStateLabel = "Go Offline";
        connStateAction = remainOffline;
        break;
    }

    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    return Scaffold(
        backgroundColor: theme.canvasColor,
        appBar: isScreenSmall
            ? AppBar(
                title: _OverviewScreenTitle(widget.mainMenu),
              )
            : AppBar(
                title: Row(children: [
                  const SizedBox(width: 10),
                  IconButton(
                      splashRadius: 20,
                      tooltip: "Create a new post",
                      onPressed: goToNewPost,
                      color: Colors.red,
                      iconSize: 20,
                      icon: Icon(
                          color: theme.dividerColor, size: 20, Icons.mode)),
                  IconButton(
                      splashRadius: 20,
                      tooltip: connStateLabel,
                      onPressed: connStateAction,
                      color: theme.dividerColor,
                      iconSize: 20,
                      icon: Icon(
                          color: theme.dividerColor, size: 20, connectedIcon)),
                  const SizedBox(width: 20),
                  _OverviewScreenTitle(widget.mainMenu),
                ]),
                leading: Builder(
                    builder: (BuildContext context) => Row(children: [
                          IconButton(
                              tooltip: "About Bison Relay",
                              iconSize: 40,
                              onPressed: goToAbout,
                              icon: Image.asset(
                                "assets/images/icon.png",
                              )),
                        ])),
              ),
        drawer: Drawer(
          backgroundColor: sidebarBackground,
          child: ListView.separated(
              separatorBuilder: (context, index) =>
                  Divider(height: 3, color: unselectedTextColor),
              itemCount: widget.mainMenu.menus.length,
              itemBuilder: (context, index) {
                var menuItem = widget.mainMenu.menus.elementAt(index);
                return menuItem.subMenuInfo.isEmpty
                    ? ListTile(
                        hoverColor: scaffoldBackgroundColor,
                        selected:
                            widget.mainMenu.activeMenu.label == menuItem.label,
                        selectedColor: selectedColor,
                        iconColor: unselectedTextColor,
                        textColor: unselectedTextColor,
                        selectedTileColor: hoverColor,
                        onTap: () {
                          switchScreen(menuItem.routeName);
                          Navigator.pop(context);
                        },
                        leading: (menuItem.label == "Chats" &&
                                    client.hasUnreadChats) ||
                                (menuItem.label == "News Feed" &&
                                    widget.feed.hasUnreadPostsComments)
                            ? menuItem.iconNotification
                            : menuItem.icon,
                        title: Text(menuItem.label,
                            style: const TextStyle(fontSize: 15)))
                    : Theme(
                        data: Theme.of(context)
                            .copyWith(dividerColor: Colors.transparent),
                        child: ExpansionTile(
                          title: Text(menuItem.label),
                          initiallyExpanded: widget.mainMenu.activeMenu.label ==
                              menuItem.label,
                          leading: (menuItem.label == "Chats" &&
                                      client.hasUnreadChats) ||
                                  (menuItem.label == "News Feed" &&
                                      widget.feed.hasUnreadPostsComments)
                              ? menuItem.iconNotification
                              : menuItem.icon,
                          children: (menuItem.subMenuInfo.map((e) => ListTile(
                              hoverColor: scaffoldBackgroundColor,
                              selected: widget.mainMenu.activeMenu.label ==
                                      menuItem.label &&
                                  widget.mainMenu.activePageTab == e.pageTab,
                              selectedColor: selectedColor,
                              iconColor: unselectedTextColor,
                              textColor: unselectedTextColor,
                              selectedTileColor: hoverColor,
                              title: Text(e.label),
                              onTap: () => goToSubMenuPage(
                                  menuItem.routeName, e.pageTab)))).toList(),
                        ));
              }),
        ),
        body: SnackbarDisplayer(
            widget.snackBar,
            Row(children: [
              isScreenSmall
                  ? const Empty()
                  : Sidebar(widget.client, widget.mainMenu, widget.ntfns,
                      navKey, widget.feed),
              Expanded(
                child: Navigator(
                  key: navKey,
                  initialRoute: widget.initialRoute == ""
                      ? FeedScreen.routeName
                      : widget.initialRoute,
                  onGenerateRoute: (settings) {
                    String routeName = settings.name!;
                    MainMenuItem? menu =
                        widget.mainMenu.menuForRoute(routeName);

                    // This update needs to be on a timer so that it is decoupled to
                    // the widget build stack frame.
                    Timer(const Duration(milliseconds: 1),
                        () async => widget.mainMenu.activeRoute = routeName);

                    return PageRouteBuilder(
                      pageBuilder: (context, animation, secondaryAnimation) =>
                          menu != null
                              ? menu.builder(context)
                              : RouteErrorPage(settings.name ?? "",
                                  OverviewScreen.routeName),
                      transitionDuration: Duration.zero,
                      //reverseTransitionDuration: Duration.zero,
                      settings: settings,
                    );
                  },
                ),
              )
            ])));
  }
}
