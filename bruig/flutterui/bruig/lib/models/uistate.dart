import 'package:bruig/models/client.dart';
import 'package:bruig/screens/chats.dart';
import 'package:bruig/screens/feed.dart';
import 'package:bruig/screens/viewpage_screen.dart';
import 'package:flutter/material.dart';

class ShowProfileModel extends BoolFlagModel {}

class CreateGroupChatModel extends BoolFlagModel {}

class ChatSideMenuActiveModel extends ChangeNotifier {
  ChatModel? _chat;
  ChatModel? get chat => _chat;
  set chat(ChatModel? v) {
    _chat = v;
    notifyListeners();
  }

  bool get empty => _chat == null;

  void clear() => chat = null;
}

class SettingsTitleModel extends ChangeNotifier {
  String _title = "Settings";
  String get title => _title;
  set title(String v) {
    _title = v;
    notifyListeners();
  }
}

enum SmallScreenActiveTab {
  chat,
  feed,
  pages,
}

class SmallScreenActiveTabModel extends ChangeNotifier {
  SmallScreenActiveTab _active = SmallScreenActiveTab.chat;
  SmallScreenActiveTab get active => _active;
  set active(SmallScreenActiveTab v) {
    _active = v;
    notifyListeners();
  }
}

class OverviewActivePath extends ChangeNotifier {
  String _route = "";
  String get route => _route;
  set route(String v) {
    _route = v;
    notifyListeners();
  }

  // onActiveBottomTab is true if the current active route is one that corresponds
  // to one of the bottom tabs ("chats", "feeds", "pages").
  bool get onActiveBottomTab => [
        ChatsScreen.routeName,
        FeedScreen.routeName,
        ViewPageScreen.routeName
      ].contains(route);
}

// UIStateModel holds state related to the app's UI.
class UIStateModel {
  final ShowProfileModel showProfile = ShowProfileModel();
  final ChatSideMenuActiveModel chatSideMenuActive = ChatSideMenuActiveModel();
  final SettingsTitleModel settingsTitle = SettingsTitleModel();
  final SmallScreenActiveTabModel smallScreenActiveTab =
      SmallScreenActiveTabModel();
  final OverviewActivePath overviewActivePath = OverviewActivePath();
  final RouteObserver<ModalRoute<void>> overviewRouteObserver =
      RouteObserver<ModalRoute<void>>();
}

bool checkIsScreenSmall(BuildContext context) =>
    MediaQuery.sizeOf(context).width <= 500;
