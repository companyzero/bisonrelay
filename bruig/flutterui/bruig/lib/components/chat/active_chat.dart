import 'dart:async';

import 'package:bruig/components/manage_gc.dart';
import 'package:bruig/util.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:bruig/components/profile.dart';
import 'package:bruig/components/chat/messages.dart';
import 'package:scrollable_positioned_list/scrollable_positioned_list.dart';
import 'package:bruig/components/chat/input.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/empty_widget.dart';

class ActiveChat extends StatefulWidget {
  final ClientModel client;
  final FocusNode inputFocusNode;
  const ActiveChat(this.client, this.inputFocusNode, {Key? key})
      : super(key: key);

  @override
  State<ActiveChat> createState() => _ActiveChatState();
}

/// TODO: Figure out a way to estimate list size to set initialOffset.
/// this way we can get rid of the "initial jump flicker"
class _ActiveChatState extends State<ActiveChat> {
  ClientModel get client => widget.client;
  FocusNode get inputFocusNode => widget.inputFocusNode;
  ChatModel? chat;
  late ItemScrollController _itemScrollController;
  late ItemPositionsListener _itemPositionsListener;
  Timer? _debounce;

  void clientChanged() {
    var newChat = client.active;
    if (newChat != chat) {
      setState(() {
        chat = newChat;
      });
    }
  }

  void sendMsg(String msg) {
    if (_debounce?.isActive ?? false) _debounce!.cancel();
    _debounce = Timer(const Duration(milliseconds: 100), () async {
      try {
        await chat?.sendMsg(msg);
        WidgetsBinding.instance.addPostFrameCallback((_) async {
          if (mounted && chat != null) {
            var len = chat?.msgs.length;
            _itemScrollController.scrollTo(
              index: len! - 1,
              alignment: 0.0,
              curve: Curves.easeOut,
              duration:
                  const Duration(milliseconds: 100), // a little bit smoother
            );
          }
        });
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
    client.addListener(clientChanged);
  }

  @override
  void didUpdateWidget(ActiveChat oldWidget) {
    oldWidget.client.removeListener(clientChanged);
    super.didUpdateWidget(oldWidget);
    client.addListener(clientChanged);
  }

  @override
  void dispose() {
    _debounce?.cancel();
    client.removeListener(clientChanged);
    //inputFocusNode.dispose();  XXX Does this need to be put back?  Errors with it
    super.dispose();
  }

  String nickCapitalLetter() =>
      chat != null && chat!.nick.isNotEmpty ? chat!.nick[0].toUpperCase() : "";

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
    var hightLightTextColor = theme.focusColor; // NAME TEXT COLOR
    var avatarColor = colorFromNick(chat.nick);
    var avatarTextColor =
        ThemeData.estimateBrightnessForColor(avatarColor) == Brightness.dark
            ? hightLightTextColor
            : darkTextColor;

    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    return isScreenSmall
        ? client.activeSubMenu.isNotEmpty
            ? Stack(
                alignment: Alignment.topRight,
                children: [
                  Column(children: [
                    Container(
                      margin: const EdgeInsets.only(top: 20, bottom: 20),
                      child: CircleAvatar(
                        radius: 75,
                        backgroundColor: avatarColor,
                        child: Text(
                          nickCapitalLetter(),
                          style:
                              TextStyle(color: avatarTextColor, fontSize: 75),
                        ),
                      ),
                    ),
                    Visibility(
                      visible: chat.isGC,
                      child: Text("Group Chat",
                          style: TextStyle(fontSize: 15, color: textColor)),
                    ),
                    Text(chat.nick,
                        style: TextStyle(fontSize: 15, color: textColor)),
                    Expanded(
                        child: ListView.builder(
                      shrinkWrap: true,
                      itemCount: client.activeSubMenu.length,
                      itemBuilder: (context, index) => ListTile(
                          title: Text(client.activeSubMenu[index].label,
                              style: const TextStyle(fontSize: 11)),
                          onTap: () {
                            client.activeSubMenu[index]
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
                        icon: Icon(color: darkTextColor, Icons.close_outlined),
                      ),
                    ),
                  ),
                ],
              )
            : Column(children: [
                Expanded(
                  child: Messages(chat, client.nick, client,
                      _itemScrollController, _itemPositionsListener),
                ),
                Input(sendMsg, chat, inputFocusNode)
              ])
        : Row(children: [
            Expanded(
              child: Column(children: [
                Expanded(
                  child: Messages(chat, client.nick, client,
                      _itemScrollController, _itemPositionsListener),
                ),
                Input(sendMsg, chat, inputFocusNode)
              ]),
            ),
            Visibility(
              visible: client.activeSubMenu.isNotEmpty,
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
                      child: CircleAvatar(
                        radius: 75,
                        backgroundColor: avatarColor,
                        child: Text(
                          nickCapitalLetter(),
                          style:
                              TextStyle(color: avatarTextColor, fontSize: 75),
                        ),
                      ),
                    ),
                    Visibility(
                      visible: chat.isGC,
                      child: Text("Group Chat",
                          style: TextStyle(fontSize: 15, color: textColor)),
                    ),
                    Text(chat.nick,
                        style: TextStyle(fontSize: 15, color: textColor)),
                    Expanded(
                        child: ListView.builder(
                      shrinkWrap: true,
                      itemCount: client.activeSubMenu.length,
                      itemBuilder: (context, index) => ListTile(
                          title: Text(client.activeSubMenu[index].label,
                              style: const TextStyle(fontSize: 11)),
                          onTap: () {
                            client.activeSubMenu[index]
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
                        icon: Icon(color: darkTextColor, Icons.close_outlined),
                      ),
                    ),
                  ),
                ]),
              ),
            )
          ]);
  }
}
