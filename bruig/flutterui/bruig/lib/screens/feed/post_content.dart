import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/models/menus.dart';
import 'package:bruig/util.dart';
import 'package:bruig/components/md_elements.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/feed.dart';
import 'package:bruig/screens/overview.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:bruig/components/user_context_menu.dart';
import 'package:bruig/theme_manager.dart';

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

class _ReceiveReceipt extends StatelessWidget {
  final ClientModel client;
  final ReceiveReceipt rr;
  const _ReceiveReceipt(this.client, this.rr, {super.key});

  @override
  Widget build(BuildContext context) {
    var nick = client.getNick(rr.user);
    var rrdt =
        DateTime.fromMillisecondsSinceEpoch(rr.serverTime).toIso8601String();

    return Container(
      width: 28,
      margin: const EdgeInsets.only(top: 0, bottom: 0, left: 5),
      child: Tooltip(
          message: "$nick - $rrdt",
          child: UserAvatarFromID(client, rr.user, disableTooltip: true)),
    );
  }
}

typedef ShowingReplyCB = Future<void> Function(String id);

typedef SendReplyCB = Future<void> Function(
    FeedCommentModel comment, String reply);

class _CommentW extends StatefulWidget {
  final FeedPostModel post;
  final FeedCommentModel comment;
  final SendReplyCB sendReply;
  final ClientModel client;
  final ShowingReplyCB showReply;
  const _CommentW(
      this.post, this.comment, this.sendReply, this.client, this.showReply,
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
    widget.showReply(widget.comment.id);
  }

  bool sendingReply = false;
  bool showChildren = true;

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

  void toggleChildren() async {
    setState(() {
      showChildren = !showChildren;
    });
  }

  void requestMediateID() {
    widget.client.requestMediateID(widget.post.summ.from, widget.comment.uid);
  }

  void subscibeToPosts(ChatModel? chat) {
    if (chat != null) {
      chat.subscribeToPosts();
      widget.client.updateUserMenu(chat.id, buildUserChatMenu(chat));
    }
  }

  void chatUpdated() => setState(() {});

  List<ReceiveReceipt>? commentRRs;
  void listReceiveReceipts() async {
    try {
      var rrs = await Golib.listPostCommentReceiveReceipts(
          widget.post.summ.id, widget.comment.id);
      setState(() {
        commentRRs = rrs;
      });
    } catch (exception) {
      showErrorSnackbar(
          context, "Unable to load comment receive receipts: $exception");
    }
  }

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
    var isSubscribed = false;
    var isSubscribing = false;
    if (hasChat) {
      nick = chat.nick;
      isSubscribed = chat.isSubscribed;
      isSubscribing = chat.isSubscribing;
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
    var darkTextColor = theme.indicatorColor;
    var textColor = theme.focusColor;
    var darkAddCommentColor = theme.hoverColor;
    var selectedBackgroundColor = theme.highlightColor;
    var errorColor = theme.errorColor;
    var dividerColor = theme.indicatorColor; // DIVIDER COLOR

    var relayedByMe = widget.post.summ.from == widget.client.publicID;

    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Column(children: [
              const SizedBox(height: 10),
              Row(
                children: [
                  SizedBox(width: widget.comment.level == 0 ? 30 : 0),
                  Container(
                    width: 28,
                    margin: const EdgeInsets.only(top: 0, bottom: 0, left: 5),
                    child: UserOrSelfAvatar(widget.client, chat,
                        postFrom: widget.post.summ.from),
                  ),
                  const SizedBox(width: 6),
                  Row(children: [
                    Text(nick,
                        style: TextStyle(
                            color: hightLightTextColor,
                            fontSize: theme.getSmallFont(context))),
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
                                icon:
                                    const Icon(Icons.connect_without_contact)))
                        : !mine && !isSubscribed
                            ? SizedBox(
                                width: 20,
                                child: IconButton(
                                    padding: const EdgeInsets.all(0),
                                    iconSize: 15,
                                    tooltip: isSubscribing
                                        ? "Requesting Subscription from user..."
                                        : "Subscribe to user's posts",
                                    onPressed: () => !isSubscribing
                                        ? subscibeToPosts(chat)
                                        : null,
                                    icon: isSubscribing
                                        ? SizedBox(
                                            height: 15,
                                            width: 15,
                                            child: CircularProgressIndicator(
                                                strokeWidth: 1,
                                                color: hightLightTextColor))
                                        : Icon(Icons.follow_the_signs_rounded)))
                            : const Empty(),
                    SizedBox(
                        width: 20,
                        child: IconButton(
                            padding: const EdgeInsets.all(0),
                            iconSize: 15,
                            tooltip: "Reply to this comment",
                            onPressed:
                                !sendingReply ? () => replying = true : null,
                            icon: Icon(!sendingReply
                                ? Icons.reply
                                : Icons.hourglass_bottom))),
                    relayedByMe
                        ? SizedBox(
                            width: 20,
                            child: IconButton(
                                padding: const EdgeInsets.all(0),
                                iconSize: 15,
                                tooltip:
                                    "View Receive Receipts for this comment",
                                onPressed: listReceiveReceipts,
                                icon: const Icon(Icons.receipt_long)))
                        : const Empty(),
                    unreadComment
                        ? Row(children: [
                            const SizedBox(width: 10),
                            const Icon(Icons.new_releases_outlined,
                                color: Colors.amber),
                            const SizedBox(width: 10),
                            Text("New Comment",
                                style: TextStyle(
                                  fontStyle: FontStyle.italic,
                                  fontSize: theme.getSmallFont(context),
                                  color: Colors.amber,
                                ))
                          ])
                        : const Empty()
                  ]),
                  strTimestamp != ""
                      ? Expanded(
                          child: Align(
                              alignment: Alignment.centerRight,
                              child: Text(strTimestamp,
                                  style: TextStyle(
                                      fontSize: theme.getSmallFont(context),
                                      color: darkTextColor))))
                      : const Empty(),
                  const SizedBox(width: 10)
                ],
              ),
              Stack(children: [
                Container(
                    decoration: BoxDecoration(
                        border: Border(
                            left: BorderSide(
                                width: 2.0, color: commentBorderColor))),
                    margin: EdgeInsets.only(
                        top: 0,
                        left: widget.comment.level == 0 ? 48 : 18,
                        bottom: 20),
                    padding:
                        const EdgeInsets.only(top: 10, bottom: 10, left: 10),
                    child: Column(children: [
                      SelectionArea(
                          child: Row(
                        children: [
                          Expanded(
                            child: MarkdownArea(widget.comment.comment, false),
                          ),
                        ],
                      )),
                      replying && !sendingReply
                          ? Column(
                              children: [
                                const SizedBox(height: 20),
                                Container(
                                    padding: const EdgeInsets.all(10),
                                    color: darkAddCommentColor,
                                    child: TextField(
                                      minLines: 3,
                                      style: TextStyle(
                                          color: textColor,
                                          fontSize:
                                              theme.getMediumFont(context),
                                          letterSpacing: 0.44),
                                      keyboardType: TextInputType.multiline,
                                      maxLines: null,
                                      onChanged: (v) => reply = v,
                                    )),
                                const SizedBox(height: 20),
                                Row(
                                  children: [
                                    ElevatedButton(
                                        onPressed: sendReply,
                                        child: const Text("Reply")),
                                    const SizedBox(width: 20),
                                    ElevatedButton(
                                      onPressed: () {
                                        replying = false;
                                      },
                                      style: ElevatedButton.styleFrom(
                                          backgroundColor: errorColor),
                                      child: const Text("Cancel"),
                                    )
                                  ],
                                )
                              ],
                            )
                          : const Text(""),
                      commentRRs != null
                          ? const SizedBox(height: 10)
                          : const Empty(),
                      commentRRs != null
                          ? Row(children: [
                              Expanded(
                                  child: Divider(
                                color: dividerColor, //color of divider
                                height: 10, //height spacing of divider
                                thickness: 1, //thickness of divier line
                                indent: 10, //spacing at the start of divider
                                endIndent: 7, //spacing at the end of divider
                              )),
                              Consumer<ThemeNotifier>(
                                  builder: (context, theme, _) => Text(
                                      commentRRs!.isEmpty
                                          ? "No receive receipts"
                                          : "Comment receive receipts",
                                      textAlign: TextAlign.center,
                                      style: TextStyle(
                                          color: darkTextColor,
                                          fontSize:
                                              theme.getSmallFont(context)))),
                              Expanded(
                                  child: Divider(
                                color: dividerColor, //color of divider
                                height: 10, //height spacing of divider
                                thickness: 1, //thickness of divier line
                                indent: 7, //spacing at the start of divider
                                endIndent: 10, //spacing at the end of divider
                              )),
                            ])
                          : const Empty(),
                      if (commentRRs != null && commentRRs!.isNotEmpty)
                        Container(
                            alignment: Alignment.centerLeft,
                            child: Wrap(
                                children: commentRRs!
                                    .map((rr) =>
                                        _ReceiveReceipt(widget.client, rr))
                                    .toList()))
                    ])),
                widget.comment.children.isNotEmpty
                    ? Positioned(
                        left: widget.comment.level == 0 ? 29 : -1,
                        bottom: -10,
                        child: Material(
                            color: textColor.withOpacity(0),
                            child: IconButton(
                                splashRadius: 9,
                                iconSize: 22,
                                hoverColor: selectedBackgroundColor,
                                onPressed: () => toggleChildren(),
                                icon: Icon(
                                    color: darkTextColor,
                                    showChildren
                                        ? Icons.do_disturb_on_outlined
                                        : Icons.add_circle_outline))))
                    : const Empty(),
              ]),
              showChildren && widget.comment.children.isNotEmpty
                  ? Container(
                      decoration: BoxDecoration(
                          border: Border(
                              left: BorderSide(
                                  width: 2.0, color: commentBorderColor))),
                      margin: EdgeInsets.only(
                          top: 0, left: widget.comment.level == 0 ? 48 : 18),
                      padding:
                          const EdgeInsets.only(top: 10, bottom: 10, left: 10),
                      child: Column(children: [
                        ...widget.comment.children.map((e) => _CommentW(
                            widget.post,
                            e,
                            widget.sendReply,
                            widget.client,
                            widget.showReply))
                      ]),
                    )
                  : const Empty(),
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
  bool showingReply = false;
  List<ReceiveReceipt> postRRs = [];

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

    try {
      var summ = widget.args.post.summ;
      if (summ.from == widget.client.publicID) {
        var rrs = await Golib.listPostReceiveReceipts(summ.id);
        setState(() {
          postRRs = rrs;
        });
      }
    } catch (exception) {
      showErrorSnackbar(context, "Unable to load receive receipts: $exception");
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

  bool _replying = false;
  bool get replying => _replying;
  set replying(bool v) {
    setState(() {
      _replying = v;
    });
  }

  void showReply() {
    setState(() {
      replying = true;
    });
  }

  Future<void> showingReplyCB(String id) async {
    setState(() {
      replying = false;
    });
  }

  Future<void> sendReply(FeedCommentModel comment, String reply) async {
    widget.args.post.addNewComment(reply);
    await Golib.commentPost(widget.args.post.summ.from,
        widget.args.post.summ.id, reply, comment.id);
  }

  Future<void> addComment() async {
    replying = false;
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
    setChildCommentsRead(widget.args.post.comments);
    var authorID = widget.args.post.summ.authorID;
    widget.client.getExistingChat(authorID)?.removeListener(authorUpdated);
  }

  void setChildCommentsRead(List<FeedCommentModel> comments) {
    for (int i = 0; i < comments.length; i++) {
      comments[i].unreadComment = false;
      setChildCommentsRead(comments[i].children);
    }
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
    var relayedByMe = relayerID == widget.client.publicID;
    if (!relayedByAuthor) {
      var relayerChat = widget.client.getExistingChat(relayerID);
      if (relayerChat != null) {
        relayer = relayerChat.nick;
      } else if (relayedByMe) {
        relayer = "me";
      } else {
        relayer = relayerID;
      }
    }

    var avatarColor = colorFromNick(authorNick, theme.brightness);
    var darkTextColor = theme.indicatorColor;
    var darkAvatarTextColor = theme.primaryColorDark;
    var lightAvatarTextColor = theme.primaryColorLight;
    var avatarTextColor =
        ThemeData.estimateBrightnessForColor(avatarColor) == Brightness.dark
            ? darkAvatarTextColor
            : lightAvatarTextColor;

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
          Consumer<ThemeNotifier>(
              builder: (context, theme, _) => Text("Comments",
                  textAlign: TextAlign.center,
                  style: TextStyle(
                      color: darkTextColor,
                      fontSize: theme.getSmallFont(context)))),
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
        !replying
            ? Consumer<ThemeNotifier>(
                builder: (context, theme, _) => Container(
                    margin: const EdgeInsets.only(left: 55),
                    child: Row(children: [
                      ElevatedButton(
                          style: ElevatedButton.styleFrom(
                              textStyle: TextStyle(
                                  color: textColor,
                                  fontSize: theme.getSmallFont(context),
                                  letterSpacing: 1),
                              padding: const EdgeInsets.only(
                                  bottom: 4, top: 4, left: 8, right: 8)),
                          onPressed: showReply,
                          child: Text(
                            "Add Comment",
                            style: TextStyle(
                                color: textColor,
                                fontSize: theme.getSmallFont(context),
                                letterSpacing: 1),
                          ))
                    ])))
            : Container(
                padding: const EdgeInsets.only(
                    left: 13, right: 13, top: 11, bottom: 11),
                margin: const EdgeInsets.only(
                    left: 55, right: 108, top: 0, bottom: 0),
                child: Column(children: [
                  Consumer<ThemeNotifier>(
                      builder: (context, theme, _) => Container(
                          padding: const EdgeInsets.all(10),
                          color: darkAddCommentColor,
                          child: TextField(
                            minLines: 3,
                            style: TextStyle(
                                color: textColor,
                                fontSize: theme.getMediumFont(context),
                                letterSpacing: 0.44),
                            controller: newCommentCtrl,
                            keyboardType: TextInputType.multiline,
                            maxLines: null,
                          ))),
                  const SizedBox(height: 20),
                  Row(
                    children: [
                      ElevatedButton(
                          onPressed: addComment,
                          child: const Text("Add Comment")),
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
                ])),
        ...comments.map((e) => _CommentW(
            widget.args.post, e, sendReply, widget.client, showingReplyCB)),
        const SizedBox(height: 20),
        newComments.isNotEmpty
            ? Consumer<ThemeNotifier>(
                builder: (context, theme, _) => Column(children: [
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
                            style: TextStyle(
                                color: darkTextColor,
                                fontSize: theme.getSmallFont(context))),
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
                                  fontSize: theme.getSmallFont(context),
                                  letterSpacing: 1))),
                      ...newComments.map((e) => Container(
                            padding: const EdgeInsets.all(10),
                            child: Text(e,
                                style: TextStyle(
                                    fontSize: theme.getSmallFont(context),
                                    color: textColor)),
                          )),
                    ]))
            : const Empty(),
        const SizedBox(height: 20),
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
        Consumer<ThemeNotifier>(
            builder: (context, theme, _) => Container(
                padding: const EdgeInsets.only(
                    top: 10, bottom: 10, left: 40, right: 40),
                child: Column(children: [
                  Text("""This is a relayed post and cannot be commented on.""",
                      style: TextStyle(
                          color: textColor,
                          fontSize: theme.getSmallFont(context),
                          letterSpacing: 1)),
                  const SizedBox(height: 10),
                  isKXSearchingAuthor
                      ? Text(
                          """Currently attempting to KX search for post author. This may take a long time to complete, as it involves contacting and performing KX with multiple peers.""",
                          style: TextStyle(
                              color: textColor,
                              fontSize: theme.getSmallFont(context),
                              letterSpacing: 1))
                      : !knowsAuthor
                          ? Column(children: [
                              Text(
                                  """In order to comment on the post, the local client needs to KX with the post author and subscribe to their posts. This may be done automatically by using the "KX Search" action. KX search may take a long time to complete, because it depends on remote peers completing KX and referring us to the original author.""",
                                  style: TextStyle(
                                      color: textColor,
                                      fontSize: theme.getSmallFont(context),
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
                                          fontSize: theme.getSmallFont(context),
                                          letterSpacing: 1)),
                                  const SizedBox(height: 10),
                                  ElevatedButton(
                                      onPressed: subscribeAndFetchPost,
                                      child: const Text(
                                          "Subscribe and Fetch Post"))
                                ])
                              : Text(
                                  """Sent subscription attempt. It may take some time until the author responds.""",
                                  style: TextStyle(
                                      color: textColor,
                                      fontSize: theme.getSmallFont(context),
                                      letterSpacing: 1)),
                ]))),
      ]);
    }

    List<Widget> receiveReceiptsWidgets = [];
    if (postRRs.isNotEmpty) {
      receiveReceiptsWidgets = [
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
          Consumer<ThemeNotifier>(
              builder: (context, theme, _) => Text("Receive Receipts",
                  textAlign: TextAlign.center,
                  style: TextStyle(
                      color: darkTextColor,
                      fontSize: theme.getSmallFont(context)))),
          Expanded(
              child: Divider(
            color: dividerColor, //color of divider
            height: 10, //height spacing of divider
            thickness: 1, //thickness of divier line
            indent: 7, //spacing at the start of divider
            endIndent: 10, //spacing at the end of divider
          )),
        ]),
        const SizedBox(height: 10),
        Wrap(
          children:
              postRRs.map((rr) => _ReceiveReceipt(widget.client, rr)).toList(),
        )
      ];
    }

    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    return Container(
        margin: const EdgeInsets.all(1),
        decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(3), color: backgroundColor),
        child: Stack(alignment: Alignment.topLeft, children: [
          ListView(
              padding:
                  const EdgeInsets.only(left: 0, right: 0, top: 10, bottom: 37),
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
                  child: Consumer<ThemeNotifier>(
                      builder: (context, theme, _) => Column(
                            children: [
                              Row(
                                children: [
                                  Container(
                                    width: 28,
                                    margin: const EdgeInsets.only(
                                        top: 0, bottom: 0, left: 5, right: 0),
                                    child: UserOrSelfAvatar(
                                        widget.client, authorChat,
                                        postFrom: widget.args.post.summ.from),
                                  ),
                                  const SizedBox(width: 6),
                                  Text(authorNick,
                                      style: TextStyle(
                                          color: hightLightTextColor,
                                          fontSize:
                                              theme.getSmallFont(context))),
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
                                                  Icons
                                                      .connect_without_contact)))
                                      : const Text(""),
                                  SizedBox(
                                    width: 20,
                                    child: IconButton(
                                      padding: const EdgeInsets.all(0),
                                      iconSize: 15,
                                      tooltip:
                                          "Relay this post to your subscribers",
                                      onPressed: relayPostToAll,
                                      icon: Icon(
                                          color: darkTextColor, Icons.send),
                                    ),
                                  ),
                                  Expanded(
                                      child: Align(
                                          alignment: Alignment.centerRight,
                                          child: Text(
                                              widget.args.post.summ.date
                                                  .toLocal()
                                                  .toIso8601String(),
                                              style: TextStyle(
                                                  fontSize: theme
                                                      .getSmallFont(context),
                                                  color: darkTextColor))))
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
                                                  fontSize: theme
                                                      .getMediumFont(context),
                                                  fontStyle: FontStyle.italic)))
                                    ]),
                              const SizedBox(height: 10),
                              SelectionArea(
                                  child: Container(
                                      padding: const EdgeInsets.all(15),
                                      child: Provider<DownloadSource>(
                                          create: (context) => DownloadSource(
                                              widget.args.post.summ.authorID),
                                          child: MarkdownArea(
                                              markdownData, false)))),
                            ],
                          )),
                ),

                ...commentsWidgets,
                ...receiveReceiptsWidgets,

                // end of post area
              ]),
          isScreenSmall
              ? const Empty()
              : IconButton(
                  alignment: Alignment.topLeft,
                  padding: const EdgeInsets.all(15),
                  iconSize: 15,
                  tooltip: "Go back",
                  onPressed: () => Navigator.of(context).pushReplacementNamed(
                      '/feed',
                      arguments:
                          PageTabs(0, [], null)), //widget.tabChange(0, null),
                  icon: Icon(color: darkTextColor, Icons.close_outlined)),
        ]));
  }
}
