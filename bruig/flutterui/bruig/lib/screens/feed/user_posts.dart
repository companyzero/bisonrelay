import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/feed.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/feed/feed_posts.dart';
import 'package:duration/duration.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/components/md_elements.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:bruig/components/user_context_menu.dart';

class UserPostW extends StatefulWidget {
  final PostListItem post;
  final ChatModel? author;
  final ClientModel client;
  final FeedModel feed;
  final Function onTabChange;
  const UserPostW(
      this.post, this.feed, this.author, this.client, this.onTabChange,
      {Key? key})
      : super(key: key);

  @override
  State<UserPostW> createState() => _UserPostWState();
}

class _UserPostWState extends State<UserPostW> {
  PostListItem get post => widget.post;
  ChatModel? get author => widget.author;
  showContent(BuildContext context) async {
    widget.feed.gettingUserPost = post.id;
    await widget.feed
        .getUserPost(author?.id ?? "", post.id, widget.onTabChange);
  }

  void authorUpdated() => setState(() {});

  @override
  initState() {
    super.initState();
    widget.author?.addListener(authorUpdated);
  }

  @override
  void didUpdateWidget(UserPostW oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.author?.removeListener(authorUpdated);
    widget.author?.addListener(authorUpdated);
  }

  @override
  void dispose() {
    widget.author?.removeListener(authorUpdated);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var authorNick = widget.author?.nick ?? "";
    var authorId = widget.author?.id ?? "";
    var postDate =
        DateTime.fromMillisecondsSinceEpoch(widget.post.timestamp * 1000);
    var postDifference = DateTime.now().difference(postDate);
    var sincePost = prettyDuration(postDifference,
        tersity: DurationTersity.hour, abbreviated: true);

    return Card.filled(
        margin: const EdgeInsets.only(right: 12, bottom: 15),
        child: Container(
            padding: const EdgeInsets.all(10),
            child: Column(children: [
              // First row: header.
              Row(
                children: [
                  Container(
                    width: 28,
                    margin: const EdgeInsets.only(
                        top: 0, bottom: 0, left: 5, right: 0),
                    child: UserContextMenu(
                        client: widget.client,
                        targetUserChat: widget.author,
                        child: UserAvatarFromID(widget.client, authorId,
                            nick: authorNick)),
                  ),
                  const SizedBox(width: 6),
                  Expanded(child: Text(authorNick)),
                  if (widget.post.timestamp > 0) Text(sincePost),
                ],
              ),

              // Second row: summary.
              const SizedBox(height: 10),
              Provider<DownloadSource>(
                  create: (context) => DownloadSource(authorId),
                  child: MarkdownArea(widget.post.title, false)),

              // Third row: footer.
              const Divider(),
              Align(
                  alignment: Alignment.centerRight,
                  child: FilledButton.tonal(
                    onPressed: () => widget.feed.gettingUserPost == ""
                        ? showContent(context)
                        : null,
                    child: Text(widget.feed.gettingUserPost != post.id
                        ? "Get Post"
                        : "Downloading"),
                  )),
            ])));
  }
}

class UserPosts extends StatefulWidget {
  final ChatModel chat;
  final FeedModel feed;
  final ClientModel client;
  final Function tabChange;
  const UserPosts(this.chat, this.feed, this.client, this.tabChange, {Key? key})
      : super(key: key);

  @override
  State<UserPosts> createState() => _UserPostsState();
}

class _UserPostsState extends State<UserPosts> {
  FeedModel get feed => widget.feed;
  ClientModel get client => widget.client;
  ChatModel get chat => widget.chat;
  List<PostListItem> get userPosts => widget.chat.userPostsList.posts;

  List<PostListItem> notReceived = [];
  List<FeedPostModel> alreadyReceived = [];

  void updateLists() {
    var authorID = widget.chat.id;
    var newAlreadyReceived =
        widget.feed.posts.where((post) => (post.summ.authorID == authorID));
    List<PostListItem> newNotReceived = [];
    for (var post in userPosts) {
      var found = false;
      for (var alreadyReceivedPost in newAlreadyReceived) {
        if (post.id == alreadyReceivedPost.summ.id) {
          found = true;
          break;
        }
      }
      if (!found) {
        newNotReceived.add(post);
      }
    }

    setState(() {
      alreadyReceived = newAlreadyReceived.toList();
      notReceived = newNotReceived;
    });
  }

  @override
  initState() {
    super.initState();
    widget.feed.addListener(updateLists);
    chat.userPostsList.addListener(updateLists);
    updateLists();
  }

  @override
  void didUpdateWidget(UserPosts oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.feed != widget.feed) {
      oldWidget.feed.removeListener(updateLists);
      widget.feed.addListener(updateLists);
    }
    if (oldWidget.chat != widget.chat) {
      oldWidget.chat.userPostsList.removeListener(updateLists);
      chat.userPostsList.addListener(updateLists);
    }
  }

  @override
  void dispose() {
    widget.feed.removeListener(updateLists);
    chat.userPostsList.removeListener(updateLists);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = checkIsScreenSmall(context);

    return Container(
        padding: isScreenSmall
            ? const EdgeInsets.only(left: 10, right: 10, top: 10, bottom: 10)
            : const EdgeInsets.only(left: 50, right: 50, top: 10, bottom: 10),
        child: SingleChildScrollView(
            child: Column(children: [
          ...notReceived
              .map((e) => UserPostW(
                    e,
                    widget.feed,
                    widget.chat,
                    widget.client,
                    widget.tabChange,
                  ))
              .toList(),
          ...alreadyReceived
              .map((e) => FeedPostW(
                    widget.feed,
                    e,
                    widget.client.getExistingChat(e.summ.authorID),
                    widget.client.getExistingChat(e.summ.from),
                    widget.client,
                    widget.tabChange,
                  ))
              .toList()
        ])));
  }
}
