import 'dart:async';

import 'package:bruig/components/chat/chat_side_menu.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/feed/user_posts.dart';
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
import 'package:bruig/theme_manager.dart';
import 'package:bruig/models/emoji.dart';

class FeedScreenTitle extends StatelessWidget {
  const FeedScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer2<MainMenuModel, ThemeNotifier>(
        builder: (context, menu, theme, child) {
      if (menu.activePageTab <= 0) {
        return const Txt.L("Feed");
      }
      var idx =
          feedScreenSub.indexWhere((e) => e.pageTab == menu.activePageTab);

      return Txt.L("Feed / ${feedScreenSub[idx].label}");
    });
  }
}

class FeedScreen extends StatefulWidget {
  static const routeName = '/feed';

  // Goes to the screen that shows the user's posts.
  static void showUsersPosts(BuildContext context, ChatModel chat) =>
      Navigator.of(context).pushReplacementNamed(FeedScreen.routeName,
          arguments: PageTabs(4, chat, null));

  // Goest to the screen that shows a specific post.
  static void showPost(BuildContext context, FeedPostModel post) =>
      Navigator.of(context).pushReplacementNamed(FeedScreen.routeName,
          arguments: PageTabs(0, null, PostContentScreenArgs(post)));

  final int tabIndex;
  final MainMenuModel mainMenu;
  final TypingEmojiSelModel typingEmoji;
  const FeedScreen(this.mainMenu, this.typingEmoji,
      {super.key, this.tabIndex = 0});

  @override
  State<FeedScreen> createState() => _FeedScreenState();
}

class _FeedScreenState extends State<FeedScreen> {
  ChatModel? userPostList;
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
          return PostContentScreen(showPost as PostContentScreenArgs,
              onItemChanged, widget.typingEmoji);
        }
      case 1:
        if (showPost == null) {
          return Consumer2<FeedModel, ClientModel>(
            builder: (context, feed, client, child) =>
                FeedPosts(feed, client, onItemChanged, true),
          );
        } else {
          return PostContentScreen(showPost as PostContentScreenArgs,
              onItemChanged, widget.typingEmoji);
        }
      case 2:
        return Consumer<ClientModel>(
            builder: (context, client, child) => PostListsScreen(client));
      case 3:
        return Consumer<FeedModel>(
            builder: (context, feed, child) => NewPostScreen(feed));
      case 4:
        if (showPost == null && userPostList != null) {
          return Consumer2<FeedModel, ClientModel>(
              builder: (context, feed, client, child) =>
                  UserPosts(userPostList!, feed, client, onItemChanged));
        } else if (showPost != null) {
          return PostContentScreen(showPost as PostContentScreenArgs,
              onItemChanged, widget.typingEmoji);
        } else {
          return Text("Active tab $tabIndex without post or userPostList");
        }
    }
    return Text("Active is $tabIndex");
  }

  void onItemChanged(int index, PostContentScreenArgs? args) {
    setState(() {
      showPost = args;
      tabIndex = index;
    });
    Timer(const Duration(milliseconds: 1),
        () async => widget.mainMenu.activePageTab = index);
  }

  @override
  void initState() {
    super.initState();
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();

    // Determine if showing a specific user's posts.
    if (ModalRoute.of(context)?.settings.arguments != null) {
      final args = ModalRoute.of(context)!.settings.arguments as PageTabs;
      tabIndex = args.tabIndex;
      setState(() {
        if (args.userPostList != null) {
          userPostList = args.userPostList;
        }
        if (args.postScreenArgs != null) {
          showPost = args.postScreenArgs;
        }
      });
    }
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
    bool isScreenSmall = checkIsScreenSmall(context);
    bool hasArgs = false;
    if (ModalRoute.of(context)?.settings.arguments is PageTabs) {
      var args = ModalRoute.of(context)?.settings.arguments as PageTabs;
      hasArgs = args.postScreenArgs != null || args.userPostList != null;
    }

    var client = Provider.of<ClientModel>(context);

    return ScreenWithChatSideMenu(
        client,
        Row(children: [
          !isScreenSmall && !hasArgs
              ? FeedBar(onItemChanged, tabIndex)
              : const Empty(),
          Expanded(child: activeTab())
        ]));
  }
}
