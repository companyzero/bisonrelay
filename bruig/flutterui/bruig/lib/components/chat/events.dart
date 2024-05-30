import 'dart:async';

import 'package:bruig/screens/feed.dart';
import 'package:flutter/material.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/components/chat/types.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:bruig/components/buttons.dart';
import 'package:bruig/models/resources.dart';
import 'package:bruig/components/md_elements.dart';
import 'package:bruig/components/user_content_list.dart';
import 'package:bruig/models/downloads.dart';
import 'package:bruig/screens/viewpage_screen.dart';
import 'package:golib_plugin/util.dart';
import 'package:intl/intl.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:open_filex/open_filex.dart';
import 'package:file_icon/file_icon.dart';
import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/user_context_menu.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/util.dart';
import 'package:bruig/theme_manager.dart';

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

class DateChange extends StatelessWidget {
  final Widget child;
  const DateChange({required this.child, Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    return Container(
        padding: const EdgeInsets.only(top: 5, bottom: 5),
        margin: const EdgeInsets.all(5),
        child: child);
  }
}

class ReceivedSentPM extends StatefulWidget {
  final ChatEventModel evnt;
  final String nick;
  final int timestamp;
  final String id;
  final String userNick;
  final bool isGC;
  final OpenReplyDMCB openReplyDM;
  final ClientModel client;
  final ChatModel chat;

  const ReceivedSentPM(this.evnt, this.nick, this.timestamp, this.id,
      this.userNick, this.isGC, this.openReplyDM, this.client, this.chat,
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

  @override
  Widget build(BuildContext context) {
    var suffix = "";
    switch (widget.evnt.sentState) {
      case CMS_sending:
        break;
      case CMS_sent:
        break;
      case CMS_errored:
        suffix = "\n\n${widget.evnt.sendError}";
        break;
      default:
    }
    var sourceID = widget.evnt.event.sid;
    if (widget.evnt.source != null) {
      sourceID = widget.evnt.source!.id;
    }
    var now = DateTime.fromMillisecondsSinceEpoch(widget.timestamp);
    var hour = DateFormat('HH:mm').format(now);
    var fullDate = DateFormat("yyyy-MM-dd HH:mm:ss").format(now);

    var msg = "${widget.evnt.event.msg}$suffix";
    msg = msg.replaceAll("\n",
        "  \n"); // Replace newlines with <space space newline> for proper md render
    var theme = Theme.of(context);
    var darkTextColor = theme.indicatorColor;
    var avatarColor = colorFromNick(widget.nick, theme.brightness);

    var selectedBackgroundColor = theme.highlightColor;
    var textColor = theme.dividerColor;
    var sentBackgroundColor = theme.dialogBackgroundColor;
    // Will show a divider and text before the last unread message.
    var firstUnread = Consumer<ThemeNotifier>(
        builder: (context, theme, _) => widget.evnt.firstUnread
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
                    style: TextStyle(
                        fontSize: theme.getSmallFont(context),
                        color: textColor)),
                Expanded(
                    child: Divider(
                  color: textColor, //color of divider
                  height: 8, //height spacing of divider
                  thickness: 1, //thickness of divier line
                  indent: 5, //spacing at the start of divider
                  endIndent: 5, //spacing at the end of divider
                )),
              ])
            : const Empty());
    var isOwnMessage = widget.userNick == widget.nick;

    List<Widget> getMessage(BuildContext context) {
      return <Widget>[
        Provider<DownloadSource>(
            create: (context) => DownloadSource(sourceID),
            child: MarkdownArea(
                msg,
                widget.userNick != widget.nick &&
                    msg.contains(widget.userNick)))
      ];
    }

    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Column(children: [
              firstUnread,
              Container(
                  margin: EdgeInsets.only(
                      top: widget.evnt.sameUser ? 2 : 10,
                      right: isOwnMessage ? 10 : 0),
                  child: Row(
                      crossAxisAlignment: CrossAxisAlignment.end,
                      mainAxisAlignment: isOwnMessage
                          ? MainAxisAlignment.end
                          : MainAxisAlignment.start,
                      children: <Widget>[
                        widget.evnt.showAvatar
                            ? Opacity(
                                opacity: widget.evnt.showAvatar && !isOwnMessage
                                    ? 1
                                    : 0,
                                child: SelectionContainer.disabled(
                                    child: Container(
                                  height: 28,
                                  width: 28,
                                  margin: const EdgeInsets.only(
                                      top: 0, bottom: 10, left: 10, right: 10),
                                  child: UserContextMenu(
                                    client: widget.client,
                                    targetUserChat: widget.evnt.source,
                                    child: UserMenuAvatar(
                                      widget.client,
                                      widget.evnt.source ?? widget.chat,
                                      showChatSideMenuOnTap: true,
                                    ),
                                  ),
                                )))
                            : const SizedBox(width: 48),
                        ConstrainedBox(
                            constraints: BoxConstraints(
                              maxWidth: isScreenSmall
                                  ? MediaQuery.of(context).size.width * 0.7
                                  : MediaQuery.of(context).size.width * 0.4,
                            ),
                            child: Column(
                                crossAxisAlignment: isOwnMessage
                                    ? CrossAxisAlignment.end
                                    : CrossAxisAlignment.start,
                                mainAxisAlignment: MainAxisAlignment.end,
                                children: <Widget>[
                                  Container(
                                    padding: const EdgeInsets.all(5),
                                    decoration: BoxDecoration(
                                      color: isOwnMessage
                                          ? sentBackgroundColor
                                          : selectedBackgroundColor,
                                      borderRadius: BorderRadius.circular(10),
                                    ),
                                    child: Column(
                                      crossAxisAlignment: isOwnMessage
                                          ? CrossAxisAlignment.end
                                          : CrossAxisAlignment.start,
                                      children: <Widget>[
                                        widget.evnt.sameUser || isOwnMessage
                                            ? const Empty()
                                            : Text(widget.nick,
                                                style: TextStyle(
                                                  fontSize: theme
                                                      .getMediumFont(context),
                                                  color: avatarColor,
                                                  fontWeight: FontWeight.w500,
                                                  letterSpacing: 0.5,
                                                )),
                                        Wrap(
                                          children: getMessage(context),
                                        ),
                                        SelectionContainer.disabled(
                                          child: Padding(
                                            padding:
                                                const EdgeInsets.only(top: 5),
                                            child: Tooltip(
                                              message: fullDate,
                                              child: Text(
                                                hour,
                                                style: TextStyle(
                                                    fontSize: theme
                                                        .getSmallFont(context),
                                                    color: darkTextColor),
                                              ),
                                            ),
                                          ),
                                        )
                                      ],
                                    ),
                                  ),
                                ])),
/*
                    // Now put reply/dm button /here if GC
                    widget.isGC &&
                            widget.userNick != widget.nick &&
                            !widget.evnt.sameUser
                        ? Material(
                            color: selectedBackgroundColor.withOpacity(0),
                            child: IconButton(
                                hoverColor: selectedBackgroundColor,
                                splashRadius: 15,
                                iconSize: 25,
                                tooltip: "Go to DM",
                                onPressed: () =>
                                    widget.openReplyDM(false, widget.nick),
                                icon: const Icon(size: 28, Icons.reply)))
                        : const Empty(),
                        
            */
                      ]))
            ]));
  }
}

class ReceivedSentMobilePM extends StatefulWidget {
  final ChatEventModel evnt;
  final String nick;
  final int timestamp;
  final ShowSubMenuCB showSubMenu;
  final String id;
  final String userNick;
  final bool isGC;
  final ClientModel client;

  const ReceivedSentMobilePM(this.evnt, this.nick, this.timestamp,
      this.showSubMenu, this.id, this.userNick, this.isGC, this.client,
      {Key? key})
      : super(key: key);

  @override
  State<ReceivedSentMobilePM> createState() => _ReceivedSentPMMobileState();
}

class _ReceivedSentPMMobileState extends State<ReceivedSentMobilePM> {
  void eventChanged() => setState(() {});

  @override
  initState() {
    super.initState();
    widget.evnt.addListener(eventChanged);
  }

  @override
  didUpdateWidget(ReceivedSentMobilePM oldWidget) {
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

  @override
  Widget build(BuildContext context) {
    var suffix = "";
    switch (widget.evnt.sentState) {
      case CMS_sending:
        break;
      case CMS_sent:
        break;
      case CMS_errored:
        suffix = "\n\n${widget.evnt.sendError}";
        break;
      default:
    }

    var sent = widget.evnt.source == null;

    var sourceID = widget.evnt.event.sid;
    if (!sent) {
      sourceID = widget.evnt.source!.id;
    }
    var now = DateTime.fromMillisecondsSinceEpoch(widget.timestamp);
    var hour = DateFormat('HH:mm').format(now);
    var fullDate = DateFormat("yyyy-MM-dd HH:mm:ss").format(now);

    var msg = "${widget.evnt.event.msg}$suffix";
    msg = msg.replaceAll("\n",
        "  \n"); // Replace newlines with <space space newline> for proper md render
    var theme = Theme.of(context);
    var darkTextColor = theme.indicatorColor;
    var textColor = theme.focusColor;
    var receivedBackgroundColor = theme.highlightColor;
    var sentBackgroundColor = theme.dialogBackgroundColor;

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Column(children: [
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
                          style: TextStyle(
                              fontSize: theme.getSmallFont(context),
                              color: textColor)),
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
              Column(children: [
                widget.evnt.sameUser
                    ? const Empty()
                    : const SizedBox(height: 20),
                sent
                    ? Row(
                        crossAxisAlignment: CrossAxisAlignment.end,
                        mainAxisAlignment: MainAxisAlignment.end,
                        children: [
                            Flexible(
                              flex: 3,
                              child: SelectionContainer.disabled(
                                  child: Padding(
                                padding: const EdgeInsets.all(4.0),
                                child: Tooltip(
                                    message: fullDate,
                                    child: Text(
                                      hour,
                                      style: TextStyle(
                                          fontSize: theme.getSmallFont(context),
                                          color: darkTextColor), // DATE COLOR
                                    )),
                              )),
                            ),
                            Flexible(
                              flex: 7,
                              child: Container(
                                  margin:
                                      const EdgeInsets.only(left: 5, right: 20),
                                  padding: const EdgeInsets.only(
                                      left: 10, right: 10, top: 5, bottom: 5),
                                  decoration: BoxDecoration(
                                    color: sentBackgroundColor,
                                    borderRadius: BorderRadius.circular(10),
                                  ),
                                  child: Provider<DownloadSource>(
                                      create: (context) =>
                                          DownloadSource(sourceID),
                                      child: MarkdownArea(
                                          msg,
                                          widget.userNick != widget.nick &&
                                              msg.contains(widget.userNick)))),
                            )
                          ])
                    : Row(
                        crossAxisAlignment: CrossAxisAlignment.end,
                        mainAxisAlignment: MainAxisAlignment.start,
                        children: [
                            Flexible(
                                flex: 7,
                                child: Container(
                                    margin: const EdgeInsets.only(
                                        left: 5, right: 20),
                                    padding: const EdgeInsets.only(
                                        left: 10, right: 10, top: 5, bottom: 5),
                                    decoration: BoxDecoration(
                                      color: receivedBackgroundColor,
                                      borderRadius: BorderRadius.circular(10),
                                    ),
                                    child: Provider<DownloadSource>(
                                        create: (context) =>
                                            DownloadSource(sourceID),
                                        child: MarkdownArea(
                                            msg,
                                            widget.userNick != widget.nick &&
                                                msg.contains(
                                                    widget.userNick))))),
                            Flexible(
                              flex: 3,
                              child: SelectionContainer.disabled(
                                  child: Padding(
                                padding: const EdgeInsets.all(4.0),
                                child: Tooltip(
                                    message: fullDate,
                                    child: Text(
                                      hour,
                                      style: TextStyle(
                                          fontSize: theme.getSmallFont(context),
                                          color: darkTextColor), // DATE COLOR
                                    )),
                              )),
                            ),
                          ]),
                const SizedBox(height: 5),
              ])
            ]));
  }
}

class PMW extends StatelessWidget {
  final ChatEventModel evnt;
  final ShowSubMenuCB showSubMenu;
  final ClientModel client;
  final ChatModel chat;
  const PMW(this.evnt, this.showSubMenu, this.client, this.chat, {Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    var timestamp = 0;
    var event = evnt.event;
    if (event is PM) {
      timestamp =
          evnt.source?.nick == null ? event.timestamp : event.timestamp * 1000;
    }

    openReplyDM(bool isGC, String id) => null;
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;

    if (isScreenSmall) {
      return ReceivedSentMobilePM(
          evnt,
          evnt.source?.nick ?? client.nick,
          timestamp,
          showSubMenu,
          evnt.source?.id ?? "",
          client.nick,
          false,
          client);
    }
    return ReceivedSentPM(evnt, evnt.source?.nick ?? client.nick, timestamp,
        evnt.source?.id ?? "", client.nick, false, openReplyDM, client, chat);
  }
}

class GCMW extends StatelessWidget {
  final ChatEventModel evnt;
  final ShowSubMenuCB showSubMenu;
  final OpenReplyDMCB openReplyDM;
  final ClientModel client;
  final ChatModel chat;
  const GCMW(
      this.evnt, this.showSubMenu, this.openReplyDM, this.client, this.chat,
      {Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    var event = evnt.event;
    var timestamp = 0;
    if (event is GCMsg) {
      timestamp =
          evnt.source?.nick == null ? event.timestamp : event.timestamp * 1000;
    }

    return ReceivedSentPM(evnt, evnt.source?.nick ?? client.nick, timestamp,
        evnt.source?.id ?? "", client.nick, true, openReplyDM, client, chat);
  }
}

class GCUserEventW extends StatelessWidget {
  final ChatEventModel evnt;
  const GCUserEventW(this.evnt, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    return Consumer<ThemeNotifier>(builder: (context, theme, _) {
      if (evnt.source != null) {
        return ServerEvent(
            child: Text("${evnt.source!.nick}:  ${evnt.event.msg}",
                style: TextStyle(
                    fontSize: theme.getSmallFont(context), color: textColor)));
      } else {
        return ServerEvent(
            child: Text(evnt.event.msg,
                style: TextStyle(
                    fontSize: theme.getSmallFont(context), color: textColor)));
      }
    });
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
    return Consumer<ThemeNotifier>(builder: (context, theme, _) {
      switch (event.sentState) {
        case CMS_canceled:
          return ServerEvent(
              child: Text("Declined GC invitation to '${invite.name}",
                  style: TextStyle(
                      fontSize: theme.getSmallFont(context),
                      color: textColor)));
        case CMS_errored:
          return ServerEvent(
              child: Text(
                  "Unable to join GC ${invite.name}: ${event.sendError}",
                  style: TextStyle(
                      fontSize: theme.getSmallFont(context),
                      color: textColor)));
        case CMS_sent:
          return ServerEvent(
              child: Text("Accepted invitation to join GC '${invite.name}'",
                  style: TextStyle(
                      fontSize: theme.getSmallFont(context),
                      color: textColor)));
        case CMS_sending:
          return ServerEvent(
              child: Text("Accepting invitation to join GC '${invite.name}'",
                  style: TextStyle(
                      fontSize: theme.getSmallFont(context),
                      color: textColor)));
      }

      return ServerEvent(
          child: Column(children: [
        Text("Received invitation to join GC '${invite.name}'",
            style: TextStyle(
                fontSize: theme.getSmallFont(context), color: textColor)),
        const SizedBox(height: 20),
        Row(mainAxisAlignment: MainAxisAlignment.center, children: [
          ElevatedButton(onPressed: acceptInvite, child: const Text("Accept")),
          const SizedBox(width: 10),
          CancelButton(onPressed: cancelInvite),
        ]),
      ]));
    });
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

    return Consumer<ThemeNotifier>(builder: (context, theme, _) {
      if (tip.state == ITS_completed) {
        child = Text(
            "✓ Requesting invoice for ${formatDCR(tip.amount)} to tip ${source.nick}!",
            style: TextStyle(
                fontSize: theme.getSmallFont(context), color: textColor));
      } else if (tip.state == ITS_errored) {
        child = Text("✗ Failed to send tip: ${tip.error}",
            style: TextStyle(
                fontSize: theme.getSmallFont(context), color: textColor));
      } else if (tip.state == ITS_received) {
        child = Text("\$ Received ${tip.amount} DCR from ${source.nick}!",
            style: TextStyle(
                fontSize: theme.getSmallFont(context), color: textColor));
      } else {
        child = Text(
            "… Requesting invoice for ${formatDCR(tip.amount)} DCR to tip ${source.nick}...",
            style: TextStyle(
                fontSize: theme.getSmallFont(context), color: textColor));
      }
      return ServerEvent(child: child);
    });
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
    return Consumer<ThemeNotifier>(builder: (context, theme, _) {
      if (event.state == SCE_sent) {
        child = Text("✓ ${widget.event.msg}",
            style: TextStyle(
                fontSize: theme.getSmallFont(context), color: textColor));
      } else if (event.state == SCE_errored) {
        child = Text("✗ Failed to ${widget.event.msg} - ${event.error}",
            style: TextStyle(
                fontSize: theme.getSmallFont(context), color: textColor));
      } else if (event.state == SCE_sending) {
        child = Text("… ${widget.event.msg}",
            style: TextStyle(
                fontSize: theme.getSmallFont(context), color: textColor));
      } else if (event.state == SCE_received) {
        child = Text(widget.event.msg,
            style: TextStyle(
                fontSize: theme.getSmallFont(context), color: textColor));
      } else {
        child = Text("? unknown state ${event.state}",
            style: TextStyle(
                fontSize: theme.getSmallFont(context), color: textColor));
      }
      return ServerEvent(child: child);
    });
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
    return Consumer2<DownloadsModel, ThemeNotifier>(
        builder: (context, downloads, theme, child) => ServerEvent(
                child: Column(children: [
              Text("User Content",
                  style: TextStyle(
                      color: Theme.of(context).focusColor,
                      fontSize: theme.getMediumFont(context))),
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
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => ServerEvent(
            child: Text("Received post '${event.title}'",
                style: TextStyle(
                    fontSize: theme.getSmallFont(context), color: textColor))));
  }
}

class PostSubscriptionEventW extends StatefulWidget {
  final PostSubscriptionResult event;
  final ClientModel client;
  const PostSubscriptionEventW(this.event, this.client, {Key? key})
      : super(key: key);

  @override
  State<PostSubscriptionEventW> createState() => _PostSubscriptionEventWState();
}

class _PostSubscriptionEventWState extends State<PostSubscriptionEventW> {
  PostSubscriptionResult get event => widget.event;
  ClientModel get client => widget.client;

  String msg = "";
  @override
  void initState() {
    super.initState();
    if (event.wasSubRequest && event.error != "") {
      msg = "Unable to subscribe to user's posts: ${event.error}";
    } else if (event.wasSubRequest) {
      msg = "Subscribed to user's posts!";
    } else if (event.error != "") {
      msg = "Unable to unsubscribe from user's posts: ${event.error}";
    } else {
      msg = "Unsubscribed from user's posts!";
    }
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => ServerEvent(
            child: Text(msg,
                style: TextStyle(
                    fontSize: theme.getSmallFont(context), color: textColor))));
  }
}

class PostsSubscriberUpdatedW extends StatelessWidget {
  final PostSubscriberUpdated event;
  const PostsSubscriberUpdatedW(this.event, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    var subTxt = event.subscribed ? "subscribed to" : "unsubscribed from";
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => ServerEvent(
            child: Text("${event.nick} $subTxt the local client's posts.",
                style: TextStyle(
                    fontSize: theme.getSmallFont(context), color: textColor))));
  }
}

class ListPostsEventW extends StatefulWidget {
  final ChatEventModel event;
  final ChatModel chat;
  const ListPostsEventW(this.event, this.chat, {Key? key}) : super(key: key);

  @override
  State<ListPostsEventW> createState() => _ListPostsWState();
}

class _ListPostsWState extends State<ListPostsEventW> {
  ChatEventModel get event => widget.event;
  ChatModel get chat => widget.chat;

  String msg = "";
  bool hasUserPosts = false;

  void update() {
    setState(() {
      if (event.sendError != null) {
        msg = "Unable to list user's posts: ${event.sendError}";
      } else if (event.sentState == CMS_sending) {
        msg = "… Listing user's posts";
      } else if (event.sentState == CMS_sent) {
        msg = "✓ Listing user's posts";
      } else {
        msg = "Unknown state when listing user's post: ${event.sentState}";
      }

      hasUserPosts = chat.userPostsList.isNotEmpty;
    });
  }

  @override
  void initState() {
    super.initState();
    update();
    event.addListener(update);
    chat.userPostsList.addListener(update);
  }

  @override
  void didUpdateWidget(ListPostsEventW oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.event != event) {
      oldWidget.event.removeListener(update);
      event.addListener(update);
      oldWidget.chat.userPostsList.removeListener(update);
      chat.userPostsList.addListener(update);
    }
  }

  @override
  void dispose() {
    event.removeListener(update);
    chat.userPostsList.removeListener(update);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => ServerEvent(
              child: hasUserPosts
                  ? TextButton(
                      onPressed: () => FeedScreen.showUsersPosts(context, chat),
                      child: Text("Show user's posts",
                          style: TextStyle(
                              fontSize: theme.getSmallFont(context),
                              color: textColor)))
                  : Text(msg,
                      style: TextStyle(
                          fontSize: theme.getSmallFont(context),
                          color: textColor)),
            ));
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
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => ServerEvent(
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
                    Text(
                      "Downloaded file ${event.diskPath}",
                      style: TextStyle(
                          fontSize: theme.getSmallFont(context),
                          color: textColor),
                    ),
                  ],
                ),
              ),
            ));
  }
}

class GCVersionWarnW extends StatelessWidget {
  final GCVersionWarn event;
  const GCVersionWarnW(this.event, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var bgColor = Colors.red[600];
    var textColor = Colors.white;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Container(
            padding: const EdgeInsets.only(left: 41, top: 5, bottom: 5),
            margin: const EdgeInsets.all(5),
            decoration: BoxDecoration(
                color: bgColor,
                borderRadius: const BorderRadius.all(Radius.circular(5))),
            child: Text(
                "Received GC definitions with unsupported version ${event.version}. Please update the software to interact in this GC.",
                style: TextStyle(
                    fontSize: theme.getSmallFont(context), color: textColor))));
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

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => ServerEvent(
            child: Text(msg,
                style: TextStyle(
                    fontSize: theme.getSmallFont(context), color: textColor))));
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

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => ServerEvent(
            child: Text(msg,
                style: TextStyle(
                    fontSize: theme.getSmallFont(context), color: textColor))));
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
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => ServerEvent(
            child: Text(msg,
                style: TextStyle(
                    fontSize: theme.getSmallFont(context), color: textColor))));
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
    var role = event.changedOwner ? "owner" : "admin";
    if (event.added != null) {
      msg += event.added!.fold("", (prev, e) {
        var nick = e == myID ? "Local client" : client.getNick(e);
        nick = nick == "" ? e : nick;
        return prev + "\n$nick added as $role";
      });
    }
    if (event.removed != null) {
      msg += event.removed!.fold("", (prev, e) {
        var nick = e == myID ? "Local client" : client.getNick(e);
        nick = nick == "" ? e : nick;
        return prev + "\n$nick removed as $role";
      });
    }

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => ServerEvent(
            child: Text(msg,
                style: TextStyle(
                    fontSize: theme.getSmallFont(context), color: textColor))));
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
      client.requestMediateID(suggest.invitee, suggest.target);
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
    return Consumer<ThemeNotifier>(builder: (context, theme, _) {
      switch (event.sentState) {
        case Suggestion_accepted:
          return ServerEvent(
              child: Text(
                  "Accepting suggestion to KX from '${suggest.inviteenick}' to '${suggest.targetnick}'",
                  style: TextStyle(
                      fontSize: theme.getSmallFont(context),
                      color: textColor)));
        case Suggestion_errored:
          return ServerEvent(
              child: Text(
                  "Unable to accept suggestion from  '${suggest.inviteenick}' to '${suggest.targetnick}'",
                  style: TextStyle(
                      fontSize: theme.getSmallFont(context),
                      color: textColor)));
        case Suggestion_canceled:
          return ServerEvent(
              child: Text(
                  "Canceled suggestion to KX from '${suggest.inviteenick}' to '${suggest.targetnick}'",
                  style: TextStyle(
                      fontSize: theme.getSmallFont(context),
                      color: textColor)));
        case Suggestion_confirmed:
          return ServerEvent(
              child: Text(
                  "Confirmed suggestion to KX from '${suggest.inviteenick}' to '${suggest.targetnick}'",
                  style: TextStyle(
                      fontSize: theme.getSmallFont(context),
                      color: textColor)));
      }

      return suggest.alreadyknown
          ? ServerEvent(
              child: Text(
                  "Received already known suggestion to KX from '${suggest.inviteenick}' to '${suggest.targetnick}'",
                  style: TextStyle(
                      fontSize: theme.getSmallFont(context), color: textColor)))
          : ServerEvent(
              child: Column(children: [
              Text(
                  "Received suggestion to KX from '${suggest.inviteenick}' to '${suggest.targetnick}'",
                  style: TextStyle(
                      fontSize: theme.getSmallFont(context), color: textColor)),
              const SizedBox(height: 20),
              Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                ElevatedButton(
                    onPressed: acceptSuggestion, child: const Text("Accept")),
                const SizedBox(width: 10),
                CancelButton(onPressed: cancelSuggestion),
              ]),
            ]));
    });
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

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => ServerEvent(
            child: Text(msg,
                style: TextStyle(
                    fontSize: theme.getSmallFont(context), color: textColor))));
  }
}

class FetchedResourceW extends StatefulWidget {
  final RequestedResourceEvent event;
  const FetchedResourceW(this.event, {super.key});

  @override
  State<FetchedResourceW> createState() => _FetchedResourceWState();
}

class _FetchedResourceWState extends State<FetchedResourceW> {
  PagesSession get sess => widget.event.session;

  void requestUpdated() {
    setState(() {});
  }

  @override
  void initState() {
    super.initState();
    sess.addListener(requestUpdated);
  }

  @override
  void didUpdateWidget(FetchedResourceW oldWidget) {
    oldWidget.event.session.removeListener(requestUpdated);
    super.didUpdateWidget(oldWidget);
    sess.addListener(requestUpdated);
  }

  @override
  void dispose() {
    sess.removeListener(requestUpdated);
    super.dispose();
  }

  void viewPage() {
    Provider.of<ResourcesModel>(context, listen: false).mostRecent = sess;
    Navigator.of(context).pushReplacementNamed(ViewPageScreen.routeName);
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    var backgroundColor = theme.highlightColor;
    return Consumer<ThemeNotifier>(builder: (context, theme, _) {
      if (sess.loading) {
        return ServerEvent(
            child: Container(
                decoration: BoxDecoration(
                    color: backgroundColor,
                    borderRadius: const BorderRadius.all(Radius.circular(5))),
                child: Text("Requested page",
                    style: TextStyle(
                        fontSize: theme.getSmallFont(context),
                        color: textColor))));
      } else {
        return ServerEvent(
            child: Container(
                decoration: BoxDecoration(
                    color: backgroundColor,
                    borderRadius: const BorderRadius.all(Radius.circular(5))),
                child: ElevatedButton(
                  onPressed: viewPage,
                  child: const Text("View Page"),
                )));
      }
    });
  }
}

class HandshakeStageW extends StatelessWidget {
  final HandshakeStage event;
  const HandshakeStageW(this.event, {super.key});

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    var backgroundColor = theme.highlightColor;

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => ServerEvent(
              child: Container(
                decoration: BoxDecoration(
                    color: backgroundColor,
                    borderRadius: const BorderRadius.all(Radius.circular(5))),
                child: Text(
                    "Completed 3-way handshake (due to receiving msg ${event.stage})",
                    style: TextStyle(
                        color: textColor,
                        fontSize: theme.getSmallFont(context))),
              ),
            ));
  }
}

class ProfileUpdatedW extends StatelessWidget {
  final ProfileUpdated event;
  const ProfileUpdatedW(this.event, {super.key});

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    var backgroundColor = theme.highlightColor;

    var fields = event.updatedFields.join(", ");

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => ServerEvent(
              child: Container(
                decoration: BoxDecoration(
                    color: backgroundColor,
                    borderRadius: const BorderRadius.all(Radius.circular(5))),
                child: Text("Profile updated ($fields)",
                    style: TextStyle(
                        color: textColor,
                        fontSize: theme.getSmallFont(context))),
              ),
            ));
  }
}

class Event extends StatelessWidget {
  final ChatEventModel event;
  final ChatModel chat;
  final ClientModel client;
  const Event(this.chat, this.event, this.client, {Key? key}) : super(key: key);

  showSubMenu() => client.ui.chatSideMenuActive.chat = chat;
  openReplyDM(bool isGC, String id) => client.setActiveByNick(id, isGC);
  @override
  Widget build(BuildContext context) {
    if (event.event is DateChangeEvent) {
      var theme = Theme.of(context);
      var textColor = theme.dividerColor;
      return Row(
          crossAxisAlignment: CrossAxisAlignment.center,
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            DateChange(
                child: Center(
                    child: Text(
                        textAlign: TextAlign.center,
                        event.event.msg,
                        style: TextStyle(color: textColor))))
          ]);
    }

    if (event.event is PM) {
      return PMW(event, showSubMenu, client, chat);
    }

    if (event.event is InflightTip) {
      return InflightTipW((event.event as InflightTip), event.source!);
    }

    if (event.event is GCMsg) {
      return GCMW(event, showSubMenu, openReplyDM, client, chat);
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

    if (event.event is UserContentList) {
      return UserContentEventW(event.event as UserContentList, chat);
    }

    if (event.event is PostSubscriptionResult) {
      return PostSubscriptionEventW(
          event.event as PostSubscriptionResult, client);
    }

    if (event.event is PostSubscriberUpdated) {
      return PostsSubscriberUpdatedW(event.event as PostSubscriberUpdated);
    }

    if (event.event is RequestedUsersPostListEvent) {
      return ListPostsEventW(event, chat);
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

    if (event.event is RequestedResourceEvent) {
      return FetchedResourceW(event.event as RequestedResourceEvent);
    }

    if (event.event is HandshakeStage) {
      return HandshakeStageW(event.event as HandshakeStage);
    }

    if (event.event is ProfileUpdated) {
      return ProfileUpdatedW(event.event as ProfileUpdated);
    }

    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    return Container(
        color: Theme.of(context).errorColor, // ERROR COLOR
        child: Text("Unknonwn chat event type",
            style: TextStyle(color: textColor)));
  }
}
