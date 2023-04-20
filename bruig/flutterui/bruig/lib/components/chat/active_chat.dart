import 'package:bruig/components/attach_file.dart';
import 'package:bruig/components/manage_gc.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/screens/feed/feed_posts.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:bruig/components/profile.dart';
import 'package:bruig/components/chat/types.dart';
import 'package:bruig/components/chat/messages.dart';
import 'package:scrollable_positioned_list/scrollable_positioned_list.dart';

class ActiveChat extends StatefulWidget {
  final ClientModel client;
  final FocusNode editLineFocusNode;
  ActiveChat(this.client, this.editLineFocusNode, {Key? key}) : super(key: key);

  @override
  State<ActiveChat> createState() => _ActiveChatState();
}

/// TODO: Figure out a way to estimate list size to set initialOffset.
/// this way we can get rid of the "initial jump flicker"
class _ActiveChatState extends State<ActiveChat> {
  ClientModel get client => widget.client;
  FocusNode get editLineFocusNode => widget.editLineFocusNode;
  ChatModel? chat;
  late ItemScrollController _itemScrollController;
  late ItemPositionsListener _itemPositionsListener;

  void clientChanged() {
    var newChat = client.active;
    if (newChat != chat) {
      setState(() {
        chat = newChat;
      });
    }
  }

  void sendMsg(String msg) async {
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
                const Duration(milliseconds: 250), // a little bit smoother
          );
        }
      });
    } catch (exception) {
      if (mounted) {
        showErrorSnackbar(context, "Unable to send message: $exception");
      }
    }
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
    client.removeListener(clientChanged);
    editLineFocusNode.dispose();
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
    //editLineFocusNode.requestFocus();
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

    return Row(children: [
      Expanded(
        child: Column(children: [
          Expanded(
            child: Messages(chat, client.nick, client, _itemScrollController,
                _itemPositionsListener),
          ),
          EditLine(sendMsg, chat, editLineFocusNode)
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
                    style: TextStyle(color: avatarTextColor, fontSize: 75),
                  ),
                ),
              ),
              Visibility(
                visible: chat.isGC,
                child: Text("Group Chat",
                    style: TextStyle(fontSize: 15, color: textColor)),
              ),
              Text(chat.nick, style: TextStyle(fontSize: 15, color: textColor)),
              ListView.builder(
                shrinkWrap: true,
                itemCount: client.activeSubMenu.length,
                itemBuilder: (context, index) => ListTile(
                    title: Text(client.activeSubMenu[index].label,
                        style: const TextStyle(fontSize: 11)),
                    onTap: () {
                      client.activeSubMenu[index].onSelected(context, client);
                      client.hideSubMenu();
                    },
                    hoverColor: Colors.black),
              )
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

class EditLine extends StatefulWidget {
  final SendMsg _send;
  final ChatModel chat;
  final FocusNode editLineFocusNode;
  EditLine(this._send, this.chat, this.editLineFocusNode, {Key? key})
      : super(key: key);

  @override
  State<EditLine> createState() => _EditLineState();
}

class _EditLineState extends State<EditLine> {
  final controller = TextEditingController();

  final FocusNode node = FocusNode();
  List<AttachmentEmbed> embeds = [];

  @override
  void initState() {
    super.initState();
    controller.text = widget.chat.workingMsg;
  }

  @override
  void didUpdateWidget(EditLine oldWidget) {
    super.didUpdateWidget(oldWidget);
    var workingMsg = widget.chat.workingMsg;
    if (workingMsg != controller.text) {
      controller.text = workingMsg;
      controller.selection = TextSelection(
          baseOffset: workingMsg.length, extentOffset: workingMsg.length);
      widget.editLineFocusNode.requestFocus();
    }
  }

  void handleKeyPress(event) {
    if (event is RawKeyUpEvent) {
      bool modPressed = event.isShiftPressed || event.isControlPressed;
      final val = controller.value;
      if (event.data.logicalKey.keyLabel == "Enter" && !modPressed) {
        final messageWithoutNewLine =
            controller.text.substring(0, val.selection.start - 1) +
                controller.text.substring(val.selection.start);
        controller.value = const TextEditingValue(
            text: "", selection: TextSelection.collapsed(offset: 0));
        final String withEmbeds = embeds.fold(
            messageWithoutNewLine.trim(), (s, e) => e.replaceInString(s));
        /*
          if (withEmbeds.length > 1024 * 1024) {
            showErrorSnackbar(context,
                "Message is larger than maximum allowed (limit: 1MiB)");
            return;
          }
          */
        if (withEmbeds != "") {
          widget._send(withEmbeds);
          widget.chat.workingMsg = "";
          setState(() {
            embeds = [];
          });
        }
      } else {
        widget.chat.workingMsg = val.text.trim();
      }
    }
  }

  void attachFile() async {
    var res = await Navigator.of(context, rootNavigator: true)
        .pushNamed(AttachFileScreen.routeName);
    if (res == null) {
      return;
    }
    var embed = res as AttachmentEmbed;
    embeds.add(embed);
    setState(() {
      controller.text = controller.text + embed.displayString() + " ";
      widget.chat.workingMsg = controller.text;
      widget.editLineFocusNode.requestFocus();
    });
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor; // MESSAGE TEXT COLOR
    var hoverColor = theme.hoverColor;
    var backgroundColor = theme.highlightColor;
    var hintTextColor = theme.dividerColor;
    return RawKeyboardListener(
      focusNode: node,
      onKey: handleKeyPress,
      child: Container(
        margin: const EdgeInsets.only(bottom: 5),
        child: Row(
          children: [
            IconButton(onPressed: attachFile, icon: Icon(Icons.attach_file)),
            Expanded(
                child: TextField(
              autofocus: true,
              focusNode: widget.editLineFocusNode,
              style: TextStyle(
                fontSize: 11,
                color: textColor,
              ),
              controller: controller,
              minLines: 1,
              maxLines: null,
              //textInputAction: TextInputAction.done,
              //style: normalTextStyle,
              keyboardType: TextInputType.multiline,
              decoration: InputDecoration(
                filled: true,
                fillColor: backgroundColor,
                hoverColor: hoverColor,
                isDense: true,
                hintText: 'Type a message',
                hintStyle: TextStyle(
                  fontSize: 11,
                  color: hintTextColor,
                ),
                border: InputBorder.none,
              ),
            )),
          ],
        ),
      ),
    );
  }
}
