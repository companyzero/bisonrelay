import 'dart:async';
import 'dart:collection';
import 'package:bruig/components/containers.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/screens/chats.dart';
import 'package:bruig/screens/contacts_msg_times.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/user_context_menu.dart';
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
    // Show 1k+ if unread cound goes about 1000
    var unreadCount = chat.unreadMsgCount > 1000 ? "1k+" : chat.unreadMsgCount;

    Widget unreadIndicator;
    if (chat.unreadMsgCount > 0) {
      // Show unread message count.
      unreadIndicator = Container(
          margin: const EdgeInsets.all(1),
          child: CircleAvatar(radius: 10, child: Txt.S("$unreadCount")));
    } else if (chat.unreadEventCount > 0) {
      // Show only a dot indicator.
      unreadIndicator = Container(
          margin: const EdgeInsets.all(1),
          child: const CircleAvatar(radius: 3));
    } else {
      // Show nothing.
      unreadIndicator = const SizedBox(width: 21);
    }

    var popMenuButton = InteractiveAvatar(
        chatNick: chat.nick,
        onTap: () {
          widget.makeActive(chat);
          widget.showSubMenu();
        },
        avatar: chat.avatar.image);

    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Container(
              child: chat.isGC
                  ? GcContexMenu(
                      mobile: isScreenSmall
                          ? (context) {
                              widget.makeActive(chat);
                              widget.showSubMenu();
                            }
                          : null,
                      client: client,
                      targetGcChat: chat,
                      child: ListTile(
                        horizontalTitleGap: 12,
                        contentPadding:
                            const EdgeInsets.only(left: 10, right: 8),
                        enabled: true,
                        title: Txt(chat.nick,
                            overflow: TextOverflow.ellipsis,
                            color: TextColor.onSurfaceVariant),
                        leading: popMenuButton,
                        trailing:
                            Row(mainAxisSize: MainAxisSize.min, children: [
                          Text("gc",
                              style: theme.extraTextStyles.chatListGcIndicator),
                          const SizedBox(width: 5),
                          unreadIndicator
                        ]),
                        selected: chat.active,
                        onTap: () => widget.makeActive(chat),
                      ),
                    )
                  : UserContextMenu(
                      client: client,
                      targetUserChat: chat,
                      child: ListTile(
                        horizontalTitleGap: 12,
                        contentPadding:
                            const EdgeInsets.only(left: 10, right: 8),
                        enabled: true,
                        title: Txt(chat.nick,
                            overflow: TextOverflow.ellipsis,
                            color: TextColor.onSurfaceVariant),
                        leading: popMenuButton,
                        trailing: unreadIndicator,
                        selected: chat.active,
                        onTap: () => widget.makeActive(chat),
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

class ActiveChatsListMenu extends StatefulWidget {
  final ClientModel client;
  final CustomInputFocusNode inputFocusNode;
  const ActiveChatsListMenu(this.client, this.inputFocusNode, {Key? key})
      : super(key: key);

  @override
  State<ActiveChatsListMenu> createState() => _ActiveChatsListMenuState();
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

class _FooterIconButton extends StatelessWidget {
  final bool onlyWhenOnline;
  final IconData icon;
  final String tooltip;
  final VoidCallback onPressed;
  const _FooterIconButton(
      {required this.icon,
      required this.tooltip,
      required this.onPressed,
      this.onlyWhenOnline = false,
      super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer2<ThemeNotifier, ConnStateModel>(
        builder: (context, theme, connState, child) => IconButton(
            splashRadius: 15,
            iconSize: 15,
            tooltip: !onlyWhenOnline || connState.isOnline
                ? tooltip
                : "Cannot perform this action when offline",
            disabledColor: theme.theme.disabledColor,
            onPressed: !onlyWhenOnline || connState.isOnline ? onPressed : null,
            icon: Icon(icon, size: 20)));
  }
}

class _SmallScreenFabIconButton extends StatelessWidget {
  final IconData icon;
  final String tooltip;
  final VoidCallback onPressed;
  const _SmallScreenFabIconButton(
      {required this.icon,
      required this.tooltip,
      required this.onPressed,
      super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer2<ThemeNotifier, ConnStateModel>(
        builder: (context, theme, connState, child) => Material(
            borderRadius: BorderRadius.circular(30),
            color: theme.colors.surfaceContainerHigh.withOpacity(0.7),
            child: IconButton(
                splashRadius: 28,
                hoverColor: theme.colors.surfaceContainerHigh,
                iconSize: 40,
                tooltip: tooltip,
                disabledColor: theme.theme.disabledColor,
                onPressed: onPressed,
                icon: Icon(icon, size: 49))));
  }
}

class _ActiveChatsListMenuState extends State<ActiveChatsListMenu> {
  ClientModel get client => widget.client;
  FocusNode get inputFocusNode => widget.inputFocusNode.inputFocusNode;
  UnmodifiableListView<ChatModel> chats = UnmodifiableListView([]);
  Timer? debounce;
  ScrollController sortedListScroll = ScrollController();

  void doUpdateState() {
    setState(() => chats = client.activeChats.sorted);
    debounce = null;
  }

  void activeChatsListUpdated() {
    // Limit changes when updating chat list very fast.
    debounce ??= Timer(const Duration(milliseconds: 250), doUpdateState);
  }

  void genInvite() async {
    await generateInvite(context);
    inputFocusNode.requestFocus();
  }

  // Returns a callback to make chat c active.
  void makeActive(ChatModel? c) => {client.active = c};

  void showSubMenu() => client.ui.chatSideMenuActive.chat = client.active;

  @override
  void initState() {
    super.initState();
    client.activeChats.addListener(activeChatsListUpdated);
    activeChatsListUpdated();
  }

  @override
  void didUpdateWidget(ActiveChatsListMenu oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.client != client) {
      oldWidget.client.activeChats.removeListener(activeChatsListUpdated);
      client.activeChats.addListener(activeChatsListUpdated);
      activeChatsListUpdated();
    }
  }

  @override
  void dispose() {
    client.activeChats.removeListener(activeChatsListUpdated);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;

    // Mobile version, display list of chats in entire screen.
    if (isScreenSmall) {
      return Container(
          padding: const EdgeInsets.all(0),
          child: Stack(children: [
            Container(
                padding:
                    const EdgeInsets.only(left: 0, right: 5, top: 5, bottom: 5),
                child: ListView.builder(
                    physics: const ScrollPhysics(),
                    controller: sortedListScroll,
                    scrollDirection: Axis.vertical,
                    shrinkWrap: true,
                    itemCount: chats.length,
                    itemBuilder: (context, index) => _ChatHeadingW(
                        chats[index], client, makeActive, showSubMenu))),
            Positioned(
                bottom: 20,
                right: 10,
                child: _SmallScreenFabIconButton(
                    tooltip: "New Message",
                    icon: Icons.edit_outlined,
                    onPressed: client.ui.showAddressBookScreen)),
            Positioned(
                bottom: 100,
                right: 10,
                child: _SmallScreenFabIconButton(
                    tooltip: "Create new group chat",
                    icon: Icons.people_outlined,
                    onPressed: client.ui.showCreateGroupChatScreen)),
          ]));
    }

    // Desktop version, display side menu.
    return SecondarySideMenuList(
        width: 205,
        list: ListView.builder(
            controller: sortedListScroll,
            scrollDirection: Axis.vertical,
            itemCount: chats.length,
            itemBuilder: (context, index) => SecondarySideMenuItem(
                _ChatHeadingW(chats[index], client, makeActive, showSubMenu))),
        footer: SizedBox(
            width: double.infinity,
            child: Wrap(alignment: WrapAlignment.start, children: [
              _FooterIconButton(
                  onlyWhenOnline: true,
                  tooltip: "Generate Invite",
                  onPressed: genInvite,
                  icon: Icons.add_outlined),
              _FooterIconButton(
                  tooltip: "List last received message time",
                  onPressed: () => gotoContactsLastMsgTimeScreen(context),
                  icon: Icons.list_outlined),
              _FooterIconButton(
                  onlyWhenOnline: true,
                  tooltip: "Fetch invite using key",
                  onPressed: () => fetchInvite(context),
                  icon: Icons.get_app_outlined),
              _FooterIconButton(
                  tooltip: "Create new group chat",
                  onPressed: client.ui.showCreateGroupChatScreen,
                  icon: Icons.people_outline),
              _FooterIconButton(
                  tooltip: "New Message",
                  onPressed: client.ui.showAddressBookScreen,
                  icon: Icons.edit_outlined),
            ])));
  }
}
