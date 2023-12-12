import 'package:flutter/material.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/components/context_menu.dart';
import 'package:bruig/components/rename_chat.dart';
import 'package:bruig/components/suggest_kx.dart';
import 'package:bruig/components/trans_reset.dart';
import 'package:bruig/components/pay_tip.dart';

class UserContextMenu extends StatelessWidget {
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

  void Function(dynamic) _handleItemTap(context) {
    return (result) {
      switch (result) {
        case 'tip':
          showPayTipModalBottom(context, targetUserChat!);
          break;
        case 'reqRatchetReset':
          targetUserChat!.requestKXReset();
          break;
        case 'subscribe':
          targetUserChat!.subscribeToPosts();
          break;
        case 'unsubscribe':
          targetUserChat!.unsubscribeToPosts();
          break;
        case 'rename':
          showRenameModalBottom(context, targetUserChat!);
          break;
        case 'suggestToKX':
          showSuggestKXModalBottom(context, targetUserChat!);
          break;
        case 'transReset':
          showTransResetModalBottom(context, targetUserChat!);
          break;
        case 'kx':
          client!.requestMediateID(postFrom!, targetUserId!);
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
      PopupMenuItem(
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
    if (targetUserChat == null) {
      if (postFrom != null && targetUserId != null) {
        return ContextMenu(
          disabled: disabled,
          handleItemTap: _handleItemTap(context),
          items: _buildUserNotKXedMenu(),
          child: child,
        );
      }
      // If we don't have a target user chat but don't have postFrom
      // and targetUserId, needed to attempt KX, do nothing and return
      // the child
      return child;
    }
    // We do have the target user chat, so show complete context user menu
    return ContextMenu(
      disabled: disabled,
      handleItemTap: _handleItemTap(context),
      items: _buildUserMenu(targetUserChat),
      child: child,
    );
  }
}
