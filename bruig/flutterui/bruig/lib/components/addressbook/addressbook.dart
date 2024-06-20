import 'dart:collection';

import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/chats.dart';
import 'package:bruig/screens/ln/components.dart';
import 'package:flutter/material.dart';
import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/addressbook/input.dart';
import 'package:bruig/theme_manager.dart';
import 'package:bruig/components/addressbook/types.dart';

class _AddressBookListingW extends StatefulWidget {
  final ChatModel chat;
  final ClientModel client;
  final bool alreadySelected;
  final CreateGCCB? addToCreateGCCB;
  final bool confirmGCInvite;

  const _AddressBookListingW(this.chat, this.client, this.alreadySelected,
      this.addToCreateGCCB, this.confirmGCInvite,
      {Key? key})
      : super(key: key);

  @override
  State<_AddressBookListingW> createState() => _AddressBookListingWState();
}

class _AddressBookListingWState extends State<_AddressBookListingW> {
  ChatModel get chat => widget.chat;
  ClientModel get client => widget.client;

  bool addToGroupChat = false;
  bool get confirmGCInvite => widget.confirmGCInvite;

  void chatUpdated() => setState(() {});

  void startChat() {
    client.active = chat;
    client.ui.hideAddressBookScreen();
  }

  @override
  void initState() {
    super.initState();
    chat.addListener(chatUpdated);
  }

  @override
  void didUpdateWidget(_AddressBookListingW oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.chat.removeListener(chatUpdated);
    chat.addListener(chatUpdated);
  }

  @override
  void dispose() {
    chat.removeListener(chatUpdated);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var alreadyOpened = false;
    alreadyOpened = client.activeChats.contains(chat);

    var popMenuButton = UserMenuAvatar(client, chat);
    addToGroupChat = widget.alreadySelected;
    return ListTile(
      onTap: () =>
          confirmGCInvite ? null : widget.addToCreateGCCB ?? startChat(),
      enabled: true,
      title: Text(chat.nick),
      leading: popMenuButton,
      trailing: confirmGCInvite
          ? const Empty()
          : widget.addToCreateGCCB != null
              ? Checkbox(
                  value: addToGroupChat,
                  onChanged: (value) {
                    setState(() {
                      addToGroupChat = value!;
                    });
                    widget.addToCreateGCCB!(value!, chat);
                  })
              : IconButton(
                  splashRadius: 15,
                  iconSize: 15,
                  tooltip: widget.addToCreateGCCB != null
                      ? "Add to group chat"
                      : alreadyOpened
                          ? "Open Chat"
                          : "Start Chat",
                  onPressed: () => startChat(),
                  icon: Icon(!alreadyOpened || widget.addToCreateGCCB != null
                      ? Icons.add
                      : Icons.arrow_right_alt_outlined)),
    );
  }
}

class AddressBook extends StatefulWidget {
  final ClientModel client;
  final CustomInputFocusNode inputFocusNode;
  const AddressBook(this.client, this.inputFocusNode, {Key? key})
      : super(key: key);

  @override
  State<AddressBook> createState() => _AddressBookState();
}

class _AddressBookState extends State<AddressBook> {
  ClientModel get client => widget.client;
  CustomInputFocusNode get inputFocusNode => widget.inputFocusNode;
  List<ChatModel> usersToInvite = [];
  bool confirmNewGC = false;
  String newGcName = "";
  var combinedChatList = [];
  String filterSearchString = "";
  UnmodifiableListView<ChatModel> filteredSearch = UnmodifiableListView([]);

  @override
  void initState() {
    super.initState();
    combinedChatList = client.hiddenChats.sorted + client.activeChats.sorted;
    combinedChatList
        .sort((a, b) => a.nick.toLowerCase().compareTo(b.nick.toLowerCase()));
  }

  void createNewGroupChat() {
    setState(() {
      client.ui.createGroupChat.val = true;
      usersToInvite = [];
    });
  }

  void cancelCreateNewGroupChat() {
    client.ui.createGroupChat.val =
        false; // XXX Make an internal bool of the state?
    setState(() {
      usersToInvite = [];
      newGcName = "";
    });
  }

  void setGcName(String gcName) {
    setState(() {
      newGcName = gcName;
    });
  }

  void onInputChanged(String value) {
    var newSearchResults =
        client.searchChats(value, ignoreGC: client.ui.createGroupChat.val);
    setState(() {
      filterSearchString = value;
      filteredSearch = newSearchResults;
    });
  }

  void addToInviteGCList(bool value, ChatModel userToInvite) {
    setState(() {
      if (value) {
        !usersToInvite.contains(userToInvite)
            ? usersToInvite.add(userToInvite)
            : null;
      } else {
        usersToInvite.contains(userToInvite)
            ? usersToInvite.remove(userToInvite)
            : null;
      }
    });
  }

  void createNewGCFromList() async {
    if (newGcName == "") return;
    try {
      await client.createNewGCAndInvite(newGcName, usersToInvite);
      client.ui.hideAddressBookScreen();
    } catch (exception) {
      showErrorSnackbar(context, "Unable to create GC: $exception");
    }
  }

  void confirmCreateNewGC() {
    setState(() {
      confirmNewGC = true;
    });
  }

  void cancelCreateNewGC() {
    client.ui.createGroupChat.val = false;
    setState(() {
      confirmNewGC = false;
      usersToInvite = [];
      newGcName = "";
    });
  }

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = checkIsScreenSmall(context);

    if (client.ui.createGroupChat.val) {
      combinedChatList = combinedChatList.where((e) => !e.isGC).toList();
    } else {
      combinedChatList = client.hiddenChats.sorted + client.activeChats.sorted;
    }
    combinedChatList
        .sort((a, b) => a.nick.toLowerCase().compareTo(b.nick.toLowerCase()));
    if (confirmNewGC) {
      return Container(
          padding:
              const EdgeInsets.only(left: 12, right: 12, top: 5, bottom: 10),
          child:
              Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            const Txt.L("Name this group chat"),
            const SizedBox(height: 10),
            Row(children: [
              Expanded(
                  child:
                      GroupChatNameInput(setGcName, inputFocusNode, newGcName)),
              const SizedBox(width: 15)
            ]),
            const SizedBox(height: 20),
            Row(mainAxisAlignment: MainAxisAlignment.spaceEvenly, children: [
              TextButton.icon(
                  onPressed: newGcName != "" ? createNewGCFromList : null,
                  icon: const Icon(Icons.group_rounded, size: 24.0),
                  label: const Txt.L("Create group chat")),
              TextButton(
                  onPressed: cancelCreateNewGC, child: const Txt.L("Cancel")),
            ]),
            const SizedBox(height: 10),
            const Divider(),
            const SizedBox(height: 10),
            Expanded(
                child: ListView.builder(
                    itemCount: usersToInvite.length,
                    itemBuilder: (context, index) => _AddressBookListingW(
                        usersToInvite[index],
                        client,
                        usersToInvite.contains(usersToInvite[index]),
                        client.ui.createGroupChat.val
                            ? addToInviteGCList
                            : null,
                        true))),
          ]));
    }
    return Container(
        padding: const EdgeInsets.only(left: 12, right: 12, top: 5, bottom: 10),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Txt.L(!client.ui.createGroupChat.val
              ? "New message"
              : "New group chat"),
          const SizedBox(height: 10),
          Row(children: [
            Expanded(
                child: ChatSearchInput(inputFocusNode,
                    client.ui.createGroupChat.val, onInputChanged)),
            isScreenSmall
                ? const SizedBox(width: 15)
                : IconButton(
                    splashRadius: 15,
                    iconSize: 15,
                    tooltip: "Cancel",
                    onPressed: client.ui.hideAddressBookScreen,
                    icon: const Icon(Icons.cancel)),
          ]),
          const SizedBox(height: 10),
          !client.ui.createGroupChat.val
              ? TextButton.icon(
                  onPressed: createNewGroupChat,
                  icon: const Icon(Icons.group_rounded, size: 24.0),
                  label: const Txt.L("New group chat"),
                )
              : Row(
                  mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                  children: [
                      TextButton(
                          onPressed: usersToInvite.isNotEmpty
                              ? confirmCreateNewGC
                              : null,
                          child: const Txt.L("Confirm group chat")),
                      TextButton(
                        onPressed: cancelCreateNewGC,
                        child: const Txt.L("Cancel"),
                      ),
                    ]),
          const SizedBox(height: 10),
          ...(filterSearchString != ""
              ? [
                  const LNInfoSectionHeader("Matching Chats"),
                  const SizedBox(height: 10),
                  filteredSearch.isEmpty
                      ? const Center(
                          child: Txt("No Matching Chats",
                              color: TextColor.onSurfaceVariant))
                      : Expanded(
                          child: ListView.builder(
                              itemCount: filteredSearch.length,
                              itemBuilder: (context, index) =>
                                  _AddressBookListingW(
                                      filteredSearch[index],
                                      client,
                                      usersToInvite
                                          .contains(filteredSearch[index]),
                                      client.ui.createGroupChat.val
                                          ? addToInviteGCList
                                          : null,
                                      false))),
                ]
              : [
                  const Divider(),
                  combinedChatList.isEmpty
                      ? const Txt("No chats available",
                          color: TextColor.onSurfaceVariant)
                      : Expanded(
                          child: ListView.builder(
                              itemCount: combinedChatList.length,
                              itemBuilder: (context, index) =>
                                  _AddressBookListingW(
                                      combinedChatList[index],
                                      client,
                                      usersToInvite
                                          .contains(combinedChatList[index]),
                                      client.ui.createGroupChat.val
                                          ? addToInviteGCList
                                          : null,
                                      false))),
                ]),
        ]));
  }
}
