import 'package:bruig/models/client.dart';
import 'package:bruig/models/feed.dart';
import 'package:bruig/components/buttons.dart';
import 'package:bruig/screens/feed/post_content.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/components/md_elements.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:bruig/components/user_context_menu.dart';
import 'package:bruig/util.dart';
import 'package:duration/duration.dart';

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

class _FeedPostWState extends State<FeedPostW> {
  FeedModel get feed => widget.feed;
  FeedPostModel get post => widget.post;
  showContent(BuildContext context) {
    feed.active = post;
    widget.onTabChange(0, PostContentScreenArgs(widget.post));
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

    Future<void> launchUrlAwait(url) async {
      if (!await launchUrl(Uri.parse(url))) {
        throw 'Could not launch $url';
      }
    }

    var theme = Theme.of(context);
    var bgColor = theme.highlightColor;
    var darkTextColor = theme.indicatorColor;
    var hightLightTextColor = theme.dividerColor; // NAME TEXT COLOR
    var avatarColor = colorFromNick(authorNick);
    var borderDividerColor = theme.backgroundColor;

    var avatarTextColor =
        ThemeData.estimateBrightnessForColor(avatarColor) == Brightness.dark
            ? hightLightTextColor
            : darkTextColor;
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

    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;

    return isScreenSmall
        ? Container(
            margin: const EdgeInsets.only(bottom: 8),
            padding: const EdgeInsets.all(0),
            decoration: BoxDecoration(
                color: bgColor, borderRadius: BorderRadius.circular(10)),
            child: Column(
              children: [
                Container(
                    padding: const EdgeInsets.only(left: 10, right: 10, top: 5),
                    child: Row(
                      children: [
                        Container(
                          width: 35,
                          margin: const EdgeInsets.only(
                              top: 0, bottom: 0, left: 5, right: 0),
                          child: UserContextMenu(
                            client: widget.client,
                            targetUserChat: widget.author,
                            disabled: mine,
                            child: hasUnreadPost
                                ? const Icon(Icons.new_releases_outlined,
                                    color: Colors.amber)
                                : CircleAvatar(
                                    backgroundColor: avatarColor,
                                    child: Text(authorNick[0].toUpperCase(),
                                        style: TextStyle(
                                            color: avatarTextColor,
                                            fontSize: 20))),
                          ),
                        ),
                        const SizedBox(width: 10),
                        Expanded(
                            child: Text(authorNick,
                                style: TextStyle(
                                    color: hightLightTextColor,
                                    fontSize: 13,
                                    fontWeight: FontWeight.w500))),
                        Expanded(
                            child: Align(
                                alignment: Alignment.centerRight,
                                child: Text(sincePost,
                                    style: TextStyle(
                                        fontSize: 13,
                                        color: hightLightTextColor,
                                        fontWeight: FontWeight.w300))))
                      ],
                    )),
                Divider(
                  color: borderDividerColor, //color of divider
                  height: 20, //height spacing of divider
                  thickness: 2, //thickness of divier line
                ),
                Container(
                    padding: const EdgeInsets.only(left: 10, right: 10),
                    child: Row(children: [
                      Expanded(
                          flex: 4,
                          child: Align(
                              alignment: Alignment.center,
                              child: Provider<DownloadSource>(
                                  create: (context) =>
                                      DownloadSource(widget.post.summ.authorID),
                                  child: MarkdownArea(markdownData, false))))
                    ])),
                Divider(
                  color: borderDividerColor, //color of divider
                  height: 20, //height spacing of divider
                  thickness: 2, //thickness of divier line
                ),
                Container(
                    padding:
                        const EdgeInsets.only(left: 10, right: 10, bottom: 10),
                    child: Row(children: [
                      hasUnreadComments
                          ? const Row(children: [
                              Icon(Icons.new_releases_outlined,
                                  color: Colors.amber),
                              SizedBox(width: 10),
                              Text("New Comments",
                                  style: TextStyle(
                                    fontStyle: FontStyle.italic,
                                    fontSize: 12,
                                    color: Colors.amber,
                                  ))
                            ])
                          : const Empty(),
                      Expanded(
                          child: Align(
                              alignment: Alignment.centerRight,
                              child: FeedReadMoreButton(
                                onPressed: () => showContent(context),
                                text: "Read More",
                              )))
                    ])),
              ],
            ),
          )
        : Container(
            width: 470,
            margin: const EdgeInsets.only(bottom: 8),
            padding: const EdgeInsets.all(10),
            decoration: BoxDecoration(
                color: bgColor,
                borderRadius: const BorderRadius.all(Radius.elliptical(5, 5))),
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
                        targetUserChat: widget.author,
                        disabled: mine,
                        child: hasUnreadPost
                            ? const Icon(Icons.new_releases_outlined,
                                color: Colors.amber)
                            : CircleAvatar(
                                backgroundColor: avatarColor,
                                child: Text(authorNick[0].toUpperCase(),
                                    style: TextStyle(
                                        color: avatarTextColor, fontSize: 20))),
                      ),
                    ),
                    const SizedBox(width: 6),
                    Expanded(
                        child: Text(authorNick,
                            style: TextStyle(
                                color: hightLightTextColor,
                                fontSize: 13,
                                fontWeight: FontWeight.w500))),
                    Expanded(
                        child: Align(
                            alignment: Alignment.centerRight,
                            child: Text(sincePost,
                                style: TextStyle(
                                    fontSize: 13,
                                    color: hightLightTextColor,
                                    fontWeight: FontWeight.w300))))
                  ],
                ),
                const SizedBox(
                  height: 10,
                ),
                Row(children: [
                  Expanded(
                      flex: 4,
                      child: Align(
                          alignment: Alignment.center,
                          child: Provider<DownloadSource>(
                              create: (context) =>
                                  DownloadSource(widget.post.summ.authorID),
                              child: MarkdownArea(markdownData, false))))
                ]),
                const SizedBox(height: 5),
                Row(children: [
                  Expanded(
                      child: Divider(
                    color: borderDividerColor, //color of divider
                    height: 10, //height spacing of divider
                    thickness: 1, //thickness of divier line
                    indent: 10, //spacing at the start of divider
                    endIndent: 10, //spacing at the end of divider
                  )),
                ]),
                const SizedBox(height: 5),
                Row(children: [
                  hasUnreadComments
                      ? const Row(children: [
                          Icon(Icons.new_releases_outlined,
                              color: Colors.amber),
                          SizedBox(width: 10),
                          Text("New Comments",
                              style: TextStyle(
                                fontStyle: FontStyle.italic,
                                fontSize: 12,
                                color: Colors.amber,
                              ))
                        ])
                      : const Empty(),
                  Expanded(
                      child: Align(
                          alignment: Alignment.centerRight,
                          child: TextButton(
                            style: TextButton.styleFrom(
                                textStyle: TextStyle(
                                  fontSize: 12,
                                  color: hightLightTextColor,
                                ),
                                foregroundColor: hightLightTextColor,
                                shape: RoundedRectangleBorder(
                                    borderRadius: const BorderRadius.all(
                                        Radius.circular(3)),
                                    side:
                                        BorderSide(color: borderDividerColor))),
                            onPressed: () => showContent(context),
                            child: const Text("Read More"),
                          )))
                ]),
              ],
            ),
          );
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
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    var theme = Theme.of(context);
    var backgroundColor = theme.backgroundColor;
    var posts = widget.onlyShowOwnPosts
        ? widget.feed.posts
            .where((post) => (post.summ.authorID == widget.client.publicID))
        : widget.feed.posts;
    return Container(
      margin: const EdgeInsets.all(1),
      decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(3), color: backgroundColor),
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
    );
  }
}
