import 'dart:collection';

import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/screens/chats.dart';
import 'package:flutter/material.dart';
import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/addressbook/input.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';
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
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    var selectedBackgroundColor = theme.highlightColor;
    var darkTextColor = theme.indicatorColor;
    var alreadyOpened = false;
    alreadyOpened = client.activeChats.contains(chat);

    var popMenuButton = UserMenuAvatar(client, chat);
    addToGroupChat = widget.alreadySelected;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Container(
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(3),
              ),
              child: ListTile(
                onTap: () => confirmGCInvite
                    ? null
                    : widget.addToCreateGCCB ?? startChat(),
                enabled: true,
                title: Text(chat.nick,
                    style: TextStyle(
                        fontSize: theme.getMediumFont(context),
                        color: textColor)),
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
                        : Material(
                            color: textColor.withOpacity(0),
                            child: IconButton(
                                splashRadius: 15,
                                iconSize: 15,
                                hoverColor: selectedBackgroundColor,
                                tooltip: widget.addToCreateGCCB != null
                                    ? "Add to group chat"
                                    : alreadyOpened
                                        ? "Open Chat"
                                        : "Start Chat",
                                onPressed: () => startChat(),
                                icon: Icon(
                                    color: darkTextColor,
                                    !alreadyOpened ||
                                            widget.addToCreateGCCB != null
                                        ? Icons.add
                                        : Icons.arrow_right_alt_outlined))),
              ),
            ));
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
    var theme = Theme.of(context);
    var normalTextColor = theme.focusColor;
    var darkTextColor = theme.indicatorColor;
    var dividerColor = theme.highlightColor;
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;

    if (client.ui.createGroupChat.val) {
      combinedChatList = combinedChatList.where((e) => !e.isGC).toList();
    } else {
      combinedChatList = client.hiddenChats.sorted + client.activeChats.sorted;
    }
    combinedChatList
        .sort((a, b) => a.nick.toLowerCase().compareTo(b.nick.toLowerCase()));
    if (confirmNewGC) {
      return Consumer<ThemeNotifier>(
          builder: (context, theme, _) => Column(children: [
                Container(
                    padding: const EdgeInsets.all(20),
                    child: Row(children: [
                      Text("Name this group chat",
                          textAlign: TextAlign.left,
                          style: TextStyle(
                              color: normalTextColor,
                              fontSize: theme.getLargeFont(context))),
                    ])),
                Row(children: [
                  const SizedBox(width: 20),
                  Expanded(
                      child: GroupChatNameInput(
                          setGcName, inputFocusNode, newGcName)),
                  isScreenSmall ? const SizedBox(width: 37) : const Empty()
                ]),
                const SizedBox(height: 20),
                Row(
                    mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                    children: [
                      OutlinedButton.icon(
                          onPressed:
                              newGcName != "" ? createNewGCFromList : null,
                          icon: Icon(
                            color: normalTextColor,
                            Icons.group_rounded,
                            size: 24.0,
                          ),
                          label: Text('Create group chat',
                              style: TextStyle(
                                  fontSize: theme.getLargeFont(context))),
                          style: OutlinedButton.styleFrom(
                              foregroundColor: normalTextColor,
                              disabledForegroundColor: darkTextColor,
                              side: BorderSide.none,
                              shape: const StadiumBorder())),
                      OutlinedButton.icon(
                        onPressed: cancelCreateNewGC,
                        icon: const Empty(),
                        label: Text('Cancel',
                            style: TextStyle(
                                color: normalTextColor,
                                fontSize: theme.getLargeFont(context))),
                        style: OutlinedButton.styleFrom(
                            side: BorderSide.none,
                            shape: const StadiumBorder()),
                      ),
                    ]),
                Expanded(
                    child: Container(
                        padding: const EdgeInsets.all(20),
                        child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Row(children: [
                                Expanded(
                                    child: Divider(
                                  color: dividerColor, //color of divider
                                  height: 10, //height spacing of divider
                                  thickness: 1, //thickness of divier line
                                  indent: 8, //spacing at the start of divider
                                  endIndent: 5, //spacing at the end of divider
                                )),
                              ]),
                              const SizedBox(height: 21),
                              Expanded(
                                  child: ListView.builder(
                                      itemCount: usersToInvite.length,
                                      itemBuilder: (context, index) =>
                                          _AddressBookListingW(
                                              usersToInvite[index],
                                              client,
                                              usersToInvite.contains(
                                                  usersToInvite[index]),
                                              client.ui.createGroupChat.val
                                                  ? addToInviteGCList
                                                  : null,
                                              true))),
                              const SizedBox(height: 21),
                            ])))
              ]));
    }
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Column(children: [
              Container(
                  padding: const EdgeInsets.all(20),
                  child: Row(children: [
                    Text(
                        !client.ui.createGroupChat.val
                            ? "New message"
                            : "New group chat",
                        textAlign: TextAlign.left,
                        style: TextStyle(
                            color: normalTextColor,
                            fontSize: theme.getLargeFont(context))),
                  ])),
              Row(children: [
                const SizedBox(width: 20),
                Expanded(
                    child: Input(inputFocusNode, client.ui.createGroupChat.val,
                        onInputChanged)),
                Material(
                    color: dividerColor.withOpacity(0),
                    child: isScreenSmall
                        ? const SizedBox(width: 37)
                        : IconButton(
                            splashRadius: 15,
                            iconSize: 15,
                            hoverColor: dividerColor,
                            tooltip: "Cancel",
                            onPressed: client.ui.hideAddressBookScreen,
                            icon: Icon(color: normalTextColor, Icons.cancel))),
              ]),
              !client.ui.createGroupChat.val
                  ? Column(children: [
                      const SizedBox(height: 20),
                      Row(children: [
                        const SizedBox(width: 20),
                        OutlinedButton.icon(
                          onPressed: createNewGroupChat,
                          icon: Icon(
                            color: normalTextColor,
                            Icons.group_rounded,
                            size: 24.0,
                          ),
                          label: Text('New group chat',
                              style: TextStyle(
                                  color: normalTextColor,
                                  fontSize: theme.getLargeFont(context))),
                          style: OutlinedButton.styleFrom(
                              side: BorderSide.none,
                              shape: const StadiumBorder()),
                        ),
                      ])
                    ])
                  : Column(children: [
                      const SizedBox(height: 20),
                      Row(
                          mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                          children: [
                            OutlinedButton.icon(
                                onPressed: usersToInvite.isNotEmpty
                                    ? confirmCreateNewGC
                                    : null,
                                icon: const Empty(),
                                label: Text('Confirm group chat',
                                    style: TextStyle(
                                        fontSize: theme.getLargeFont(context))),
                                style: OutlinedButton.styleFrom(
                                    foregroundColor: normalTextColor,
                                    disabledForegroundColor: darkTextColor,
                                    side: BorderSide.none,
                                    shape: const StadiumBorder())),
                            OutlinedButton.icon(
                              onPressed: cancelCreateNewGC,
                              icon: const Empty(),
                              label: Text('Cancel',
                                  style: TextStyle(
                                      color: normalTextColor,
                                      fontSize: theme.getLargeFont(context))),
                              style: OutlinedButton.styleFrom(
                                  side: BorderSide.none,
                                  shape: const StadiumBorder()),
                            ),
                          ])
                    ]),
              Expanded(
                  child: Container(
                      padding: const EdgeInsets.all(20),
                      child: filterSearchString != ""
                          ? Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                  Row(children: [
                                    Text("Matching Chats",
                                        textAlign: TextAlign.left,
                                        style: TextStyle(
                                            color: normalTextColor,
                                            fontSize:
                                                theme.getMediumFont(context))),
                                    Expanded(
                                        child: Divider(
                                      color: dividerColor, //color of divider
                                      height: 10, //height spacing of divider
                                      thickness: 1, //thickness of divier line
                                      indent:
                                          8, //spacing at the start of divider
                                      endIndent:
                                          5, //spacing at the end of divider
                                    )),
                                  ]),
                                  const SizedBox(height: 21),
                                  filteredSearch.isNotEmpty
                                      ? Expanded(
                                          child: ListView.builder(
                                              itemCount: filteredSearch.length,
                                              itemBuilder: (context, index) =>
                                                  _AddressBookListingW(
                                                      filteredSearch[index],
                                                      client,
                                                      usersToInvite.contains(
                                                          filteredSearch[
                                                              index]),
                                                      client.ui.createGroupChat
                                                              .val
                                                          ? addToInviteGCList
                                                          : null,
                                                      false)))
                                      : Center(
                                          //padding: const EdgeInsets.only(left: 50),
                                          child: Text("No Matching Chats",
                                              textAlign: TextAlign.center,
                                              style: TextStyle(
                                                  color: normalTextColor,
                                                  fontSize: theme.getMediumFont(
                                                      context)))),
                                  const SizedBox(height: 21),
                                ])
                          : Column(children: [
                              combinedChatList.isNotEmpty
                                  ? Expanded(
                                      child: Column(
                                          crossAxisAlignment:
                                              CrossAxisAlignment.start,
                                          children: [
                                          Row(children: [
                                            Expanded(
                                                child: Divider(
                                              color:
                                                  dividerColor, //color of divider
                                              height:
                                                  10, //height spacing of divider
                                              thickness:
                                                  1, //thickness of divier line
                                              indent:
                                                  8, //spacing at the start of divider
                                              endIndent:
                                                  5, //spacing at the end of divider
                                            )),
                                          ]),
                                          const SizedBox(height: 21),
                                          Expanded(
                                              child: ListView.builder(
                                                  itemCount:
                                                      combinedChatList.length,
                                                  itemBuilder: (context,
                                                          index) =>
                                                      _AddressBookListingW(
                                                          combinedChatList[
                                                              index],
                                                          client,
                                                          usersToInvite.contains(
                                                              combinedChatList[
                                                                  index]),
                                                          client
                                                                  .ui
                                                                  .createGroupChat
                                                                  .val
                                                              ? addToInviteGCList
                                                              : null,
                                                          false))),
                                          const SizedBox(height: 21),
                                        ]))
                                  : const Empty(),
                            ]))),
            ]));
  }
}
