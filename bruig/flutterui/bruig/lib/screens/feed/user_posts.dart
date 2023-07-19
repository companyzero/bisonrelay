import 'package:bruig/models/client.dart';
import 'package:bruig/models/feed.dart';
import 'package:bruig/screens/feed/post_content.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/components/md_elements.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:bruig/components/user_context_menu.dart';
import 'package:bruig/util.dart';
import 'package:golib_plugin/golib_plugin.dart';

class UserPostW extends StatefulWidget {
  final PostListItem post;
  final ChatModel? author;
  final ClientModel client;
  final Function onTabChange;
  const UserPostW(this.post, this.author, this.client, this.onTabChange,
      {Key? key})
      : super(key: key);

  @override
  State<UserPostW> createState() => _UserPostWState();
}

class _UserPostWState extends State<UserPostW> {
  PostListItem get post => widget.post;
  ChatModel? get author => widget.author;
  showContent(BuildContext context) async {
    await Golib.getUserPost(author?.id ?? "", post.id);
    widget.onTabChange(0, null);
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
    var borderDividerColor = theme.dialogBackgroundColor;
    var avatarTextColor =
        ThemeData.estimateBrightnessForColor(avatarColor) == Brightness.dark
            ? hightLightTextColor
            : darkTextColor;

    return Container(
      //height: 100,
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
                margin:
                    const EdgeInsets.only(top: 0, bottom: 0, left: 5, right: 0),
                child: UserContextMenu(
                  client: widget.client,
                  targetUserChat: widget.author,
                  child: CircleAvatar(
                      backgroundColor: avatarColor,
                      child: Text(authorNick[0].toUpperCase(),
                          style:
                              TextStyle(color: avatarTextColor, fontSize: 20))),
                ),
              ),
              const SizedBox(width: 6),
              Expanded(
                  child: Text(authorNick,
                      style:
                          TextStyle(color: hightLightTextColor, fontSize: 11))),
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
                        create: (context) => DownloadSource(authorId),
                        child: MarkdownArea(widget.post.title, false))))
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
                              borderRadius:
                                  const BorderRadius.all(Radius.circular(3)),
                              side: BorderSide(color: borderDividerColor))),
                      onPressed: () => showContent(context),
                      child: const Text("Read More"),
                    )))
          ]),
        ],
      ),
    );
  }
}

class UserPosts extends StatefulWidget {
  //final FeedModel feed;
  final List<PostListItem> posts;
  final ClientModel client;
  final Function tabChange;
  const UserPosts(this.posts, this.client, this.tabChange, {Key? key})
      : super(key: key);

  @override
  State<UserPosts> createState() => _UserPostsState();
}

class _UserPostsState extends State<UserPosts> {
  void feedChanged() async {
    setState(() {});
  }

  @override
  void didUpdateWidget(UserPosts oldWidget) {
    super.didUpdateWidget(oldWidget);
  }

  @override
  void dispose() {
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    var theme = Theme.of(context);
    var backgroundColor = theme.backgroundColor;
    var posts = widget.posts;
    var authorID = widget.client.userPostListID;
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
            var author = widget.client.getExistingChat(authorID);
            return UserPostW(post, author, widget.client, widget.tabChange);
          }),
    );
  }
}
