import 'dart:async';
import 'dart:collection';
import 'package:bruig/components/containers.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/realtimechat.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/chat/new_gc_screen.dart';
import 'package:bruig/screens/chat/new_message_screen.dart';
import 'package:bruig/screens/chats.dart';
import 'package:bruig/screens/contacts_msg_times.dart';
import 'package:bruig/screens/gc_invitations.dart';
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
  final bool isActiveRTC;

  const _ChatHeadingW(this.chat, this.client, this.makeActive, this.showSubMenu,
      this.isActiveRTC);

  @override
  State<_ChatHeadingW> createState() => _ChatHeadingWState();
}

class _ChatHeadingWState extends State<_ChatHeadingW> {
  ChatModel get chat => widget.chat;
  ClientModel get client => widget.client;
  bool get isActiveRTC => widget.isActiveRTC;

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
      avatar: chat.avatar.image,
      toolTip: true,
    );
    bool isScreenSmall = checkIsScreenSmall(context);
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
                        tileColor: isActiveRTC ? Colors.green.shade600 : null,
                        selectedTileColor:
                            isActiveRTC ? Colors.green.shade600 : null,
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
  final RealtimeChatModel rtc;
  const ActiveChatsListMenu(this.client, this.inputFocusNode, this.rtc,
      {super.key});

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
  if (context.mounted) {
    Navigator.of(context, rootNavigator: true)
        .pushNamed('/verifyInvite', arguments: invite);
  }
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
      this.onlyWhenOnline = false});

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
      {required this.icon, required this.tooltip, required this.onPressed});

  @override
  Widget build(BuildContext context) {
    return Consumer2<ThemeNotifier, ConnStateModel>(
        builder: (context, theme, connState, child) => Material(
            borderRadius: BorderRadius.circular(30),
            color: theme.colors.surfaceContainerHigh.withValues(alpha: 0.7),
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

class _ActiveChatsListMenuState extends State<ActiveChatsListMenu>
    with SingleTickerProviderStateMixin {
  ClientModel get client => widget.client;
  FocusNode get inputFocusNode => widget.inputFocusNode.inputFocusNode;
  RealtimeChatModel get rtc => widget.rtc;
  UnmodifiableListView<ChatModel> chats = UnmodifiableListView([]);
  Timer? debounce;
  ScrollController sortedListScroll = ScrollController();

  void doUpdateState() {
    if (mounted) {
      setState(() {
        chats = client.activeChats.sorted;
      });
    }
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

  void showGCInvitationsScreen() {
    Navigator.of(context, rootNavigator: true)
        .pushNamed(GCInvitationsScreen.routeName);
  }

  // Returns a callback to make chat c active.
  void makeActive(ChatModel? c) => {client.active = c};

  void showSubMenu() => client.ui.chatSideMenuActive.chat = client.active;

  void gotoNewMessage() =>
      Navigator.of(context).pushNamed(NewMessageScreen.routeName);

  void gotoNewGroupChat() =>
      Navigator.of(context).pushNamed(NewGcScreen.routeName);

  bool hasLiveRTCSess = false;
  bool hasHotAudio = false;
  bool get hasAnimation => hasLiveRTCSess || hasHotAudio;

  late AnimationController bgColorCtrl;
  late Animation<Color?> bgColorAnim;

  void rtcChanged() {
    bool newHasHotAudio = rtc.hotAudioSession.active?.inLiveSession ?? false;
    bool newHasLive = rtc.liveSessions.hasSessions;
    if (newHasLive != hasLiveRTCSess || newHasHotAudio != hasHotAudio) {
      setState(() {
        hasLiveRTCSess = newHasLive;
        hasHotAudio = newHasHotAudio;
      });
      if (hasAnimation) {
        bgColorCtrl.repeat();
      } else {
        bgColorCtrl.stop();
      }
    }
  }

  @override
  void initState() {
    super.initState();
    client.activeChats.addListener(activeChatsListUpdated);
    activeChatsListUpdated();

    rtc.hotAudioSession.addListener(rtcChanged);
    rtc.liveSessions.addListener(rtcChanged);

    // Initialize animation controller
    bgColorCtrl = AnimationController(
      duration: const Duration(seconds: 2),
      vsync: this,
    );

    // Create the color animation sequence
    bgColorAnim = TweenSequence<Color?>([
      TweenSequenceItem(
        weight: 1.0,
        tween: ColorTween(
          begin: Colors.green.shade600,
          end: Colors.green.shade900,
        ),
      ),
      TweenSequenceItem(
        weight: 1.0,
        tween: ColorTween(
          begin: Colors.green.shade900,
          end: Colors.green.shade600,
        ),
      ),
    ]).animate(bgColorCtrl);
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

    bgColorCtrl.dispose();
    rtc.hotAudioSession.removeListener(rtcChanged);
    rtc.liveSessions.removeListener(rtcChanged);

    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = checkIsScreenSmall(context);

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
                        chats[index],
                        client,
                        makeActive,
                        showSubMenu,
                        chats[index].hasInstantCall))),
            Positioned(
                bottom: 20,
                right: 10,
                child: _SmallScreenFabIconButton(
                    tooltip: "New Message",
                    icon: Icons.edit_outlined,
                    onPressed: gotoNewMessage)),
            Positioned(
                bottom: 100,
                right: 10,
                child: _SmallScreenFabIconButton(
                    tooltip: "Create new group chat",
                    icon: Icons.people_outlined,
                    onPressed: gotoNewGroupChat)),
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
                _ChatHeadingW(chats[index], client, makeActive, showSubMenu,
                    chats[index].hasInstantCall))),
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
                  tooltip: "Fetch, import or accept invite",
                  onPressed: () => fetchInvite(context),
                  icon: Icons.get_app_outlined),
              _FooterIconButton(
                  tooltip: "Create new group chat",
                  onPressed: gotoNewGroupChat,
                  icon: Icons.people_outline),
              _FooterIconButton(
                  tooltip: "New Message",
                  onPressed: gotoNewMessage,
                  icon: Icons.edit_outlined),
              _FooterIconButton(
                  tooltip: "Show GC Invitations",
                  onPressed: showGCInvitationsScreen,
                  icon: Icons.groups),
            ])));
  }
}
