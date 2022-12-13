import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/manage_gc.dart';
import 'package:bruig/screens/feed/feed_posts.dart';
import 'package:bruig/components/md_elements.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:bruig/components/user_content_list.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/downloads.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:intl/intl.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';
import 'package:bruig/components/profile.dart';
import 'package:url_launcher/url_launcher.dart';

class ActiveChat extends StatelessWidget {
  const ActiveChat({Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    return Consumer<ClientModel>(builder: (context, chats, child) {
      var activeHeading = chats.active;
      if (activeHeading == null) return Container();
      var chat = chats.getExistingChat(activeHeading.id);
      if (chat == null) return Container();
      var profile = chats.profile;
      if (profile != null) {
        if (chat.isGC) {
          return ManageGCScreen();
        } else {
          return UserProfile(chats, profile);
        }
      }
      return ActiveChatFor(chat, chats.nick);
    });
  }
}

typedef SendMsg = void Function(String msg);

class EditLine extends StatelessWidget {
  final controller = TextEditingController();
  final SendMsg _send;
  final FocusNode node = FocusNode();
  EditLine(this._send, {Key? key}) : super(key: key);

  void handleKeyPress(event) {
    // TODO: debounce event.
    () async {
      if (event is RawKeyUpEvent) {
        bool modPressed = event.isShiftPressed || event.isControlPressed;
        if (event.data.logicalKey.keyLabel == "Enter" && !modPressed) {
          final val = controller.value;
          final trimmed = val.text.trim();
          controller.value = const TextEditingValue(
              text: "", selection: TextSelection.collapsed(offset: 0));
          _send(trimmed);
        }
      }
    }();
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
          child: TextField(
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
          ),
        ));
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
  const ReceivedSentPM(this.evnt, this.nick, this.timestamp, {Key? key})
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
    var prefix = " ";
    var suffix = "";
    switch (widget.evnt.sentState) {
      case CMS_sending:
        prefix = "… ";
        break;
      case CMS_sent:
        prefix = "✓ ";
        break;
      case CMS_errored:
        prefix = "✗ ";
        suffix = "\n\n${widget.evnt.sendError}";
        break;
      default:
    }
    var now = DateTime.fromMillisecondsSinceEpoch(widget.timestamp);
    var formatter = DateFormat('yyyy-MM-dd HH:mm:ss');
    var date = formatter.format(now);

    var msg = "$prefix${widget.evnt.event.msg}$suffix";

    var theme = Theme.of(context);
    var bgColor = theme.backgroundColor; // CHAT BUBBLE COLOR
    var darkTextColor = theme.indicatorColor; // CHAT BUBBLE BORDER COLOR
    var textColor = theme.focusColor; // MESSAGE TEXT COLOR
    var hightLightTextColor = theme.dividerColor; // NAME TEXT COLOR
    var avatarColor = colorFromNick(widget.nick);
    var avatarTextColor =
        ThemeData.estimateBrightnessForColor(avatarColor) == Brightness.dark
            ? hightLightTextColor
            : darkTextColor;

    return LayoutBuilder(
        builder: (BuildContext context, BoxConstraints constraints) {
      return Row(children: [
        Container(
          width: 28,
          margin: const EdgeInsets.only(top: 0, bottom: 0, left: 5, right: 0),
          child: CircleAvatar(
              backgroundColor: avatarColor,
              child: Text(widget.nick[0].toUpperCase(),
                  style: TextStyle(color: avatarTextColor, fontSize: 20))),
        ),
        Expanded(
            child: Container(
          decoration: BoxDecoration(
              color: bgColor,
              borderRadius: const BorderRadius.all(Radius.elliptical(10, 10))),
          padding: const EdgeInsets.all(5),
          margin: const EdgeInsets.only(top: 5, bottom: 5, left: 10, right: 10),
          alignment: Alignment.centerLeft,
          child:
              Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
            Row(children: [
              Text(
                widget.nick,
                style: TextStyle(
                    fontSize: 12,
                    color: hightLightTextColor, // NAME TEXT COLOR,
                    fontWeight: FontWeight.bold,
                    fontStyle: FontStyle.italic),
              ),
              const SizedBox(width: 10),
              Text(
                date,
                style:
                    TextStyle(fontSize: 9, color: darkTextColor), // DATE COLOR
              )
            ]),
            const SizedBox(height: 10),
            MarkdownBody(
                styleSheet: MarkdownStyleSheet(
                  p: TextStyle(
                      color: textColor,
                      fontSize: 13,
                      fontWeight: FontWeight.w300,
                      letterSpacing: 0.44),
                  h1: TextStyle(color: textColor),
                  h2: TextStyle(color: textColor),
                  h3: TextStyle(color: textColor),
                  h4: TextStyle(color: textColor),
                  h5: TextStyle(color: textColor),
                  h6: TextStyle(color: textColor),
                  em: TextStyle(color: textColor),
                  strong: TextStyle(color: textColor),
                  del: TextStyle(color: textColor),
                  blockquote: TextStyle(color: textColor),
                ),
                selectable: true,
                data: msg,
                builders: {
                  //"video": VideoMarkdownElementBuilder(basedir),
                  "image": ImageMarkdownElementBuilder(),
                  "download": DownloadLinkElementBuilder(),
                },
                onTapLink: (text, url, blah) {
                  launchUrlAwait(url);
                },
                inlineSyntaxes: [
                  //VideoInlineSyntax(),
                  //ImageInlineSyntax()
                  EmbedInlineSyntax(),
                ]),
          ]),
        ))
      ]);
    });
  }
}

class PMW extends StatelessWidget {
  final ChatEventModel evnt;
  final String nick;
  const PMW(this.evnt, this.nick, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var timestamp = 0;
    var event = evnt.event;
    if (event is PM) {
      timestamp =
          evnt.source?.nick == null ? event.timestamp : event.timestamp * 1000;
    }
    return ReceivedSentPM(evnt, evnt.source?.nick ?? nick, timestamp);
  }
}

class GCMW extends StatelessWidget {
  final ChatEventModel evnt;
  final String nick;
  const GCMW(this.evnt, this.nick, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var event = evnt.event;
    var timestamp = 0;
    if (event is GCMsg) {
      timestamp =
          evnt.source?.nick == null ? event.timestamp : event.timestamp * 1000;
    }
    return ReceivedSentPM(evnt, evnt.source?.nick ?? nick, timestamp);
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
                  Expanded(
                      child: Text(posts[index].title,
                          style: TextStyle(color: textColor))),
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
      child = Text("✓ Sent ${tip.amount} DCR to ${source.nick}!",
          style: TextStyle(fontSize: 9, color: textColor));
    } else if (tip.state == ITS_errored) {
      child = Text("✗ Failed to send tip: ${tip.error}",
          style: TextStyle(fontSize: 9, color: textColor));
    } else if (tip.state == ITS_received) {
      child = Text("\$ Received ${tip.amount} DCR from ${source.nick}!",
          style: TextStyle(fontSize: 9, color: textColor));
    } else {
      child = Text("… Sending ${tip.amount} DCR to ${source.nick}...",
          style: TextStyle(fontSize: 9, color: textColor));
    }
    return ServerEvent(child: child);
  }
}

class InflightSubscribeToPostsW extends StatefulWidget {
  final InflightSubscribeToPosts event;
  const InflightSubscribeToPostsW(this.event, {Key? key}) : super(key: key);

  @override
  State<InflightSubscribeToPostsW> createState() =>
      _InflightSubscribeToPostsWState();
}

class _InflightSubscribeToPostsWState extends State<InflightSubscribeToPostsW> {
  @override
  initState() {
    super.initState();
    widget.event.addListener(stateChanged);
  }

  @override
  didUpdateWidget(InflightSubscribeToPostsW oldWidget) {
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
    if (event.state == ISPS_subscribed) {
      child = Text("✓ Subscribed to posts!",
          style: TextStyle(fontSize: 9, color: textColor));
    } else if (event.state == ISPS_errored) {
      child = Text("✗ Failed to subscribe to posts ${event.error}",
          style: TextStyle(fontSize: 9, color: textColor));
    } else if (event.state == ISPS_sending) {
      child = Text("… Subscribing to posts",
          style: TextStyle(fontSize: 9, color: textColor));
    } else {
      child = Text("? unknown state ${event.state}",
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

class FileDownloadedEventW extends StatelessWidget {
  final FileDownloadedEvent event;
  const FileDownloadedEventW(this.event, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    // TODO: add button to open file.
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    var backgroundColor = theme.highlightColor;
    return ServerEvent(
        child: Container(
            decoration: BoxDecoration(
                color: backgroundColor,
                borderRadius: const BorderRadius.all(Radius.circular(5))),
            child: SelectableText("Downloaded file ${event.diskPath}",
                style: TextStyle(fontSize: 9, color: textColor))));
  }
}

class Event extends StatelessWidget {
  final ChatEventModel event;
  final ChatModel chat;
  final String nick;
  final Function() scrollToBottom;
  const Event(this.chat, this.event, this.nick, this.scrollToBottom, {Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    if (event.event is PM) {
      return PMW(event, nick);
    }

    if (event.event is InflightTip) {
      return InflightTipW((event.event as InflightTip), event.source!);
    }

    if (event.event is GCMsg) {
      return GCMW(event, nick);
    }

    if (event.event is GCUserEvent) {
      return GCUserEventW(event);
    }

    if (event.event is InflightSubscribeToPosts) {
      return InflightSubscribeToPostsW(event.event as InflightSubscribeToPosts);
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
  const Messages(this.chat, this.nick, {Key? key}) : super(key: key);

  @override
  State<Messages> createState() => _MessagesState();
}

class _MessagesState extends State<Messages> {
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
        itemBuilder: (context, index) =>
            Event(chat, msgs[index], nick, scrollToBottom));
  }
}

class ActiveChatFor extends StatelessWidget {
  final ChatModel chat;
  final String nick;
  const ActiveChatFor(this.chat, this.nick, {Key? key}) : super(key: key);

  void sendMsg(String msg) {
    chat.sendMsg(msg);
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Expanded(child: Messages(chat, nick)),
        Row(children: [Expanded(child: EditLine(sendMsg))])
      ],
    );
  }
}
