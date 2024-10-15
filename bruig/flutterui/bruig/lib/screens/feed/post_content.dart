import 'package:bruig/components/feed/comment_input.dart';
import 'package:bruig/components/typing_emoji_panel.dart';
import 'package:bruig/components/containers.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/icons.dart';
import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/md_elements.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/feed.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/overview.dart';
import 'package:bruig/screens/chats.dart';
import 'package:bruig/util.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:bruig/theme_manager.dart';
import 'package:bruig/models/emoji.dart';

class PostContentScreenArgs {
  final FeedPostModel post;
  PostContentScreenArgs(this.post);
}

class PostContentScreen extends StatelessWidget {
  final PostContentScreenArgs args;
  final Function tabChange;
  final TypingEmojiSelModel typingEmoji;
  const PostContentScreen(this.args, this.tabChange, this.typingEmoji,
      {super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer2<ClientModel, FeedModel>(
        builder: (context, client, feed, child) => _PostContentScreenForArgs(
            args, client, tabChange, feed, typingEmoji));
  }
}

class _PostContentScreenForArgs extends StatefulWidget {
  final PostContentScreenArgs args;
  final ClientModel client;
  final Function tabChange;
  final FeedModel feed;
  final TypingEmojiSelModel typingEmoji;
  const _PostContentScreenForArgs(
      this.args, this.client, this.tabChange, this.feed, this.typingEmoji);

  @override
  State<_PostContentScreenForArgs> createState() =>
      _PostContentScreenForArgsState();
}

class _ReceiveReceipt extends StatelessWidget {
  final ClientModel client;
  final ReceiveReceipt rr;
  const _ReceiveReceipt(this.client, this.rr);

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
  final bool canComment;
  final CustomInputFocusNode inputFocusNode;
  const _CommentW(this.post, this.comment, this.sendReply, this.client,
      this.showReply, this.canComment, this.inputFocusNode);

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

  void sendReply(String msg) async {
    replying = false;
    setState(() {
      sendingReply = true;
    });
    try {
      await widget.sendReply(widget.comment, msg);
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
          this, "Unable to load comment receive receipts: $exception");
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
    var fullStrTimestamp = "";
    if (timestamp != "") {
      intTimestamp = int.parse(timestamp, radix: 16);
      var ts = DateTime.fromMillisecondsSinceEpoch(intTimestamp * 1000);
      strTimestamp = formatTerseTime(ts);
      fullStrTimestamp = ts.toIso8601String();
    }

    var mine = widget.comment.uid == widget.client.publicID;
    var kxing = widget.client.requestedMediateID(widget.comment.uid);
    var unreadComment = widget.comment.unreadComment;

    var relayedByMe = widget.post.summ.from == widget.client.publicID;

    var theme = Provider.of<ThemeNotifier>(context, listen: false);
    var isScreenSmall = checkIsScreenSmall(context);

    return Column(children: [
      const SizedBox(height: 10),
      Row(children: [
        Container(
          width: 28,
          margin:
              EdgeInsets.only(top: 0, bottom: 0, left: isScreenSmall ? 0 : 2),
          child: UserAvatarFromID(widget.client, widget.comment.uid,
              postFrom: widget.post.summ.from,
              showChatSideMenuOnTap: true,
              nick: nick),
        ),
        const SizedBox(width: 6),
        Row(children: [
          Txt.S(nick),
          const SizedBox(width: 8),
          !mine && !hasChat && !kxing
              ? SizedBox(
                  width: 20,
                  child: IconButton(
                      padding: const EdgeInsets.all(0),
                      iconSize: 15,
                      tooltip: "Attempt to KX with the author of this comment",
                      onPressed: requestMediateID,
                      icon: const Icon(Icons.connect_without_contact)))
              : !mine && !isSubscribed
                  ? SizedBox(
                      width: 20,
                      child: IconButton(
                          padding: const EdgeInsets.all(0),
                          iconSize: 15,
                          tooltip: isSubscribing
                              ? "Requesting Subscription from user..."
                              : "Subscribe to user's posts",
                          onPressed: () =>
                              !isSubscribing ? subscibeToPosts(chat) : null,
                          icon: isSubscribing
                              ? const SizedBox(
                                  height: 15,
                                  width: 15,
                                  child:
                                      CircularProgressIndicator(strokeWidth: 1))
                              : const Icon(Icons.follow_the_signs_rounded)))
                  : const Empty(),
          if (widget.canComment)
            SizedBox(
                width: 20,
                child: IconButton(
                    padding: const EdgeInsets.all(0),
                    iconSize: 15,
                    tooltip: "Reply to this comment",
                    onPressed: !sendingReply ? () => replying = true : null,
                    icon: Icon(
                        !sendingReply ? Icons.reply : Icons.hourglass_bottom))),
          relayedByMe
              ? SizedBox(
                  width: 20,
                  child: IconButton(
                      padding: const EdgeInsets.all(0),
                      iconSize: 15,
                      tooltip: "View Receive Receipts for this comment",
                      onPressed: listReceiveReceipts,
                      icon: const Icon(Icons.receipt_long)))
              : const Empty(),
          unreadComment
              ? const Row(children: [
                  SizedBox(width: 10),
                  Icon(Icons.new_releases_outlined, color: Colors.amber),
                  SizedBox(width: 10),
                  Txt.S("New Comment",
                      style: TextStyle(
                        fontStyle: FontStyle.italic,
                        color: Colors.amber,
                      ))
                ])
              : const Empty()
        ]),
        strTimestamp != ""
            ? Expanded(
                child: Align(
                    alignment: Alignment.centerRight,
                    child: Tooltip(
                        message: fullStrTimestamp, child: Txt.S(strTimestamp))))
            : const Empty(),
        const SizedBox(width: 10)
      ]), // End of comment header Row.
      Stack(children: [
        Container(
            margin: EdgeInsets.only(
                top: 0, left: isScreenSmall ? 0 : 2, bottom: 20),
            padding: const EdgeInsets.only(top: 10, bottom: 10, left: 0),
            child: Column(children: [
              SelectionArea(
                  child: Row(children: [
                Expanded(child: MarkdownArea(widget.comment.comment, false))
              ])),
              replying && !sendingReply
                  ? Column(children: [
                      Consumer<TypingEmojiSelModel>(
                          builder: (context, typingEmoji, child) =>
                              TypingEmojiPanel(
                                model: typingEmoji,
                                focusNode: widget.inputFocusNode,
                              )),
                      const SizedBox(height: 5),
                      Container(
                          padding: const EdgeInsets.only(
                              left: 13, right: 13, top: 11, bottom: 11),
                          margin: const EdgeInsets.symmetric(horizontal: 10),
                          child: CommentInput(sendReply, "Reply",
                              "Reply to this comment", widget.inputFocusNode))
                    ])
                  : const Text(""),
              commentRRs != null ? const SizedBox(height: 10) : const Empty(),
              commentRRs != null
                  ? Row(children: [
                      const Expanded(child: Divider()),
                      const SizedBox(width: 8),
                      Txt.S(
                          commentRRs!.isEmpty
                              ? "No receive receipts"
                              : "Comment receive receipts",
                          color: TextColor.onSurfaceVariant),
                      const SizedBox(width: 8),
                      const Expanded(child: Divider()),
                    ])
                  : const Empty(),
              if (commentRRs != null && commentRRs!.isNotEmpty)
                Container(
                    alignment: Alignment.centerLeft,
                    child: Wrap(
                        children: commentRRs!
                            .map((rr) => _ReceiveReceipt(widget.client, rr))
                            .toList()))
            ])),
        widget.comment.children.isNotEmpty
            ? Positioned(
                left: isScreenSmall ? -13 : -1,
                bottom: 0,
                child: IconButton(
                    iconSize: isScreenSmall ? 18 : 22,
                    onPressed: () => toggleChildren(),
                    icon: Icon(showChildren
                        ? Icons.do_disturb_on_outlined
                        : Icons.add_circle_outline)))
            : const Empty(),
      ]),
      showChildren && widget.comment.children.isNotEmpty
          ? Container(
              decoration: BoxDecoration(
                  border: Border(
                      left: BorderSide(
                          width: 2.0, color: theme.colors.outlineVariant))),
              margin: EdgeInsets.only(top: 0, left: isScreenSmall ? 5 : 18),
              padding: const EdgeInsets.only(top: 10, bottom: 10, left: 10),
              child: Column(children: [
                ...widget.comment.children.map((e) => _CommentW(
                    widget.post,
                    e,
                    widget.sendReply,
                    widget.client,
                    widget.showReply,
                    widget.canComment,
                    widget.inputFocusNode))
              ]),
            )
          : const Empty(),
    ]);
  }
}

class _PostContentScreenForArgsState extends State<_PostContentScreenForArgs> {
  bool loading = false;
  String markdownData = "";
  Iterable<FeedCommentModel> comments = [];
  bool knowsAuthor = false;
  bool isKXSearchingAuthor = false;
  bool sentSubscribeAttempt = false;
  bool showingReply = false;
  List<ReceiveReceipt> postRRs = [];
  late CustomInputFocusNode inputFocusNode;

  void loadContent() async {
    var snackbar = SnackBarModel.of(context);
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
      snackbar.error('Unable to load content: $exception');
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
      snackbar.error("Unable to load receive receipts: $exception");
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

  void addComment(String msg) async {
    replying = false;
    widget.args.post.addNewComment(msg);
    await Golib.commentPost(
        widget.args.post.summ.from, widget.args.post.summ.id, msg, null);
  }

  void kxSearchAuthor() async {
    var snackbar = SnackBarModel.of(context);
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

      snackbar.error("Unable to KX search post author: $exception");
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
    inputFocusNode = CustomInputFocusNode(widget.typingEmoji);
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
    var snackbar = SnackBarModel.of(context);
    try {
      var summ = widget.args.post.summ;
      await Golib.subscribeToPostsAndFetch(summ.authorID, summ.id);
      setState(() => sentSubscribeAttempt = true);
    } catch (exception) {
      snackbar.error("Unable to subscribe to posts: $exception");
    }
  }

  Future<void> subscribe() async {
    var snackbar = SnackBarModel.of(context);
    try {
      var authorChat =
          widget.client.getExistingChat(widget.args.post.summ.authorID);
      if (authorChat != null) {
        authorChat.subscribeToPosts();
      }
    } catch (exception) {
      snackbar.error("Unable to subscribe to posts: $exception");
    }
  }

  @override
  Widget build(BuildContext context) {
    if (loading) {
      return const Center(
        child: Text("Loading..."),
      );
    }

    var authorNick = widget.args.post.summ.authorNick;
    var authorID = widget.args.post.summ.authorID;
    var relayer = "";
    var myPost = authorID == widget.client.publicID;
    var authorChat = widget.client.getExistingChat(authorID);
    var hasChat = authorChat != null;
    var canComment = false;
    if (myPost) {
      authorNick = "me";
      canComment = true;
    }
    if (authorChat != null) {
      authorNick = authorChat.nick;
      canComment = authorChat.isSubscribed;
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

    List<Widget> commentsWidgets = [];
    var newComments = widget.args.post.newComments;
    if (relayedByAuthor) {
      commentsWidgets.addAll([
        const SizedBox(height: 20),
        const Row(children: [
          Expanded(child: Divider()),
          SizedBox(width: 8),
          Txt.S("Comments", color: TextColor.onSurfaceVariant),
          SizedBox(width: 8),
          Expanded(child: Divider()),
        ]),
        const SizedBox(height: 20),
        !replying
            ? canComment
                ? SizedBox(
                    width: 150,
                    child: OutlinedButton(
                        onPressed: showReply,
                        child: const Txt.S("Add Comment")))
                : Container(
                    padding: const EdgeInsets.only(
                        left: 13, right: 13, top: 11, bottom: 11),
                    margin: const EdgeInsets.symmetric(horizontal: 30),
                    child: Column(children: [
                      const Txt.S(
                          "Cannot comment while unsubscribed to the user's posts."),
                      const SizedBox(height: 10),
                      (authorChat?.isSubscribing ?? false)
                          ? const Txt.S(
                              "Waiting for author to accept our post subscription request.")
                          : const Empty(),
                      (authorChat != null &&
                              !authorChat.isSubscribed &&
                              !authorChat.isSubscribing)
                          ? OutlinedButton(
                              onPressed: subscribe,
                              child: const Txt.S("Subscribe to User's Posts"))
                          : const Empty(),
                    ]))
            : Column(children: [
                Consumer<TypingEmojiSelModel>(
                    builder: (context, typingEmoji, child) => TypingEmojiPanel(
                          model: typingEmoji,
                          focusNode: inputFocusNode,
                        )),
                const SizedBox(height: 5),
                Container(
                    padding: const EdgeInsets.only(
                        left: 13, right: 13, top: 11, bottom: 11),
                    margin: const EdgeInsets.symmetric(horizontal: 30),
                    child: CommentInput(addComment, "Add Comment",
                        "Add a comment to this post", inputFocusNode)),
              ]),
        ...comments.map((e) => _CommentW(widget.args.post, e, sendReply,
            widget.client, showingReplyCB, canComment, inputFocusNode)),
        const SizedBox(height: 20),
        newComments.isNotEmpty
            ? Column(children: [
                const Row(children: [
                  Expanded(child: Divider()),
                  SizedBox(width: 8),
                  Txt.S("Unreplicated Comments",
                      color: TextColor.onSurfaceVariant),
                  SizedBox(width: 8),
                  Expanded(child: Divider()),
                ]),
                Container(
                    padding: const EdgeInsets.symmetric(
                        vertical: 10, horizontal: 40),
                    child: const Txt.S(
                      "Unreplicated comments are those that have been sent to the post's "
                      "relayer for replication but which the relayer has not yet sent back "
                      "to the local client. Comment replication requires the remote user to "
                      "be online so it may take some time until the comment is received back.",
                      color: TextColor.onSurfaceVariant,
                    )),
                ...newComments.map((e) => Container(
                      padding: const EdgeInsets.symmetric(horizontal: 40),
                      child: Column(children: [
                        const SizedBox(width: 300, child: Divider()),
                        Txt.S(e, color: TextColor.onSurfaceVariant)
                      ]),
                    )),
              ])
            : const Empty(),
        const SizedBox(height: 20),
      ]);
    } else {
      commentsWidgets.addAll([
        const SizedBox(height: 10),
        const Divider(),
        Container(
            padding: const EdgeInsets.symmetric(horizontal: 40, vertical: 10),
            child: Column(children: [
              const Txt.S("This is a relayed post and cannot be commented on."),
              const SizedBox(height: 10),
              isKXSearchingAuthor
                  ? const Txt.S(
                      "Currently attempting to KX search for post author. This may "
                      "take a long time to complete, as it involves contacting and "
                      "performing KX with multiple peers.")
                  : !knowsAuthor
                      ? Column(children: [
                          const Txt.S(
                              "In order to comment on the post, the local client "
                              "needs to KX with the post author and subscribe to their "
                              "posts. This may be done automatically by using the \"KX "
                              "Search\" action. KX search may take a long time to "
                              "complete, because it depends on remote peers completing "
                              "KX and referring us to the original author."),
                          const SizedBox(height: 20),
                          OutlinedButton(
                              onPressed: kxSearchAuthor,
                              child: const Text("Start KX Search Attempt"))
                        ])
                      : !sentSubscribeAttempt
                          ? Column(children: [
                              const Txt.S(
                                  "In order to comment on the post, the local client "
                                  "needs subscribe to the author's posts and then fetch "
                                  "this post. The process to do this can be started "
                                  "automatically, but it may take some time until the "
                                  "author responds."),
                              const SizedBox(height: 10),
                              OutlinedButton(
                                  onPressed: subscribeAndFetchPost,
                                  child: const Text("Subscribe and Fetch Post"))
                            ])
                          : const Txt.S(
                              "Sent subscription attempt. It may take some time until the author responds."),
            ])),
      ]);
    }

    List<Widget> receiveReceiptsWidgets = [];
    if (postRRs.isNotEmpty) {
      receiveReceiptsWidgets = [
        const Row(children: [
          Expanded(child: Divider()),
          SizedBox(width: 8),
          Txt.S("Receive Receipts", color: TextColor.onSurfaceVariant),
          SizedBox(width: 8),
          Expanded(child: Divider()),
        ]),
        const SizedBox(height: 10),
        Wrap(
          children:
              postRRs.map((rr) => _ReceiveReceipt(widget.client, rr)).toList(),
        )
      ];
    }

    bool isScreenSmall = checkIsScreenSmall(context);
    return Align(
        alignment: Alignment.topLeft,
        child: Stack(alignment: Alignment.topLeft, children: [
          SingleChildScrollView(
              padding: const EdgeInsets.symmetric(horizontal: 10),
              child:
                  Column(mainAxisAlignment: MainAxisAlignment.start, children: [
                const SizedBox(height: 10),

                // Post Area
                Box(
                    margin: isScreenSmall
                        ? const EdgeInsets.only(
                            left: 19, right: 10, top: 0, bottom: 0)
                        : const EdgeInsets.only(
                            left: 50, right: 50, top: 0, bottom: 0),
                    borderRadius: BorderRadius.circular(3),
                    color: SurfaceColor.secondaryContainer,
                    padding: const EdgeInsets.all(16),
                    child: Column(
                      children: [
                        // Header row.
                        Row(
                          children: [
                            Container(
                              width: 28,
                              margin: const EdgeInsets.only(left: 5),
                              child: UserAvatarFromID(widget.client, authorID,
                                  postFrom: widget.args.post.summ.from,
                                  showChatSideMenuOnTap: true,
                                  nick: authorNick),
                            ),
                            const SizedBox(width: 6),
                            Txt.S(authorNick),
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
                                        icon: const ColoredIcon(
                                            Icons.connect_without_contact,
                                            color: TextColor
                                                .onSecondaryContainer)))
                                : const Text(""),
                            SizedBox(
                              width: 20,
                              child: IconButton(
                                padding: const EdgeInsets.all(0),
                                iconSize: 15,
                                tooltip: "Relay this post to your subscribers",
                                onPressed: relayPostToAll,
                                icon: const ColoredIcon(Icons.send,
                                    color: TextColor.onSecondaryContainer),
                              ),
                            ),
                            Expanded(
                                child: Align(
                                    alignment: Alignment.centerRight,
                                    child: Txt.S(widget.args.post.summ.date
                                        .toLocal()
                                        .toIso8601String())))
                          ],
                        ),

                        // Relayer line
                        const SizedBox(height: 10),
                        relayer == ""
                            ? const Empty()
                            : Row(children: [
                                Expanded(
                                    child: Txt.S("Relayed by $relayer",
                                        style: const TextStyle(
                                            fontStyle: FontStyle.italic)))
                              ]),

                        // Post content
                        const SizedBox(height: 10),
                        SelectionArea(
                            child: Container(
                                alignment: Alignment.topLeft,
                                padding: const EdgeInsets.all(15),
                                child: Provider<DownloadSource>(
                                    create: (context) => DownloadSource(
                                        widget.args.post.summ.authorID),
                                    child: MarkdownArea(markdownData, false)))),
                      ],
                    )),

                // Comments section
                ...commentsWidgets,
                ...receiveReceiptsWidgets,
              ])),

          // Back button on desktop.
          if (!isScreenSmall)
            IconButton(
                alignment: Alignment.topLeft,
                padding: const EdgeInsets.all(15),
                iconSize: 15,
                tooltip: "Go back",
                onPressed: () => Navigator.of(context).pushReplacementNamed(
                    '/feed',
                    arguments:
                        PageTabs(0, null, null)), //widget.tabChange(0, null),
                icon: const ColoredIcon(Icons.close_outlined,
                    color: TextColor.onSurface)),
        ]));
  }
}
