import 'dart:async';
import 'package:bruig/models/client.dart';
import 'package:bruig/screens/contacts_msg_times.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/user_context_menu.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/gc_context_menu.dart';
import 'package:bruig/components/chat/types.dart';
import 'package:bruig/util.dart';
import 'package:bruig/components/addressbook/addressbook.dart';
import 'package:flutter/gestures.dart';

class _ChatHeadingW extends StatefulWidget {
  final ChatModel chat;
  final ClientModel client;
  final MakeActiveCB makeActive;
  final ShowSubMenuCB showSubMenu;
  final bool isGc;

  const _ChatHeadingW(
      this.chat, this.client, this.makeActive, this.showSubMenu, this.isGc,
      {Key? key})
      : super(key: key);

  @override
  State<_ChatHeadingW> createState() => _ChatHeadingWState();
}

class _ChatHeadingWState extends State<_ChatHeadingW> {
  ChatModel get chat => widget.chat;
  ClientModel get client => widget.client;
  bool get isGc => widget.isGc;

  void chatUpdated() => setState(() {});

  @override
  void initState() {
    super.initState();
    chat.addListener(chatUpdated);
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
    var unreadMessageIconColor = theme.indicatorColor;
    var darkTextColor = theme.indicatorColor;

    // Show 1k+ if unread cound goes about 1000
    var unreadCount = chat.unreadMsgCount > 1000 ? "1k+" : chat.unreadMsgCount;

    Widget? trailing;
    if (chat.active) {
      // Do we want to do any text color changes on active?
    } else if (chat.unreadMsgCount > 0) {
      textColor = hightLightTextColor;
      trailing = Container(
          margin: const EdgeInsets.all(1),
          child: CircleAvatar(
              backgroundColor: unreadMessageIconColor,
              radius: 10,
              child: Text("$unreadCount",
                  style: TextStyle(color: hightLightTextColor, fontSize: 10))));
    } else if (chat.unreadEventCount > 0) {
      textColor = hightLightTextColor;
      trailing = Container(
          margin: const EdgeInsets.all(1),
          child:
              CircleAvatar(backgroundColor: unreadMessageIconColor, radius: 3));
    }

    var avatarColor = colorFromNick(chat.nick);
    var avatarTextColor =
        ThemeData.estimateBrightnessForColor(avatarColor) == Brightness.dark
            ? hightLightTextColor
            : darkTextColor;
    var popMenuButton = InteractiveAvatar(
        bgColor: selectedBackgroundColor,
        chatNick: chat.nick,
        onTap: () {
          widget.makeActive(chat);
          widget.showSubMenu(chat.id);
        },
        avatarColor: avatarColor,
        avatarTextColor: avatarTextColor);

    return Container(
      decoration: BoxDecoration(
        color: chat.active ? selectedBackgroundColor : null,
        borderRadius: BorderRadius.circular(3),
      ),
      child: isGc
          ? GcContexMenu(
              client: client,
              targetGcChat: chat,
              child: ListTile(
                enabled: true,
                title: Text(chat.nick,
                    style: TextStyle(fontSize: 11, color: textColor)),
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
                enabled: true,
                title: Text(chat.nick,
                    style: TextStyle(fontSize: 11, color: textColor)),
                leading: popMenuButton,
                trailing: trailing,
                selected: chat.active,
                onTap: () => widget.makeActive(chat),
                selectedColor: selectedBackgroundColor,
              ),
            ),
    );
  }
}

class _ChatHeadingRibbonW extends StatefulWidget {
  final ChatModel chat;
  final ClientModel client;
  final MakeActiveCB makeActive;
  final ShowSubMenuCB showSubMenu;
  final bool isGc;

  const _ChatHeadingRibbonW(
      this.chat, this.client, this.makeActive, this.showSubMenu, this.isGc,
      {Key? key})
      : super(key: key);

  @override
  State<_ChatHeadingRibbonW> createState() => _ChatHeadingRibbonWState();
}

class _ChatHeadingRibbonWState extends State<_ChatHeadingRibbonW> {
  ChatModel get chat => widget.chat;
  ClientModel get client => widget.client;
  bool get isGc => widget.isGc;

  void chatUpdated() => setState(() {});

  @override
  void initState() {
    super.initState();
    chat.addListener(chatUpdated);
  }

  @override
  void didUpdateWidget(_ChatHeadingRibbonW oldWidget) {
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
    var unreadMessageIconColor = theme.indicatorColor;
    var darkTextColor = theme.indicatorColor;

    // Show 1k+ if unread cound goes about 1000
    var unreadCount = chat.unreadMsgCount > 1000 ? "1k+" : chat.unreadMsgCount;

    Widget? trailing;
    if (chat.active) {
      // Do we want to do any text color changes on active?
    } else if (chat.unreadMsgCount > 0) {
      textColor = hightLightTextColor;
      trailing = Container(
          margin: const EdgeInsets.all(1),
          child: CircleAvatar(
              backgroundColor: unreadMessageIconColor,
              radius: 10,
              child: Text("$unreadCount",
                  style: TextStyle(color: hightLightTextColor, fontSize: 10))));
    } else if (chat.unreadEventCount > 0) {
      textColor = hightLightTextColor;
      trailing = Container(
          margin: const EdgeInsets.all(1),
          child:
              CircleAvatar(backgroundColor: unreadMessageIconColor, radius: 3));
    }

    var avatarColor = colorFromNick(chat.nick);
    var avatarTextColor =
        ThemeData.estimateBrightnessForColor(avatarColor) == Brightness.dark
            ? hightLightTextColor
            : darkTextColor;
    var popMenuButton = InteractiveAvatar(
        bgColor: selectedBackgroundColor,
        chatNick: chat.nick,
        onTap: () {
          widget.makeActive(chat);
          widget.showSubMenu(chat.id);
        },
        avatarColor: avatarColor,
        avatarTextColor: avatarTextColor);

    return Container(
      decoration: BoxDecoration(
        color: chat.active ? selectedBackgroundColor : null,
        borderRadius: BorderRadius.circular(3),
      ),
      child: isGc
          ? GcContexMenu(
              client: client,
              targetGcChat: chat,
              child: ListTile(
                enabled: true,
                subtitle: Text(chat.nick,
                    style: TextStyle(fontSize: 11, color: textColor)),
                title: popMenuButton,
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
                enabled: true,
                subtitle: Text(chat.nick,
                    style: TextStyle(fontSize: 11, color: textColor)),
                title: popMenuButton,
                trailing: trailing,
                selected: chat.active,
                onTap: () => widget.makeActive(chat),
                selectedColor: selectedBackgroundColor,
              ),
            ),
    );
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
  final FocusNode inputFocusNode;
  const _ChatsList(this.client, this.inputFocusNode, {Key? key})
      : super(key: key);

  @override
  State<_ChatsList> createState() => _ChatsListState();
}

class _ChatsListState extends State<_ChatsList> {
  ClientModel get client => widget.client;
  FocusNode get inputFocusNode => widget.inputFocusNode;
  Timer? _debounce;

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
    void showAddressBook() async {
      client.showAddressBookScreen();
    }

    void genInvite() async {
      await generateInvite(context);
      inputFocusNode.requestFocus();
    }

    var theme = Theme.of(context);
    var sidebarBackground = theme.backgroundColor;
    var hoverColor = theme.hoverColor;
    var darkTextColor = theme.focusColor;
    var selectedBackgroundColor = theme.highlightColor;

    var gcList = client.gcChats.toList();
    var chatList = client.userChats.toList();

    makeActive(ChatModel? c) => {client.active = c};

    showGCSubMenu(String id) => {client.showSubMenu(true, id)};
    showUserSubMenu(String id) => {client.showSubMenu(false, id)};
    return Column(children: [
      Container(
          height: 230,
          margin: const EdgeInsets.all(1),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(3),
            gradient: LinearGradient(
                begin: Alignment.centerRight,
                end: Alignment.centerLeft,
                colors: [
                  hoverColor,
                  sidebarBackground,
                  sidebarBackground,
                ],
                stops: const [
                  0,
                  0.51,
                  1
                ]),
          ),
          child: Stack(children: [
            Container(
              padding: const EdgeInsets.only(bottom: 40),
              child: ListView.builder(
                itemCount: gcList.length,
                itemBuilder: (context, index) => _ChatHeadingW(
                    gcList[index], client, makeActive, showGCSubMenu, true),
              ),
            ),
            !client.showAddressBook
                ? Positioned(
                    bottom: 5,
                    right: 5,
                    child: Material(
                        color: selectedBackgroundColor.withOpacity(0),
                        child: IconButton(
                            splashRadius: 15,
                            iconSize: 15,
                            hoverColor: selectedBackgroundColor,
                            tooltip: "Address Book",
                            onPressed: () => showAddressBook(),
                            icon: Icon(color: darkTextColor, Icons.add))))
                : const Empty()
          ])),
      Expanded(
          child: Container(
        margin: const EdgeInsets.all(1),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(3),
          gradient: LinearGradient(
              begin: Alignment.centerRight,
              end: Alignment.centerLeft,
              colors: [
                hoverColor,
                sidebarBackground,
                sidebarBackground,
              ],
              stops: const [
                0,
                0.51,
                1
              ]),
        ),
        child: Stack(children: [
          Container(
              padding: const EdgeInsets.only(bottom: 40),
              child: ListView.builder(
                  itemCount: chatList.length,
                  itemBuilder: (context, index) => _ChatHeadingW(
                      chatList[index],
                      client,
                      makeActive,
                      showUserSubMenu,
                      false))),
          !client.showAddressBook
              ? Positioned(
                  bottom: 5,
                  right: 5,
                  child: Material(
                      color: selectedBackgroundColor.withOpacity(0),
                      child: IconButton(
                          hoverColor: selectedBackgroundColor,
                          splashRadius: 15,
                          iconSize: 15,
                          tooltip: "Address Book",
                          onPressed: () => showAddressBook(),
                          icon:
                              Icon(size: 15, color: darkTextColor, Icons.add))))
              : const Empty(),
          Positioned(
              bottom: 5,
              right: 25,
              child: Material(
                  color: selectedBackgroundColor.withOpacity(0),
                  child: IconButton(
                      hoverColor: selectedBackgroundColor,
                      splashRadius: 15,
                      iconSize: 15,
                      tooltip: client.isOnline
                          ? "Fetch invite using key"
                          : "Cannot fetch invite while client is offline",
                      onPressed:
                          client.isOnline ? () => fetchInvite(context) : null,
                      icon: Icon(
                          size: 15,
                          color: darkTextColor,
                          Icons.get_app_sharp)))),
          Positioned(
              bottom: 5,
              left: 30,
              child: Material(
                  color: selectedBackgroundColor.withOpacity(0),
                  child: IconButton(
                      hoverColor: selectedBackgroundColor,
                      splashRadius: 15,
                      iconSize: 15,
                      tooltip: "List last received message time",
                      onPressed: () => gotoContactsLastMsgTimeScreen(context),
                      icon: Icon(
                          size: 15,
                          color: darkTextColor,
                          Icons.list_rounded)))),
          Positioned(
              bottom: 5,
              left: 5,
              child: Material(
                  color: selectedBackgroundColor.withOpacity(0),
                  child: IconButton(
                      hoverColor: selectedBackgroundColor,
                      splashRadius: 15,
                      iconSize: 15,
                      tooltip: client.isOnline
                          ? "Generate Invite"
                          : "Cannot generate invite while offline",
                      onPressed: client.isOnline ? genInvite : null,
                      icon:
                          Icon(size: 15, color: darkTextColor, Icons.people))))
        ]),
      ))
    ]);
  }
}

class _ChatsRibbonList extends StatefulWidget {
  final ClientModel chats;
  final FocusNode inputFocusNode;
  const _ChatsRibbonList(this.chats, this.inputFocusNode, {Key? key})
      : super(key: key);

  @override
  State<_ChatsRibbonList> createState() => _ChatsRibbonListState();
}

class _ChatsRibbonListState extends State<_ChatsRibbonList> {
  ClientModel get chats => widget.chats;
  FocusNode get inputFocusNode => widget.inputFocusNode;

  void chatsUpdated() => setState(() {});

  final scrollController = ScrollController();

  @override
  void initState() {
    super.initState();
    chats.addListener(chatsUpdated);
  }

  @override
  void didUpdateWidget(_ChatsRibbonList oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.chats.removeListener(chatsUpdated);
    chats.addListener(chatsUpdated);
  }

  @override
  void dispose() {
    chats.removeListener(chatsUpdated);
    scrollController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    void createGC() async {
      Navigator.of(context, rootNavigator: true).pushNamed('/newGC');
    }

    void genInvite() async {
      await generateInvite(context);
      inputFocusNode.requestFocus();
    }

    void closeMenus(ClientModel client) {
      inputFocusNode.requestFocus();
    }

    var theme = Theme.of(context);
    var sidebarBackground = theme.backgroundColor;
    var hoverColor = theme.hoverColor;
    var darkTextColor = theme.focusColor;
    var selectedBackgroundColor = theme.highlightColor;

    var gcList = chats.gcChats.toList();
    var chatList = chats.userChats.toList();

    makeActive(ChatModel? c) => {chats.active = c};

    showGCSubMenu(String id) => {chats.showSubMenu(true, id)};
    showUserSubMenu(String id) => {chats.showSubMenu(false, id)};
    return Expanded(
        child: Listener(
            onPointerSignal: (event) {
              if (event is PointerScrollEvent) {
                scrollController.animateTo(
                    scrollController.offset + event.scrollDelta.dy,
                    duration: const Duration(milliseconds: 2),
                    curve: Curves.bounceIn);
              }
            },
            child: ListView(
                controller: scrollController,
                scrollDirection: Axis.horizontal,
                children: [
                  ListView.builder(
                    scrollDirection: Axis.horizontal,
                    shrinkWrap: true,
                    itemCount: gcList.length,
                    itemBuilder: (context, index) => SizedBox(
                        height: 100,
                        width: 100,
                        child: _ChatHeadingRibbonW(gcList[index], chats,
                            makeActive, showGCSubMenu, true)),
                  ),
                  Material(
                      color: selectedBackgroundColor.withOpacity(0),
                      child: IconButton(
                          splashRadius: 40,
                          iconSize: 50,
                          hoverColor: selectedBackgroundColor,
                          tooltip: "Add GC",
                          onPressed: () => createGC(),
                          icon: Icon(color: darkTextColor, Icons.add))),
                  ListView.builder(
                      scrollDirection: Axis.horizontal,
                      shrinkWrap: true,
                      itemCount: chatList.length,
                      itemBuilder: (context, index) => SizedBox(
                          height: 100,
                          width: 100,
                          child: _ChatHeadingRibbonW(chatList[index], chats,
                              makeActive, showUserSubMenu, false))),
                  Material(
                      color: selectedBackgroundColor.withOpacity(0),
                      child: IconButton(
                          hoverColor: selectedBackgroundColor,
                          splashRadius: 15,
                          iconSize: 50,
                          tooltip: chats.isOnline
                              ? "Load Invite"
                              : "Cannot load invite while client is offline",
                          onPressed:
                              chats.isOnline ? () => loadInvite(context) : null,
                          icon:
                              Icon(size: 15, color: darkTextColor, Icons.add))),
                  Material(
                      color: selectedBackgroundColor.withOpacity(0),
                      child: IconButton(
                          hoverColor: selectedBackgroundColor,
                          splashRadius: 15,
                          iconSize: 50,
                          tooltip: chats.isOnline
                              ? "Fetch invite using key"
                              : "Cannot fetch invite while client is offline",
                          onPressed: chats.isOnline
                              ? () => fetchInvite(context)
                              : null,
                          icon: Icon(
                              size: 15,
                              color: darkTextColor,
                              Icons.get_app_sharp))),
                  Material(
                      color: selectedBackgroundColor.withOpacity(0),
                      child: IconButton(
                          hoverColor: selectedBackgroundColor,
                          splashRadius: 15,
                          iconSize: 50,
                          tooltip: "List last received message time",
                          onPressed: () =>
                              gotoContactsLastMsgTimeScreen(context),
                          icon: Icon(
                              size: 15,
                              color: darkTextColor,
                              Icons.list_rounded))),
                  Material(
                      color: selectedBackgroundColor.withOpacity(0),
                      child: IconButton(
                          hoverColor: selectedBackgroundColor,
                          splashRadius: 15,
                          iconSize: 50,
                          tooltip: chats.isOnline
                              ? "Generate Invite"
                              : "Cannot generate invite while offline",
                          onPressed: chats.isOnline ? genInvite : null,
                          icon: Icon(
                              size: 15, color: darkTextColor, Icons.people)))
                ])));
  }
}

class ChatDrawerMenu extends StatelessWidget {
  final FocusNode inputFocusNode;
  const ChatDrawerMenu(this.inputFocusNode, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    return Consumer<ClientModel>(builder: (context, client, child) {
      return Column(
          children: [Expanded(child: _ChatsList(client, inputFocusNode))]);
    });
  }
}

class ChatRibbonMenu extends StatelessWidget {
  final FocusNode inputFocusNode;
  const ChatRibbonMenu(this.inputFocusNode, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    return Consumer<ClientModel>(builder: (context, chats, child) {
      return _ChatsRibbonList(chats, inputFocusNode);
    });
  }
}
