import 'package:bruig/components/pay_tip.dart';
import 'package:bruig/components/rename_chat.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/log.dart';
import 'package:bruig/models/notifications.dart';
import 'package:bruig/screens/chats.dart';
import 'package:bruig/screens/feed.dart';
import 'package:bruig/screens/ln_management.dart';
import 'package:bruig/screens/log.dart';
import 'package:bruig/screens/manage_content_screen.dart';
import 'package:bruig/screens/paystats.dart';
import 'package:bruig/screens/settings.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';
import 'package:file_picker/file_picker.dart';
import 'package:path/path.dart' as path;

class MainMenuItem {
  final String label;
  final String routeName;
  final WidgetBuilder builder;
  final WidgetBuilder titleBuilder;
  final IconData icon;

  MainMenuItem(
      this.label, this.routeName, this.builder, this.titleBuilder, this.icon);
}

MainMenuItem _emptyMenu = MainMenuItem("", "", (context) => const Text(""),
    (context) => const Text(""), Icons.question_mark);

final List<MainMenuItem> mainMenu = [
  MainMenuItem(
    "News Feed",
    FeedScreen.routeName,
    (context) => const FeedScreen(),
    (context) => const FeedScreenTitle(),
    Icons.list_alt,
  ),
  MainMenuItem(
    "Chats",
    ChatsScreen.routeName,
    (context) => Consumer2<ClientModel, AppNotifications>(
        builder: (context, client, ntfns, child) => ChatsScreen(client, ntfns)),
    (context) => const ChatsScreenTitle(),
    Icons.chat_bubble_outline,
  ),
  MainMenuItem(
    "LN Management",
    LNScreen.routeName,
    (context) => const LNScreen(),
    (context) => const LNScreenTitle(),
    Icons.device_hub,
  ),
  MainMenuItem(
    "Manage Content",
    ManageContentScreen.routeName,
    (context) => const ManageContentScreen(),
    (context) => const ManageContentScreenTitle(),
    Icons.file_download,
  ),
  MainMenuItem(
    "Payment Stats",
    PayStatsScreen.routeName,
    (context) => Consumer<ClientModel>(
        builder: (context, client, child) => PayStatsScreen(client)),
    (context) => const PayStatsScreenTitle(),
    Icons.wallet_outlined,
  ),
  MainMenuItem(
      "Settings",
      SettingsScreen.routeName,
      (context) => Consumer<ClientModel>(
          builder: (context, client, child) => SettingsScreen(client)),
      (context) => const SettingsScreenTitle(),
      Icons.settings_rounded),
  MainMenuItem(
      "Logs",
      LogScreen.routeName,
      (context) =>
          Consumer<LogModel>(builder: (context, log, child) => LogScreen(log)),
      (context) => const LogScreenTitle(),
      Icons.list_rounded),
];

class MainMenuModel extends ChangeNotifier {
  final List<MainMenuItem> menus = mainMenu;

  String _activeRoute = "";
  MainMenuItem _activeMenu = _emptyMenu;
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

List<ChatMenuItem> buildUserChatMenu(ChatModel chat) {
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
      chat.append(ChatEventModel(
          SynthChatEvent("Sending file \"$fname\" to user", SCE_sent), null));
    } catch (exception) {
      showErrorSnackbar(context, "Unable to send file: $exception");
    }
  }

  void listUserPosts(BuildContext context, ChatModel chat) async {
    var event = SynthChatEvent("Listing user posts", SCE_sending);
    try {
      chat.append(ChatEventModel(event, null));
      await Golib.listUserPosts(chat.id);
      event.state = SCE_sent;
    } catch (exception) {
      event.error = Exception("Unable to list user posts: $exception");
    }
  }

  void listUserContent(BuildContext context, ChatModel chat) async {
    var event = SynthChatEvent("Listing user content", SCE_sending);
    try {
      chat.append(ChatEventModel(event, null));
      await Golib.listUserContent(chat.id);
      event.state = SCE_sent;
    } catch (exception) {
      event.error = Exception("Unable to list user content: $exception");
    }
  }

  return <ChatMenuItem>[
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
    ChatMenuItem(
      "Subscribe to Posts",
      (context, chats) => chats.active!.subscribeToPosts(),
    ),
    ChatMenuItem(
      "List Posts",
      (context, chats) => listUserPosts(context, chats.active!),
    ),
    ChatMenuItem(
      "Send File",
      (context, chats) => sendFile(context, chats.active!),
    ),
    ChatMenuItem(
      "Rename User",
      (context, chats) => showRenameModalBottom(context, chats.active!),
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
        chat.append(ChatEventModel(msg, null));
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
