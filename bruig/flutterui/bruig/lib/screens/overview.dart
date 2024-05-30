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
import 'package:bruig/models/uistate.dart';
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

class _OverviewScreenTitle extends StatelessWidget {
  const _OverviewScreenTitle();

  @override
  Widget build(BuildContext context) {
    return Consumer<MainMenuModel>(
        builder: (context, mainMenu, child) =>
            mainMenu.activeMenu.titleBuilder(context));
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

class _OverviewScreenAppBarConnState {
  final IconData icon;
  final String label;
  final Widget tag;

  _OverviewScreenAppBarConnState(
      {required this.icon, required this.label, required this.tag});
}

final _connStateStyles = {
  connStateCheckingWallet: _OverviewScreenAppBarConnState(
      icon: Icons.cloud_off,
      label: "Skip Wallet Check",
      tag: const Image(
        color: null,
        image: AssetImage("assets/images/checktag.png"),
      )),
  connStateOffline: _OverviewScreenAppBarConnState(
      icon: Icons.cloud_off,
      label: "Go online",
      tag: const Image(
        color: null,
        image: AssetImage("assets/images/offlinetag.png"),
      )),
  connStateOnline: _OverviewScreenAppBarConnState(
      icon: Icons.cloud, label: "Go offline", tag: const Empty()),
};

AppBar _buildAppBar(BuildContext context, ClientModel client, FeedModel feed,
    MainMenuModel mainMenu, GlobalKey<NavigatorState> navKey) {
  void goToNewPost(BuildContext context) {
    navKey.currentState
        ?.pushReplacementNamed('/feed', arguments: PageTabs(3, null, null));
  }

  void goToAbout(BuildContext context) {
    Navigator.of(context, rootNavigator: true).pushNamed("/about");
  }

  void goOnline(BuildContext context) async {
    try {
      await Golib.goOnline();
      showSuccessSnackbar(context, "Going online...");
    } catch (exception) {
      showErrorSnackbar(context, "Unable to go online: $exception");
    }
  }

  void remainOffline(BuildContext context) async {
    try {
      await Golib.remainOffline();
      showSuccessSnackbar(context, "Going offline...");
    } catch (exception) {
      showErrorSnackbar(context, "Unable to go offline: $exception");
    }
  }

  void skipWalletCheck(BuildContext context) async {
    try {
      await Golib.skipWalletCheck();
      showSuccessSnackbar(context, "Skipping next wallet check...");
    } catch (exception) {
      showErrorSnackbar(context, "Unable to skip wallet check: $exception");
    }
  }

  void takeConnStateAction(BuildContext context, int state) {
    switch (state) {
      case connStateCheckingWallet:
        skipWalletCheck(context);
        break;
      case connStateOffline:
        goOnline(context);
        break;
      case connStateOnline:
        remainOffline(context);
        break;
    }
  }

  void switchScreen(String route, {Object? args}) {
    navKey.currentState!.pushReplacementNamed(route, arguments: args);
  }

  var theme = Theme.of(context);
  var sidebarBackground = theme.backgroundColor;

  bool isScreenSmall = checkIsScreenSmall(context);

  if (!isScreenSmall) {
    return AppBar(
        backgroundColor: sidebarBackground,
        titleSpacing: 0.0,
        title: const _OverviewScreenTitle(),
        leadingWidth: 156,
        leading: Row(children: [
          IconButton(
              tooltip: "About Bison Relay",
              splashRadius: 20,
              iconSize: 40,
              onPressed: () => goToAbout(context),
              icon: Image.asset(
                "assets/images/icon.png",
              )),
          IconButton(
              splashRadius: 20,
              tooltip: "Create a new post",
              onPressed: () => goToNewPost(context),
              color: Colors.red,
              iconSize: 20,
              icon: Icon(color: theme.dividerColor, size: 20, Icons.mode)),
          Consumer<ConnStateModel>(builder: (context, connState, child) {
            var stateStyle = _connStateStyles[connState.state.state];
            return IconButton(
                splashRadius: 20,
                tooltip: stateStyle?.label ??
                    "Unknown state ${connState.state.state}",
                onPressed: () =>
                    takeConnStateAction(context, connState.state.state),
                color: theme.dividerColor,
                iconSize: 20,
                icon: Icon(
                    color: theme.dividerColor,
                    size: 20,
                    stateStyle?.icon ?? Icons.question_mark));
          }),
          const SizedBox(width: 20),
        ]));
  }

  List<ChatMenuItem?> contextMenu = [];
  if (mainMenu.activeMenu.label == "Chat") {
    contextMenu = buildChatContextMenu();
  }

  return AppBar(
      backgroundColor: sidebarBackground,
      leadingWidth: 60,
      titleSpacing: 0.0,
      title: const _OverviewScreenTitle(),
      leading: Builder(builder: (BuildContext context) {
        return InkWell(
            onTap: () {
              if (client.ui.showAddressBook.val) {
                client.ui.showAddressBook.val = false;
              } else if (!client.ui.overviewActivePath.onActiveBottomTab ||
                  client.active != null) {
                !client.ui.chatSideMenuActive.empty
                    ? client.ui.chatSideMenuActive.clear()
                    : client.active = null;
                if (!client.ui.overviewActivePath.onActiveBottomTab) {
                  switchScreen(ChatsScreen.routeName);
                }
              } else if (feed.active != null) {
                feed.active = null;
                switchScreen(FeedScreen.routeName,
                    args: PageTabs(0, null, null));
              } else {
                switchScreen(SettingsScreen.routeName);
              }
            },
            child: Consumer4<OverviewActivePath, ActiveChatModel,
                    ShowAddressBookModel, FeedModel>(
                builder: (context, overviewActivePath, activeChat, showAddrBook,
                        feed, child) =>
                    Stack(children: [
                      !overviewActivePath.onActiveBottomTab ||
                              !activeChat.empty ||
                              showAddrBook.val ||
                              feed.active != null
                          ? Positioned(
                              left: 25,
                              top: 17,
                              child: Icon(Icons.keyboard_arrow_left_rounded,
                                  color: Theme.of(context).focusColor))
                          : Container(
                              margin: const EdgeInsets.all(10),
                              child: SelfAvatar(client)),
                      // connectedTag, // Tag when offline/checking wallet.
                    ])));
      }),
      actions: [
        // Only render page context menu if the mainMenu ONLY has
        // a context menu OR a sub page menu.
        (mainMenu.activeMenu.subMenuInfo.isNotEmpty && contextMenu.isEmpty) ||
                (contextMenu.isNotEmpty &&
                    mainMenu.activeMenu.subMenuInfo.isEmpty)
            ? PageContextMenu(
                menuItem: mainMenu.activeMenu,
                subMenu: mainMenu.activeMenu.subMenuInfo,
                contextMenu: contextMenu,
                navKey: navKey,
              )
            : const Empty()
      ]);
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

  void _onItemTapped(int index) {
    setState(() {
      switch (index) {
        case 0:
          switchScreen(ChatsScreen.routeName);
          client.ui.smallScreenActiveTab.active = SmallScreenActiveTab.chat;
          //Navigator.pop(context);
          break;
        case 1:
          switchScreen(FeedScreen.routeName);
          client.ui.smallScreenActiveTab.active = SmallScreenActiveTab.feed;
          //Navigator.pop(context);
          break;
        case 2:
          switchScreen(ViewPageScreen.routeName);
          client.ui.smallScreenActiveTab.active = SmallScreenActiveTab.pages;
          // Navigator.pop(context);
          break;
      }
      selectedIndex = index;
    });
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var selectedColor = theme.dividerColor;
    var unselectedTextColor = theme.focusColor;
    var sidebarBackground = theme.backgroundColor;
    var scaffoldBackgroundColor = theme.canvasColor;
    var hoverColor = theme.hoverColor;

    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    return Scaffold(
      key: scaffoldKey,
      backgroundColor: theme.canvasColor,
      appBar: _buildAppBar(context, client, feed, widget.mainMenu, navKey),
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
                      leading: (menuItem.label == "Chat" &&
                                  client.activeChats.hasUnreadMsgs) ||
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
                                    client.activeChats.hasUnreadMsgs) ||
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
                  client.ui.overviewActivePath.route = routeName;
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
                        icon: client.activeChats.hasUnreadMsgs
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
