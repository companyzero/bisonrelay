import 'dart:async';

import 'package:bruig/models/client.dart';
import 'package:bruig/screens/overview.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/models/feed.dart';
import 'package:bruig/screens/feed/feed_posts.dart';
import 'package:bruig/components/feed_bar.dart';
import 'package:bruig/screens/feed/post_content.dart';
import 'package:bruig/screens/feed/new_post.dart';
import 'package:bruig/screens/feed/post_lists.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/models/menus.dart';

class FeedScreenTitle extends StatelessWidget {
  const FeedScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer<MainMenuModel>(builder: (context, menu, child) {
      if (menu.activePageTab <= 0) {
        return Text("Bison Relay / News Feed",
            style:
                TextStyle(fontSize: 15, color: Theme.of(context).focusColor));
      }
      var idx = LnScreenSub.indexWhere((e) => e.pageTab == menu.activePageTab);

      return Text("Bison Relay / News Feed / ${FeedScreenSub[idx].label}",
          style: TextStyle(fontSize: 15, color: Theme.of(context).focusColor));
    });
  }
}

class FeedScreen extends StatefulWidget {
  static const routeName = '/feed';
  final int tabIndex;
  final MainMenuModel mainMenu;
  const FeedScreen(this.mainMenu, {Key? key, this.tabIndex = 0})
      : super(key: key);

  @override
  State<FeedScreen> createState() => _FeedScreenState();
}

class _FeedScreenState extends State<FeedScreen> {
  int tabIndex = 0;
  PostContentScreenArgs? showPost;
  GlobalKey<NavigatorState> navKey = GlobalKey(debugLabel: "overview nav key");

  Widget activeTab() {
    switch (tabIndex) {
      case 0:
        if (showPost == null) {
          return Consumer2<FeedModel, ClientModel>(
              builder: (context, feed, client, child) =>
                  FeedPosts(feed, client, onItemChanged, false));
        } else {
          return PostContentScreen(
              showPost as PostContentScreenArgs, onItemChanged);
        }
      case 1:
        if (showPost == null) {
          return Consumer2<FeedModel, ClientModel>(
            builder: (context, feed, client, child) =>
                FeedPosts(feed, client, onItemChanged, true),
          );
        } else {
          return PostContentScreen(
              showPost as PostContentScreenArgs, onItemChanged);
        }
      case 2:
        return Consumer<ClientModel>(
            builder: (context, client, child) => PostListsScreen(client));
      case 3:
        return Consumer<FeedModel>(
            builder: (context, feed, child) => NewPostScreen(feed));
    }
    return Text("Active is $tabIndex");
  }

  void onItemChanged(int index, PostContentScreenArgs? args) {
    setState(() => {showPost = args, tabIndex = index});
    Timer(const Duration(milliseconds: 1),
        () async => widget.mainMenu.activePageTab = index);
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
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    if (ModalRoute.of(context)!.settings.arguments != null) {
      final args = ModalRoute.of(context)!.settings.arguments as PageTabs;
      tabIndex = args.tabIndex;
    }

    return Row(children: [
      ModalRoute.of(context)!.settings.arguments == null
          ? isScreenSmall
              ? const Empty()
              : FeedBar(onItemChanged, tabIndex)
          : const Empty(),
      Expanded(child: activeTab())
    ]);
  }
}
