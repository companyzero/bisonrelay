import 'dart:convert';

import 'package:bruig/models/client.dart';
import 'package:bruig/models/feed.dart';
import 'package:bruig/screens/feed/post_content.dart';
import 'package:flutter/material.dart';
import 'package:crypto/crypto.dart';
import 'package:provider/provider.dart';
import 'package:bruig/components/md_elements.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:bruig/components/user_context_menu.dart';

// return a consistent color for each nick. Pretty dumb so far.
Color colorFromNick(String nick) {
  var buff = md5.convert(utf8.encode(nick)).bytes;
  var i = (buff[0] << 16) + (buff[1] << 8) + buff[2];
  // var h = (i / 0xffffff) * 360;
  var c = HSVColor.fromAHSV(1, (i / 0xffffff) * 360, 0.5, 1);
  return c.toColor();
}

class FeedPostW extends StatelessWidget {
  final FeedPostModel post;
  final ChatModel? author;
  final ChatModel? from;
  final ClientModel client;
  final Function onTabChange;
  const FeedPostW(
      this.post, this.author, this.from, this.client, this.onTabChange,
      {Key? key})
      : super(key: key);

  showContent(BuildContext context) {
    onTabChange(0, PostContentScreenArgs(post));
  }

  @override
  Widget build(BuildContext context) {
    var authorNick = author?.nick ?? "";
    var authorID = post.summ.authorID;
    var mine = authorID == client.publicID;
    if (mine) {
      authorNick = "me";
    } else if (authorNick == "") {
      authorNick = post.summ.authorNick;
      if (authorNick == "") {
        authorNick = "[${post.summ.authorID}]";
      }
    }

    Future<void> launchUrlAwait(url) async {
      if (!await launchUrl(Uri.parse(url))) {
        throw 'Could not launch $url';
      }
    }

    var theme = Theme.of(context);
    var bgColor = theme.highlightColor;
    var textColor = theme.focusColor;
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
                  client: client,
                  targetUserChat: author,
                  disabled: mine,
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
              Expanded(
                  child: Align(
                      alignment: Alignment.centerRight,
                      child: Text(post.summ.date.toIso8601String(),
                          style: TextStyle(fontSize: 9, color: darkTextColor))))
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
                        create: (context) => DownloadSource(post.summ.authorID),
                        child: MarkdownArea(post.summ.title, false))))
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

class FeedPosts extends StatefulWidget {
  final FeedModel feed;
  final ClientModel client;
  final Function tabChange;
  const FeedPosts(this.feed, this.client, this.tabChange, {Key? key})
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
    var theme = Theme.of(context);
    var backgroundColor = theme.backgroundColor;

    return Consumer<ClientModel>(
        builder: (context, client, child) => Container(
              margin: const EdgeInsets.all(1),
              decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(3),
                  color: backgroundColor),
              padding: const EdgeInsets.only(
                  left: 117, right: 117, top: 10, bottom: 10),
              child: ListView.builder(
                  itemCount: widget.feed.posts.length,
                  itemBuilder: (context, index) {
                    var post = widget.feed.posts.elementAt(index);
                    var author =
                        widget.client.getExistingChat(post.summ.authorID);
                    var from = widget.client.getExistingChat(post.summ.from);
                    return FeedPostW(
                        post, author, from, client, widget.tabChange);
                  }),
            ));
  }
}
