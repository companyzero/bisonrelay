import 'dart:async';

import 'package:bruig/components/chat/chat_side_menu.dart';
import 'package:bruig/components/chat/record_audio.dart';
import 'package:bruig/components/manage_gc.dart';
import 'package:bruig/components/typing_emoji_panel.dart';
import 'package:bruig/models/emoji.dart';
import 'package:bruig/models/audio.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/chats.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/storage_manager.dart';
import 'package:flutter/material.dart';
import 'package:bruig/components/profile.dart';
import 'package:bruig/components/chat/messages.dart';
import 'package:provider/provider.dart';
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

  void _doSendMsg(String msg) async {
    var snackbar = SnackBarModel.of(context);
    try {
      await chat!.sendMsg(msg);
      client.newSentMsg(chat!);
    } catch (exception) {
      snackbar.error("Unable to send message: $exception");
    }
  }

  void sendMsg(String msg) {
    if (this.chat == null) {
      return;
    }
    ChatModel chat = this.chat!;

    if (_debounce?.isActive ?? false) _debounce!.cancel();
    _debounce = Timer(const Duration(milliseconds: 100), () async {
      if (!mounted) return;

      // The first time this client sends a message on a GC with unkxd members,
      // warn the user about it.
      var notifyGCUnkxdMembers = chat.isGC &&
          !(await StorageManager.readBool(
              StorageManager.notifiedGCUnkxdMembers)) &&
          (chat.unkxdMembers.value?.isNotEmpty ?? false);
      if (!mounted) return;
      if (notifyGCUnkxdMembers) {
        showModalBottomSheet(
            context: context,
            builder: (context) => Container(
                padding: const EdgeInsets.all(20),
                child: Column(mainAxisSize: MainAxisSize.min, children: [
                  const Text(
                      "Note: This GC contains un-kx'd members - other people whom this "
                      "client has not exchanged keys with. These people won't receive any "
                      "messages until the KX process has completed, which usually happens "
                      "automatically, once they come back online.\n\n"
                      "It is also common on large, public GCs to have people that never "
                      "come online because they have stopped using the software and have "
                      "not yet been removed from the GC.\n\n"
                      "You may wait until the warning indicator disappears or you "
                      "may keep sending messages (keeping in mind that not every "
                      "member of this GC will receive them).\n\n"
                      "This warning will be displayed only once."),
                  const SizedBox(height: 10),
                  TextButton(
                    onPressed: () {
                      StorageManager.saveBool(
                          StorageManager.notifiedGCUnkxdMembers, true);
                      Navigator.of(context).pop();
                      _doSendMsg(msg);
                    },
                    child: const Text("Ok"),
                  )
                ])));
        return;
      }

      _doSendMsg(msg);
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
              child: Stack(children: [
                Messages(chat, client, _itemScrollController,
                    _itemPositionsListener),
                Positioned(
                    bottom: 10,
                    left: 10,
                    right: 10,
                    child: Consumer<TypingEmojiSelModel>(
                        builder: (context, typingEmoji, child) =>
                            TypingEmojiPanel(
                              model: typingEmoji,
                              focusNode: inputFocusNode,
                            ))),
                if (isScreenSmall)
                  Positioned(
                      left: 10,
                      bottom: 10,
                      right: 10,
                      child: Consumer<AudioModel>(
                          builder: (context, audio, child) =>
                              SmallScreenRecordInfoPanel(audio: audio))),
              ]),
            ),
            if (!chat.killed)
              Container(
                  padding: isScreenSmall
                      ? const EdgeInsets.all(10)
                      : const EdgeInsets.all(5),
                  child: ChatInput(sendMsg, chat, inputFocusNode))
          ],
        ));
  }
}
