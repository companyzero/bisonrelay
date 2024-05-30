import 'package:bruig/models/client.dart';

class ShowProfileModel extends BoolFlagModel {}

class ShowAddressBookModel extends BoolFlagModel {}

class CreateGroupChatModel extends BoolFlagModel {}

// UIStateModel holds state related to the app's UI.
class UIStateModel {
  final ShowProfileModel showProfile = ShowProfileModel();
  final ShowAddressBookModel showAddressBook = ShowAddressBookModel();
  final CreateGroupChatModel createGroupChat = CreateGroupChatModel();

  void showCreateGroupChatScreen() {
    createGroupChat.val = true;
    showAddressBook.val = true;
  }

  void showAddressBookScreen() {
    createGroupChat.val = false;
    showAddressBook.val = true;
  }

  void hideAddressBookScreen() {
    createGroupChat.val = false;
    showAddressBook.val = false;
  }
}
