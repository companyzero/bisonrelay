import 'package:flutter/material.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/components/context_menu.dart';
import 'package:bruig/components/rename_chat.dart';
import 'package:bruig/components/suggest_kx.dart';
import 'package:bruig/components/trans_reset.dart';
import 'package:bruig/components/pay_tip.dart';

class UserContextMenu extends StatefulWidget {
  const UserContextMenu(
      {super.key,
      required this.child,
      this.client,
      this.targetUserChat,
      this.disabled,
      this.postFrom,
      this.targetUserId});

  final bool? disabled;
  final ClientModel? client;
  final ChatModel? targetUserChat;
  final Widget child;
  final String? targetUserId;
  final String? postFrom;

  @override
  State<UserContextMenu> createState() => _UserContextMenuState();
}

class _UserContextMenuState extends State<UserContextMenu> {
  void Function(dynamic) _handleItemTap(context) {
    return (result) {
      switch (result) {
        case 'tip':
          showPayTipModalBottom(context, widget.targetUserChat!);
          break;
        case 'reqRatchetReset':
          widget.targetUserChat!.requestKXReset();
          break;
        case 'subscribe':
          widget.targetUserChat!.subscribeToPosts();
          break;
        case 'unsubscribe':
          widget.targetUserChat!.unsubscribeToPosts();
          break;
        case 'rename':
          showRenameModalBottom(context, widget.targetUserChat!);
          break;
        case 'suggestToKX':
          showSuggestKXModalBottom(context, widget.targetUserChat!);
          break;
        case 'transReset':
          showTransResetModalBottom(context, widget.targetUserChat!);
          break;
        case 'kx':
          widget.client!
              .requestMediateID(widget.postFrom!, widget.targetUserId!);
          break;
      }
    };
  }

  PopMenuList _buildUserMenu(ChatModel? chat) {
    bool isSubscribed = false;
    bool isSubscribing = false;
    if (chat != null) {
      isSubscribed = chat.isSubscribed;
      isSubscribing = chat.isSubscribing;
    }
    return [
      const PopupMenuItem(
        value: 'tip',
        child: Text('Pay Tip'),
      ),
      const PopupMenuItem(
        value: 'reqRatchetReset',
        child: Text('Request Ratchet Reset'),
      ),
      isSubscribed
          ? const PopupMenuItem(
              value: 'unsubscribe',
              child: Text('Unsubscribe to Posts'),
            )
          : !isSubscribing
              ? const PopupMenuItem(
                  value: 'subscribe',
                  child: Text('Subscribe to Posts'),
                )
              : null,
      const PopupMenuItem(
        value: 'rename',
        child: Text('Rename User'),
      ),
      const PopupMenuItem(
        value: 'suggestToKX',
        child: Text('Suggest User to KX'),
      ),
      const PopupMenuItem(
        value: 'transReset',
        child: Text('Issue Transitive Reset with User'),
      ),
    ];
  }

  PopMenuList _buildUserNotKXedMenu() {
    return const [
      PopupMenuItem(
        value: 'kx',
        child: Text('Attempt to KX'),
      )
    ];
  }

  @override
  Widget build(BuildContext context) {
    // If we don't have a target user chat, means we are not KXed
    // with user. The Context Menu should show option to attempt
    // to KX.
    if (widget.targetUserChat == null) {
      if (widget.postFrom != null && widget.targetUserId != null) {
        return ContextMenu(
          disabled: widget.disabled,
          handleItemTap: _handleItemTap(context),
          items: _buildUserNotKXedMenu(),
          child: widget.child,
        );
      }
      // If we don't have a target user chat but don't have postFrom
      // and targetUserId, needed to attempt KX, do nothing and return
      // the child
      return widget.child;
    }
    // We do have the target user chat, so show complete context user menu
    return ContextMenu(
      disabled: widget.disabled,
      handleItemTap: _handleItemTap(context),
      items: _buildUserMenu(widget.targetUserChat),
      child: widget.child,
    );
  }
}
