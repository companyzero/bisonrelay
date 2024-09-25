import 'dart:async';

import 'package:bruig/components/chat/chat_side_menu.dart';
import 'package:bruig/components/manage_gc.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/chats.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:bruig/components/profile.dart';
import 'package:bruig/components/chat/messages.dart';
import 'package:scrollable_positioned_list/scrollable_positioned_list.dart';
import 'package:bruig/components/chat/input.dart';

class ActiveChat extends StatefulWidget {
  final ClientModel client;
  final CustomInputFocusNode inputFocusNode;
  const ActiveChat(this.client, this.inputFocusNode, {super.key});

  @override
  State<ActiveChat> createState() => _ActiveChatState();
}

class _ActiveChatState extends State<ActiveChat> {
  ClientModel get client => widget.client;
  UIStateModel get ui => widget.client.ui;
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
    var snackbar = SnackBarModel.of(context);

    if (_debounce?.isActive ?? false) _debounce!.cancel();
    _debounce = Timer(const Duration(milliseconds: 100), () async {
      if (!mounted) return;
      try {
        await chat.sendMsg(msg);
        client.newSentMsg(chat);
      } catch (exception) {
        if (mounted) {
          snackbar.error("Unable to send message: $exception");
        }
      }
    });
  }

  void showProfileChanged() {
    setState(() {});
  }

  void chatSideMenuActiveChanged() {
    setState(() {});
  }

  @override
  void initState() {
    super.initState();
    _itemScrollController = ItemScrollController();
    _itemPositionsListener = ItemPositionsListener.create();
    chat = client.active;
    client.activeChat.addListener(activeChatChanged);
    ui.showProfile.addListener(showProfileChanged);
    ui.chatSideMenuActive.addListener(chatSideMenuActiveChanged);
  }

  @override
  void didUpdateWidget(ActiveChat oldWidget) {
    super.didUpdateWidget(oldWidget);

    if (oldWidget.client != widget.client) {
      oldWidget.client.removeListener(activeChatChanged);
      client.addListener(activeChatChanged);
      activeChatChanged();
      oldWidget.client.ui.showProfile.removeListener(showProfileChanged);
      ui.showProfile.addListener(showProfileChanged);
      oldWidget.client.ui.chatSideMenuActive
          .removeListener(chatSideMenuActiveChanged);
      ui.chatSideMenuActive.addListener(chatSideMenuActiveChanged);
    } else if (client.active != chat) {
      activeChatChanged();
    }
  }

  @override
  void dispose() {
    ui.showProfile.removeListener(showProfileChanged);
    ui.chatSideMenuActive.removeListener(chatSideMenuActiveChanged);
    _debounce?.cancel();
    client.activeChat.removeListener(activeChatChanged);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (this.chat == null) return Container();
    var chat = this.chat!;

    if (ui.showProfile.val) {
      if (chat.isGC) {
        return ManageGCScreen(client, chat);
      } else {
        return UserProfile(client);
      }
    }

    bool isScreenSmall = checkIsScreenSmall(context);

    return ScreenWithChatSideMenu(
        client,
        Column(
          children: [
            Expanded(
              child: Messages(
                  chat, client, _itemScrollController, _itemPositionsListener),
            ),
            Container(
                padding: isScreenSmall
                    ? const EdgeInsets.all(10)
                    : const EdgeInsets.all(5),
                child: ChatInput(sendMsg, chat, inputFocusNode))
          ],
        ));
  }
}
