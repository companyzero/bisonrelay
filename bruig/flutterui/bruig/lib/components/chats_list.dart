import 'dart:async';
import 'package:bruig/models/client.dart';
import 'package:bruig/screens/chats.dart';
import 'package:bruig/screens/contacts_msg_times.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/user_context_menu.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/gc_context_menu.dart';
import 'package:bruig/components/chat/types.dart';
import 'package:bruig/theme_manager.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:file_picker/file_picker.dart';

class _ChatHeadingW extends StatefulWidget {
  final ChatModel chat;
  final ClientModel client;
  final MakeActiveCB makeActive;
  final ShowSubMenuCB showSubMenu;

  const _ChatHeadingW(this.chat, this.client, this.makeActive, this.showSubMenu,
      {Key? key})
      : super(key: key);

  @override
  State<_ChatHeadingW> createState() => _ChatHeadingWState();
}

class _ChatHeadingWState extends State<_ChatHeadingW> {
  ChatModel get chat => widget.chat;
  ClientModel get client => widget.client;

  void chatUpdated() => setState(() {});

  @override
  void initState() {
    chat.addListener(chatUpdated);
    super.initState();
  }

  @override
  void didUpdateWidget(_ChatHeadingW oldWidget) {
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
    var unreadMessageIconColor = theme.indicatorColor.withOpacity(0.5);
    var darkTextColor = theme.indicatorColor;

    // Show 1k+ if unread cound goes about 1000
    var unreadCount = chat.unreadMsgCount > 1000 ? "1k+" : chat.unreadMsgCount;

    Widget? trailing;
    if (chat.unreadMsgCount > 0) {
      textColor = hightLightTextColor;
      trailing = Consumer<ThemeNotifier>(
          builder: (context, theme, _) =>
              Row(mainAxisSize: MainAxisSize.min, children: [
                chat.isGC
                    ? Text("gc",
                        style: TextStyle(
                            fontStyle: FontStyle.italic,
                            color: darkTextColor,
                            fontSize: theme.getMediumFont(context)))
                    : const Empty(),
                const SizedBox(width: 5),
                Container(
                    margin: const EdgeInsets.all(1),
                    child: CircleAvatar(
                        backgroundColor: unreadMessageIconColor,
                        radius: 10,
                        child: Text("$unreadCount",
                            style: TextStyle(
                                color: hightLightTextColor,
                                fontSize: theme.getSmallFont(context)))))
              ]));
    } else if (chat.unreadEventCount > 0) {
      textColor = hightLightTextColor;
      trailing = Consumer<ThemeNotifier>(
          builder: (context, theme, _) =>
              Row(mainAxisSize: MainAxisSize.min, children: [
                chat.isGC
                    ? Text("gc",
                        style: TextStyle(
                            color: darkTextColor,
                            fontStyle: FontStyle.italic,
                            fontSize: theme.getMediumFont(context)))
                    : const Empty(),
                const SizedBox(width: 5),
                Container(
                    margin: const EdgeInsets.all(1),
                    child: CircleAvatar(
                        backgroundColor: unreadMessageIconColor, radius: 3))
              ]));
    } else {
      trailing = Consumer<ThemeNotifier>(
          builder: (context, theme, _) =>
              Row(mainAxisSize: MainAxisSize.min, children: [
                chat.isGC
                    ? Text("gc",
                        style: TextStyle(
                            fontStyle: FontStyle.italic,
                            color: darkTextColor,
                            fontSize: theme.getMediumFont(context)))
                    : const Empty(),
                const SizedBox(width: 5),
                const SizedBox(width: 21),
              ]));
    }

    var popMenuButton = InteractiveAvatar(
        chatNick: chat.nick,
        onTap: () {
          widget.makeActive(chat);
          widget.showSubMenu(chat.isGC, chat.id);
        },
        avatar: chat.avatar.image);

    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Container(
              decoration: BoxDecoration(
                color: chat.active ? selectedBackgroundColor : null,
                borderRadius: BorderRadius.circular(3),
              ),
              child: chat.isGC
                  ? GcContexMenu(
                      mobile: isScreenSmall
                          ? (context) {
                              widget.makeActive(chat);
                              widget.showSubMenu(chat.isGC, chat.id);
                            }
                          : null,
                      client: client,
                      targetGcChat: chat,
                      child: ListTile(
                        horizontalTitleGap: 10,
                        contentPadding:
                            const EdgeInsets.only(left: 10, right: 8),
                        enabled: true,
                        title: Container(
                          padding: const EdgeInsets.only(left: 5),
                          child: Text(chat.nick,
                              overflow: TextOverflow.ellipsis,
                              style: TextStyle(
                                  color: textColor,
                                  fontSize: theme.getMediumFont(context))),
                        ),
                        leading: popMenuButton,
                        trailing: trailing,
                        selected: chat.active,
                        onTap: () => widget.makeActive(chat),
                        selectedColor: selectedBackgroundColor,
                      ),
                    )
                  : UserContextMenu(
                      client: client,
                      targetUserChat: chat,
                      child: ListTile(
                        horizontalTitleGap: 10,
                        contentPadding:
                            const EdgeInsets.only(left: 10, right: 8),
                        enabled: true,
                        title: Container(
                            padding: const EdgeInsets.only(left: 5),
                            child: Text(chat.nick,
                                overflow: TextOverflow.ellipsis,
                                style: TextStyle(
                                    fontSize: theme.getMediumFont(context),
                                    color: textColor))),
                        leading: popMenuButton,
                        trailing: trailing,
                        selected: chat.active,
                        onTap: () => widget.makeActive(chat),
                        selectedColor: selectedBackgroundColor,
                      ),
                    ),
            ));
  }
}

Future<void> generateInvite(BuildContext context) async {
  Navigator.of(context, rootNavigator: true).pushNamed('/generateInvite');
}

Future<void> fetchInvite(BuildContext context) async {
  Navigator.of(context, rootNavigator: true).pushNamed('/fetchInvite');
}

void gotoContactsLastMsgTimeScreen(BuildContext context) {
  Navigator.of(context, rootNavigator: true)
      .pushNamed(ContactsLastMsgTimesScreen.routeName);
}

class _ChatsList extends StatefulWidget {
  final ClientModel client;
  final CustomInputFocusNode inputFocusNode;
  const _ChatsList(this.client, this.inputFocusNode, {Key? key})
      : super(key: key);

  @override
  State<_ChatsList> createState() => _ChatsListState();
}

Future<void> loadInvite(BuildContext context) async {
  // Decode the invite and send to the user verification screen.
  var filePickRes = await FilePicker.platform.pickFiles();
  if (filePickRes == null) return;
  var filePath = filePickRes.files.first.path;
  if (filePath == null) return;
  filePath = filePath.trim();
  if (filePath == "") return;
  var invite = await Golib.decodeInvite(filePath);
  Navigator.of(context, rootNavigator: true)
      .pushNamed('/verifyInvite', arguments: invite);
}

class _ChatsListState extends State<_ChatsList> {
  ClientModel get client => widget.client;
  FocusNode get inputFocusNode => widget.inputFocusNode.inputFocusNode;
  Timer? _debounce;
  bool showAddressbookRoomsButton = false;
  bool showAddressbookUsersButton = false;

  void clientUpdated() => setState(() {});

  @override
  void initState() {
    super.initState();
    client.addListener(clientUpdated);
  }

  @override
  void didUpdateWidget(_ChatsList oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.client.removeListener(clientUpdated);
    client.addListener(clientUpdated);
  }

  @override
  void dispose() {
    client.removeListener(clientUpdated);
    _debounce?.cancel();
    super.dispose();
  }

  void debouncedLoadInvite(BuildContext context) {
    if (_debounce?.isActive ?? false) _debounce!.cancel();
    _debounce = Timer(const Duration(milliseconds: 500), () {
      loadInvite(context);
    });
  }

  @override
  Widget build(BuildContext context) {
    void showGroupChat() async {
      client.createGroupChat = true;
      client.showAddressBookScreen();
    }

    void showAddressBook() async {
      client.createGroupChat = false;
      client.showAddressBookScreen();
    }

    void genInvite() async {
      await generateInvite(context);
      inputFocusNode.requestFocus();
    }

    var theme = Theme.of(context);
    var hoverColor = theme.hoverColor;
    var darkTextColor = theme.dividerColor;
    var selectedBackgroundColor = theme.highlightColor;
    var backgroundColor = theme.backgroundColor;
    var newMessageHoverColor = theme.indicatorColor;

    var sortedList = client.sortedChats.toList();

    var sortedListScroll = ScrollController();

    makeActive(ChatModel? c) => {client.active = c};

    showSubMenu(bool isGC, String id) => {client.showSubMenu(isGC, id)};
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    if (isScreenSmall) {
      return Consumer<ThemeNotifier>(
          builder: (context, theme, _) => Container(
              margin: const EdgeInsets.all(1),
              decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(3),
                  color: backgroundColor),
              padding: const EdgeInsets.all(0),
              child: Stack(children: [
                Container(
                    padding: const EdgeInsets.only(
                        left: 0, right: 5, top: 5, bottom: 5),
                    child: ListView.builder(
                        physics: const ScrollPhysics(),
                        controller: sortedListScroll,
                        scrollDirection: Axis.vertical,
                        shrinkWrap: true,
                        itemCount: sortedList.length,
                        itemBuilder: (context, index) => _ChatHeadingW(
                            sortedList[index],
                            client,
                            makeActive,
                            showSubMenu))),
                Positioned(
                    bottom: 20,
                    right: 10,
                    child: Material(
                        borderRadius: BorderRadius.circular(30),
                        color: selectedBackgroundColor,
                        child: IconButton(
                            hoverColor: newMessageHoverColor.withOpacity(0.25),
                            splashRadius: 28,
                            iconSize: 40,
                            tooltip: "New Message",
                            onPressed: showAddressBook,
                            icon: Icon(
                                size: 40,
                                color: darkTextColor,
                                Icons.edit_outlined)))),
                Positioned(
                    bottom: 90,
                    right: 10,
                    child: Material(
                        borderRadius: BorderRadius.circular(30),
                        color: selectedBackgroundColor,
                        child: IconButton(
                            hoverColor: newMessageHoverColor.withOpacity(0.25),
                            splashRadius: 28,
                            iconSize: 40,
                            tooltip: "Create new group chat",
                            onPressed: showGroupChat,
                            icon: Icon(
                                size: 40,
                                color: darkTextColor,
                                Icons.people_outline)))),
              ])));
    }
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Container(
              margin: const EdgeInsets.all(1),
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(3),
                gradient: LinearGradient(
                    begin: Alignment.centerRight,
                    end: Alignment.centerLeft,
                    colors: [
                      hoverColor,
                      backgroundColor,
                      backgroundColor,
                    ],
                    stops: const [
                      0,
                      0.51,
                      1
                    ]),
              ),
              child: Stack(children: [
                Container(
                    margin: const EdgeInsets.only(bottom: 50),
                    child: ListView.builder(
                        controller: sortedListScroll,
                        scrollDirection: Axis.vertical,
                        shrinkWrap: true,
                        itemCount: sortedList.length,
                        itemBuilder: (context, index) => _ChatHeadingW(
                            sortedList[index],
                            client,
                            makeActive,
                            showSubMenu))),
                !client.showAddressBook
                    ? Positioned(
                        bottom: 5,
                        right: 0,
                        child: Material(
                            color: selectedBackgroundColor.withOpacity(0),
                            child: IconButton(
                                hoverColor: selectedBackgroundColor,
                                splashRadius: 15,
                                iconSize: 15,
                                tooltip: "New Message",
                                onPressed: showAddressBook,
                                icon: Icon(
                                    size: 20,
                                    color: darkTextColor,
                                    Icons.edit_outlined))))
                    : const Empty(),
                Positioned(
                    bottom: 5,
                    right: 40,
                    child: Material(
                        color: selectedBackgroundColor.withOpacity(0),
                        child: IconButton(
                            hoverColor: selectedBackgroundColor,
                            splashRadius: 15,
                            iconSize: 15,
                            tooltip: "Create new group chat",
                            onPressed: showGroupChat,
                            icon: Icon(
                                size: 20,
                                color: darkTextColor,
                                Icons.people_outline)))),
                Positioned(
                    bottom: 5,
                    right: 80,
                    child: Material(
                        color: selectedBackgroundColor.withOpacity(0),
                        child: IconButton(
                            hoverColor: selectedBackgroundColor,
                            splashRadius: 15,
                            iconSize: 15,
                            tooltip: client.connState.isOnline
                                ? "Fetch invite using key"
                                : "Cannot fetch invite while client is offline",
                            onPressed: client.connState.isOnline
                                ? () => fetchInvite(context)
                                : null,
                            disabledColor: backgroundColor,
                            icon: Icon(
                                size: 20,
                                color: client.connState.isOnline
                                    ? darkTextColor
                                    : hoverColor,
                                Icons.get_app_outlined)))),
                Positioned(
                    bottom: 5,
                    right: 120,
                    child: Material(
                        color: selectedBackgroundColor.withOpacity(0),
                        child: IconButton(
                            hoverColor: selectedBackgroundColor,
                            splashRadius: 15,
                            iconSize: 15,
                            tooltip: "List last received message time",
                            onPressed: () =>
                                gotoContactsLastMsgTimeScreen(context),
                            icon: Icon(
                                size: 20,
                                color: darkTextColor,
                                Icons.list_outlined)))),
                Positioned(
                    bottom: 5,
                    right: 160,
                    child: Material(
                        color: selectedBackgroundColor.withOpacity(0),
                        child: IconButton(
                            hoverColor: selectedBackgroundColor,
                            splashRadius: 15,
                            iconSize: 15,
                            tooltip: client.connState.isOnline
                                ? "Generate Invite"
                                : "Cannot generate invite while offline",
                            onPressed:
                                client.connState.isOnline ? genInvite : null,
                            disabledColor: backgroundColor,
                            icon: Icon(
                                size: 20,
                                color: client.connState.isOnline
                                    ? darkTextColor
                                    : hoverColor,
                                Icons.add_outlined))))
              ]),
            ));
  }
}

class ChatDrawerMenu extends StatelessWidget {
  final CustomInputFocusNode inputFocusNode;
  const ChatDrawerMenu(this.inputFocusNode, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    return Consumer<ClientModel>(builder: (context, client, child) {
      return Column(
          children: [Expanded(child: _ChatsList(client, inputFocusNode))]);
    });
  }
}
