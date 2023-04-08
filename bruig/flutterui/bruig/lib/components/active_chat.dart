import 'dart:async';

import 'package:bruig/components/attach_file.dart';
import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/chats_list.dart';
import 'package:bruig/components/manage_gc.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/screens/feed/feed_posts.dart';
import 'package:bruig/components/md_elements.dart';
import 'package:bruig/components/user_content_list.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/downloads.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:golib_plugin/util.dart';
import 'package:intl/intl.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';
import 'package:bruig/components/profile.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:open_filex/open_filex.dart';
import 'package:file_icon/file_icon.dart';

class ActiveChat extends StatefulWidget {
  final ClientModel client;
  final FocusNode editLineFocusNode;
  ActiveChat(this.client, this.editLineFocusNode, {Key? key}) : super(key: key);

  @override
  State<ActiveChat> createState() => _ActiveChatState();
}

class _ActiveChatState extends State<ActiveChat> {
  ClientModel get client => widget.client;
  FocusNode get editLineFocusNode => widget.editLineFocusNode;
  ChatModel? chat;

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
    } catch (exception) {
      if (mounted) {
        showErrorSnackbar(context, "Unable to send message: $exception");
      }
    }
  }

  @override
  void initState() {
    super.initState();
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

    showSubMenu(String id) => chat.isGC
        ? client.showSubMenu(true, id)
        : client.showSubMenu(false, id);

    return Row(children: [
      Expanded(
          child: Column(children: [
        Expanded(child: Messages(chat, client.nick, showSubMenu, client)),
        Row(children: [
          Expanded(child: EditLine(sendMsg, chat, editLineFocusNode))
        ])
      ])),
      client.activeSubMenu.isEmpty
          ? const Empty()
          : Container(
              width: 250,
              decoration: BoxDecoration(
                  border: Border(
                      left: BorderSide(width: 2, color: subMenuBorderColor))),
              child: Stack(alignment: Alignment.topRight, children: [
                Column(children: [
                  Container(
                      margin: const EdgeInsets.only(top: 20, bottom: 20),
                      child: CircleAvatar(
                          radius: 75,
                          backgroundColor: avatarColor,
                          child: Text(nickCapitalLetter(),
                              style: TextStyle(
                                  color: avatarTextColor, fontSize: 75)))),
                  chat.isGC
                      ? Text("Group Chat",
                          style: TextStyle(fontSize: 15, color: textColor))
                      : Empty(),
                  Text(chat.nick,
                      style: TextStyle(fontSize: 15, color: textColor)),
                  ListView.builder(
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
                            icon: Icon(
                                color: darkTextColor, Icons.close_outlined)))),
              ]))
    ]);
  }
}

typedef SendMsg = void Function(String msg);

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
            child: Row(children: [
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
            ])));
  }
}

class ServerEvent extends StatelessWidget {
  final Widget child;
  const ServerEvent({required this.child, Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var bgColor = theme.highlightColor;
    return Container(
        padding: const EdgeInsets.only(left: 41, top: 5, bottom: 5),
        margin: const EdgeInsets.all(5),
        decoration: BoxDecoration(
            color: bgColor,
            borderRadius: const BorderRadius.all(Radius.circular(5))),
        child: child);
  }
}

class ReceivedSentPM extends StatefulWidget {
  final ChatEventModel evnt;
  final String nick;
  final int timestamp;
  final ShowSubMenuCB showSubMenu;
  final String id;
  final String userNick;

  const ReceivedSentPM(this.evnt, this.nick, this.timestamp, this.showSubMenu,
      this.id, this.userNick,
      {Key? key})
      : super(key: key);

  @override
  State<ReceivedSentPM> createState() => _ReceivedSentPMState();
}

class _ReceivedSentPMState extends State<ReceivedSentPM> {
  void eventChanged() => setState(() {});

  @override
  initState() {
    super.initState();
    widget.evnt.addListener(eventChanged);
  }

  @override
  didUpdateWidget(ReceivedSentPM oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.evnt.removeListener(eventChanged);
    widget.evnt.addListener(eventChanged);
  }

  @override
  dispose() {
    widget.evnt.removeListener(eventChanged);
    super.dispose();
  }

  Future<void> launchUrlAwait(url) async {
    if (!await launchUrl(Uri.parse(url))) {
      throw 'Could not launch $url';
    }
  }

  String nickCapitalLetter() =>
      widget.nick.isNotEmpty ? widget.nick[0].toUpperCase() : "";

  @override
  Widget build(BuildContext context) {
    var prefix = "";
    var suffix = "";
    switch (widget.evnt.sentState) {
      case CMS_sending:
        prefix = "…";
        break;
      case CMS_sent:
        prefix = "✓";
        break;
      case CMS_errored:
        prefix = "✗";
        suffix = "\n\n${widget.evnt.sendError}";
        break;
      default:
    }
    var sourceID = widget.evnt.event.sid;
    if (widget.evnt.source != null) {
      sourceID = widget.evnt.source!.id;
    }
    var now = DateTime.fromMillisecondsSinceEpoch(widget.timestamp);
    var formatter = DateFormat('yyyy-MM-dd HH:mm:ss');
    var date = formatter.format(now);

    var msg = "${widget.evnt.event.msg}$suffix";
    msg = msg.replaceAll("\n",
        "  \n"); // Replace newlines with <space space newline> for proper md render
    var theme = Theme.of(context);
    var bgColor = theme.backgroundColor;
    var darkTextColor = theme.indicatorColor;
    var hightLightTextColor = theme.dividerColor; // NAME TEXT COLOR
    var avatarColor = colorFromNick(widget.nick);
    var avatarTextColor =
        ThemeData.estimateBrightnessForColor(avatarColor) == Brightness.dark
            ? hightLightTextColor
            : darkTextColor;
    var selectedBackgroundColor = theme.highlightColor;
    var textColor = theme.dividerColor;

    return Column(children: [
      widget.evnt.firstUnread
          ? Row(children: [
              Expanded(
                  child: Divider(
                color: textColor, //color of divider
                height: 8, //height spacing of divider
                thickness: 1, //thickness of divier line
                indent: 5, //spacing at the start of divider
                endIndent: 5, //spacing at the end of divider
              )),
              Text("Last read posts",
                  style: TextStyle(fontSize: 9, color: textColor)),
              Expanded(
                  child: Divider(
                color: textColor, //color of divider
                height: 8, //height spacing of divider
                thickness: 1, //thickness of divier line
                indent: 5, //spacing at the start of divider
                endIndent: 5, //spacing at the end of divider
              )),
            ])
          : const Empty(),
      widget.evnt.sameUser
          ? const Empty()
          : Row(children: [
              Container(
                  width: 28,
                  margin: const EdgeInsets.only(
                      top: 0, bottom: 0, left: 5, right: 0),
                  child: IconButton(
                    splashRadius: 20,
                    hoverColor: selectedBackgroundColor,
                    icon: CircleAvatar(
                        backgroundColor: avatarColor,
                        child: Text(nickCapitalLetter(),
                            style: TextStyle(
                                color: avatarTextColor, fontSize: 20))),
                    padding: const EdgeInsets.all(0),
                    tooltip: widget.nick,
                    onPressed: () {
                      widget.showSubMenu(widget.id);
                    },
                  )),
              const SizedBox(width: 10),
              Text(
                widget.nick,
                style: TextStyle(
                  fontSize: 12,
                  color: avatarColor, // NAME TEXT COLOR,
                  fontWeight: FontWeight.bold,
                ),
              ),
            ]),
      //const SizedBox(height: 10),
      Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
          const SizedBox(width: 13),
          SizedBox(
              width: 5,
              child: Text(
                prefix,
                style: TextStyle(
                    fontSize: 12,
                    color: hightLightTextColor, // NAME TEXT COLOR,
                    fontWeight: FontWeight.bold,
                    fontStyle: FontStyle.italic),
              )),
          const SizedBox(width: 24),
          Expanded(
              child: Provider<DownloadSource>(
                  create: (context) => DownloadSource(sourceID),
                  child: MarkdownArea(
                      msg,
                      widget.userNick != widget.nick &&
                          msg.contains(widget.userNick)))),
          Text(
            date,
            style: TextStyle(fontSize: 9, color: darkTextColor), // DATE COLOR
          ),
          const SizedBox(width: 10)
        ]),
        const SizedBox(height: 5),
      ])
    ]);
  }
}

class PMW extends StatelessWidget {
  final ChatEventModel evnt;
  final String nick;
  final ShowSubMenuCB showSubMenu;
  const PMW(this.evnt, this.nick, this.showSubMenu, {Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    var timestamp = 0;
    var event = evnt.event;
    if (event is PM) {
      timestamp =
          evnt.source?.nick == null ? event.timestamp : event.timestamp * 1000;
    }
    return ReceivedSentPM(evnt, evnt.source?.nick ?? nick, timestamp,
        showSubMenu, evnt.source?.id ?? "", nick);
  }
}

class GCMW extends StatelessWidget {
  final ChatEventModel evnt;
  final String nick;
  final ShowSubMenuCB showSubMenu;
  const GCMW(this.evnt, this.nick, this.showSubMenu, {Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    var event = evnt.event;
    var timestamp = 0;
    if (event is GCMsg) {
      timestamp =
          evnt.source?.nick == null ? event.timestamp : event.timestamp * 1000;
    }
    return ReceivedSentPM(evnt, evnt.source?.nick ?? nick, timestamp,
        showSubMenu, evnt.source?.id ?? "", nick);
  }
}

class GCUserEventW extends StatelessWidget {
  final ChatEventModel evnt;
  const GCUserEventW(this.evnt, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    if (evnt.source != null) {
      return ServerEvent(
          child: SelectableText("${evnt.source!.nick}:  ${evnt.event.msg}",
              style: TextStyle(fontSize: 9, color: textColor)));
    } else {
      return ServerEvent(
          child: SelectableText(evnt.event.msg,
              style: TextStyle(fontSize: 9, color: textColor)));
    }
  }
}

class JoinGCEventW extends StatefulWidget {
  final ChatEventModel event;
  final GCInvitation invite;
  const JoinGCEventW(this.event, this.invite, {Key? key}) : super(key: key);

  @override
  State<JoinGCEventW> createState() => _JoinGCEventWState();
}

class _JoinGCEventWState extends State<JoinGCEventW> {
  GCInvitation get invite => widget.invite;
  ChatEventModel get event => widget.event;

  void acceptInvite() async {
    try {
      event.sentState = CMS_sending;
      setState(() {});
      await Golib.acceptGCInvite(invite.iid);
      event.sentState = CMS_sent;
      setState(() {});
    } catch (exception) {
      event.sendError = "exception";
      setState(() {});
    }
  }

  void cancelInvite() {
    event.sentState = CMS_canceled;
    setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    switch (event.sentState) {
      case CMS_canceled:
        return ServerEvent(
            child: Text("Declined GC invitation to '${invite.name}",
                style: TextStyle(fontSize: 9, color: textColor)));
      case CMS_errored:
        return ServerEvent(
            child: SelectableText(
                "Unable to join GC ${invite.name}: ${event.sendError}",
                style: TextStyle(fontSize: 9, color: textColor)));
      case CMS_sent:
        return ServerEvent(
            child: Text("Accepted invitation to join GC '${invite.name}'",
                style: TextStyle(fontSize: 9, color: textColor)));
      case CMS_sending:
        return ServerEvent(
            child: Text("Accepting invitation to join GC '${invite.name}'",
                style: TextStyle(fontSize: 9, color: textColor)));
    }

    return ServerEvent(
        child: Column(children: [
      Text("Received invitation to join GC '${invite.name}'",
          style: TextStyle(fontSize: 9, color: textColor)),
      const SizedBox(height: 20),
      Row(mainAxisAlignment: MainAxisAlignment.center, children: [
        ElevatedButton(onPressed: acceptInvite, child: const Text("Accept")),
        const SizedBox(width: 10),
        CancelButton(onPressed: cancelInvite),
      ]),
    ]));
  }
}

class PostsListW extends StatefulWidget {
  final UserPostList posts;
  final ChatModel chat;
  final Function() scrollToBottom;
  const PostsListW(this.chat, this.posts, this.scrollToBottom, {Key? key})
      : super(key: key);

  @override
  State<PostsListW> createState() => _PostsListWState();
}

class _PostsListWState extends State<PostsListW> {
  List<PostListItem> get posts => widget.posts.posts;
  ChatModel get chat => widget.chat;

  void getPost(int index) async {
    var post = posts[index];
    var event =
        SynthChatEvent("Fetching user post '${post.title}'", SCE_sending);
    widget.scrollToBottom();
    try {
      chat.append(ChatEventModel(event, null));
      await Golib.getUserPost(chat.id, post.id);
      event.state = SCE_sent;
    } catch (exception) {
      event.error = Exception("Unable to get user post: $exception");
    }
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    return ServerEvent(
        child: Column(
      children: [
        Text("User Posts", style: TextStyle(color: textColor)),
        ListView.builder(
            shrinkWrap: true,
            itemCount: posts.length,
            itemBuilder: (BuildContext context, int index) {
              return Row(
                children: [
                  IconButton(
                    onPressed: () {
                      getPost(index);
                    },
                    icon: const Icon(Icons.download),
                    tooltip: "Fetch post ${posts[index].id}",
                  ),
                  Expanded(child: MarkdownArea(posts[index].title, false))
                ],
              );
            }),
      ],
    ));
  }
}

class InflightTipW extends StatefulWidget {
  final InflightTip tip;
  final ChatModel source;
  const InflightTipW(this.tip, this.source, {Key? key}) : super(key: key);

  @override
  State<InflightTipW> createState() => _InflightTipState();
}

class _InflightTipState extends State<InflightTipW> {
  @override
  initState() {
    super.initState();
    widget.tip.addListener(tipChanged);
  }

  @override
  didUpdateWidget(InflightTipW oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.tip.removeListener(tipChanged);
    widget.tip.addListener(tipChanged);
  }

  @override
  dispose() {
    widget.tip.removeListener(tipChanged);
    super.dispose();
  }

  void tipChanged() {
    setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    var tip = widget.tip;
    var source = widget.source;
    late Widget child;
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    if (tip.state == ITS_completed) {
      child = Text(
          "✓ Requesting invoice for ${formatDCR(tip.amount)} to tip ${source.nick}!",
          style: TextStyle(fontSize: 9, color: textColor));
    } else if (tip.state == ITS_errored) {
      child = Text("✗ Failed to send tip: ${tip.error}",
          style: TextStyle(fontSize: 9, color: textColor));
    } else if (tip.state == ITS_received) {
      child = Text("\$ Received ${tip.amount} DCR from ${source.nick}!",
          style: TextStyle(fontSize: 9, color: textColor));
    } else {
      child = Text(
          "… Requesting invoice for ${formatDCR(tip.amount)} DCR to tip ${source.nick}...",
          style: TextStyle(fontSize: 9, color: textColor));
    }
    return ServerEvent(child: child);
  }
}

class SynthEventW extends StatefulWidget {
  final SynthChatEvent event;
  const SynthEventW(this.event, {Key? key}) : super(key: key);

  @override
  State<SynthEventW> createState() => _SynthEventWState();
}

class _SynthEventWState extends State<SynthEventW> {
  @override
  initState() {
    super.initState();
    widget.event.addListener(stateChanged);
  }

  @override
  didUpdateWidget(SynthEventW oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.event.removeListener(stateChanged);
    widget.event.addListener(stateChanged);
  }

  @override
  dispose() {
    widget.event.removeListener(stateChanged);
    super.dispose();
  }

  void stateChanged() {
    setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    var event = widget.event;
    late Widget child;
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;

    if (event.state == SCE_sent) {
      child = Text("✓ ${widget.event.msg}",
          style: TextStyle(fontSize: 9, color: textColor));
    } else if (event.state == SCE_errored) {
      child = Text("✗ Failed to ${widget.event.msg} - ${event.error}",
          style: TextStyle(fontSize: 9, color: textColor));
    } else if (event.state == SCE_sending) {
      child = Text("… ${widget.event.msg}",
          style: TextStyle(fontSize: 9, color: textColor));
    } else if (event.state == SCE_received) {
      child = Text(widget.event.msg,
          style: TextStyle(fontSize: 9, color: textColor));
    } else {
      child = Text("? unknown state ${event.state}",
          style: TextStyle(fontSize: 9, color: textColor));
    }
    return ServerEvent(child: child);
  }
}

class UserContentEventW extends StatefulWidget {
  final UserContentList content;
  final ChatModel chat;
  const UserContentEventW(this.content, this.chat, {Key? key})
      : super(key: key);

  @override
  State<UserContentEventW> createState() => _UserContentEventWState();
}

class _UserContentEventWState extends State<UserContentEventW> {
  @override
  Widget build(BuildContext context) {
    return Consumer<DownloadsModel>(
        builder: (context, downloads, child) => ServerEvent(
                child: Column(children: [
              Text("User Content",
                  style: TextStyle(
                      color: Theme.of(context).focusColor, fontSize: 15)),
              const SizedBox(height: 20),
              UserContentListW(widget.chat, downloads, widget.content),
            ])));
  }
}

class PostEventW extends StatelessWidget {
  final FeedPostEvent event;
  const PostEventW(this.event, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    return ServerEvent(
        child: SelectableText("Received post '${event.title}'",
            style: TextStyle(fontSize: 9, color: textColor)));
  }
}

class PostSubscriptionEventW extends StatelessWidget {
  final PostSubscriptionResult event;
  const PostSubscriptionEventW(this.event, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    String msg;
    if (event.wasSubRequest && event.error != "") {
      msg = "Unable to subscribe to user's posts: ${event.error}";
    } else if (event.wasSubRequest) {
      msg = "Subscribed to user's posts!";
    } else if (event.error != "") {
      msg = "Unable to unsubscribe from user's posts: ${event.error}";
    } else {
      msg = "Unsubscribed from user's posts!";
    }

    return ServerEvent(
        child: SelectableText(msg,
            style: TextStyle(fontSize: 9, color: textColor)));
  }
}

class FileDownloadedEventW extends StatelessWidget {
  final FileDownloadedEvent event;
  const FileDownloadedEventW(this.event, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    var backgroundColor = theme.highlightColor;
    return ServerEvent(
      child: Container(
        padding: const EdgeInsets.all(0),
        decoration: BoxDecoration(
          color: backgroundColor,
          borderRadius: const BorderRadius.all(Radius.circular(5)),
        ),
        child: Row(
          children: [
            Material(
              color: backgroundColor,
              child: IconButton(
                onPressed: () {
                  OpenFilex.open(event.diskPath);
                },
                splashRadius: 20,
                icon: FileIcon(event.diskPath, size: 24),
              ),
            ),
            const SizedBox(width: 10),
            SelectableText(
              "Downloaded file ${event.diskPath}",
              style: TextStyle(fontSize: 9, color: textColor),
            ),
          ],
        ),
      ),
    );
  }
}

class GCVersionWarnW extends StatelessWidget {
  final GCVersionWarn event;
  const GCVersionWarnW(this.event, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var bgColor = Colors.red[600];
    var textColor = Colors.white;
    return Container(
        padding: const EdgeInsets.only(left: 41, top: 5, bottom: 5),
        margin: const EdgeInsets.all(5),
        decoration: BoxDecoration(
            color: bgColor,
            borderRadius: const BorderRadius.all(Radius.circular(5))),
        child: SelectableText(
            "Received GC definitions with unsupported version ${event.version}. Please update the software to interact in this GC.",
            style: TextStyle(fontSize: 9, color: textColor)));
  }
}

class GCAddedMembersW extends StatelessWidget {
  final GCAddedMembers event;
  final ClientModel client;
  const GCAddedMembersW(this.event, this.client, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    String msg = "Added to GC:\n";
    event.uids.forEach((uid) {
      var nick = client.getNick(uid);
      if (nick == "") {
        msg += "Unknown user $uid\n";
      } else {
        msg += "User '$nick'\n";
      }
    });

    return ServerEvent(
        child: SelectableText(msg,
            style: TextStyle(fontSize: 9, color: textColor)));
  }
}

class GCPartedMemberW extends StatelessWidget {
  final GCMemberParted event;
  final ClientModel client;
  const GCPartedMemberW(this.event, this.client, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    var nick = client.getNick(event.uid);
    if (nick == "") {
      nick = event.uid;
    }
    String msg;
    if (event.kicked) {
      msg = "User '$nick' kicked from GC. Reason: '${event.reason}'";
    } else {
      msg = "User '$nick' parted from GC. Reason: '${event.reason}'";
    }

    return ServerEvent(
        child: SelectableText(msg,
            style: TextStyle(fontSize: 9, color: textColor)));
  }
}

class GCUpgradedVersionW extends StatelessWidget {
  final GCUpgradedVersion event;
  const GCUpgradedVersionW(this.event, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    String msg =
        "GC Upgraded from version ${event.oldVersion} to ${event.newVersion}";
    return ServerEvent(
        child: SelectableText(msg,
            style: TextStyle(fontSize: 9, color: textColor)));
  }
}

class GCAdminsChangedW extends StatelessWidget {
  final GCAdminsChanged event;
  final ClientModel client;
  const GCAdminsChangedW(this.event, this.client, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    var srcNick = client.getNick(event.source);
    String msg = "$srcNick modified the GC admins:\n";
    var myID = client.publicID;
    if (event.added != null) {
      msg += event.added!.fold("", (prev, e) {
        var nick = e == myID ? "Local client" : client.getNick(e);
        nick = nick == "" ? e : nick;
        return prev + "\n$nick added as admin";
      });
    }
    if (event.removed != null) {
      msg += event.removed!.fold("", (prev, e) {
        var nick = e == myID ? "Local client" : client.getNick(e);
        nick = nick == "" ? e : nick;
        return prev + "\n$nick removed as admin";
      });
    }

    return ServerEvent(
        child: SelectableText(msg,
            style: TextStyle(fontSize: 9, color: textColor)));
  }
}

class KXSuggestedW extends StatefulWidget {
  final ChatEventModel event;
  final KXSuggested suggest;
  final ClientModel client;
  const KXSuggestedW(this.event, this.suggest, this.client, {Key? key})
      : super(key: key);

  @override
  State<KXSuggestedW> createState() => _KXSuggestedWState();
}

class _KXSuggestedWState extends State<KXSuggestedW> {
  ChatEventModel get event => widget.event;
  KXSuggested get suggest => widget.suggest;
  ClientModel get client => widget.client;

  void acceptSuggestion() async {
    try {
      event.sentState = Suggestion_accepted;
      setState(() {});
      client.requestMediateID(suggest.inviteeid, suggest.targetid);
      event.sentState = Suggestion_confirmed;
      setState(() {});
    } catch (exception) {
      event.sentState = Suggestion_errored;

      setState(() {});
    }
  }

  void cancelSuggestion() {
    event.sentState = Suggestion_canceled;
    setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;

    switch (event.sentState) {
      case Suggestion_accepted:
        return ServerEvent(
            child: Text(
                "Accepting suggestion to KX from '${suggest.inviteenick}' to '${suggest.targetnick}'",
                style: TextStyle(fontSize: 9, color: textColor)));
      case Suggestion_errored:
        return ServerEvent(
            child: SelectableText(
                "Unable to accept suggestion from  '${suggest.inviteenick}' to '${suggest.targetnick}'",
                style: TextStyle(fontSize: 9, color: textColor)));
      case Suggestion_canceled:
        return ServerEvent(
            child: Text(
                "Canceled suggestion to KX from '${suggest.inviteenick}' to '${suggest.targetnick}'",
                style: TextStyle(fontSize: 9, color: textColor)));
      case Suggestion_confirmed:
        return ServerEvent(
            child: Text(
                "Confirmed suggestion to KX from '${suggest.inviteenick}' to '${suggest.targetnick}'",
                style: TextStyle(fontSize: 9, color: textColor)));
    }

    return suggest.alreadyknown
        ? ServerEvent(
            child: Text(
                "Received already known suggestion to KX from '${suggest.inviteenick}' to '${suggest.targetnick}'",
                style: TextStyle(fontSize: 9, color: textColor)))
        : ServerEvent(
            child: Column(children: [
            Text(
                "Received suggestion to KX from '${suggest.inviteenick}' to '${suggest.targetnick}'",
                style: TextStyle(fontSize: 9, color: textColor)),
            const SizedBox(height: 20),
            Row(mainAxisAlignment: MainAxisAlignment.center, children: [
              ElevatedButton(
                  onPressed: acceptSuggestion, child: const Text("Accept")),
              const SizedBox(width: 10),
              CancelButton(onPressed: cancelSuggestion),
            ]),
          ]));
  }
}

class TipUserProgressW extends StatelessWidget {
  final TipProgressEvent event;
  const TipUserProgressW(this.event, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;

    var dcrAmount = formatDCR(milliatomsToDCR(event.amountMAtoms));
    String msg;
    if (event.completed) {
      msg = "Tip attempt of $dcrAmount completed successfully!";
    } else if (event.willRetry) {
      msg =
          "Tip attempt of $dcrAmount failed due to ${event.attemptErr}. Will try to tip again automatically.";
    } else {
      msg =
          "Tip attempt of $dcrAmount failed due to ${event.attemptErr}. Given up on attempting to tip.";
    }

    return ServerEvent(
        child: SelectableText(msg,
            style: TextStyle(fontSize: 9, color: textColor)));
  }
}

class Event extends StatelessWidget {
  final ChatEventModel event;
  final ChatModel chat;
  final String nick;
  final ShowSubMenuCB showSubMenu;
  final ClientModel client;
  final Function() scrollToBottom;
  const Event(this.chat, this.event, this.nick, this.client,
      this.scrollToBottom, this.showSubMenu,
      {Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    if (event.event is PM) {
      return PMW(event, nick, showSubMenu);
    }

    if (event.event is InflightTip) {
      return InflightTipW((event.event as InflightTip), event.source!);
    }

    if (event.event is GCMsg) {
      return GCMW(event, nick, showSubMenu);
    }

    if (event.event is GCUserEvent) {
      return GCUserEventW(event);
    }

    if (event.event is FeedPostEvent) {
      return PostEventW(event.event as FeedPostEvent);
    }

    if (event.event is SynthChatEvent) {
      return SynthEventW(event.event as SynthChatEvent);
    }

    if (event.event is FileDownloadedEvent) {
      return FileDownloadedEventW(event.event as FileDownloadedEvent);
    }

    if (event.event is GCInvitation) {
      return JoinGCEventW(event, event.event as GCInvitation);
    }

    if (event.event is UserPostList) {
      return PostsListW(chat, event.event as UserPostList, scrollToBottom);
    }

    if (event.event is UserContentList) {
      return UserContentEventW(event.event as UserContentList, chat);
    }

    if (event.event is PostSubscriptionResult) {
      return PostSubscriptionEventW(event.event as PostSubscriptionResult);
    }

    if (event.event is GCVersionWarn) {
      return GCVersionWarnW(event.event as GCVersionWarn);
    }

    if (event.event is GCAddedMembers) {
      return GCAddedMembersW(event.event as GCAddedMembers, client);
    }

    if (event.event is GCMemberParted) {
      return GCPartedMemberW(event.event as GCMemberParted, client);
    }

    if (event.event is GCUpgradedVersion) {
      return GCUpgradedVersionW(event.event as GCUpgradedVersion);
    }

    if (event.event is GCAdminsChanged) {
      return GCAdminsChangedW(event.event as GCAdminsChanged, client);
    }

    if (event.event is KXSuggested) {
      return KXSuggestedW(event, event.event as KXSuggested, client);
    }

    if (event.event is TipProgressEvent) {
      return TipUserProgressW(event.event as TipProgressEvent);
    }

    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    return Container(
        color: Theme.of(context).errorColor, // ERROR COLOR
        child: Text("Unknonwn chat event type",
            style: TextStyle(color: textColor)));
  }
}

class Messages extends StatefulWidget {
  final ChatModel chat;
  final String nick;
  final ShowSubMenuCB showSubMenu;
  final ClientModel client;
  const Messages(this.chat, this.nick, this.showSubMenu, this.client,
      {Key? key})
      : super(key: key);

  @override
  State<Messages> createState() => _MessagesState();
}

class _MessagesState extends State<Messages> {
  ClientModel get client => widget.client;
  ChatModel get chat => widget.chat;
  String get nick => widget.nick;
  final ScrollController _scroller = ScrollController();
  bool _firstAutoscrollDone = false;
  bool _shouldAutoscroll = false;

  void scrollToBottom() {
    _scroller.jumpTo(_scroller.position.maxScrollExtent);
  }

  void scrollListener() {
    _firstAutoscrollDone = true;
    if (_scroller.hasClients &&
        _scroller.position.pixels == _scroller.position.maxScrollExtent) {
      _shouldAutoscroll = true;
    } else {
      _shouldAutoscroll = false;
    }
  }

  void maybeScrollToBottom() {
    if (_scroller.hasClients && (_shouldAutoscroll || !_firstAutoscrollDone)) {
      scrollToBottom();
    }
  }

  void onChatChanged() {
    setState(() {
      maybeScrollToBottom();
    });
    Future.delayed(const Duration(milliseconds: 50), () {
      setState(maybeScrollToBottom);
    });
  }

  @override
  initState() {
    super.initState();
    _scroller.addListener(scrollListener);
    chat.addListener(onChatChanged);
  }

  @override
  void didUpdateWidget(Messages oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.chat.removeListener(onChatChanged);
    chat.addListener(onChatChanged);
    _firstAutoscrollDone = false;
    _shouldAutoscroll = true;
    Future.delayed(const Duration(milliseconds: 1), () {
      setState(maybeScrollToBottom);
    });
  }

  @override
  dispose() {
    _scroller.removeListener(scrollListener);
    chat.removeListener(onChatChanged);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var msgs = chat.msgs; // Probably inneficient to regenerate every render...
    return ListView.builder(
        controller: _scroller,
        itemCount: msgs.length,
        itemBuilder: (context, index) => Event(chat, msgs[index], nick, client,
            scrollToBottom, widget.showSubMenu));
  }
}
