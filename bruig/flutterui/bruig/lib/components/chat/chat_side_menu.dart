import 'package:bruig/components/containers.dart';
import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/icons.dart';
import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/menus.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/theme_manager.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

class ChatSideMenu extends StatefulWidget {
  final ClientModel client;
  const ChatSideMenu(this.client, {super.key});

  @override
  State<ChatSideMenu> createState() => _ChatSideMenuState();
}

class _ChatSideMenuState extends State<ChatSideMenu> {
  ChatModel? chat;
  ClientModel get client => widget.client;
  List<ChatMenuItem> menus = [];

  void rebuildMenu() {
    if (client.ui.chatSideMenuActive.empty) {
      setState(() {
        chat = null;
        menus = [];
      });
      return;
    }

    var newChat = client.ui.chatSideMenuActive.chat!;
    var newMenus =
        newChat.isGC ? buildGCMenu(newChat) : buildUserChatMenu(newChat);
    setState(() {
      chat = newChat;
      menus = newMenus;
    });
  }

  @override
  void initState() {
    super.initState();
    rebuildMenu();
    client.ui.chatSideMenuActive.addListener(rebuildMenu);
  }

  @override
  void didUpdateWidget(ChatSideMenu oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.client != client) {
      oldWidget.client.ui.chatSideMenuActive.removeListener(rebuildMenu);
      client.ui.chatSideMenuActive.addListener(rebuildMenu);
      rebuildMenu();
    }
  }

  @override
  void dispose() {
    client.ui.chatSideMenuActive.removeListener(rebuildMenu);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (this.chat == null) {
      return Container();
    }

    ChatModel chat = this.chat!;

    bool isScreenSmall = checkIsScreenSmall(context);

    return Stack(alignment: Alignment.topRight, children: [
      Column(children: [
        Container(
          margin: const EdgeInsets.only(top: 20, bottom: 20),
          child: UserMenuAvatar(client, chat, radius: 75),
        ),
        Visibility(
          visible: chat.isGC,
          child: const Txt.S("Group Chat", color: TextColor.onSurfaceVariant),
        ),
        Txt.S(chat.nick, color: TextColor.onSurfaceVariant),
        Container(
            margin: const EdgeInsets.all(10),
            child: Copyable.txt(Txt.S(
              chat.id,
              overflow: TextOverflow.ellipsis,
              color: TextColor.onSurfaceVariant,
            ))),
        Expanded(
            child: ListView.builder(
                shrinkWrap: true,
                itemCount: menus.length,
                itemBuilder: (context, index) => ListTile(
                    title: Txt.S(menus[index].label),
                    onTap: () {
                      menus[index].onSelected(context, client);
                      client.ui.chatSideMenuActive.clear();
                    }))),
      ]),
      if (!isScreenSmall)
        Positioned(
            top: 5,
            right: 5,
            child: IconButton(
              tooltip: "Close",
              splashRadius: 15,
              iconSize: 15,
              onPressed: client.ui.chatSideMenuActive.clear,
              icon: const ColoredIcon(Icons.close_outlined,
                  color: TextColor.onSurface),
            ))
    ]);
  }
}

// ScreenWithChatSideMenu is a screen that can show the chat side menu.
class ScreenWithChatSideMenu extends StatelessWidget {
  final Widget child;
  final ClientModel client;
  const ScreenWithChatSideMenu(this.client, this.child, {super.key});

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = checkIsScreenSmall(context);

    if (isScreenSmall) {
      return Consumer2<ChatSideMenuActiveModel, ClientModel>(
          child: child,
          builder: (context, chatSideMenuActive, client, child) =>
              chatSideMenuActive.empty
                  ? child ?? const Empty()
                  : ChatSideMenu(client));
    }

    // Alternate chat menu layout: stack on top of main view (avoids reflows
    // and overflow errors).
    return Stack(children: [
      child,
      Consumer2<ChatSideMenuActiveModel, ClientModel>(
          builder: (context, chatSideMenuActive, client, child) => Visibility(
                visible: !chatSideMenuActive.empty,
                child: Positioned(
                    right: 0,
                    width: 250,
                    top: 0,
                    height: MediaQuery.sizeOf(context).height - 60,
                    child: Box(
                        color: SurfaceColor.surface,
                        child: ChatSideMenu(client))),
              )),
    ]);

    // Original layout: reflow main window.
    // return Row(children: [
    //   Expanded(child: child),
    //   Consumer2<ChatSideMenuActiveModel, ClientModel>(
    //       builder: (context, chatSideMenuActive, client, child) => Visibility(
    //           visible: !chatSideMenuActive.empty,
    //           child: SizedBox(width: 250, child: ChatSideMenu(client)))),
    // ]);
  }
}
