import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';

class ShowProfileModel extends BoolFlagModel {}

class ShowAddressBookModel extends BoolFlagModel {}

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

// UIStateModel holds state related to the app's UI.
class UIStateModel {
  final ShowProfileModel showProfile = ShowProfileModel();
  final ShowAddressBookModel showAddressBook = ShowAddressBookModel();
  final CreateGroupChatModel createGroupChat = CreateGroupChatModel();
  final ChatSideMenuActiveModel chatSideMenuActive = ChatSideMenuActiveModel();

  void showCreateGroupChatScreen() {
    chatSideMenuActive.chat = null;
    createGroupChat.val = true;
    showAddressBook.val = true;
  }

  void showAddressBookScreen() {
    chatSideMenuActive.chat = null;
    createGroupChat.val = false;
    showAddressBook.val = true;
  }

  void hideAddressBookScreen() {
    createGroupChat.val = false;
    showAddressBook.val = false;
  }
}

bool checkIsScreenSmall(BuildContext context) =>
    MediaQuery.of(context).size.width <= 500;
