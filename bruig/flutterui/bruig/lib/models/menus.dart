import 'package:bruig/components/pay_tip.dart';
import 'package:bruig/components/rename_chat.dart';
import 'package:bruig/components/suggest_kx.dart';
import 'package:bruig/components/trans_reset.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/log.dart';
import 'package:bruig/models/notifications.dart';
import 'package:bruig/models/resources.dart';
import 'package:bruig/screens/chats.dart';
import 'package:bruig/screens/feed.dart';
import 'package:bruig/screens/ln_management.dart';
import 'package:bruig/screens/log.dart';
import 'package:bruig/screens/manage_content_screen.dart';
import 'package:bruig/screens/paystats.dart';
import 'package:bruig/screens/settings.dart';
import 'package:bruig/screens/viewpage_screen.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';
import 'package:file_picker/file_picker.dart';
import 'package:path/path.dart' as path;
import 'package:flutter_svg/flutter_svg.dart';

class MainMenuItem {
  final String label;
  final String routeName;
  final WidgetBuilder builder;
  final WidgetBuilder titleBuilder;
  final Widget? icon;
  final List<SubMenuInfo> subMenuInfo;

  MainMenuItem(this.label, this.routeName, this.builder, this.titleBuilder,
      this.icon, this.subMenuInfo);
}

MainMenuItem _emptyMenu = MainMenuItem("", "", (context) => const Text(""),
    (context) => const Text(""), null, <SubMenuInfo>[]);

class SubMenuInfo {
  final int pageTab;
  final String label;
  SubMenuInfo(this.pageTab, this.label);
}

final List<SubMenuInfo> FeedScreenSub = [
  SubMenuInfo(0, "Feed"),
  SubMenuInfo(1, "Your Posts"),
  SubMenuInfo(2, "Subscriptions"),
  SubMenuInfo(3, "New Post")
];

final List<SubMenuInfo> ManageContentScreenSub = [
  SubMenuInfo(0, "Add"),
  SubMenuInfo(1, "Shared"),
  SubMenuInfo(2, "Downloads"),
];

final List<SubMenuInfo> LnScreenSub = [
  SubMenuInfo(0, "Overview"),
  SubMenuInfo(1, "Accounts"),
  SubMenuInfo(2, "On-Chain"),
  SubMenuInfo(3, "Channels"),
  SubMenuInfo(4, "Payments"),
  SubMenuInfo(5, "Network"),
  SubMenuInfo(6, "Backups")
];

final List<MainMenuItem> mainMenu = [
  MainMenuItem(
      "Chats",
      ChatsScreen.routeName,
      (context) => Consumer2<ClientModel, AppNotifications>(
          builder: (context, client, ntfns, child) =>
              ChatsScreen(client, ntfns)),
      (context) => const ChatsScreenTitle(),
      const SidebarSvgIcon("assets/icons/icons-menu-chat.svg"),
      <SubMenuInfo>[]),
  MainMenuItem(
      "Feed",
      FeedScreen.routeName,
      (context) => Consumer<MainMenuModel>(
          builder: (context, menu, child) => FeedScreen(menu)),
      (context) => const FeedScreenTitle(),
      const SidebarSvgIcon("assets/icons/icons-menu-news.svg"),
      FeedScreenSub),
  MainMenuItem(
      "LN Management",
      LNScreen.routeName,
      (context) => Consumer<MainMenuModel>(
          builder: (context, menu, child) => LNScreen(menu)),
      (context) => const LNScreenTitle(),
      const SidebarSvgIcon("assets/icons/icons-menu-lnmng.svg"),
      LnScreenSub),
  MainMenuItem(
      "Pages Browser",
      ViewPageScreen.routeName,
      (context) => Consumer2<ClientModel, ResourcesModel>(
          builder: (context, client, resources, child) =>
              ViewPageScreen(resources, client)),
      (context) => const ViewPagesScreenTitle(),
      const SidebarSvgIcon("assets/icons/icons-menu-pages.svg"),
      <SubMenuInfo>[]),
  MainMenuItem(
      "Manage Content",
      ManageContentScreen.routeName,
      (context) => Consumer<MainMenuModel>(
          builder: (context, menu, child) => ManageContentScreen(menu)),
      (context) => const ManageContentScreenTitle(),
      const SidebarSvgIcon("assets/icons/icons-menu-files.svg"),
      ManageContentScreenSub),
  MainMenuItem(
      "Stats",
      PayStatsScreen.routeName,
      (context) => Consumer<ClientModel>(
          builder: (context, client, child) => PayStatsScreen(client)),
      (context) => const PayStatsScreenTitle(),
      const SidebarSvgIcon("assets/icons/icons-menu-stats.svg"),
      <SubMenuInfo>[]),
  MainMenuItem(
      "Settings",
      SettingsScreen.routeName,
      (context) => Consumer<ClientModel>(
          builder: (context, client, child) => SettingsScreen(client)),
      (context) => const SettingsScreenTitle(),
      const SidebarSvgIcon("assets/icons/icons-menu-settings.svg"),
      <SubMenuInfo>[]),
  MainMenuItem(
      "Logs",
      LogScreen.routeName,
      (context) =>
          Consumer<LogModel>(builder: (context, log, child) => LogScreen(log)),
      (context) => const LogScreenTitle(),
      const SidebarIcon(Icons.list_rounded, false),
      <SubMenuInfo>[]),
];

class MainMenuModel extends ChangeNotifier {
  final List<MainMenuItem> menus = mainMenu;

  String _activeRoute = "";
  MainMenuItem _activeMenu = _emptyMenu;
  int _activePageTab = 0;
  int _activeIndex = 0;
  int get activeIndex => _activeIndex;
  MainMenuItem get activeMenu => _activeMenu;
  String get activeRoute => _activeRoute;
  set activeRoute(String newRoute) {
    var idx = menus.indexWhere((e) => e.routeName == newRoute);
    if (idx < 0) {
      return;
    }
    _activeMenu = menus[idx];
    _activeRoute = newRoute;
    _activeIndex = idx;
    _activePageTab = 0;
    notifyListeners();
  }

  int get activePageTab => _activePageTab;
  set activePageTab(int pageTab) {
    _activePageTab = pageTab;
    notifyListeners();
  }

  MainMenuItem? menuForRoute(String route) {
    var idx = menus.indexWhere((e) => e.routeName == route);
    if (idx < 0) {
      return null;
    }
    return menus[idx];
  }
}

class ChatMenuItem {
  final String label;
  final Function(BuildContext context, ClientModel chats) onSelected;
  const ChatMenuItem(this.label, this.onSelected);
}

List<ChatMenuItem?> buildUserChatMenu(ChatModel chat) {
  void sendFile(BuildContext context, ChatModel chat) async {
    var filePickRes = await FilePicker.platform.pickFiles();
    if (filePickRes == null) return;
    var filePath = filePickRes.files.first.path;
    if (filePath == null) return;
    filePath = filePath.trim();
    if (filePath == "") return;

    try {
      await Golib.sendFile(chat.id, filePath);
      var fname = path.basename(filePath);
      chat.append(
          ChatEventModel(
              SynthChatEvent("Sending file \"$fname\" to user", SCE_sent),
              null),
          false);
    } catch (exception) {
      showErrorSnackbar(context, "Unable to send file: $exception");
    }
  }

  void listUserPosts(
      BuildContext context, ClientModel client, ChatModel chat) async {
    var event = SynthChatEvent("Listing user posts", SCE_sending);
    try {
      chat.append(ChatEventModel(event, null), false);
      if (chat.userPostList.isEmpty) {
        chat.userPostListID = chat.id;
        await Golib.listUserPosts(chat.id);
      } else {
        client.activeUserPostList = chat.userPostList;
      }
      event.state = SCE_sent;
    } catch (exception) {
      event.error = Exception("Unable to list user posts: $exception");
    }
  }

  void listUserContent(BuildContext context, ChatModel chat) async {
    var event = SynthChatEvent("Listing user content", SCE_sending);
    try {
      chat.append(ChatEventModel(event, null), false);
      await Golib.listUserContent(chat.id);
      event.state = SCE_sent;
    } catch (exception) {
      event.error = Exception("Unable to list user content: $exception");
    }
  }

  void viewPages(BuildContext context, ChatModel chat) async {
    var path = ["index.md"];
    try {
      var resources = Provider.of<ResourcesModel>(context, listen: false);
      var sess = await resources.fetchPage(chat.id, path, 0, 0, null);
      var event = RequestedResourceEvent(chat.id, sess);
      chat.append(ChatEventModel(event, null), false);
    } catch (exception) {
      var event = SynthChatEvent("", SCE_sending);
      event.error = Exception("Unable to fetch page: $exception");
      chat.append(ChatEventModel(event, null), false);
    }
  }

  void handshake(BuildContext context, ChatModel chat) async {
    try {
      await Golib.handshake(chat.id);
      var event =
          SynthChatEvent("Requested 3-way handshake with user", SCE_sent);
      chat.append(ChatEventModel(event, null), false);
    } catch (exception) {
      var event = SynthChatEvent("", SCE_sending);
      event.error = Exception("Unable to perform handshake: $exception");
      chat.append(ChatEventModel(event, null), false);
    }
  }

  void subscribeToPosts(
      BuildContext context, ClientModel client, ChatModel chat) async {
    chat.subscribeToPosts();
    client.updateUserMenu(chat.id, buildUserChatMenu(chat));
  }

  return <ChatMenuItem?>[
    ChatMenuItem(
        "User Profile", (context, chats) => chats.profile = chats.active),
    //.of(context, rootNavigator: true).pushNamed('/userProfile', arguments: UserProfileArgs(chat))),
    ChatMenuItem(
      "Pay Tip",
      (context, chats) => showPayTipModalBottom(context, chats.active!),
    ),
    ChatMenuItem(
      "Request Ratchet Reset",
      (context, chats) => chats.active!.requestKXReset(),
    ),
    ChatMenuItem(
      "Show Content",
      (context, chats) => listUserContent(context, chats.active!),
    ),
    chat.isSubscribed
        ? ChatMenuItem(
            "Unsubscribe to Posts",
            (context, chats) => chats.active!.unsubscribeToPosts(),
          )
        : !chat.isSubscribing
            ? ChatMenuItem(
                "Subscribe to Posts",
                (context, chats) =>
                    subscribeToPosts(context, chats, chats.active!),
              )
            : ChatMenuItem(
                "Subscribing to Posts",
                (context, chats) => null,
              ),
    chat.isSubscribed
        ? ChatMenuItem(
            "List Posts",
            (context, chats) => listUserPosts(context, chats, chats.active!),
          )
        : null,
    ChatMenuItem(
      "Send File",
      (context, chats) => sendFile(context, chats.active!),
    ),
    ChatMenuItem(
      "View Pages",
      (context, chats) => viewPages(context, chats.active!),
    ),
    ChatMenuItem(
      "Rename User",
      (context, chats) => showRenameModalBottom(context, chats.active!),
    ),
    ChatMenuItem(
      "Suggest User to KX",
      (context, chats) => showSuggestKXModalBottom(context, chats.active!),
    ),
    ChatMenuItem(
      "Issue Transitive Reset with User",
      (context, chats) => showTransResetModalBottom(context, chats.active!),
    ),
    ChatMenuItem(
      "Perform Handshake",
      (context, chats) => handshake(context, chats.active!),
    ),
  ];
}

List<ChatMenuItem> buildGCMenu(ChatModel chat) {
  return [
    ChatMenuItem("Manage GC", (context, chats) => chats.profile = chats.active),
    ChatMenuItem(
      "Rename GC",
      (context, chats) => showRenameModalBottom(context, chats.active!),
    ),
    ChatMenuItem(
      "Resend GC List",
      (context, chats) async {
        var msg = SynthChatEvent("Resending GC list to members");
        msg.state = SCE_sending;
        chat.append(ChatEventModel(msg, null), false);
        try {
          await chat.resendGCList();
          msg.state = SCE_sent;
        } catch (exception) {
          msg.error = Exception("Unable to resend GC list: $exception");
        }
      },
    ),
  ];
}

class SidebarIcon extends StatelessWidget {
  final IconData icon;
  final bool alert;
  const SidebarIcon(this.icon, this.alert, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var unselectedTextColor = theme.dividerColor;
    if (alert) {
      return Icon(icon, color: Colors.amber);
    } else {
      return Icon(icon, color: unselectedTextColor);
    }
  }
}

class SidebarSvgIcon extends StatelessWidget {
  final String assetName;
  const SidebarSvgIcon(this.assetName, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var unselectedTextColor = theme.iconTheme.color;
    return SvgPicture.asset(
      assetName,
      colorFilter: ColorFilter.mode(
          unselectedTextColor ?? const Color(0xFF8E8D98), BlendMode.srcIn),
    );
  }
}
