import 'dart:async';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/theme_manager.dart';
import 'package:flutter/material.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/components/chat/events.dart';
import 'package:provider/provider.dart';
import 'package:scrollable_positioned_list/scrollable_positioned_list.dart';

PageStorageBucket _pageStorageBucket = PageStorageBucket();

class Messages extends StatefulWidget {
  final ChatModel chat;
  final ClientModel client;
  final ItemScrollController itemScrollController;
  final ItemPositionsListener itemPositionsListener;
  const Messages(this.chat, this.client, this.itemScrollController,
      this.itemPositionsListener,
      {super.key});

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
  int _maxItem = 0;
  bool _showFAB = false;
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
                .reduce((ItemPosition max, ItemPosition position) {
                return position.itemLeadingEdge < max.itemLeadingEdge
                    ? position
                    : max;
              }).index
            : 0;
        if (mounted && newMaxItem != _maxItem) {
          _maxItem = newMaxItem;
          if (_maxItem > 5) {
            setState(() {
              _showFAB = true;
            });
          } else {
            setState(() {
              _showFAB = false;
            });
          }
          if (_maxItem > 2) {
            chat.scrollPosition = newMaxItem;
          } else {
            chat.scrollPosition = 0;
          }
        }
      });
    });
    chat.addListener(onChatChanged);
    _maybeScrollToBottom();
  }

  @override
  void didUpdateWidget(Messages oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.chat.removeListener(onChatChanged);
    chat.addListener(onChatChanged);
    _maybeScrollToBottom();
    onChatChanged();
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

  void _scrollToFirstUnread() {
    final firstUnreadIndex = chat.firstUnreadIndex();
    if (chat.msgs.isNotEmpty && firstUnreadIndex != -1) {
      // In the future, track messages shown to user and instead of clearing all
      // unread, move the marker down to keep track of that was read.
      Timer(const Duration(seconds: 5), chat.removeFirstUnread);
      WidgetsBinding.instance.addPostFrameCallback((_) async {
        if (mounted) {
          widget.itemScrollController.scrollTo(
            index: firstUnreadIndex <= 5 ? 0 : firstUnreadIndex - 5,
            alignment: 0.0,
            duration: const Duration(
                microseconds: 1), // a little bit smoother than a jump
          );
        }
      });
    }
  }

  Widget? _getFAB() {
    var showScrollToFirstUnread = (chat.firstUnreadIndex() != -1 &&
        chat.firstUnreadIndex() - _maxItem > 5);
    var showScrollToMostRecent = _showFAB;

    if (!(showScrollToFirstUnread || showScrollToMostRecent)) {
      return null;
    }

    return Consumer<ThemeNotifier>(builder: (context, theme, child) {
      if (showScrollToFirstUnread) {
        return FloatingActionButton(
          onPressed: _scrollToFirstUnread,
          tooltip: "Scroll to first unread message",
          backgroundColor: theme.colors.surface.withValues(alpha: 0.65),
          foregroundColor: theme.colors.onSurfaceVariant,
          elevation: 0,
          hoverElevation: 0,
          mini: true,
          shape: RoundedRectangleBorder(
              side: BorderSide(width: 2, color: theme.colors.onSurfaceVariant),
              borderRadius: BorderRadius.circular(100)),
          child: const Icon(Icons.keyboard_arrow_up_outlined),
        );
      } else if (showScrollToMostRecent) {
        return FloatingActionButton(
          onPressed: _scrollToBottom,
          tooltip: "Scroll to most recent messages",
          backgroundColor: theme.colors.surface.withValues(alpha: 0.65),
          foregroundColor: theme.colors.onSurfaceVariant,
          elevation: 0,
          hoverElevation: 0,
          mini: true,
          shape: RoundedRectangleBorder(
              side: BorderSide(width: 2, color: theme.colors.onSurfaceVariant),
              borderRadius: BorderRadius.circular(100)),
          child: const Icon(Icons.keyboard_arrow_down_outlined),
        );
      }

      return const Empty();
    });
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
    return Scaffold(
        resizeToAvoidBottomInset: true,
        floatingActionButton: _getFAB(),
        body: SelectionArea(
          child: PageStorage(
            bucket: _pageStorageBucket,
            child: ScrollablePositionedList.builder(
              reverse: true,
              key: PageStorageKey<String>('chat ${chat.nick}'),
              itemCount: chat.msgs.length,
              physics: const ClampingScrollPhysics(),
              itemBuilder: (context, index) =>
                  Event(chat, chat.msgs[index], client),
              itemScrollController: widget.itemScrollController,
              itemPositionsListener: widget.itemPositionsListener,
            ),
          ),
        ));
  }
}
