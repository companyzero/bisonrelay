import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/screens/feed/feed_posts.dart';
import 'package:bruig/components/md_elements.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/feed.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';

class PostContentScreenArgs {
  final FeedPostModel post;
  PostContentScreenArgs(this.post);
}

class PostContentScreen extends StatelessWidget {
  final PostContentScreenArgs args;
  final Function tabChange;
  const PostContentScreen(this.args, this.tabChange, {Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    return Consumer<ClientModel>(
        builder: (context, client, child) =>
            _PostContentScreenForArgs(args, client, tabChange));
  }
}

class _PostContentScreenForArgs extends StatefulWidget {
  final PostContentScreenArgs args;
  final ClientModel client;
  final Function tabChange;
  const _PostContentScreenForArgs(this.args, this.client, this.tabChange,
      {Key? key})
      : super(key: key);

  @override
  State<_PostContentScreenForArgs> createState() =>
      _PostContentScreenForArgsState();
}

typedef SendReplyCB = Future<void> Function(
    FeedCommentModel comment, String reply);

class _CommentW extends StatefulWidget {
  final FeedPostModel post;
  final FeedCommentModel comment;
  final SendReplyCB sendReply;
  final ClientModel client;
  const _CommentW(this.post, this.comment, this.sendReply, this.client,
      {Key? key})
      : super(key: key);

  @override
  State<_CommentW> createState() => _CommentWState();
}

class _CommentWState extends State<_CommentW> {
  String reply = "";

  bool _replying = false;
  bool get replying => _replying;
  set replying(bool v) {
    setState(() {
      _replying = v;
    });
  }

  bool sendingReply = false;

  void sendReply() async {
    replying = false;
    setState(() {
      sendingReply = true;
    });
    try {
      await widget.sendReply(widget.comment, reply);
    } finally {
      setState(() {
        sendingReply = false;
      });
    }
  }

  void requestMediateID() {
    widget.client.requestMediateID(widget.post.summ.from, widget.comment.uid);
  }

  @override
  Widget build(BuildContext context) {
    var nick = widget.comment.nick;
    var timestamp = widget.comment.timestamp;
    var chat = widget.client.getExistingChat(widget.comment.uid);
    var hasChat = chat != null;
    if (chat != null) {
      nick = chat.nick;
    }

    var intTimestamp = 0;
    var strTimestamp = "";
    if (timestamp != "") {
      intTimestamp = int.parse(timestamp, radix: 16);
      strTimestamp = DateTime.fromMillisecondsSinceEpoch(intTimestamp * 1000)
          .toIso8601String();
    }

    var mine = widget.comment.uid == widget.client.publicID;
    var kxing = widget.client.requestedMediateID(widget.comment.uid);

    var theme = Theme.of(context);
    var hightLightTextColor = theme.dividerColor;
    var textColor = theme.focusColor;
    var backgroundColor = theme.highlightColor;
    var avatarColor = colorFromNick(nick);
    var darkTextColor = theme.indicatorColor;
    var avatarTextColor =
        ThemeData.estimateBrightnessForColor(avatarColor) == Brightness.dark
            ? hightLightTextColor
            : darkTextColor;

    return Container(
        decoration: BoxDecoration(
            border: Border(
          left: BorderSide(
              width: 2.0,
              color: widget.comment.level != 0
                  ? const Color(0xFF3A384B)
                  : backgroundColor),
        )),
        margin: EdgeInsets.only(
            top: 5, left: 144 + widget.comment.level * 20, right: 108),
        padding: const EdgeInsets.all(10),
        child: Column(children: [
          Row(
            children: [
              Container(
                width: 28,
                margin:
                    const EdgeInsets.only(top: 0, bottom: 0, left: 5, right: 0),
                child: CircleAvatar(
                    backgroundColor: avatarColor,
                    child: Text(nick[0].toUpperCase(),
                        style:
                            TextStyle(color: avatarTextColor, fontSize: 20))),
              ),
              const SizedBox(width: 6),
              Row(children: [
                Text(nick,
                    style: TextStyle(color: hightLightTextColor, fontSize: 11)),
                const SizedBox(width: 8),
                !mine && !hasChat && !kxing
                    ? SizedBox(
                        width: 20,
                        child: IconButton(
                            padding: const EdgeInsets.all(0),
                            iconSize: 15,
                            tooltip:
                                "Attempt to KX with the author of this comment",
                            onPressed: requestMediateID,
                            icon: const Icon(Icons.connect_without_contact)))
                    : const Text(""),
                SizedBox(
                    width: 20,
                    child: IconButton(
                        padding: const EdgeInsets.all(0),
                        iconSize: 15,
                        tooltip: "Reply to this comment",
                        onPressed: !sendingReply ? () => replying = true : null,
                        icon: Icon(!sendingReply
                            ? Icons.reply
                            : Icons.hourglass_bottom))),
              ]),
              strTimestamp != ""
                  ? Expanded(
                      child: Align(
                          alignment: Alignment.centerRight,
                          child: Text(strTimestamp,
                              style: TextStyle(
                                  fontSize: 9, color: darkTextColor))))
                  : const Empty()
            ],
          ),
          Row(
            children: [
              Expanded(
                child: MarkdownArea(widget.comment.comment, false),
              ),
            ],
          ),
          replying && !sendingReply
              ? Column(
                  children: [
                    TextField(
                      keyboardType: TextInputType.multiline,
                      maxLines: null,
                      onChanged: (v) => reply = v,
                    ),
                    const SizedBox(height: 20),
                    Row(
                      children: [
                        ElevatedButton(
                            onPressed: sendReply, child: const Text("Reply")),
                        const SizedBox(width: 20),
                        ElevatedButton(
                          onPressed: () {
                            replying = false;
                          },
                          style: ElevatedButton.styleFrom(
                              backgroundColor: theme.errorColor),
                          child: const Text("Cancel"),
                        )
                      ],
                    )
                  ],
                )
              : const Text(""),
        ]));
  }
}

class _PostContentScreenForArgsState extends State<_PostContentScreenForArgs> {
  bool loading = false;
  String markdownData = "";
  Iterable<FeedCommentModel> comments = [];
  TextEditingController newCommentCtrl = TextEditingController();

  void loadContent() async {
    setState(() {
      loading = true;
      markdownData = "";
    });

    try {
      await widget.args.post.readPost();
      await widget.args.post.readComments();
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to load content: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  void postUpdated() {
    setState(() {
      markdownData = widget.args.post.content;
      comments = widget.args.post.comments;
    });
  }

  Future<void> sendReply(FeedCommentModel comment, String reply) async {
    widget.args.post.addNewComment(reply);
    await Golib.commentPost(widget.args.post.summ.from,
        widget.args.post.summ.id, reply, comment.id);
  }

  Future<void> addComment() async {
    var newComment = newCommentCtrl.text;
    setState(() {
      newCommentCtrl.clear();
    });
    widget.args.post.addNewComment(newComment);
    await Golib.commentPost(
        widget.args.post.summ.from, widget.args.post.summ.id, newComment, null);
  }

  void kxSearchAuthor() async {
    try {
      await Golib.kxSearchPostAuthor(
          widget.args.post.summ.from, widget.args.post.summ.id);
    } catch (exception) {
      if (!mounted) {
        return;
      }

      showErrorSnackbar(context, "Unable to KX search post author: $exception");
    }
  }

  void relayPostToAll() {
    Golib.relayPostToAll(widget.args.post.summ.from, widget.args.post.summ.id);
  }

  @override
  void initState() {
    super.initState();
    widget.args.post.addListener(postUpdated);
    loadContent();
  }

  @override
  void didUpdateWidget(_PostContentScreenForArgs oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.args.post.removeListener(postUpdated);
    widget.args.post.addListener(postUpdated);
  }

  @override
  void dispose() {
    widget.args.post.removeListener(postUpdated);
    super.dispose();
  }

  Future<void> launchUrlAwait(url) async {
    if (!await launchUrl(Uri.parse(url))) {
      throw 'Could not launch $url';
    }
  }

  @override
  Widget build(BuildContext context) {
    if (loading) {
      return const Center(
        child: Text("Loading..."),
      );
    }

    var theme = Theme.of(context);

    var inverseBackgroundColor = theme.focusColor;
    var hightLightTextColor = theme.dividerColor; // NAME TEXT COLOR
    var dividerColor = theme.indicatorColor; // DIVIDER COLOR
    var textColor = theme.focusColor;
    var backgroundColor = theme.backgroundColor;
    var postBackgroundColor = theme.highlightColor;
    var darkAddCommentColor = theme.hoverColor;

    var pid = widget.args.post.summ.id;
    var authorNick = widget.args.post.summ.authorNick;
    var authorID = widget.args.post.summ.authorID;
    var relayer = "";
    var myPost = authorID == widget.client.publicID;
    var authorChat = widget.client.getExistingChat(authorID);
    var hasChat = authorChat != null;
    if (myPost) {
      authorNick = "me";
    }
    if (authorChat != null) {
      authorNick = authorChat.nick;
    }
    if (authorNick == "") {
      authorNick = "[$authorID]";
    }

    var relayerID = widget.args.post.summ.from;
    if (relayerID != authorID) {
      var relayerChat = widget.client.getExistingChat(relayerID);
      if (relayerChat != null) {
        relayer = relayerChat.nick;
      } else if (relayerID == widget.client.publicID) {
        relayer = "me";
      } else {
        relayer = relayerID;
      }
    }

    var avatarColor = colorFromNick(authorNick);
    var darkTextColor = theme.indicatorColor;
    var avatarTextColor =
        ThemeData.estimateBrightnessForColor(avatarColor) == Brightness.dark
            ? hightLightTextColor
            : darkTextColor;

    List<Widget> newCommentsW = [];
    var newComments = widget.args.post.newComments;
    if (newComments.isNotEmpty) {
      newCommentsW = [
        const SizedBox(height: 20),
        Row(children: [
          Expanded(
              child: Divider(
            color: dividerColor, //color of divider
            height: 10, //height spacing of divider
            thickness: 1, //thickness of divier line
            indent: 10, //spacing at the start of divider
            endIndent: 7, //spacing at the end of divider
          )),
          Text("Unreplicated Comments",
              textAlign: TextAlign.center,
              style: TextStyle(color: darkTextColor, fontSize: 11)),
          Expanded(
              child: Divider(
            color: dividerColor, //color of divider
            height: 10, //height spacing of divider
            thickness: 1, //thickness of divier line
            indent: 7, //spacing at the start of divider
            endIndent: 10, //spacing at the end of divider
          )),
        ]),
        ...newComments.map((e) => Container(
              padding: const EdgeInsets.all(10),
              child: Text(e, style: TextStyle(fontSize: 11, color: textColor)),
            )),
        const SizedBox(height: 20),
      ];
    }

    return Container(
        margin: const EdgeInsets.all(1),
        decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(3), color: backgroundColor),
        child: Stack(alignment: Alignment.topLeft, children: [
          ListView(
              padding:
                  const EdgeInsets.only(left: 0, right: 0, top: 44, bottom: 37),
              children: [
                Container(
                  // Post area
                  margin: const EdgeInsets.only(
                      left: 114, right: 108, top: 0, bottom: 0),
                  decoration: BoxDecoration(
                      borderRadius: BorderRadius.circular(3),
                      color: postBackgroundColor),
                  padding: const EdgeInsets.all(16),
                  child: Column(
                    children: [
                      Row(
                        children: [
                          Container(
                            width: 28,
                            margin: const EdgeInsets.only(
                                top: 0, bottom: 0, left: 5, right: 0),
                            child: CircleAvatar(
                                backgroundColor: avatarColor,
                                child: Text(authorNick[0].toUpperCase(),
                                    style: TextStyle(
                                        color: avatarTextColor, fontSize: 20))),
                          ),
                          const SizedBox(width: 6),
                          Text(authorNick,
                              style: TextStyle(
                                  color: hightLightTextColor, fontSize: 11)),
                          const SizedBox(width: 8),
                          !myPost && !hasChat
                              ? SizedBox(
                                  width: 20,
                                  child: IconButton(
                                      padding: const EdgeInsets.all(0),
                                      iconSize: 15,
                                      tooltip:
                                          "Attempt to KX with the author of this comment",
                                      onPressed: kxSearchAuthor,
                                      icon: Icon(
                                          color: darkTextColor,
                                          Icons.connect_without_contact)))
                              : const Text(""),
                          SizedBox(
                            width: 20,
                            child: IconButton(
                              padding: const EdgeInsets.all(0),
                              iconSize: 15,
                              tooltip: "Relay this post to your subscribers",
                              onPressed: relayPostToAll,
                              icon: Icon(color: darkTextColor, Icons.send),
                            ),
                          ),
                          Expanded(
                              child: Align(
                                  alignment: Alignment.centerRight,
                                  child: Text(
                                      widget.args.post.summ.date
                                          .toIso8601String(),
                                      style: TextStyle(
                                          fontSize: 9, color: darkTextColor))))
                        ],
                      ),
                      const SizedBox(height: 10),
                      relayer == ""
                          ? const Empty()
                          : Row(children: [
                              Expanded(
                                  child: Text("Relayed by $relayer",
                                      style: TextStyle(
                                          color: textColor,
                                          fontSize: 16,
                                          fontStyle: FontStyle.italic)))
                            ]),
                      const SizedBox(height: 20),
                      Container(
                          padding: const EdgeInsets.all(15),
                          child: Provider<DownloadSource>(
                              create: (context) => DownloadSource(
                                  widget.args.post.summ.authorID),
                              child: MarkdownArea(markdownData, false))),
                    ],
                  ),
                ),
                const SizedBox(height: 20),
                Row(children: [
                  Expanded(
                      child: Divider(
                    color: dividerColor, //color of divider
                    height: 10, //height spacing of divider
                    thickness: 1, //thickness of divier line
                    indent: 10, //spacing at the start of divider
                    endIndent: 7, //spacing at the end of divider
                  )),
                  Text("Comments",
                      textAlign: TextAlign.center,
                      style: TextStyle(color: darkTextColor, fontSize: 11)),
                  Expanded(
                      child: Divider(
                    color: dividerColor, //color of divider
                    height: 10, //height spacing of divider
                    thickness: 1, //thickness of divier line
                    indent: 7, //spacing at the start of divider
                    endIndent: 10, //spacing at the end of divider
                  )),
                ]),

                const SizedBox(height: 20),
                ...comments.map((e) =>
                    _CommentW(widget.args.post, e, sendReply, widget.client)),
                const SizedBox(height: 20),
                ...newCommentsW,
                Container(
                    color: darkAddCommentColor,
                    padding: const EdgeInsets.only(
                        left: 13, right: 13, top: 11, bottom: 11),
                    margin: const EdgeInsets.only(
                        left: 114, right: 108, top: 0, bottom: 0),
                    child: TextField(
                      minLines: 3,
                      style: TextStyle(
                          color: textColor, fontSize: 13, letterSpacing: 0.44),
                      controller: newCommentCtrl,
                      keyboardType: TextInputType.multiline,
                      maxLines: null,
                    )),
                const SizedBox(height: 20),
                Row(children: [
                  const SizedBox(width: 114),
                  ElevatedButton(
                      style: ElevatedButton.styleFrom(
                          textStyle: TextStyle(
                              color: textColor, fontSize: 11, letterSpacing: 1),
                          padding: const EdgeInsets.only(
                              bottom: 4, top: 4, left: 8, right: 8)),
                      onPressed: addComment,
                      child: Text(
                        "Add Comment",
                        style: TextStyle(
                            color: textColor, fontSize: 11, letterSpacing: 1),
                      ))
                ]),

                // end of post area
              ]),
          IconButton(
              alignment: Alignment.topLeft,
              padding: const EdgeInsets.all(15),
              iconSize: 15,
              tooltip: "Go back",
              onPressed: () =>
                  widget.tabChange(0, null), //widget.tabChange(0, null),
              icon: Icon(color: darkTextColor, Icons.close_outlined)),
        ]));
  }
}
