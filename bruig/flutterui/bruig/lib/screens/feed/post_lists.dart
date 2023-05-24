import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';

class PostListsScreenTitle extends StatelessWidget {
  const PostListsScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return Text("Bison Relay / Subscriptions ",
        style: TextStyle(color: Theme.of(context).focusColor));
  }
}

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
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var highlightColor = theme.highlightColor;
    var backgroundColor = theme.backgroundColor;
    return Container(
      color: index.isEven ? highlightColor : backgroundColor,
      margin: isScreenSmall
          ? const EdgeInsets.only(left: 10, right: 10, top: 8)
          : const EdgeInsets.only(left: 117, right: 108, top: 8),
      padding: const EdgeInsets.only(top: 4, bottom: 4, left: 8, right: 8),
      child: Row(
        children: [
          Expanded(
              flex: 2,
              child: Text(chat?.nick ?? "",
                  style: TextStyle(color: textColor, fontSize: 11))),
          Expanded(
              flex: 10,
              child: Text(id,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                      color: textColor,
                      fontSize: 11,
                      fontWeight: FontWeight.w200))),
          (remoteSub
              ? IconButton(
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
    );
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
    try {
      var newSubscribers = await Golib.listSubscribers();
      var newSubscriptions = await Golib.listSubscriptions();

      setState(() {
        subscribers = newSubscribers;
        subscriptions = newSubscriptions;
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to load post lists: $exception");
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
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var backgroundColor = theme.backgroundColor;
    var darkTextColor = theme.indicatorColor;
    var dividerColor = theme.highlightColor;

    if (firstLoading) {
      return Center(
        child: Text("Loading...",
            style: TextStyle(color: textColor, fontSize: 21)),
      );
    }
    return Container(
      margin: const EdgeInsets.all(1),
      decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(3), color: backgroundColor),
      padding: const EdgeInsets.all(16),
      child: Column(
        children: [
          Row(children: [
            Expanded(
                child: Divider(
              color: dividerColor, //color of divider
              height: 10, //height spacing of divider
              thickness: 1, //thickness of divier line
              indent: 10, //spacing at the start of divider
              endIndent: 7, //spacing at the end of divider
            )),
            Text("Subscribers to local posts",
                textAlign: TextAlign.center,
                style: TextStyle(color: darkTextColor, fontSize: 11)),
            Expanded(
                child: Divider(
              color: dividerColor, //color of divider
              height: 10, //height spacing of divider
              thickness: 1, //thickness of divier line
              indent: 7, //spacing at the start of divider
              endIndent: 10, //spacing at the end of divider
            )),
          ]),
          const SizedBox(height: 20),
          Expanded(
              child: Align(
                  alignment: Alignment.center,
                  child: ListView.builder(
                    controller: subcribersCtrl,
                    itemCount: subscribers.length,
                    itemBuilder: (context, index) => _SubItem(
                        index,
                        subscribers[index],
                        client.getExistingChat(subscribers[index]),
                        false,
                        unsub),
                  ))),
          const SizedBox(height: 20),
          Row(children: [
            Expanded(
                child: Divider(
              color: dividerColor, //color of divider
              height: 10, //height spacing of divider
              thickness: 1, //thickness of divier line
              indent: 10, //spacing at the start of divider
              endIndent: 7, //spacing at the end of divider
            )),
            Text("Subscriptions to remote posters",
                textAlign: TextAlign.center,
                style: TextStyle(color: darkTextColor, fontSize: 11)),
            Expanded(
                child: Divider(
              color: dividerColor, //color of divider
              height: 10, //height spacing of divider
              thickness: 1, //thickness of divier line
              indent: 7, //spacing at the start of divider
              endIndent: 10, //spacing at the end of divider
            )),
          ]),
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
        ],
      ),
    );
  }
}
