import 'dart:async';

import 'package:bruig/components/page_context_menu.dart';
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
import 'package:bruig/screens/chats.dart';
import 'package:bruig/screens/feed.dart';
import 'package:bruig/screens/feed/post_content.dart';
import 'package:bruig/notification_service.dart';
import 'package:bruig/screens/settings.dart';
import 'package:bruig/screens/viewpage_screen.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';
import 'package:bruig/util.dart';

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
  final List<PostListItem> userPostList;
  final PostContentScreenArgs? postScreenArgs;

  PageTabs(this.tabIndex, this.userPostList, this.postScreenArgs);
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
  FeedModel get feed => widget.feed;
  ServerSessionState connState = ServerSessionState.empty();
  GlobalKey<NavigatorState> navKey = GlobalKey(debugLabel: "overview nav key");

  bool removeBottomBar = false;
  var selectedIndex = 0;
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
        .pushReplacementNamed(route, arguments: PageTabs(pageTab, [], null));
    Timer(const Duration(milliseconds: 1),
        () async => widget.mainMenu.activePageTab = pageTab);
    Navigator.pop(context);
  }

  void goToNewPost() {
    navKey.currentState!
        .pushReplacementNamed('/feed', arguments: PageTabs(3, [], null));
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

  void _configureDidReceiveLocalNotificationSubject() {
    NotificationService()
        .didReceiveLocalNotificationStream
        .stream
        .listen((ReceivedNotification receivedNotification) async {});
  }

  // This sets up the listener for notification tapping actions.  When
  // a user taps a chat notification they should be brought to the corresponding
  // chat.  When a user taps a post/comment notification they are brought to the
  // corresponding post.
  void _configureSelectNotificationSubject() {
    NotificationService()
        .selectNotificationStream
        .stream
        .listen((String? payload) async {
      if (payload != null) {
        if (payload.contains("chat")) {
          switchScreen(ChatsScreen.routeName);
          var nick = payload.split(":");
          if (nick.length > 1) {
            client.setActiveByNick(nick[1], payload.contains("gc"));
          }
        } else if (payload.contains("post")) {
          var authorPostIDs = payload.split(":");
          if (authorPostIDs.length > 2) {
            var authorID = authorPostIDs[1];
            var pid = authorPostIDs[2];
            var post = feed.getPost(authorID, pid);
            if (post != null) {
              navKey.currentState!.pushReplacementNamed("/feed",
                  arguments: PageTabs(0, [], PostContentScreenArgs(post)));
              feed.active = post;
            }
          }
        }
      }
    });
  }

  @override
  void initState() {
    super.initState();
    connState = widget.client.connState;
    widget.client.addListener(clientChanged);
    _configureDidReceiveLocalNotificationSubject();
    _configureSelectNotificationSubject();
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
    NotificationService().didReceiveLocalNotificationStream.close();
    NotificationService().selectNotificationStream.close();
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

    void _onItemTapped(int index) {
      setState(() {
        switch (index) {
          case 0:
            switchScreen(ChatsScreen.routeName);
            //Navigator.pop(context);
            break;
          case 1:
            switchScreen(FeedScreen.routeName);
            //Navigator.pop(context);
            break;
          case 2:
            switchScreen(ViewPageScreen.routeName);
            // Navigator.pop(context);
            break;
        }
        selectedIndex = index;
      });
    }

    var avatarColor = colorFromNick(client.nick);
    var darkTextColor = theme.indicatorColor;
    var hightLightTextColor = theme.dividerColor; // NAME TEXT COLOR
    var avatarTextColor =
        ThemeData.estimateBrightnessForColor(avatarColor) == Brightness.dark
            ? hightLightTextColor
            : darkTextColor;
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    return Scaffold(
      backgroundColor: theme.canvasColor,
      appBar: isScreenSmall
          ? AppBar(
              titleSpacing: 0.0,
              title: _OverviewScreenTitle(widget.mainMenu),
              leading: client.settingsPageTitle != "Settings"
                  ? Builder(builder: (BuildContext context) {
                      return IconButton(
                          iconSize: 20,
                          splashRadius: 20,
                          onPressed: () {
                            //client.settingsPageTitle = "Settings";
                            switchScreen(SettingsScreen.routeName);
                          },
                          icon: Icon(Icons.keyboard_arrow_left_rounded,
                              color: Theme.of(context).focusColor));
                    })
                  : Builder(builder: (BuildContext context) {
                      return removeBottomBar ||
                              client.active != null ||
                              client.showAddressBook
                          ? IconButton(
                              iconSize: 20,
                              splashRadius: 20,
                              onPressed: () {
                                client.activeSubMenu.isNotEmpty
                                    ? client.activeSubMenu = []
                                    : client.active = null;
                                if (removeBottomBar) {
                                  removeBottomBar = false;
                                  switchScreen(ChatsScreen.routeName);
                                  selectedIndex = 0;
                                }
                              },
                              icon: Icon(Icons.keyboard_arrow_left_rounded,
                                  color: Theme.of(context).focusColor))
                          : feed.active != null
                              ? IconButton(
                                  iconSize: 20,
                                  splashRadius: 20,
                                  icon: Icon(Icons.keyboard_arrow_left_rounded,
                                      color: Theme.of(context).focusColor),
                                  onPressed: () {
                                    feed.active = null;
                                    navKey.currentState!.pushReplacementNamed(
                                        '/feed',
                                        arguments: PageTabs(0, [], null));
                                  })
                              : Container(
                                  margin: const EdgeInsets.all(10),
                                  child: InkWell(
                                      onTap: () {
                                        switchScreen(SettingsScreen.routeName);
                                        removeBottomBar = true;
                                      },
                                      child: CircleAvatar(
                                          //radius: 10,
                                          backgroundColor:
                                              colorFromNick(client.nick),
                                          backgroundImage: client.myAvatar,
                                          child: client.myAvatar != null
                                              ? const Empty()
                                              : Text(
                                                  client.nick != ""
                                                      ? client.nick[0]
                                                          .toUpperCase()
                                                      : "",
                                                  style: TextStyle(
                                                      color: avatarTextColor,
                                                      fontSize: 20)))));
                    }),
              actions: [
                  widget.mainMenu.activeMenu.subMenuInfo.isNotEmpty
                      ? PageContextMenu(
                          menuItem: widget.mainMenu.activeMenu,
                          subMenu: widget.mainMenu.activeMenu.subMenuInfo,
                          navKey: navKey,
                        )
                      : const Empty()
                ])
          : AppBar(
              titleSpacing: 0.0,
              title: Row(children: [
                _OverviewScreenTitle(widget.mainMenu),
              ]),
              leadingWidth: 156,
              leading: Row(children: [
                IconButton(
                    tooltip: "About Bison Relay",
                    splashRadius: 20,
                    iconSize: 40,
                    onPressed: goToAbout,
                    icon: Image.asset(
                      "assets/images/icon.png",
                    )),
                IconButton(
                    splashRadius: 20,
                    tooltip: "Create a new post",
                    onPressed: goToNewPost,
                    color: Colors.red,
                    iconSize: 20,
                    icon:
                        Icon(color: theme.dividerColor, size: 20, Icons.mode)),
                IconButton(
                    splashRadius: 20,
                    tooltip: connStateLabel,
                    onPressed: connStateAction,
                    color: theme.dividerColor,
                    iconSize: 20,
                    icon: Icon(
                        color: theme.dividerColor, size: 20, connectedIcon)),
                const SizedBox(width: 20),
              ])),
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
                              (menuItem.label == "Feed" &&
                                  widget.feed.hasUnreadPostsComments)
                          ? Stack(children: [
                              Container(
                                  padding: const EdgeInsets.all(3),
                                  child: menuItem.icon ?? const Empty()),
                              const Positioned(
                                  top: 1,
                                  right: 1,
                                  child: CircleAvatar(
                                      backgroundColor: Colors.red, radius: 4)),
                            ])
                          : Container(
                              padding: const EdgeInsets.all(3),
                              child: menuItem.icon),
                      title: Consumer<ThemeNotifier>(
                          builder: (context, theme, child) => Text(
                              menuItem.label,
                              style: TextStyle(
                                  fontSize: theme.getMediumFont(context)))))
                  : Theme(
                      data: Theme.of(context)
                          .copyWith(dividerColor: Colors.transparent),
                      child: ExpansionTile(
                        title: Text(menuItem.label),
                        initiallyExpanded:
                            widget.mainMenu.activeMenu.label == menuItem.label,
                        leading: (menuItem.label == "Chats" &&
                                    client.hasUnreadChats) ||
                                (menuItem.label == "Feed" &&
                                    widget.feed.hasUnreadPostsComments)
                            ? Stack(children: [
                                Container(
                                    padding: const EdgeInsets.all(3),
                                    child: menuItem.icon ?? const Empty()),
                                const Positioned(
                                    top: 1,
                                    right: 1,
                                    child: CircleAvatar(
                                        backgroundColor: Colors.red,
                                        radius: 4)),
                              ])
                            : Container(
                                padding: const EdgeInsets.all(3),
                                child: menuItem.icon),
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
                : Sidebar(widget.client, widget.mainMenu, widget.ntfns, navKey,
                    widget.feed),
            Expanded(
              child: Navigator(
                key: navKey,
                initialRoute: widget.initialRoute == ""
                    ? ChatsScreen.routeName
                    : widget.initialRoute,
                onGenerateRoute: (settings) {
                  String routeName = settings.name!;
                  MainMenuItem? menu = widget.mainMenu.menuForRoute(routeName);

                  // This update needs to be on a timer so that it is decoupled to
                  // the widget build stack frame.
                  Timer(const Duration(milliseconds: 1),
                      () async => widget.mainMenu.activeRoute = routeName);

                  return PageRouteBuilder(
                    pageBuilder: (context, animation, secondaryAnimation) =>
                        menu != null
                            ? menu.builder(context)
                            : RouteErrorPage(
                                settings.name ?? "", OverviewScreen.routeName),
                    transitionDuration: Duration.zero,
                    //reverseTransitionDuration: Duration.zero,
                    settings: settings,
                  );
                },
              ),
            )
          ])),
      bottomNavigationBar: isScreenSmall && !removeBottomBar
          ? BottomNavigationBar(
              iconSize: 40,
              items: <BottomNavigationBarItem>[
                BottomNavigationBarItem(
                  icon: client.hasUnreadChats
                      ? Stack(children: [
                          Container(
                              padding: const EdgeInsets.all(3),
                              child: const SidebarSvgIcon(
                                  "assets/icons/icons-menu-chat.svg")),
                          const Positioned(
                              top: 1,
                              right: 1,
                              child: CircleAvatar(
                                  backgroundColor: Colors.red, radius: 4)),
                        ])
                      : Container(
                          padding: const EdgeInsets.all(3),
                          child: const SidebarSvgIcon(
                              "assets/icons/icons-menu-chat.svg")),
                  label: 'Chat',
                ),
                BottomNavigationBarItem(
                  icon: widget.feed.hasUnreadPostsComments
                      ? Stack(children: [
                          Container(
                              padding: const EdgeInsets.all(3),
                              child: const SidebarSvgIcon(
                                  "assets/icons/icons-menu-news.svg")),
                          const Positioned(
                              top: 1,
                              right: 1,
                              child: CircleAvatar(
                                  backgroundColor: Colors.red, radius: 4)),
                        ])
                      : Container(
                          padding: const EdgeInsets.all(3),
                          child: const SidebarSvgIcon(
                              "assets/icons/icons-menu-news.svg")),
                  label: 'Feed',
                ),
                BottomNavigationBarItem(
                  icon: Container(
                      padding: const EdgeInsets.all(3),
                      child: const SidebarSvgIcon(
                          "assets/icons/icons-menu-pages.svg")),
                  label: 'Pages',
                ),
              ],

              currentIndex: selectedIndex, //New
              onTap: _onItemTapped, //New
            )
          : null,
    );
  }
}
