import 'package:flutter/material.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/components/chat/events.dart';
import 'package:scrollable_positioned_list/scrollable_positioned_list.dart';

/// TODO: make restoreScrollOffset work.
/// For some reason when trying to use PageStorage the app throws:
/// 'type 'ItemPosition' is not a subtype of type 'double?' in type cast'
class Messages extends StatefulWidget {
  final ChatModel chat;
  final String nick;
  final ClientModel client;
  final ItemScrollController itemScrollController;
  final ItemPositionsListener itemPositionsListener;
  const Messages(this.chat, this.nick, this.client, this.itemScrollController,
      this.itemPositionsListener,
      {Key? key})
      : super(key: key);

  @override
  State<Messages> createState() => _MessagesState();
}

/// Messages scroller states:
/// 1. should scroll bottom - No unread messages
/// 2. should scroll to first unread - If there's one
/// 3. should keep in the bottom - If user has reached end of scroll
class _MessagesState extends State<Messages> {
  ClientModel get client => widget.client;
  ChatModel get chat => widget.chat;
  String get nick => widget.nick;
  bool shouldHoldPosition = false;
  int _maxItem = 0;
  late ChatModel lastChat;

  void onChatChanged() {
    setState(() {});
  }

  @override
  initState() {
    super.initState();
    widget.itemPositionsListener.itemPositions.addListener(() {
      _maxItem = widget.itemPositionsListener.itemPositions.value.isNotEmpty
          ? widget.itemPositionsListener.itemPositions.value
              .where((ItemPosition position) => position.itemLeadingEdge < 1)
              .reduce((ItemPosition max, ItemPosition position) =>
                  position.itemLeadingEdge > max.itemLeadingEdge
                      ? position
                      : max)
              .index
          : 0;
    });
    chat.addListener(onChatChanged);
    _maybeScrollToFirstUnread();
    _maybeScrollToBottom();
    lastChat = chat;
  }

  @override
  void didUpdateWidget(Messages oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.chat.removeListener(onChatChanged);
    chat.addListener(onChatChanged);
    var isSameChat = chat.id == lastChat.id;
    var anotherSender =
        chat.msgs.isNotEmpty && chat.msgs.last.source?.id != client.publicID;
    var receivedNewMsg = isSameChat && anotherSender;
    // user received a msg and is reading history (not on scroll maxExtent)
    if (receivedNewMsg && _maxItem < lastChat.msgs.length - 2) {
      shouldHoldPosition = true;
    } else {
      shouldHoldPosition = false;
    }
    _maybeScrollToFirstUnread();
    _maybeScrollToBottom();
    lastChat = chat;
  }

  @override
  dispose() {
    chat.removeListener(onChatChanged);
    super.dispose();
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      if (mounted) {
        widget.itemScrollController.scrollTo(
          index: chat.msgs.length - 1,
          alignment: 0.0,
          duration: const Duration(
              microseconds: 1), // a little bit smoother than a jump
        );
      }
    });
  }

  void _maybeScrollToBottom() {
    final firstUnreadIndex = chat.firstUnreadIndex();
    if (chat.msgs.isNotEmpty &&
        firstUnreadIndex == -1 &&
        !shouldHoldPosition &&
        _maxItem < chat.msgs.length - 1) {
      _scrollToBottom();
    }
  }

  void _maybeScrollToFirstUnread() {
    final firstUnreadIndex = chat.firstUnreadIndex();
    if (chat.msgs.isNotEmpty && firstUnreadIndex != -1) {
      WidgetsBinding.instance.addPostFrameCallback((_) async {
        if (mounted) {
          widget.itemScrollController.scrollTo(
            index: firstUnreadIndex,
            alignment: 0.0,
            duration: const Duration(
                microseconds: 1), // a little bit smoother than a jump
          );
        }
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return ScrollablePositionedList.builder(
      itemCount: chat.msgs.length,
      physics: const ClampingScrollPhysics(),
      itemBuilder: (context, index) {
        return Event(chat, chat.msgs[index], nick, client, _scrollToBottom);
      },
      itemScrollController: widget.itemScrollController,
      itemPositionsListener: widget.itemPositionsListener,
    );
  }
}
