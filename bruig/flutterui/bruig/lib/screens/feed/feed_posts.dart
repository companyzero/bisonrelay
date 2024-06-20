import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/feed.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/feed/post_content.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/components/md_elements.dart';
import 'package:duration/duration.dart';

class _AvatarOrUnread extends StatelessWidget {
  final ClientModel client;
  final ChatModel? chat;
  final bool hasUnread;
  const _AvatarOrUnread(this.client, this.chat, this.hasUnread);

  @override
  Widget build(BuildContext context) {
    return hasUnread
        ? const Icon(Icons.new_releases_outlined, color: Colors.amber)
        : UserOrSelfAvatar(client, chat, showChatSideMenuOnTap: true);
  }
}

class FeedPostW extends StatefulWidget {
  final FeedModel feed;
  final FeedPostModel post;
  final ChatModel? author;
  final ChatModel? from;
  final ClientModel client;
  final Function onTabChange;
  const FeedPostW(this.feed, this.post, this.author, this.from, this.client,
      this.onTabChange,
      {Key? key})
      : super(key: key);

  @override
  State<FeedPostW> createState() => _FeedPostWState();
}

class _NewCommentTag extends StatelessWidget {
  const _NewCommentTag();

  @override
  Widget build(BuildContext context) {
    return const Row(mainAxisSize: MainAxisSize.min, children: [
      Icon(Icons.new_releases_outlined, color: Colors.amber),
      SizedBox(width: 10),
      Text("New Comments",
          style: TextStyle(
            fontStyle: FontStyle.italic,
            fontSize: 12, // fontSize(TextSize.small),
            color: Colors.amber,
          ))
    ]);
  }
}

class _FeedPostWState extends State<FeedPostW> {
  FeedModel get feed => widget.feed;
  FeedPostModel get post => widget.post;
  showContent(BuildContext context) {
    feed.active = post;
    widget.onTabChange(0, PostContentScreenArgs(post));
  }

  void authorUpdated() => setState(() {});

  @override
  initState() {
    super.initState();
    widget.author?.addListener(authorUpdated);
  }

  @override
  void didUpdateWidget(FeedPostW oldWidget) {
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
    var hasUnreadComments = post.hasUnreadComments;
    var hasUnreadPost = post.hasUnreadPost;
    var authorNick = widget.author?.nick ?? "";
    var authorID = widget.post.summ.authorID;
    var mine = authorID == widget.client.publicID;
    if (mine) {
      authorNick = "me";
    } else if (authorNick == "") {
      authorNick = widget.post.summ.authorNick;
      if (authorNick == "") {
        authorNick = "[${widget.post.summ.authorID}]";
      }
    }

    var markdownData = widget.post.summ.title;
    if (widget.post.summ.title.contains("--embed[type=")) {
      // This will pluck the first embed in a post.  Then we can display just
      // that in feedposts without the rest of the post content.
      var firstIndex = widget.post.content.indexOf("--");
      var nextIndex = widget.post.content.indexOf("--", firstIndex + 1);
      markdownData = widget.post.content.substring(firstIndex, nextIndex + 2);
    }
    var postDate = widget.post.summ.date;
    var postDifference = DateTime.now().difference(postDate);
    var sincePost = prettyDuration(postDifference,
        tersity: DurationTersity.hour, abbreviated: true);

    return Card.filled(
        margin: const EdgeInsets.only(right: 12, bottom: 15),
        child: Container(
            padding: const EdgeInsets.all(10),
            child: Column(children: [
              // Header row: Avatar, nick and post time.
              Row(children: [
                SizedBox(
                    width: 28,
                    child: _AvatarOrUnread(
                        widget.client, widget.author, hasUnreadPost)),
                const SizedBox(width: 6),
                Expanded(child: Text(authorNick)),
                Text(sincePost),
              ]),

              // Second row: post summary.
              Provider<DownloadSource>(
                  create: (context) =>
                      DownloadSource(widget.post.summ.authorID),
                  child: MarkdownArea(markdownData, false)),

              // Third row: read more button.
              const Divider(),
              SizedBox(
                  width: double.infinity,
                  child: Wrap(
                      alignment: WrapAlignment.spaceBetween,
                      runSpacing: 10,
                      children: [
                        hasUnreadComments
                            ? const _NewCommentTag()
                            : const Empty(),
                        OutlinedButton(
                          onPressed: () => showContent(context),
                          child: const Txt.S("Read More"),
                        )
                      ])),
            ])));
  }
}

class FeedPosts extends StatefulWidget {
  final FeedModel feed;
  final ClientModel client;
  final Function tabChange;
  final bool onlyShowOwnPosts;
  const FeedPosts(this.feed, this.client, this.tabChange, this.onlyShowOwnPosts,
      {Key? key})
      : super(key: key);

  @override
  State<FeedPosts> createState() => _FeedPostsState();
}

class _FeedPostsState extends State<FeedPosts> {
  void feedChanged() async {
    setState(() {});
  }

  @override
  void didUpdateWidget(FeedPosts oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.feed.removeListener(feedChanged);
    widget.feed.addListener(feedChanged);
  }

  @override
  void dispose() {
    widget.feed.removeListener(feedChanged);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = checkIsScreenSmall(context);
    var posts = widget.onlyShowOwnPosts
        ? widget.feed.posts
            .where((post) => (post.summ.authorID == widget.client.publicID))
        : widget.feed.posts;
    return SelectionArea(
        child: Container(
      padding: isScreenSmall
          ? const EdgeInsets.only(left: 10, right: 10, top: 10, bottom: 10)
          : const EdgeInsets.only(left: 50, right: 50, top: 10, bottom: 10),
      child: ListView.builder(
          itemCount: posts.length,
          itemBuilder: (context, index) {
            var post = posts.elementAt(index);
            var author = widget.client.getExistingChat(post.summ.authorID);
            var from = widget.client.getExistingChat(post.summ.from);
            return FeedPostW(widget.feed, post, author, from, widget.client,
                widget.tabChange);
          }),
    ));
  }
}
