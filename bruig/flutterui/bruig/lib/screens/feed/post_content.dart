import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/util.dart';
import 'package:bruig/components/md_elements.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/feed.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:bruig/components/user_context_menu.dart';

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
    return Consumer2<ClientModel, FeedModel>(
        builder: (context, client, feed, child) =>
            _PostContentScreenForArgs(args, client, tabChange, feed));
  }
}

class _PostContentScreenForArgs extends StatefulWidget {
  final PostContentScreenArgs args;
  final ClientModel client;
  final Function tabChange;
  final FeedModel feed;
  const _PostContentScreenForArgs(
      this.args, this.client, this.tabChange, this.feed,
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

  void chatUpdated() => setState(() {});

  @override
  void initState() {
    super.initState();
    widget.client.getExistingChat(widget.comment.uid)?.addListener(chatUpdated);
  }

  @override
  void didUpdateWidget(_CommentW oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.client
        .getExistingChat(widget.comment.uid)
        ?.removeListener(chatUpdated);
    widget.client.getExistingChat(widget.comment.uid)?.addListener(chatUpdated);
  }

  @override
  void dispose() {
    widget.client
        .getExistingChat(widget.comment.uid)
        ?.removeListener(chatUpdated);
    super.dispose();
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
    var unreadComment = widget.comment.unreadComment;
    var theme = Theme.of(context);
    var hightLightTextColor = theme.dividerColor;
    var commentBorderColor = theme.dialogBackgroundColor;
    var avatarColor = colorFromNick(nick);
    var darkTextColor = theme.indicatorColor;
    var avatarTextColor =
        ThemeData.estimateBrightnessForColor(avatarColor) == Brightness.dark
            ? hightLightTextColor
            : darkTextColor;

    return Container(
        decoration: BoxDecoration(
            border: Border(
                left: widget.comment.level != 0
                    ? BorderSide(width: 2.0, color: commentBorderColor)
                    : BorderSide.none)),
        margin: EdgeInsets.only(
            top: 5,
            left:
                widget.comment.level != 0 ? 50 + widget.comment.level * 25 : 50,
            right: 50),
        padding: const EdgeInsets.all(10),
        child: Column(children: [
          Row(
            children: [
              Container(
                width: 28,
                margin:
                    const EdgeInsets.only(top: 0, bottom: 0, left: 5, right: 0),
                child: UserContextMenu(
                  client: widget.client,
                  targetUserChat: chat,
                  targetUserId: widget.comment.uid,
                  disabled: mine,
                  postFrom: widget.post.summ.from,
                  child: CircleAvatar(
                      backgroundColor: avatarColor,
                      child: Text(nick[0].toUpperCase(),
                          style:
                              TextStyle(color: avatarTextColor, fontSize: 20))),
                ),
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
                unreadComment
                    ? Row(children: const [
                        SizedBox(width: 10),
                        Icon(Icons.new_releases_outlined, color: Colors.amber),
                        SizedBox(width: 10),
                        Text("New Comment",
                            style: TextStyle(
                              fontStyle: FontStyle.italic,
                              fontSize: 12,
                              color: Colors.amber,
                            ))
                      ])
                    : const Empty(),
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
  bool knowsAuthor = false;
  bool isKXSearchingAuthor = false;
  bool sentSubscribeAttempt = false;

  void loadContent() async {
    setState(() {
      loading = true;
      markdownData = "";
    });

    try {
      await widget.args.post.readPost();
      //await widget.args.post.readComments();

      bool newIsKxSearching = false;
      var summ = widget.args.post.summ;
      var newKnowsAuthor = widget.client.getExistingChat(summ.authorID) != null;
      try {
        if (summ.authorID != summ.from && !newKnowsAuthor) {
          var kxSearch = await Golib.getKXSearch(summ.authorID);
          newIsKxSearching = kxSearch.target == summ.authorID;
        }
      } catch (exception) {
        // ignore as it means we're not KX searching the author.
      }
      setState(() {
        knowsAuthor = newKnowsAuthor;
        isKXSearchingAuthor = newIsKxSearching;
      });
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to load content: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  void postUpdated() {
    if (widget.args.post.replacedByAuthorVersion) {
      // Relayed post replaced by the author version. Switch to the author version.
      var summ = widget.args.post.summ;
      var post = widget.feed.getPost(summ.authorID, summ.id);
      if (post != null) {
        (() async {
          await post.readPost();
          //await post.readComments();
          widget.tabChange(0, PostContentScreenArgs(post));
        })();
      }
    }
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
      setState(() {
        isKXSearchingAuthor = true;
      });
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

  void authorUpdated() => setState(() {});

  @override
  void initState() {
    super.initState();
    widget.args.post.addListener(postUpdated);
    var authorID = widget.args.post.summ.authorID;
    widget.client.getExistingChat(authorID)?.addListener(authorUpdated);
    loadContent();
  }

  @override
  void didUpdateWidget(_PostContentScreenForArgs oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.args.post.removeListener(postUpdated);
    widget.args.post.addListener(postUpdated);
    var authorID = widget.args.post.summ.authorID;
    oldWidget.client.getExistingChat(authorID)?.removeListener(authorUpdated);
    widget.client.getExistingChat(authorID)?.addListener(authorUpdated);
  }

  @override
  void dispose() {
    super.dispose();
    widget.args.post.removeListener(postUpdated);
    for (int i = 0; i < widget.args.post.comments.length; i++) {
      if (widget.args.post.comments[i].unreadComment) {
        widget.args.post.comments[i].unreadComment = false;
      }
    }
    var authorID = widget.args.post.summ.authorID;
    widget.client.getExistingChat(authorID)?.removeListener(authorUpdated);
  }

  Future<void> launchUrlAwait(url) async {
    if (!await launchUrl(Uri.parse(url))) {
      throw 'Could not launch $url';
    }
  }

  Future<void> subscribeAndFetchPost() async {
    try {
      var summ = widget.args.post.summ;
      await Golib.subscribeToPostsAndFetch(summ.authorID, summ.id);
      setState(() => sentSubscribeAttempt = true);
    } catch (exception) {
      showErrorSnackbar(context, "Unable to subscribe to posts: $exception");
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

    var hightLightTextColor = theme.dividerColor; // NAME TEXT COLOR
    var dividerColor = theme.indicatorColor; // DIVIDER COLOR
    var textColor = theme.focusColor;
    var backgroundColor = theme.backgroundColor;
    var postBackgroundColor = theme.highlightColor;
    var darkAddCommentColor = theme.hoverColor;

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
    var relayedByAuthor = relayerID == authorID;
    if (!relayedByAuthor) {
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

    List<Widget> commentsWidgets = [];
    var newComments = widget.args.post.newComments;
    if (relayedByAuthor) {
      commentsWidgets.addAll([
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
        ...comments.map(
            (e) => _CommentW(widget.args.post, e, sendReply, widget.client)),
        const SizedBox(height: 20),
        newComments.isNotEmpty
            ? Column(children: [
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
                Container(
                    padding: const EdgeInsets.symmetric(
                        vertical: 10, horizontal: 40),
                    child: Text(
                        """Unreplicated comments are those that have been sent to the post's relayer for replication but which the relayer has not yet sent back to the local client. Comment replication requires the remote user to be online so it may take some time until the comment is received back.""",
                        style: TextStyle(
                            color: hightLightTextColor,
                            fontSize: 11,
                            letterSpacing: 1))),
                ...newComments.map((e) => Container(
                      padding: const EdgeInsets.all(10),
                      child: Text(e,
                          style: TextStyle(fontSize: 11, color: textColor)),
                    )),
              ])
            : const Empty(),
        const SizedBox(height: 20),
        Container(
            color: darkAddCommentColor,
            padding:
                const EdgeInsets.only(left: 13, right: 13, top: 11, bottom: 11),
            margin:
                const EdgeInsets.only(left: 114, right: 108, top: 0, bottom: 0),
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
                style:
                    TextStyle(color: textColor, fontSize: 11, letterSpacing: 1),
              ))
        ]),
      ]);
    } else {
      commentsWidgets.addAll([
        Row(children: [
          Expanded(
              child: Divider(
            color: dividerColor, //color of divider
            height: 10, //height spacing of divider
            thickness: 1, //thickness of divier line
            indent: 10, //spacing at the start of divider
            endIndent: 7, //spacing at the end of divider
          )),
        ]),
        Container(
            padding:
                const EdgeInsets.only(top: 10, bottom: 10, left: 40, right: 40),
            child: Column(children: [
              Text("""This is a relayed post and cannot be commented on.""",
                  style: TextStyle(
                      color: textColor, fontSize: 11, letterSpacing: 1)),
              const SizedBox(height: 10),
              isKXSearchingAuthor
                  ? Text(
                      """Currently attempting to KX search for post author. This may take a long time to complete, as it involves contacting and performing KX with multiple peers.""",
                      style: TextStyle(
                          color: textColor, fontSize: 11, letterSpacing: 1))
                  : !knowsAuthor
                      ? Column(children: [
                          Text(
                              """In order to comment on the post, the local client needs to KX with the post author and subscribe to their posts. This may be done automatically by using the "KX Search" action. KX search may take a long time to complete, because it depends on remote peers completing KX and referring us to the original author.""",
                              style: TextStyle(
                                  color: textColor,
                                  fontSize: 11,
                                  letterSpacing: 1)),
                          const SizedBox(height: 10),
                          ElevatedButton(
                              onPressed: kxSearchAuthor,
                              child: const Text("Start KX Search Attempt"))
                        ])
                      : !sentSubscribeAttempt
                          ? Column(children: [
                              Text(
                                  """In order to comment on the post, the local client needs subscribe to the author's posts and then fetch this post. The process to do this can be started automatically, but it may take some time until the author responds.""",
                                  style: TextStyle(
                                      color: textColor,
                                      fontSize: 11,
                                      letterSpacing: 1)),
                              const SizedBox(height: 10),
                              ElevatedButton(
                                  onPressed: subscribeAndFetchPost,
                                  child: const Text("Subscribe and Fetch Post"))
                            ])
                          : Text(
                              """Sent subscription attempt. It may take some time until the author responds.""",
                              style: TextStyle(
                                  color: textColor,
                                  fontSize: 11,
                                  letterSpacing: 1)),
            ])),
      ]);
    }

    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
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
                  margin: isScreenSmall
                      ? const EdgeInsets.only(
                          left: 19, right: 10, top: 0, bottom: 0)
                      : const EdgeInsets.only(
                          left: 50, right: 50, top: 0, bottom: 0),
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
                            child: UserContextMenu(
                              client: widget.client,
                              targetUserChat: authorChat,
                              disabled: myPost,
                              postFrom: widget.args.post.summ.from,
                              child: CircleAvatar(
                                  backgroundColor: avatarColor,
                                  child: Text(authorNick[0].toUpperCase(),
                                      style: TextStyle(
                                          color: avatarTextColor,
                                          fontSize: 20))),
                            ),
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
                      const SizedBox(height: 10),
                      Container(
                          padding: const EdgeInsets.all(15),
                          child: Provider<DownloadSource>(
                              create: (context) => DownloadSource(
                                  widget.args.post.summ.authorID),
                              child: MarkdownArea(markdownData, false))),
                    ],
                  ),
                ),

                ...commentsWidgets,

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
