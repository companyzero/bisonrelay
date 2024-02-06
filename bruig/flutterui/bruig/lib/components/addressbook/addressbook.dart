import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:bruig/util.dart';
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

  void startChat(bool open) {
    client.startChat(chat, open);
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
    var hightLightTextColor = theme.focusColor;
    var selectedBackgroundColor = theme.highlightColor;
    var darkTextColor = theme.indicatorColor;
    var alreadyOpened = false;
    alreadyOpened = client.sortedChats.contains(chat);
    var avatarColor = colorFromNick(chat.nick);
    var avatarTextColor =
        ThemeData.estimateBrightnessForColor(avatarColor) == Brightness.dark
            ? hightLightTextColor
            : darkTextColor;
    var popMenuButton = InteractiveAvatar(
        bgColor: selectedBackgroundColor,
        chatNick: chat.nick,
        onTap: () {},
        avatarColor: avatarColor,
        avatarTextColor: avatarTextColor);
    addToGroupChat = widget.alreadySelected;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Container(
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(3),
              ),
              child: ListTile(
                onTap: () => confirmGCInvite
                    ? null
                    : widget.addToCreateGCCB ?? startChat(alreadyOpened),
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
                                onPressed: () => startChat(alreadyOpened),
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
  final FocusNode inputFocusNode;
  final bool createGroupChat;
  const AddressBook(this.client, this.inputFocusNode, this.createGroupChat,
      {Key? key})
      : super(key: key);

  @override
  State<AddressBook> createState() => _AddressBookState();
}

class _AddressBookState extends State<AddressBook> {
  ClientModel get client => widget.client;
  FocusNode get inputFocusNode => widget.inputFocusNode;
  List<ChatModel> usersToInvite = [];
  bool confirmNewGC = false;
  String newGcName = "";
  var combinedChatList = [];

  @override
  void initState() {
    super.initState();
    combinedChatList = client.hiddenChats + client.sortedChats;
    combinedChatList
        .sort((a, b) => a.nick.toLowerCase().compareTo(b.nick.toLowerCase()));
  }

  void hideAddressBook() async {
    client.hideAddressBookScreen();
  }

  void createNewGroupChat() {
    setState(() {
      client.createGroupChat = true;
      usersToInvite = [];
    });
  }

  void cancelCreateNewGroupChat() {
    setState(() {
      client.createGroupChat = false;
      usersToInvite = [];
      newGcName = "";
    });
  }

  void setGcName(String gcName) {
    setState(() {
      newGcName = gcName;
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
    client.createNewGCAndInvite(newGcName, usersToInvite);
  }

  void confirmCreateNewGC() {
    setState(() {
      confirmNewGC = true;
    });
  }

  void cancelCreateNewGC() {
    setState(() {
      client.createGroupChat = false;
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

    if (client.createGroupChat) {
      combinedChatList = combinedChatList.where((e) => !e.isGC).toList();
    } else {
      combinedChatList = client.hiddenChats + client.sortedChats;
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
                                              client.createGroupChat
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
                        !client.createGroupChat
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
                    child:
                        Input(client, inputFocusNode, client.createGroupChat)),
                Material(
                    color: dividerColor.withOpacity(0),
                    child: isScreenSmall
                        ? const SizedBox(width: 37)
                        : IconButton(
                            splashRadius: 15,
                            iconSize: 15,
                            hoverColor: dividerColor,
                            tooltip: "Cancel",
                            onPressed: () => hideAddressBook(),
                            icon: Icon(color: normalTextColor, Icons.cancel))),
              ]),
              !client.createGroupChat
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
                      child: client.filteredSearchString != ""
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
                                  client.filteredSearch.isNotEmpty
                                      ? Expanded(
                                          child: ListView.builder(
                                              itemCount:
                                                  client.filteredSearch.length,
                                              itemBuilder: (context, index) =>
                                                  _AddressBookListingW(
                                                      client.filteredSearch[
                                                          index],
                                                      client,
                                                      usersToInvite.contains(
                                                          client.filteredSearch[
                                                              index]),
                                                      client.createGroupChat
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
                                                          client.createGroupChat
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
