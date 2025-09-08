import 'dart:async';

import 'package:bruig/components/clipper.dart';
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
import 'package:bruig/models/realtimechat.dart';
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
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

// These are hacks. Find a way to remove them.
final GlobalKey<ScaffoldState> scaffoldKey = GlobalKey<ScaffoldState>();
final GlobalKey<NavigatorState> overviewNavKey = GlobalKey<NavigatorState>();

class OverviewNavigatorModel extends ChangeNotifier {
  final GlobalKey<NavigatorState> navKey;

  OverviewNavigatorModel(this.navKey);

  static OverviewNavigatorModel of(BuildContext context,
          {bool listen = true}) =>
      Provider.of<OverviewNavigatorModel>(context, listen: listen);
}

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
  final RealtimeChatModel rtc;
  const OverviewScreen(this.down, this.client, this.ntfns, this.initialRoute,
      this.mainMenu, this.feed, this.snackBar, this.rtc,
      {super.key});

  @override
  State<OverviewScreen> createState() => _OverviewScreenState();
}

class _OverviewScreenAppBarConnState {
  final Widget tag;

  _OverviewScreenAppBarConnState({required this.tag});
}

const _connStateTagClipPath =
    "M 0.31234165,80.167689 79.855347,0 37.064542,0.10411388 0,37.793339 Z";

const connStateUpdate = 999;

final _connStateStyles = {
  connStateCheckingWallet: _OverviewScreenAppBarConnState(
      tag: ClipPath(
          clipper:
              SVGClipper(_connStateTagClipPath, offset: const Offset(-10, 0)),
          child: Image.asset("assets/images/checktag.png", width: 50))),
  connStateOffline: _OverviewScreenAppBarConnState(
      tag: ClipPath(
          clipper:
              SVGClipper(_connStateTagClipPath, offset: const Offset(-10, 0)),
          child: Image.asset("assets/images/offlinetag.png", width: 50))),
  connStateOnline: _OverviewScreenAppBarConnState(tag: const Empty()),
  connStateUpdate: _OverviewScreenAppBarConnState(
      tag: ClipPath(
          clipper:
              SVGClipper(_connStateTagClipPath, offset: const Offset(-10, 0)),
          child: Image.asset("assets/images/updatetag.png", width: 50))),
};

class _MainAppBar extends StatefulWidget {
  final ClientModel client;
  final FeedModel feed;
  final RealtimeChatModel rtc;
  final MainMenuModel mainMenu;
  final GlobalKey<NavigatorState> navKey;
  const _MainAppBar(
      this.client, this.feed, this.rtc, this.mainMenu, this.navKey);

  @override
  State<_MainAppBar> createState() => __MainAppBarState();
}

class __MainAppBarState extends State<_MainAppBar>
    with SingleTickerProviderStateMixin {
  GlobalKey<NavigatorState> get navKey => widget.navKey;
  MainMenuModel get mainMenu => widget.mainMenu;
  ClientModel get client => widget.client;
  FeedModel get feed => widget.feed;
  RealtimeChatModel get rtc => widget.rtc;

  late AnimationController bgColorCtrl;
  late Animation<Color?> bgColorAnim;

  bool hasLiveRTCSess = false;
  bool hasHotAudio = false;
  bool get hasAnimation => hasLiveRTCSess || hasHotAudio;

  void goToNewPost(BuildContext context) {
    navKey.currentState
        ?.pushReplacementNamed('/feed', arguments: PageTabs(3, null, null));
  }

  void goToAbout(BuildContext context) {
    Navigator.of(context, rootNavigator: true).pushNamed("/about");
  }

  void switchScreen(String route, {Object? args}) {
    navKey.currentState!.pushReplacementNamed(route, arguments: args);
  }

  void rtcChanged() {
    bool newHasHotAudio = rtc.hotAudioSession.active?.inLiveSession ?? false;
    bool newHasLive = rtc.liveSessions.hasSessions;
    if (newHasLive != hasLiveRTCSess || newHasHotAudio != hasHotAudio) {
      setState(() {
        hasLiveRTCSess = newHasLive;
        hasHotAudio = newHasHotAudio;
      });
      if (hasAnimation) {
        bgColorCtrl.repeat();
      } else {
        bgColorCtrl.stop();
      }
    }
  }

  @override
  void initState() {
    super.initState();

    rtc.hotAudioSession.addListener(rtcChanged);
    rtc.liveSessions.addListener(rtcChanged);

    // Initialize animation controller
    bgColorCtrl = AnimationController(
      duration: const Duration(seconds: 2),
      vsync: this,
    );

    // Create the color animation sequence
    bgColorAnim = TweenSequence<Color?>([
      TweenSequenceItem(
        weight: 1.0,
        tween: ColorTween(
          begin: Colors.green.shade600,
          end: Colors.green.shade900,
        ),
      ),
      TweenSequenceItem(
        weight: 1.0,
        tween: ColorTween(
          begin: Colors.green.shade900,
          end: Colors.green.shade600,
        ),
      ),
    ]).animate(bgColorCtrl);
  }

  @override
  void dispose() {
    bgColorCtrl.dispose();
    rtc.hotAudioSession.removeListener(rtcChanged);
    rtc.liveSessions.removeListener(rtcChanged);
    super.dispose();
  }

  AppBar buildAppBar(BuildContext context) {
    bool isScreenSmall = checkIsScreenSmall(context);

    if (!isScreenSmall) {
      return AppBar(
          titleSpacing: 0.0,
          title: ChangeNotifierProvider.value(
              value: OverviewNavigatorModel(navKey),
              builder: (context, _) => const _OverviewScreenTitle()),
          leadingWidth: 112,
          backgroundColor:
              hasHotAudio || hasLiveRTCSess ? bgColorAnim.value : null,
          leading: Row(children: [
            Consumer<ConnStateModel>(builder: (context, connState, child) {
              var connStateTagKey = connState.state.state;
              if (connStateTagKey == connStateOnline &&
                  connState.suggestedVersion != "") {
                connStateTagKey = connStateUpdate;
              }
              return Stack(children: [
                Row(children: [
                  const SizedBox(width: 10),
                  IconButton(
                      tooltip: "About Bison Relay",
                      splashRadius: 20,
                      iconSize: 40,
                      onPressed: () => goToAbout(context),
                      icon: Image.asset(
                        "assets/images/icon.png",
                      ))
                ]),
                _connStateStyles[connStateTagKey]?.tag ??
                    const SizedBox(width: 100),
              ]);
            }),
            IconButton(
                splashRadius: 20,
                tooltip: "Create a new post",
                onPressed: () => goToNewPost(context),
                iconSize: 20,
                icon: const Icon(size: 20, Icons.mode)),
            const SizedBox(width: 20),
          ]));
    }

    List<ChatMenuItem?> contextMenu = [];
    if (mainMenu.activeMenu.label == "Chat") {
      contextMenu = buildChatContextMenu(navKey);
    }

    return AppBar(
        leadingWidth: 60,
        titleSpacing: 0.0,
        title: ChangeNotifierProvider.value(
            value: OverviewNavigatorModel(navKey),
            builder: (context, _) => const _OverviewScreenTitle()),
        backgroundColor:
            hasHotAudio || hasLiveRTCSess ? bgColorAnim.value : null,
        leading: Builder(builder: (BuildContext context) {
          return InkWell(onTap: () {
            // if (client.ui.showAddressBook.val) { // FIXME: How is this triggered?
            //   client.ui.showAddressBook.val = false;
            // } else
            if (!client.ui.chatSideMenuActive.empty) {
              client.ui.chatSideMenuActive.chat = null;
            } else if (client.ui.showProfile.val) {
              client.ui.showProfile.val = false;
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
              switchScreen(FeedScreen.routeName, args: PageTabs(0, null, null));
            } else {
              switchScreen(SettingsScreen.routeName);
            }
          }, child: Consumer5<OverviewActivePath, ActiveChatModel, FeedModel,
                  ChatSideMenuActiveModel, ConnStateModel>(
              builder: (context, overviewActivePath, activeChat, feed,
                  chatSideMenuActive, connState, child) {
            var connStateTagKey = connState.state.state;
            if (connStateTagKey == connStateOnline &&
                connState.suggestedVersion != "") {
              connStateTagKey = connStateUpdate;
            }

            return Stack(children: [
              !overviewActivePath.onActiveBottomTab ||
                      !activeChat.empty ||
                      feed.active != null ||
                      !chatSideMenuActive.empty
                  ? const Positioned(
                      left: 25,
                      top: 17,
                      child: Icon(Icons.keyboard_arrow_left_rounded))
                  : Container(
                      margin: const EdgeInsets.all(10),
                      child: SelfAvatar(client)),
              _connStateStyles[connStateTagKey]?.tag ?? const Empty(),
            ]);
          }));
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

  @override
  Widget build(BuildContext context) {
    if (hasAnimation) {
      return AnimatedBuilder(
          animation: bgColorAnim,
          builder: (context, child) => buildAppBar(context));
    }

    return buildAppBar(context);
  }
}

class _OverviewScreenState extends State<OverviewScreen> {
  ClientModel get client => widget.client;
  AppNotifications get ntfns => widget.ntfns;
  DownloadsModel get down => widget.down;
  FeedModel get feed => widget.feed;
  RealtimeChatModel get rtc => widget.rtc;
  ServerSessionState connState = ServerSessionState.empty();
  GlobalKey<NavigatorState> navKey =
      overviewNavKey; // GlobalKey(debugLabel: "overview nav key");

  bool removeBottomBar = false;
  var selectedIndex = 0;
  bool hasInstantCall = false;

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

  void checkInstantCall() {
    var newInstantCallState = rtc.active.active != null;
    if (newInstantCallState != hasInstantCall) {
      setState(() {
        hasInstantCall = newInstantCallState;
        if (hasInstantCall) {
          removeBottomBar = true;
        } else {
          removeBottomBar = false;
        }
      });
    }
  }

  void goToSubMenuPage(String route, int pageTab) {
    navKey.currentState!
        .pushReplacementNamed(route, arguments: PageTabs(pageTab, null, null));
    Timer(const Duration(milliseconds: 1),
        () async => widget.mainMenu.activePageTab = pageTab);
    Navigator.pop(context);
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
      debugPrint("Bruig: Processing system notification (payload $payload)");
      if (payload != null) {
        if (payload.startsWith("chat:") || payload.startsWith("gc:")) {
          switchScreen(ChatsScreen.routeName);
          var uid = payload.split(":")[1];
          bool isGC = payload.startsWith("gc:");
          if (uid.length > 1) {
            client.setActiveByUID(uid, isGC: isGC);
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
    widget.rtc.active.addListener(checkInstantCall);
    _configureSelectNotificationSubject();
  }

  @override
  void didUpdateWidget(OverviewScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.client != widget.client) {
      oldWidget.client.connState.removeListener(connStateChanged);
      widget.client.connState.addListener(connStateChanged);
    }
    if (oldWidget.rtc.active != widget.rtc.active) {
      oldWidget.rtc.active.removeListener(checkInstantCall);
      widget.rtc.active.addListener(checkInstantCall);
    }
  }

  @override
  void dispose() {
    widget.client.active?.removeListener(checkInstantCall);
    widget.client.connState.removeListener(connStateChanged);
    NotificationService().selectNotificationStream.close();
    super.dispose();
  }

  void switchScreen(String route) {
    // Do not change screen if already there.
    String currentPath = "";
    navKey.currentState?.popUntil((route) {
      currentPath = route.settings.name ?? "";
      return true;
    });

    if (currentPath == route) {
      return;
    }

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
    bool isScreenSmall = checkIsScreenSmall(context);
    return Scaffold(
      // key: scaffoldKey,
      appBar: PreferredSize(
        preferredSize: const Size.fromHeight(kToolbarHeight),
        child: _MainAppBar(client, feed, widget.rtc, widget.mainMenu, navKey),
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
                observers: [client.ui.overviewRouteObserver],
                initialRoute: widget.initialRoute == ""
                    ? ChatsScreen.routeName
                    : widget.initialRoute,
                onGenerateRoute: (settings) {
                  String routeName = settings.name!;
                  MainMenuItem? menu = widget.mainMenu.menuForRoute(routeName);

                  // These updates needs to be on a timer so that they are decoupled to
                  // the widget build stack frame.
                  Timer(const Duration(milliseconds: 1), () async {
                    widget.mainMenu.activeRoute = routeName;
                    client.ui.overviewActivePath.route = routeName;
                  });

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
                    selectedFontSize: fontSize(TextSize.large)!,
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
