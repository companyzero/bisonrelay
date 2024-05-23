import 'dart:async';

import 'package:bruig/components/indicator.dart';
import 'package:bruig/components/interactive_avatar.dart';
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

final GlobalKey<ScaffoldState> scaffoldKey = GlobalKey<ScaffoldState>();

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
  final ChatModel? userPostList;
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
  void connStateChanged() {
    var newConnState = client.connState.state;
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
        .pushReplacementNamed(route, arguments: PageTabs(pageTab, null, null));
    Timer(const Duration(milliseconds: 1),
        () async => widget.mainMenu.activePageTab = pageTab);
    Navigator.pop(context);
  }

  void goToNewPost() {
    navKey.currentState!
        .pushReplacementNamed('/feed', arguments: PageTabs(3, null, null));
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
                  arguments: PageTabs(0, null, PostContentScreenArgs(post)));
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
    connState = widget.client.connState.state;
    widget.client.connState.addListener(connStateChanged);
    _configureDidReceiveLocalNotificationSubject();
    _configureSelectNotificationSubject();
  }

  @override
  void didUpdateWidget(OverviewScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.client != widget.client) {
      oldWidget.client.connState.removeListener(connStateChanged);
      widget.client.connState.addListener(connStateChanged);
    }
  }

  @override
  void dispose() {
    widget.client.connState.removeListener(connStateChanged);
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
    Widget connectedTag = const Empty(); // Used in small screens.
    List<ChatMenuItem?> contextMenu = [];
    if (widget.mainMenu.activeMenu.label == "Chat") {
      contextMenu = buildChatContextMenu();
    }
    String connStateLabel;
    GestureTapCallback connStateAction;

    switch (connState.state) {
      case connStateCheckingWallet:
        connectedIcon = Icons.cloud_off;
        connStateLabel = "Skip Wallet Check";
        connStateAction = skipWalletCheck;
        connectedTag = const Image(
          color: null,
          image: AssetImage("assets/images/checktag.png"),
        );
        break;
      case connStateOffline:
        connectedIcon = Icons.cloud_off;
        connStateLabel = "Go Online";
        connStateAction = goOnline;
        connectedTag = const Image(
          color: null,
          image: AssetImage("assets/images/offlinetag.png"),
        );
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

    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    return Scaffold(
      key: scaffoldKey,
      backgroundColor: theme.canvasColor,
      appBar: isScreenSmall
          ? AppBar(
              backgroundColor: sidebarBackground,
              leadingWidth: 60,
              titleSpacing: 0.0,
              title: _OverviewScreenTitle(widget.mainMenu),
              leading: Builder(builder: (BuildContext context) {
                return InkWell(
                    onTap: () {
                      if (removeBottomBar ||
                          client.active != null ||
                          client.showAddressBook) {
                        client.activeSubMenu.isNotEmpty
                            ? client.activeSubMenu = []
                            : client.active = null;
                        if (removeBottomBar) {
                          removeBottomBar = false;
                          switchScreen(ChatsScreen.routeName);
                          selectedIndex = 0;
                        }
                      } else if (feed.active != null) {
                        feed.active = null;
                        navKey.currentState!.pushReplacementNamed('/feed',
                            arguments: PageTabs(0, null, null));
                      } else {
                        switchScreen(SettingsScreen.routeName);
                        removeBottomBar = true;
                      }
                    },
                    child: Stack(children: [
                      removeBottomBar ||
                              client.active != null ||
                              client.showAddressBook ||
                              feed.active != null
                          ? Positioned(
                              left: 25,
                              top: 17,
                              child: Icon(Icons.keyboard_arrow_left_rounded,
                                  color: Theme.of(context).focusColor))
                          : Container(
                              margin: const EdgeInsets.all(10),
                              child: SelfAvatar(client)),
                      connectedTag, // Tag when offline/checking wallet.
                    ]));
              }),
              actions: [
                  // Only render page context menu if the mainMenu ONLY has
                  // a context menu OR a sub page menu.
                  (widget.mainMenu.activeMenu.subMenuInfo.isNotEmpty &&
                              contextMenu.isEmpty) ||
                          (contextMenu.isNotEmpty &&
                              widget.mainMenu.activeMenu.subMenuInfo.isEmpty)
                      ? PageContextMenu(
                          menuItem: widget.mainMenu.activeMenu,
                          subMenu: widget.mainMenu.activeMenu.subMenuInfo,
                          contextMenu: contextMenu,
                          navKey: navKey,
                        )
                      : const Empty()
                ])
          : AppBar(
              backgroundColor: sidebarBackground,
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
                      leading:
                          (menuItem.label == "Chat" && client.hasUnreadChats) ||
                                  (menuItem.label == "Feed" &&
                                      widget.feed.hasUnreadPostsComments)
                              ? Stack(children: [
                                  Container(
                                      padding: const EdgeInsets.all(3),
                                      child: menuItem.icon ?? const Empty()),
                                  const Positioned(
                                      top: 1,
                                      right: 1,
                                      child: RedDotIndicator()),
                                ])
                              : Container(
                                  padding: const EdgeInsets.all(3),
                                  child: menuItem.icon),
                      title: Consumer<ThemeNotifier>(
                          builder: (context, theme, _) => Text(menuItem.label,
                              style: TextStyle(
                                  fontSize: theme.getMediumFont(context)))))
                  : Theme(
                      data: Theme.of(context)
                          .copyWith(dividerColor: Colors.transparent),
                      child: ExpansionTile(
                        title: Text(menuItem.label),
                        initiallyExpanded:
                            widget.mainMenu.activeMenu.label == menuItem.label,
                        leading: (menuItem.label == "Chat" &&
                                    client.hasUnreadChats) ||
                                (menuItem.label == "Feed" &&
                                    widget.feed.hasUnreadPostsComments)
                            ? Stack(children: [
                                Container(
                                    padding: const EdgeInsets.all(3),
                                    child: menuItem.icon ?? const Empty()),
                                const Positioned(
                                    top: 1, right: 1, child: RedDotIndicator()),
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
          ? Consumer<ThemeNotifier>(
              builder: (context, theme, _) => BottomNavigationBar(
                    selectedFontSize: theme.getLargeFont(context),
                    unselectedFontSize: theme.getMediumFont(context),
                    selectedItemColor: selectedColor,
                    unselectedItemColor: unselectedTextColor,
                    selectedLabelStyle:
                        const TextStyle(fontWeight: FontWeight.w700),
                    unselectedLabelStyle:
                        const TextStyle(fontWeight: FontWeight.w200),
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
                                    top: 1, right: 1, child: RedDotIndicator()),
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
                                    top: 1, right: 1, child: RedDotIndicator()),
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
                  ))
          : null,
    );
  }
}
