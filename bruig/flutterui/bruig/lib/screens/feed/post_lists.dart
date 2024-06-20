import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/ln/components.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

class PostListsScreen extends StatefulWidget {
  static String routeName = "/postsLists";
  final ClientModel client;
  const PostListsScreen(this.client, {Key? key}) : super(key: key);

  @override
  State<PostListsScreen> createState() => _PostListsScreenState();
}

typedef _UnsubFunc = void Function(ChatModel chat);

class _SubItem extends StatelessWidget {
  final int index;
  final String id;
  final ChatModel? chat;
  final bool remoteSub;
  final _UnsubFunc unsub;
  const _SubItem(this.index, this.id, this.chat, this.remoteSub, this.unsub,
      {Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = checkIsScreenSmall(context);
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Container(
              color: index.isEven
                  ? theme.colors.surfaceContainerHigh
                  : theme.colors.surface,
              margin: isScreenSmall
                  ? const EdgeInsets.only(left: 10, right: 10, top: 8)
                  : const EdgeInsets.only(left: 50, right: 50, top: 8),
              padding:
                  const EdgeInsets.only(top: 4, bottom: 4, left: 8, right: 8),
              child: Row(
                children: [
                  Expanded(flex: 2, child: Txt.S(chat?.nick ?? "")),
                  Expanded(
                      flex: 10,
                      child: Txt.S(id, overflow: TextOverflow.ellipsis)),
                  (remoteSub
                      ? IconButton(
                          visualDensity: VisualDensity.compact,
                          tooltip: "Unsubscribe from users's posts",
                          onPressed: chat != null
                              ? () {
                                  unsub(chat!);
                                }
                              : null,
                          icon: const Icon(Icons.remove_circle_outline_rounded))
                      : const Empty())
                ],
              ),
            ));
  }
}

class _PostListsScreenState extends State<PostListsScreen> {
  bool firstLoading = true;
  ScrollController subcribersCtrl = ScrollController();
  ScrollController subscriptnsCtrl = ScrollController();
  List<String> subscribers = [];
  List<String> subscriptions = [];
  ClientModel get client => widget.client;

  void loadLists() async {
    var snackbar = SnackBarModel.of(context);
    try {
      var newSubscribers = await Golib.listSubscribers();
      var newSubscriptions = await Golib.listSubscriptions();

      setState(() {
        subscribers = newSubscribers;
        subscriptions = newSubscriptions;
      });
    } catch (exception) {
      snackbar.error("Unable to load post lists: $exception");
    } finally {
      setState(() {
        firstLoading = false;
      });
    }
  }

  void unsub(ChatModel chat) async {
    await chat.unsubscribeToPosts();
    loadLists();
  }

  @override
  void initState() {
    super.initState();
    loadLists();
  }

  @override
  Widget build(BuildContext context) {
    if (firstLoading) {
      return const Text("Loading...");
    }

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Container(
            padding: const EdgeInsets.all(16),
            child: Column(children: [
              const LNInfoSectionHeader("Subscribers to local posts"),
              const SizedBox(height: 20),
              Expanded(
                  child: ListView.builder(
                      controller: subcribersCtrl,
                      itemCount: subscribers.length,
                      itemBuilder: (context, index) => _SubItem(
                          index,
                          subscribers[index],
                          client.getExistingChat(subscribers[index]),
                          false,
                          unsub))),
              const LNInfoSectionHeader("Subscriptions to remote posters"),
              const SizedBox(height: 20),
              Expanded(
                  child: ListView.builder(
                controller: subscriptnsCtrl,
                itemCount: subscriptions.length,
                itemBuilder: (context, index) => _SubItem(
                    index,
                    subscriptions[index],
                    client.getExistingChat(subscriptions[index]),
                    true,
                    unsub),
              )),
            ])));
  }
}
