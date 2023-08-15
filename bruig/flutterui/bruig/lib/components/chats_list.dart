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
          widget.showSubMenu(chat.isGC, chat.id);
        },
        avatarColor: avatarColor,
        avatarTextColor: avatarTextColor);

    return Container(
      decoration: BoxDecoration(
        color: chat.active ? selectedBackgroundColor : null,
        borderRadius: BorderRadius.circular(3),
      ),
      child: chat.isGC
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
  bool expandRooms = true;
  bool expandUsers = true;
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
    void createGC() async {
      Navigator.of(context, rootNavigator: true).pushNamed('/newGC');
    }

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
    var darkTextColor = theme.dividerColor;
    var selectedBackgroundColor = theme.highlightColor;
    var dividerColor = theme.highlightColor;
    var backgroundColor = theme.backgroundColor;

    var gcList = client.gcChats.toList();
    var chatList = client.userChats.toList();

    makeActive(ChatModel? c) => {client.active = c};

    showSubMenu(bool isGC, String id) => {client.showSubMenu(isGC, id)};
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    if (isScreenSmall) {
      return Container(
        margin: const EdgeInsets.all(1),
        decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(3), color: backgroundColor),
        padding: const EdgeInsets.all(16),
        child: ListView(children: [
          Container(
            padding:
                const EdgeInsets.only(left: 20, right: 5, top: 5, bottom: 5),
            decoration: BoxDecoration(
              color: dividerColor,
              borderRadius: BorderRadius.circular(10),
            ),
            child:
                Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Row(mainAxisAlignment: MainAxisAlignment.spaceBetween, children: [
                Row(children: [
                  Text("Room",
                      style: TextStyle(
                          color: darkTextColor,
                          fontSize: 18,
                          fontWeight: FontWeight.w400)),
                  IconButton(
                      onPressed: () => createGC(),
                      tooltip: 'Create a new group chat',
                      icon: Icon(
                          color: darkTextColor,
                          size: 25,
                          weight: 1,
                          Icons.add_outlined)),
                ]),
                IconButton(
                    onPressed: () => setState(() {
                          expandRooms = !expandRooms;
                        }),
                    icon: Icon(
                        color: darkTextColor,
                        size: 25,
                        expandRooms
                            ? Icons.keyboard_arrow_up_outlined
                            : Icons.keyboard_arrow_down_outlined)),
              ]),
              expandRooms
                  ? ListView.builder(
                      scrollDirection: Axis.vertical,
                      shrinkWrap: true,
                      itemCount: gcList.length,
                      itemBuilder: (context, index) => _ChatHeadingW(
                          gcList[index], client, makeActive, showSubMenu))
                  : const Empty()
            ]),
          ),
          const SizedBox(height: 10),
          Container(
              padding:
                  const EdgeInsets.only(left: 20, right: 5, top: 5, bottom: 5),
              decoration: BoxDecoration(
                color: dividerColor,
                borderRadius: BorderRadius.circular(10),
              ),
              child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                        children: [
                          Row(children: [
                            Text("Users",
                                style: TextStyle(
                                    color: darkTextColor,
                                    fontSize: 18,
                                    fontWeight: FontWeight.w400)),
                            IconButton(
                                onPressed: () => showAddressBook(),
                                tooltip: 'Address book',
                                icon: Icon(
                                    color: darkTextColor,
                                    size: 25,
                                    Icons.add_outlined)),
                          ]),
                          IconButton(
                              onPressed: () => setState(() {
                                    expandUsers = !expandUsers;
                                  }),
                              icon: Icon(
                                  color: darkTextColor,
                                  size: 25,
                                  expandUsers
                                      ? Icons.keyboard_arrow_up_outlined
                                      : Icons.keyboard_arrow_down_outlined)),
                        ]),
                    expandUsers
                        ? ListView.builder(
                            scrollDirection: Axis.vertical,
                            shrinkWrap: true,
                            itemCount: chatList.length,
                            itemBuilder: (context, index) => _ChatHeadingW(
                                chatList[index],
                                client,
                                makeActive,
                                showSubMenu))
                        : const Empty()
                  ])),
          const SizedBox(height: 10),
          Container(
              padding:
                  const EdgeInsets.only(left: 20, right: 20, top: 5, bottom: 5),
              decoration: BoxDecoration(
                color: dividerColor,
                borderRadius: BorderRadius.circular(10),
              ),
              child: Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Material(
                        color: selectedBackgroundColor.withOpacity(0),
                        child: IconButton(
                            hoverColor: selectedBackgroundColor,
                            splashRadius: 15,
                            iconSize: 25,
                            tooltip: "Address Book",
                            onPressed: () => showAddressBook(),
                            icon: Icon(
                                size: 25, color: darkTextColor, Icons.add))),
                    Material(
                        color: selectedBackgroundColor.withOpacity(0),
                        child: IconButton(
                            hoverColor: selectedBackgroundColor,
                            splashRadius: 15,
                            iconSize: 25,
                            tooltip: client.isOnline
                                ? "Fetch invite using key"
                                : "Cannot fetch invite while client is offline",
                            onPressed: client.isOnline
                                ? () => fetchInvite(context)
                                : null,
                            icon: Icon(
                                size: 25,
                                color: darkTextColor,
                                Icons.get_app_sharp))),
                    Material(
                        color: selectedBackgroundColor.withOpacity(0),
                        child: IconButton(
                            hoverColor: selectedBackgroundColor,
                            splashRadius: 15,
                            iconSize: 25,
                            tooltip: "List last received message time",
                            onPressed: () =>
                                gotoContactsLastMsgTimeScreen(context),
                            icon: Icon(
                                size: 25,
                                color: darkTextColor,
                                Icons.list_rounded))),
                    Material(
                        color: selectedBackgroundColor.withOpacity(0),
                        child: IconButton(
                            hoverColor: selectedBackgroundColor,
                            splashRadius: 15,
                            iconSize: 25,
                            tooltip: client.isOnline
                                ? "Generate Invite"
                                : "Cannot generate invite while offline",
                            onPressed: client.isOnline ? genInvite : null,
                            icon: Icon(
                                size: 25, color: darkTextColor, Icons.people)))
                  ]))
        ]),
      );
    }
    return Container(
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
            margin: const EdgeInsets.only(bottom: 50),
            child: ListView(children: [
              Column(children: [
                Row(children: [
                  Expanded(
                      child: TextButton(
                    onPressed: () => setState(() {
                      expandRooms = !expandRooms;
                    }),
                    onHover: (show) => setState(() {
                      showAddressbookRoomsButton = show;
                    }),
                    child: Row(children: [
                      RichText(
                          text: TextSpan(children: [
                        WidgetSpan(
                            alignment: PlaceholderAlignment.middle,
                            child: Icon(
                                color: darkTextColor,
                                size: 15,
                                expandRooms
                                    ? Icons.arrow_drop_down_sharp
                                    : Icons.arrow_drop_up_sharp)),
                        TextSpan(
                            text: "  Rooms",
                            style: TextStyle(
                                color: darkTextColor,
                                fontSize: 15,
                                fontWeight: FontWeight.w200)),
                      ])),
                    ]),
                  )),
                  showAddressbookRoomsButton
                      ? TextButton(
                          onPressed: () => createGC(),
                          onHover: (show) => setState(() {
                                showAddressbookRoomsButton = show;
                              }),
                          child: Tooltip(
                              message: 'Create a new group chat',
                              child: Icon(
                                  color: darkTextColor,
                                  size: 15,
                                  Icons.more_vert)))
                      : const SizedBox(height: 20, width: 15)
                ]),
                expandRooms
                    ? ListView.builder(
                        scrollDirection: Axis.vertical,
                        shrinkWrap: true,
                        itemCount: gcList.length,
                        itemBuilder: (context, index) => _ChatHeadingW(
                            gcList[index], client, makeActive, showSubMenu))
                    : const Empty()
              ]),
              Column(children: [
                Row(children: [
                  Expanded(
                      child: TextButton(
                    onPressed: () => setState(() {
                      expandUsers = !expandUsers;
                    }),
                    onHover: (show) => setState(() {
                      showAddressbookUsersButton = show;
                    }),
                    child: Row(children: [
                      RichText(
                          text: TextSpan(children: [
                        WidgetSpan(
                            alignment: PlaceholderAlignment.middle,
                            child: Icon(
                                color: darkTextColor,
                                size: 15,
                                expandUsers
                                    ? Icons.arrow_drop_down_sharp
                                    : Icons.arrow_drop_up_sharp)),
                        TextSpan(
                            text: "  Users",
                            style: TextStyle(
                                color: darkTextColor,
                                fontSize: 15,
                                fontWeight: FontWeight.w200)),
                      ])),
                    ]),
                  )),
                  showAddressbookUsersButton
                      ? TextButton(
                          onPressed: () => showAddressBook(),
                          onHover: (show) => setState(() {
                                showAddressbookUsersButton = show;
                              }),
                          child: Tooltip(
                              message: 'Start a new chat',
                              child: Icon(
                                  color: darkTextColor,
                                  size: 15,
                                  Icons.more_vert)))
                      : const SizedBox(height: 20, width: 15)
                ]),
                expandUsers
                    ? ListView.builder(
                        scrollDirection: Axis.vertical,
                        shrinkWrap: true,
                        itemCount: chatList.length,
                        itemBuilder: (context, index) => _ChatHeadingW(
                            chatList[index], client, makeActive, showSubMenu))
                    : const Empty()
              ]),
            ])),
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
                        icon: Icon(size: 15, color: darkTextColor, Icons.add))))
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
                        size: 15, color: darkTextColor, Icons.get_app_sharp)))),
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
                        size: 15, color: darkTextColor, Icons.list_rounded)))),
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
                    icon: Icon(size: 15, color: darkTextColor, Icons.people))))
      ]),
    );
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
