import 'dart:async';

import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/manage_gc.dart';
import 'package:bruig/models/menus.dart';
import 'package:bruig/screens/chats.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:bruig/components/profile.dart';
import 'package:bruig/components/chat/messages.dart';
import 'package:scrollable_positioned_list/scrollable_positioned_list.dart';
import 'package:bruig/components/chat/input.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';
import 'package:bruig/components/empty_widget.dart';

class ActiveChat extends StatefulWidget {
  final ClientModel client;
  final CustomInputFocusNode inputFocusNode;
  const ActiveChat(this.client, this.inputFocusNode, {Key? key})
      : super(key: key);

  @override
  State<ActiveChat> createState() => _ActiveChatState();
}

/// TODO: Figure out a way to estimate list size to set initialOffset.
/// this way we can get rid of the "initial jump flicker"
class _ActiveChatState extends State<ActiveChat> {
  ClientModel get client => widget.client;
  CustomInputFocusNode get inputFocusNode => widget.inputFocusNode;
  ChatModel? chat;
  late ItemScrollController _itemScrollController;
  late ItemPositionsListener _itemPositionsListener;
  Timer? _debounce;

  void activeChatChanged() {
    var newChat = client.active;
    if (newChat != chat) {
      setState(() {
        chat = newChat;
      });
    }
  }

  void sendMsg(String msg) {
    if (this.chat == null) {
      return;
    }
    ChatModel chat = this.chat!;

    if (_debounce?.isActive ?? false) _debounce!.cancel();
    _debounce = Timer(const Duration(milliseconds: 100), () async {
      try {
        await chat.sendMsg(msg);
        client.newSentMsg(chat);
      } catch (exception) {
        if (mounted) {
          showErrorSnackbar(context, "Unable to send message: $exception");
        }
      }
    });
  }

  @override
  void initState() {
    super.initState();
    _itemScrollController = ItemScrollController();
    _itemPositionsListener = ItemPositionsListener.create();
    chat = client.active;
    client.activeChat.addListener(activeChatChanged);
  }

  @override
  void didUpdateWidget(ActiveChat oldWidget) {
    super.didUpdateWidget(oldWidget);

    if (oldWidget.client != widget.client) {
      oldWidget.client.removeListener(activeChatChanged);
      client.addListener(activeChatChanged);
      activeChatChanged();
    }
  }

  @override
  void dispose() {
    _debounce?.cancel();
    client.activeChat.removeListener(activeChatChanged);
    //inputFocusNode.dispose();  XXX Does this need to be put back?  Errors with it
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (this.chat == null) return Container();
    var chat = this.chat!;
    var profile = client.profile;
    if (profile != null) {
      if (chat.isGC) {
        return const ManageGCScreen();
      } else {
        return UserProfile(client, profile);
      }
    }
    //inputFocusNode.requestFocus();
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    var darkTextColor = theme.indicatorColor;
    var selectedBackgroundColor = theme.highlightColor;
    var subMenuBorderColor = theme.canvasColor;

    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    List<ChatMenuItem> activeSubMenu =
        client.activeSubMenu.whereType<ChatMenuItem>().toList();
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => isScreenSmall
            ? activeSubMenu.isNotEmpty
                ? Stack(
                    alignment: Alignment.topRight,
                    children: [
                      Column(children: [
                        Container(
                          margin: const EdgeInsets.only(top: 20, bottom: 20),
                          child: UserMenuAvatar(client, chat, radius: 75),
                        ),
                        Visibility(
                          visible: chat.isGC,
                          child: Text("Group Chat",
                              style: TextStyle(
                                  fontSize: theme.getMediumFont(context),
                                  color: textColor)),
                        ),
                        Text(chat.nick,
                            style: TextStyle(
                                fontSize: theme.getMediumFont(context),
                                color: textColor)),
                        Expanded(
                            child: ListView.builder(
                          shrinkWrap: true,
                          itemCount: activeSubMenu.length,
                          itemBuilder: (context, index) => ListTile(
                              title: Text(activeSubMenu[index].label,
                                  style: TextStyle(
                                      fontSize: theme.getSmallFont(context))),
                              onTap: () {
                                activeSubMenu[index]
                                    .onSelected(context, client);
                                client.hideSubMenu();
                              },
                              hoverColor: Colors.black),
                        )),
                      ]),
                      Positioned(
                        top: 5,
                        right: 5,
                        child: Material(
                          color: selectedBackgroundColor.withOpacity(0),
                          child: IconButton(
                            tooltip: "Close",
                            hoverColor: selectedBackgroundColor,
                            splashRadius: 15,
                            iconSize: 15,
                            onPressed: () => client.hideSubMenu(),
                            icon: Icon(
                                color: darkTextColor, Icons.close_outlined),
                          ),
                        ),
                      ),
                    ],
                  )
                : Column(children: [
                    Expanded(
                      child: Messages(chat, client, _itemScrollController,
                          _itemPositionsListener),
                    ),
                    Container(
                        margin: const EdgeInsets.all(10),
                        child: Input(sendMsg, chat, inputFocusNode))
                  ])
            : Row(children: [
                Expanded(
                  child: Column(children: [
                    Expanded(
                      child: Messages(chat, client, _itemScrollController,
                          _itemPositionsListener),
                    ),
                    Container(
                        padding: const EdgeInsets.all(5),
                        child: Input(sendMsg, chat, inputFocusNode))
                  ]),
                ),
                Visibility(
                  visible: activeSubMenu.isNotEmpty,
                  child: Container(
                    width: 250,
                    decoration: BoxDecoration(
                      border: Border(
                        left: BorderSide(width: 2, color: subMenuBorderColor),
                      ),
                    ),
                    child: Stack(alignment: Alignment.topRight, children: [
                      Column(children: [
                        Container(
                          margin: const EdgeInsets.only(top: 20, bottom: 20),
                          child: UserMenuAvatar(client, chat, radius: 75),
                        ),
                        Visibility(
                          visible: chat.isGC,
                          child: Text("Group Chat",
                              style: TextStyle(
                                  fontSize: theme.getMediumFont(context),
                                  color: textColor)),
                        ),
                        Text(chat.nick,
                            style: TextStyle(
                                fontSize: theme.getMediumFont(context),
                                color: textColor)),
                        Container(
                            margin: const EdgeInsets.all(10),
                            child: Copyable(
                                chat.id,
                                TextStyle(
                                    fontSize: theme.getSmallFont(context),
                                    color: textColor,
                                    overflow: TextOverflow.ellipsis))),
                        Expanded(
                            child: ListView.builder(
                          shrinkWrap: true,
                          itemCount: activeSubMenu.length,
                          itemBuilder: (context, index) => ListTile(
                              title: Text(activeSubMenu[index].label,
                                  style: TextStyle(
                                      fontSize: theme.getSmallFont(context))),
                              onTap: () {
                                activeSubMenu[index]
                                    .onSelected(context, client);
                                client.hideSubMenu();
                              },
                              hoverColor: Colors.black),
                        )),
                      ]),
                      isScreenSmall
                          ? const Empty()
                          : Positioned(
                              top: 5,
                              right: 5,
                              child: Material(
                                color: selectedBackgroundColor.withOpacity(0),
                                child: IconButton(
                                  tooltip: "Close",
                                  hoverColor: selectedBackgroundColor,
                                  splashRadius: 15,
                                  iconSize: 15,
                                  onPressed: () => client.hideSubMenu(),
                                  icon: Icon(
                                      color: darkTextColor,
                                      Icons.close_outlined),
                                ),
                              ),
                            ),
                    ]),
                  ),
                )
              ]));
  }
}
