import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/feed.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/feed.dart';
import 'package:bruig/screens/feed/feed_posts.dart';
import 'package:duration/duration.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/components/md_elements.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:bruig/components/user_context_menu.dart';

typedef FetchPostCB = Future<void> Function(String pid);

class UserPostW extends StatefulWidget {
  final PostListItem post;
  final ChatModel? author;
  final ClientModel client;
  final FeedModel feed;
  final FetchPostCB fetchPost;
  const UserPostW(
      this.post, this.feed, this.author, this.client, this.fetchPost,
      {Key? key})
      : super(key: key);

  @override
  State<UserPostW> createState() => _UserPostWState();
}

class _UserPostWState extends State<UserPostW> {
  PostListItem get post => widget.post;
  ChatModel? get author => widget.author;
  final List<String> waitingPosts = [];

  void authorUpdated() {
    if (!mounted) {
      return;
    }

    setState(() {});
  }

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
                    onPressed: () => !widget.feed.dowloadingUserPost(post.id)
                        ? widget.fetchPost(post.id)
                        : null,
                    child: Text(!widget.feed.dowloadingUserPost(post.id)
                        ? "Get Post"
                        : "Post Requested"),
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

  final List<String> waitingPosts = [];

  Future<void> fetchPost(String pid) async {
    try {
      waitingPosts.add(pid);
      await widget.feed.getUserPost(chat.id, pid);
    } catch (exception) {
      showErrorSnackbar(this, "Unable to fetch user post: $exception");
    }
  }

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

    // If we received a post we were waiting for, make it the active post. This
    // will only happen if we receive the post while the list of user posts
    // screen was still active (the remote user was online and the post was
    // received fast enough).
    List<FeedPostModel> receivedWaiting = newAlreadyReceived.fold([], (l, e) {
      if (waitingPosts.contains(e.summ.id)) {
        waitingPosts.remove(e.summ.id);
        l.add(e);
      }
      return l;
    });
    if (receivedWaiting.isNotEmpty) {
      // Go to the first one that was fetched.
      widget.feed.active = receivedWaiting[0];
      FeedScreen.showPost(context, receivedWaiting[0]);
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
                    fetchPost,
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
