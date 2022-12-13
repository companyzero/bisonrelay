import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/models/feed.dart';
import 'package:bruig/screens/feed/feed_posts.dart';
import 'package:bruig/components/feed_bar.dart';
import 'package:bruig/screens/feed/post_content.dart';
import 'package:bruig/screens/feed/new_post.dart';
import 'package:bruig/screens/feed/post_lists.dart';

/*

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var backgroundColor = theme.backgroundColor;
    return Consumer<ThemeNotifier>(
      builder: (context, theme, _) => Container(
        margin: const EdgeInsets.all(1),
        decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(3), color: backgroundColor),
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            Row(children: [
              const Expanded(
                child: Text("News Feed",
                    style: TextStyle(
                      fontSize: 20,
                    )),
              ),
              ElevatedButton(
                  onPressed: () {
                    Navigator.of(context, rootNavigator: true)
                        .pushNamed('/newPost');
                  },
                  child: const Text("New Post")),
              const SizedBox(width: 20)
            ]),
            const SizedBox(height: 20),
            Expanded(
                child: 
            )),
            const SizedBox(height: 20),
          ],
        ),
      ),
    );
  }
}

*/

class FeedScreenTitle extends StatelessWidget {
  const FeedScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return Text("Bison Relay / News Feed",
        style: TextStyle(fontSize: 15, color: Theme.of(context).focusColor));
  }
}

class FeedScreen extends StatefulWidget {
  static const routeName = '/feed';
  const FeedScreen({Key? key}) : super(key: key);

  @override
  State<FeedScreen> createState() => _FeedScreenState();
}

class _FeedScreenState extends State<FeedScreen> {
  int tabIndex = 0;
  PostContentScreenArgs? showPost;

  Widget activeTab() {
    switch (tabIndex) {
      case 0:
        if (showPost == null) {
          return Consumer2<FeedModel, ClientModel>(
              builder: (context, feed, client, child) =>
                  FeedPosts(feed, client, onItemChanged));
        } else {
          return PostContentScreen(
              showPost as PostContentScreenArgs, onItemChanged);
        }
      case 1:
        return Consumer<ClientModel>(
            builder: (context, client, child) => PostListsScreen(client));
      case 2:
        return Consumer<FeedModel>(
            builder: (context, feed, child) => NewPostScreen(feed));
    }
    return Text("Active is $tabIndex");
  }

  void onItemChanged(int index, PostContentScreenArgs? args) {
    setState(() => {showPost = args, tabIndex = index});
  }

  @override
  void initState() {
    super.initState();
  }

  @override
  void didUpdateWidget(FeedScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
  }

  @override
  void dispose() {
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Row(children: [
      FeedBar(onItemChanged, tabIndex),
      Expanded(child: activeTab())
    ]);
  }
}
