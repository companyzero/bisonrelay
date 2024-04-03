import 'dart:async';
import 'package:bruig/components/empty_widget.dart';
import 'package:flutter/material.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/components/chat/events.dart';
import 'package:scrollable_positioned_list/scrollable_positioned_list.dart';

PageStorageBucket _pageStorageBucket = PageStorageBucket();

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
  int _maxItem = 0;
  bool _showFAB = false;
  late ChatModel _lastChat;
  Timer? _debounce;

  void onChatChanged() {
    setState(() {});
  }

  @override
  initState() {
    super.initState();
    widget.itemPositionsListener.itemPositions.addListener(() {
      if (_debounce?.isActive ?? false) _debounce!.cancel();
      _debounce = Timer(const Duration(milliseconds: 50), () {
        var newMaxItem = widget
                .itemPositionsListener.itemPositions.value.isNotEmpty
            ? widget.itemPositionsListener.itemPositions.value
                .where((ItemPosition position) => position.itemLeadingEdge < 1)
                .reduce((ItemPosition max, ItemPosition position) =>
                    position.itemLeadingEdge > max.itemLeadingEdge
                        ? position
                        : max)
                .index
            : 0;
        print("old maxItem $_maxItem newMaxItem $newMaxItem");
        if (mounted && newMaxItem != _maxItem) {
          _maxItem = newMaxItem;
          if (_maxItem < 5) {
            setState(() {
              _showFAB = true;
            });
          } else {
            setState(() {
              _showFAB = false;
            });
          }
          if (_maxItem < 2) {
            chat.scrollPosition = newMaxItem;
          } else {
            chat.scrollPosition = 0;
          }
        }
      });
    });
    chat.addListener(onChatChanged);
    //_maybeScrollToFirstUnread();
    //_maybeScrollToBottom();
    _lastChat = chat;
  }

  @override
  void didUpdateWidget(Messages oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.chat.removeListener(onChatChanged);
    chat.addListener(onChatChanged);
    //_maybeScrollToFirstUnread();
    //_maybeScrollToBottom();
    onChatChanged();
    _lastChat = chat;
  }

  @override
  dispose() {
    _debounce?.cancel();
    chat.removeListener(onChatChanged);
    super.dispose();
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      if (mounted) {
        widget.itemScrollController.scrollTo(
          index: 0,
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
        chat.scrollPosition == 0) {
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

  Widget _getFAB(Color textColor, Color backgroundColor) {
    if (_showFAB) {
      return FloatingActionButton(
        onPressed: _scrollToBottom,
        tooltip: "Scroll to most recent messages",
        foregroundColor: textColor,
        backgroundColor: backgroundColor,
        elevation: 0,
        hoverElevation: 0,
        mini: true,
        shape: RoundedRectangleBorder(
            side: BorderSide(width: 2, color: textColor),
            borderRadius: BorderRadius.circular(100)),
        child: const Icon(Icons.keyboard_arrow_down),
      );
    }
    return const Empty();
  }

  int calculateTotalMessageCount() {
    int count = 0;
    for (var dayGCMsgs in chat.dayGCMsgs) {
      count += dayGCMsgs.msgs.length + 1; // +1 for the day change message
    }
    return count;
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    var backgroundColor = theme.backgroundColor;
    return Scaffold(
        resizeToAvoidBottomInset: true,
        floatingActionButton: _getFAB(textColor, backgroundColor),
        body: SelectionArea(
          child: PageStorage(
            bucket: _pageStorageBucket,
            child: ScrollablePositionedList.builder(
              reverse: true,
              key: PageStorageKey<String>('chat ${chat.nick}'),
              itemCount: chat.msgs.length,
              physics: const ClampingScrollPhysics(),
              itemBuilder: (context, index) {
                return Event(
                    chat, chat.msgs[index], nick, client, _scrollToBottom);
              },
              itemScrollController: widget.itemScrollController,
              itemPositionsListener: widget.itemPositionsListener,
            ),
          ),
        ));
  }
}
