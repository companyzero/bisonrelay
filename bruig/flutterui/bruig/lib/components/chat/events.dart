import 'dart:async';
import 'dart:math';

import 'package:bruig/components/containers.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/realtimechat.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/feed.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
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
import 'package:bruig/theme_manager.dart';

class ServerEvent extends StatelessWidget {
  final Widget? child;
  final String? msg;
  const ServerEvent({this.child, this.msg, super.key});

  @override
  Widget build(BuildContext context) {
    assert(child != null || msg != null);

    return Box(
        padding: const EdgeInsets.only(left: 41, top: 5, bottom: 5),
        margin: const EdgeInsets.all(5),
        color: SurfaceColor.surfaceContainer,
        child: child ?? Txt.S(msg!));
  }
}

class InstantCall extends StatelessWidget {
  final InstantCallEvent event;
  const InstantCall({required this.event, super.key});

  @override
  Widget build(BuildContext context) {
    var state = "";
    if (event.state == ICE_starting) {
      state = "started";
    } else if (event.state == ICE_finished) {
      state = "ended";
    } else if (event.state == ICE_canceled) {
      return Container(
          padding: const EdgeInsets.only(top: 5, bottom: 5),
          margin: const EdgeInsets.all(5),
          alignment: Alignment.center,
          child: Text(
              textAlign: TextAlign.center,
              "Call was rejected at: ${event.msg}"));
    }
    return Container(
        padding: const EdgeInsets.only(top: 5, bottom: 5),
        margin: const EdgeInsets.all(5),
        alignment: Alignment.center,
        child: Text(
            textAlign: TextAlign.center, "You $state a call at: ${event.msg}"));
  }
}

class DateChange extends StatelessWidget {
  final DateChangeEvent event;
  const DateChange({required this.event, super.key});

  @override
  Widget build(BuildContext context) {
    return Container(
        padding: const EdgeInsets.only(top: 5, bottom: 5),
        margin: const EdgeInsets.all(5),
        alignment: Alignment.center,
        child: Text(textAlign: TextAlign.center, event.msg));
  }
}

class _FirstUnreadIndicator extends StatelessWidget {
  const _FirstUnreadIndicator();

  @override
  Widget build(BuildContext context) {
    return const Row(children: [
      Expanded(child: Divider()),
      SizedBox(width: 8),
      Txt.S("Last read posts", color: TextColor.onSurfaceVariant),
      SizedBox(width: 8),
      Expanded(child: Divider()),
    ]);
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
      {super.key});

  @override
  State<ReceivedSentPM> createState() => _ReceivedSentPMState();
}

class _ReceivedSentPMState extends State<ReceivedSentPM> {
  final ContextMenuController _contextMenuController = ContextMenuController();
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

  void copy(BuildContext context, String toCopy) {
    Clipboard.setData(ClipboardData(text: toCopy));

    var textMsg = toCopy.substring(0, min(toCopy.length, 36));
    if (textMsg.length < toCopy.length) {
      textMsg += "...";
    }
    showSuccessSnackbar(context, "Copied \"$textMsg\" to clipboard");
  }

  void messageSecondaryTapContext(
      TapDownDetails details, String msg, String fullDate, String nick) {
    var toCopy = "$fullDate $nick - $msg";
    _contextMenuController.show(
      context: context,
      contextMenuBuilder: (context) {
        return AdaptiveTextSelectionToolbar.buttonItems(
          anchors: TextSelectionToolbarAnchors(
            primaryAnchor: details.globalPosition,
          ),
          buttonItems: [
            ContextMenuButtonItem(
              onPressed: () {
                copy(context, toCopy);
                _contextMenuController.remove();
                //.hide();
                // Handle custom action
              },
              label: 'Copy',
            ),
          ],
        );
      },
    );
  }

  void messageLongDownContext(
      LongPressDownDetails details, String msg, String fullDate, String nick) {
    var toCopy = "$fullDate $nick - $msg";
    _contextMenuController.show(
      context: context,
      contextMenuBuilder: (context) {
        return AdaptiveTextSelectionToolbar.buttonItems(
          anchors: TextSelectionToolbarAnchors(
            primaryAnchor: details.globalPosition,
          ),
          buttonItems: [
            ContextMenuButtonItem(
              onPressed: () {
                copy(context, toCopy);
                _contextMenuController.remove();
                //.hide();
                // Handle custom action
              },
              label: 'Copy',
            ),
          ],
        );
      },
    );
  }

  Widget buildMessage(BuildContext context) {
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

    var isOwnMessage = widget.userNick == widget.nick;

    // List<Widget> getMessage(BuildContext context) {
    //   return <Widget>[
    //     Provider<DownloadSource>(
    //         create: (context) => DownloadSource(sourceID),
    //         child: MarkdownArea(
    //             msg,
    //             widget.userNick != widget.nick &&
    //                 msg.contains(widget.userNick)))
    //   ];
    // }

    bool isScreenSmall = checkIsScreenSmall(context);

    var showAvatar = !widget.evnt.showAvatar && !isOwnMessage && !isScreenSmall;
    var showNick = !(widget.evnt.sameUser || isOwnMessage) && !isScreenSmall;

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Container(
            margin: EdgeInsets.only(
                top: widget.evnt.sameUser ? 2 : 10,
                right: isOwnMessage ? 20 : 0),
            child: Row(
                crossAxisAlignment: CrossAxisAlignment.end,
                mainAxisAlignment: isOwnMessage
                    ? MainAxisAlignment.end
                    : MainAxisAlignment.start,
                children: <Widget>[
                  if (showAvatar)
                    Container(
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
                    )
                  else
                    isScreenSmall
                        ? const SizedBox(width: 20)
                        : const SizedBox(width: 48),
                  Flexible(
                      child: GestureDetector(
                          onSecondaryTapDown: (details) {
                            if (!isScreenSmall) {
                              messageSecondaryTapContext(
                                  details, msg, fullDate, widget.nick);
                            }
                          },
                          onLongPressDown: (details) {
                            if (isScreenSmall) {
                              messageLongDownContext(
                                  details, msg, fullDate, widget.nick);
                            }
                          },
                          child: ConstrainedBox(
                              constraints: BoxConstraints(
                                maxWidth: isScreenSmall
                                    ? MediaQuery.sizeOf(context).width * 0.75
                                    : MediaQuery.sizeOf(context).width * 0.4,
                              ),
                              child: Container(
                                  padding: const EdgeInsets.only(
                                      top: 5, left: 10, right: 10, bottom: 5),
                                  decoration: BoxDecoration(
                                    color: isOwnMessage
                                        ? theme.colors.surfaceContainer
                                        : theme.colors.surfaceContainerHighest,
                                    borderRadius: BorderRadius.circular(10),
                                  ),
                                  child: Column(
                                      crossAxisAlignment: isOwnMessage
                                          ? CrossAxisAlignment.end
                                          : CrossAxisAlignment.start,
                                      children: <Widget>[
                                        if (showNick)
                                          Text(widget.nick,
                                              style: theme.textStyleForNick(
                                                  widget.nick)),
                                        Provider<DownloadSource>(
                                            create: (context) =>
                                                DownloadSource(sourceID),
                                            child: MarkdownArea(
                                                msg,
                                                widget.userNick !=
                                                        widget.nick &&
                                                    msg.contains(
                                                        widget.userNick))),
                                        Padding(
                                            padding:
                                                const EdgeInsets.only(top: 5),
                                            child: Tooltip(
                                                message: fullDate,
                                                child: Txt.S(hour,
                                                    color: TextColor
                                                        .onSurfaceVariant)))
                                      ])))))
                ])));
  }

  @override
  Widget build(BuildContext context) {
    if (widget.evnt.firstUnread) {
      return Column(children: [
        const _FirstUnreadIndicator(),
        buildMessage(context),
      ]);
    }

    return buildMessage(context);
  }
}

// class ReceivedSentMobilePM extends StatefulWidget {
//   final ChatEventModel evnt;
//   final String nick;
//   final int timestamp;
//   final ShowSubMenuCB showSubMenu;
//   final String id;
//   final String userNick;
//   final bool isGC;
//   final ClientModel client;

//   const ReceivedSentMobilePM(this.evnt, this.nick, this.timestamp,
//       this.showSubMenu, this.id, this.userNick, this.isGC, this.client,
//       {Key? key})
//       : super(key: key);

//   @override
//   State<ReceivedSentMobilePM> createState() => _ReceivedSentPMMobileState();
// }

// class _ReceivedSentPMMobileState extends State<ReceivedSentMobilePM> {
//   void eventChanged() => setState(() {});

//   @override
//   initState() {
//     super.initState();
//     widget.evnt.addListener(eventChanged);
//   }

//   @override
//   didUpdateWidget(ReceivedSentMobilePM oldWidget) {
//     super.didUpdateWidget(oldWidget);
//     oldWidget.evnt.removeListener(eventChanged);
//     widget.evnt.addListener(eventChanged);
//   }

//   @override
//   dispose() {
//     widget.evnt.removeListener(eventChanged);
//     super.dispose();
//   }

//   Future<void> launchUrlAwait(url) async {
//     if (!await launchUrl(Uri.parse(url))) {
//       throw 'Could not launch $url';
//     }
//   }

//   @override
//   Widget build(BuildContext context) {
//     var suffix = "";
//     switch (widget.evnt.sentState) {
//       case CMS_sending:
//         break;
//       case CMS_sent:
//         break;
//       case CMS_errored:
//         suffix = "\n\n${widget.evnt.sendError}";
//         break;
//       default:
//     }

//     var sent = widget.evnt.source == null;

//     var sourceID = widget.evnt.event.sid;
//     if (!sent) {
//       sourceID = widget.evnt.source!.id;
//     }
//     var now = DateTime.fromMillisecondsSinceEpoch(widget.timestamp);
//     var hour = DateFormat('HH:mm').format(now);
//     var fullDate = DateFormat("yyyy-MM-dd HH:mm:ss").format(now);

//     var msg = "${widget.evnt.event.msg}$suffix";
//     msg = msg.replaceAll("\n",
//         "  \n"); // Replace newlines with <space space newline> for proper md render
//     var theme = Theme.of(context);
//     var darkTextColor = theme.indicatorColor;
//     var receivedBackgroundColor = theme.highlightColor;
//     var sentBackgroundColor = theme.dialogBackgroundColor;

//     return Consumer<ThemeNotifier>(
//         builder: (context, theme, _) => Column(children: [
//               widget.evnt.firstUnread
//                   ? const _FirstUnreadIndicator()
//                   : const Empty(),
//               Column(children: [
//                 widget.evnt.sameUser
//                     ? const Empty()
//                     : const SizedBox(height: 20),
//                 sent
//                     ? Row(
//                         crossAxisAlignment: CrossAxisAlignment.end,
//                         mainAxisAlignment: MainAxisAlignment.end,
//                         children: [
//                             Flexible(
//                               flex: 3,
//                               child: SelectionContainer.disabled(
//                                   child: Padding(
//                                 padding: const EdgeInsets.all(4.0),
//                                 child: Tooltip(
//                                     message: fullDate,
//                                     child: Txt.S(
//                                       hour,
//                                     )),
//                               )),
//                             ),
//                             Flexible(
//                               flex: 7,
//                               child: Container(
//                                   margin:
//                                       const EdgeInsets.only(left: 5, right: 20),
//                                   padding: const EdgeInsets.only(
//                                       left: 10, right: 10, top: 5, bottom: 5),
//                                   decoration: BoxDecoration(
//                                     color: sentBackgroundColor,
//                                     borderRadius: BorderRadius.circular(10),
//                                   ),
//                                   child: Provider<DownloadSource>(
//                                       create: (context) =>
//                                           DownloadSource(sourceID),
//                                       child: MarkdownArea(
//                                           msg,
//                                           widget.userNick != widget.nick &&
//                                               msg.contains(widget.userNick)))),
//                             )
//                           ])
//                     : Row(
//                         crossAxisAlignment: CrossAxisAlignment.end,
//                         mainAxisAlignment: MainAxisAlignment.start,
//                         children: [
//                             Flexible(
//                                 flex: 7,
//                                 child: Container(
//                                     margin: const EdgeInsets.only(
//                                         left: 5, right: 20),
//                                     padding: const EdgeInsets.only(
//                                         left: 10, right: 10, top: 5, bottom: 5),
//                                     decoration: BoxDecoration(
//                                       color: receivedBackgroundColor,
//                                       borderRadius: BorderRadius.circular(10),
//                                     ),
//                                     child: Provider<DownloadSource>(
//                                         create: (context) =>
//                                             DownloadSource(sourceID),
//                                         child: MarkdownArea(
//                                             msg,
//                                             widget.userNick != widget.nick &&
//                                                 msg.contains(
//                                                     widget.userNick))))),
//                             Flexible(
//                               flex: 3,
//                               child: SelectionContainer.disabled(
//                                   child: Padding(
//                                 padding: const EdgeInsets.all(4.0),
//                                 child: Tooltip(
//                                     message: fullDate,
//                                     child: Txt.S(
//                                       hour,
//                                     )),
//                               )),
//                             ),
//                           ]),
//                 const SizedBox(height: 5),
//               ])
//             ]));
//   }
// }

class PMW extends StatelessWidget {
  final ChatEventModel evnt;
  final ShowSubMenuCB showSubMenu;
  final ClientModel client;
  final ChatModel chat;
  const PMW(this.evnt, this.showSubMenu, this.client, this.chat, {super.key});

  @override
  Widget build(BuildContext context) {
    var timestamp = 0;
    var event = evnt.event;
    if (event is PM) {
      timestamp =
          evnt.source?.nick == null ? event.timestamp : event.timestamp * 1000;
    }

    openReplyDM(bool isGC, String id) => null;
    // bool isScreenSmall = checkIsScreenSmall(context);
    // if (isScreenSmall) {
    //   return ReceivedSentMobilePM(
    //       evnt,
    //       evnt.source?.nick ?? client.nick,
    //       timestamp,
    //       showSubMenu,
    //       evnt.source?.id ?? "",
    //       client.nick,
    //       false,
    //       client);
    // }

    return ReceivedSentPM(
      evnt,
      evnt.source?.nick ?? client.nick,
      timestamp,
      evnt.source?.id ?? "",
      client.nick,
      false,
      openReplyDM,
      client,
      chat,
    );
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
      {super.key});

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
  const GCUserEventW(this.evnt, {super.key});

  @override
  Widget build(BuildContext context) {
    if (evnt.source != null) {
      return ServerEvent(msg: "${evnt.source!.nick}:  ${evnt.event.msg}");
    } else {
      return ServerEvent(msg: evnt.event.msg);
    }
  }
}

class JoinGCEventW extends StatefulWidget {
  final ChatEventModel event;
  final GCInvitation invite;
  const JoinGCEventW(this.event, this.invite, {super.key});

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
    switch (event.sentState) {
      case CMS_canceled:
        return ServerEvent(msg: "Declined GC invitation to '${invite.name}");
      case CMS_errored:
        return ServerEvent(
            msg: "Unable to join GC ${invite.name}: ${event.sendError}");
      case CMS_sent:
        return ServerEvent(
            msg: "Accepted invitation to join GC '${invite.name}'");
      case CMS_sending:
        return ServerEvent(
            msg: "Accepting invitation to join GC '${invite.name}'");
    }

    return ServerEvent(
        child: Column(children: [
      Txt.S("Received invitation to join GC '${invite.name}'"),
      const SizedBox(height: 20),
      Row(mainAxisAlignment: MainAxisAlignment.center, children: [
        ElevatedButton(onPressed: acceptInvite, child: const Text("Accept")),
        const SizedBox(width: 10),
        CancelButton(onPressed: cancelInvite),
      ]),
    ]));
  }
}

class InflightTipW extends StatefulWidget {
  final InflightTip tip;
  final ChatModel source;
  const InflightTipW(this.tip, this.source, {super.key});

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
    late String msg;

    if (tip.state == ITS_completed) {
      msg =
          "✓ Requesting invoice for ${formatDCR(tip.amount)} to tip ${source.nick}!";
    } else if (tip.state == ITS_errored) {
      msg = "✗ Failed to send tip: ${tip.error}";
    } else if (tip.state == ITS_received) {
      msg = "\$ Received ${tip.amount} DCR from ${source.nick}!";
    } else {
      msg =
          "… Requesting invoice for ${formatDCR(tip.amount)} DCR to tip ${source.nick}...";
    }
    return ServerEvent(msg: msg);
  }
}

class SynthEventW extends StatefulWidget {
  final SynthChatEvent event;
  const SynthEventW(this.event, {super.key});

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
    late String msg;

    if (event.state == SCE_sent) {
      msg = "✓ ${widget.event.msg}";
    } else if (event.state == SCE_errored) {
      msg = "✗ Failed to ${widget.event.msg} - ${event.error}";
    } else if (event.state == SCE_sending) {
      msg = "… ${widget.event.msg}";
    } else if (event.state == SCE_received || event.state == SCE_history) {
      msg = widget.event.msg;
    } else {
      msg = "? unknown state ${event.state} : ${widget.event.msg}";
    }
    return ServerEvent(msg: msg);
  }
}

class UserContentEventW extends StatefulWidget {
  final UserContentList content;
  final ChatModel chat;
  const UserContentEventW(this.content, this.chat, {super.key});

  @override
  State<UserContentEventW> createState() => _UserContentEventWState();
}

class _UserContentEventWState extends State<UserContentEventW> {
  @override
  Widget build(BuildContext context) {
    return Consumer<DownloadsModel>(
        builder: (context, downloads, child) => ServerEvent(
                child: Column(children: [
              const Txt.L("User Content"),
              const SizedBox(height: 20),
              UserContentListW(widget.chat, downloads, widget.content),
            ])));
  }
}

class PostEventW extends StatelessWidget {
  final FeedPostEvent event;
  const PostEventW(this.event, {super.key});

  @override
  Widget build(BuildContext context) {
    return ServerEvent(msg: "Received post '${event.title}'");
  }
}

class PostSubscriptionEventW extends StatefulWidget {
  final PostSubscriptionResult event;
  final ClientModel client;
  const PostSubscriptionEventW(this.event, this.client, {super.key});

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
    return ServerEvent(msg: msg);
  }
}

class PostsSubscriberUpdatedW extends StatelessWidget {
  final PostSubscriberUpdated event;
  const PostsSubscriberUpdatedW(this.event, {super.key});

  @override
  Widget build(BuildContext context) {
    var subTxt = event.subscribed ? "subscribed to" : "unsubscribed from";
    return ServerEvent(msg: "${event.nick} $subTxt the local client's posts.");
  }
}

class ListPostsEventW extends StatefulWidget {
  final ChatEventModel event;
  final ChatModel chat;
  const ListPostsEventW(this.event, this.chat, {super.key});

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
    return ServerEvent(
      child: hasUserPosts
          ? TextButton(
              onPressed: () => FeedScreen.showUsersPosts(context, chat),
              child: const Text("Show user's posts"))
          : Txt.S(msg),
    );
  }
}

class FileDownloadedEventW extends StatelessWidget {
  final FileDownloadedEvent event;
  const FileDownloadedEventW(this.event, {super.key});

  @override
  Widget build(BuildContext context) {
    return ServerEvent(
        child: Row(children: [
      IconButton(
          onPressed: () {
            OpenFilex.open(event.diskPath);
          },
          splashRadius: 20,
          icon: FileIcon(event.diskPath, size: 24)),
      const SizedBox(width: 10),
      Txt.S("Downloaded file ${event.diskPath}"),
    ]));
  }
}

class GCVersionWarnW extends StatelessWidget {
  final GCVersionWarn event;
  const GCVersionWarnW(this.event, {super.key});

  @override
  Widget build(BuildContext context) {
    return Box(
        padding: const EdgeInsets.only(left: 41, top: 5, bottom: 5),
        margin: const EdgeInsets.all(5),
        color: SurfaceColor.errorContainer,
        child: Txt(
          "Received GC definitions with unsupported version ${event.version}. Please update the software to interact in this GC.",
        ));
  }
}

class GCAddedMembersW extends StatelessWidget {
  final GCAddedMembers event;
  final ClientModel client;
  const GCAddedMembersW(this.event, this.client, {super.key});

  @override
  Widget build(BuildContext context) {
    String msg = "Added to GC:\n";
    for (var uid in event.uids) {
      var nick = client.getNick(uid);
      if (nick == "") {
        msg += "Unknown user $uid\n";
      } else {
        msg += "User '$nick'\n";
      }
    }

    return ServerEvent(msg: msg);
  }
}

class GCPartedMemberW extends StatelessWidget {
  final GCMemberParted event;
  final ClientModel client;
  const GCPartedMemberW(this.event, this.client, {super.key});

  @override
  Widget build(BuildContext context) {
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

    return ServerEvent(msg: msg);
  }
}

class GCUpgradedVersionW extends StatelessWidget {
  final GCUpgradedVersion event;
  const GCUpgradedVersionW(this.event, {super.key});

  @override
  Widget build(BuildContext context) {
    String msg =
        "GC Upgraded from version ${event.oldVersion} to ${event.newVersion}";
    return ServerEvent(msg: msg);
  }
}

class GCAdminsChangedW extends StatelessWidget {
  final GCAdminsChanged event;
  final ClientModel client;
  const GCAdminsChangedW(this.event, this.client, {super.key});

  @override
  Widget build(BuildContext context) {
    var srcNick = client.getNick(event.source);
    String msg = "$srcNick modified the GC admins:\n";
    var myID = client.publicID;
    var role = event.changedOwner ? "owner" : "admin";
    if (event.added != null) {
      msg += event.added!.fold("", (prev, e) {
        var nick = e == myID ? "Local client" : client.getNick(e);
        nick = nick == "" ? e : nick;
        return "$prev\n$nick added as $role";
      });
    }
    if (event.removed != null) {
      msg += event.removed!.fold("", (prev, e) {
        var nick = e == myID ? "Local client" : client.getNick(e);
        nick = nick == "" ? e : nick;
        return "$prev\n$nick removed as $role";
      });
    }

    return ServerEvent(msg: msg);
  }
}

class KXSuggestedW extends StatefulWidget {
  final ChatEventModel event;
  final KXSuggested suggest;
  final ClientModel client;
  const KXSuggestedW(this.event, this.suggest, this.client, {super.key});

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

  void cancelSuggestion() async {
    try {
      await Golib.declineSuggestKX(suggest.invitee, suggest.target);
      event.sentState = Suggestion_canceled;
    } catch (exception) {
      event.sendError = "Unable to decline KX suggestion: $exception";
    }
    setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    switch (event.sentState) {
      case Suggestion_accepted:
        return ServerEvent(
            msg:
                "Accepting suggestion to KX from '${suggest.inviteenick}' to '${suggest.targetnick}'");
      case Suggestion_errored:
        return ServerEvent(
            msg:
                "Unable to accept suggestion from  '${suggest.inviteenick}' to '${suggest.targetnick}'");
      case Suggestion_canceled:
        return ServerEvent(
            msg:
                "Canceled suggestion to KX from '${suggest.inviteenick}' to '${suggest.targetnick}'");
      case Suggestion_confirmed:
        return ServerEvent(
            msg:
                "Confirmed suggestion to KX from '${suggest.inviteenick}' to '${suggest.targetnick}'");
    }

    return suggest.alreadyknown
        ? ServerEvent(
            msg:
                "Received already known suggestion to KX from '${suggest.inviteenick}' to '${suggest.targetnick}'")
        : ServerEvent(
            child: Column(children: [
            Txt.S(
                "Received suggestion to KX from '${suggest.inviteenick}' to '${suggest.targetnick}'"),
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
  const TipUserProgressW(this.event, {super.key});

  @override
  Widget build(BuildContext context) {
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

    return ServerEvent(msg: msg);
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
    if (sess.loading) {
      return const ServerEvent(msg: "Requested page");
    } else {
      return ServerEvent(
          child: ElevatedButton(
              onPressed: viewPage, child: const Text("View Page")));
    }
  }
}

class HandshakeStageW extends StatelessWidget {
  final HandshakeStage event;
  const HandshakeStageW(this.event, {super.key});

  @override
  Widget build(BuildContext context) {
    return ServerEvent(
        msg: "Completed 3-way handshake (due to receiving msg ${event.stage})");
  }
}

class ProfileUpdatedW extends StatelessWidget {
  final ProfileUpdated event;
  const ProfileUpdatedW(this.event, {super.key});

  @override
  Widget build(BuildContext context) {
    var fields = event.updatedFields.join(", ");

    return ServerEvent(msg: "Profile updated ($fields)");
  }
}

class RTDTInviteW extends StatefulWidget {
  final InvitedToRTDTSess event;
  final RealtimeChatModel rtc;
  final ChatModel chat;
  const RTDTInviteW(this.event, this.rtc, this.chat, {super.key});

  @override
  State<RTDTInviteW> createState() => _RTDTInviteWState();
}

class _RTDTInviteWState extends State<RTDTInviteW> {
  InvitedToRTDTSess get event => widget.event;
  RealtimeChatModel get rtc => widget.rtc;
  ChatModel get chat => widget.chat;

  bool acceptingInvite = false;
  String? acceptError;
  void acceptInvite() async {
    setState(() => acceptingInvite = true);
    try {
      await rtc.acceptInvite(event);
      setState(() => acceptingInvite = false);
    } catch (exception) {
      setState(() => acceptError = "$exception");
    }
  }

  @override
  Widget build(BuildContext context) {
    print("building invite accept again");
    if (rtc.isInviteCanceled(event)) {
      return const ServerEvent(msg: "Canceled realtime chat invite");
    }

    if (acceptError != null) {
      return ServerEvent(
          msg: "Error accepting realtime chat invite: ${acceptError!}");
    }

    if (acceptingInvite) {
      return const ServerEvent(msg: "Accepting realtime chat invite...");
    }

    if (rtc.isInviteAccepted(event)) {
      return const ServerEvent(msg: "Accepted realtime chat invite");
    }

    return ServerEvent(
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      const Txt.L("Invited to Realtime Chat"),
      Txt.S("RV: ${event.invite.rv}"),
      Txt.S("Size: ${event.invite.size}"),
      Txt.S("Description: ${event.invite.description}"),
      const SizedBox(height: 10),
      Row(mainAxisAlignment: MainAxisAlignment.spaceBetween, children: [
        TextButton.icon(
          onPressed: acceptInvite,
          icon: const Icon(Icons.check),
          label: const Text("Accept"),
        ),
        CancelButton(onPressed: () {
          rtc.cancelInvite(event);
          setState(() {});
        }),
      ]),
    ]));
  }
}

class Event extends StatelessWidget {
  final ChatEventModel event;
  final ChatModel chat;
  final ClientModel client;
  const Event(this.chat, this.event, this.client, {super.key});

  showSubMenu() => client.ui.chatSideMenuActive.chat = chat;
  openReplyDM(bool isGC, String id) => client.setActiveByNick(id, isGC);
  @override
  Widget build(BuildContext context) {
    if (event.event is InstantCallEvent) {
      // return Row(
      //     crossAxisAlignment: CrossAxisAlignment.center,
      //     mainAxisAlignment: MainAxisAlignment.center,
      //     children: [
      //       DateChange(
      //           child: Center(
      //               child: Text(textAlign: TextAlign.center, event.event.msg)))
      //     ]);
      return InstantCall(event: event.event as InstantCallEvent);
    }
    if (event.event is DateChangeEvent) {
      // return Row(
      //     crossAxisAlignment: CrossAxisAlignment.center,
      //     mainAxisAlignment: MainAxisAlignment.center,
      //     children: [
      //       DateChange(
      //           child: Center(
      //               child: Text(textAlign: TextAlign.center, event.event.msg)))
      //     ]);
      return DateChange(event: event.event as DateChangeEvent);
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

    if (event.event is GCKilled) {
      var reason = (event.event as GCKilled).reason;
      return ServerEvent(msg: "GC Killed (reason: \"$reason\")");
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

    if (event.event is InvitedToRTDTSess) {
      return RTDTInviteW(event.event as InvitedToRTDTSess,
          RealtimeChatModel.of(context, listen: false), chat);
    }

    return const Box(
        color: SurfaceColor.errorContainer,
        child: Text("Unknonwn chat event type"));
  }
}
