import 'package:flutter/material.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/components/context_menu.dart';
import 'package:bruig/components/rename_chat.dart';
import 'package:bruig/components/suggest_kx.dart';
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
        case 'rename':
          showRenameModalBottom(context, targetUserChat!);
          break;
        case 'suggestToKX':
          showSuggestKXModalBottom(context, targetUserChat!);
          break;
        case 'kx':
          client!.requestMediateID(postFrom!, targetUserId!);
          break;
      }
    };
  }

  PopMenuList _buildUserMenu() {
    return const [
      PopupMenuItem(
        value: 'tip',
        child: Text('Pay Tip'),
      ),
      PopupMenuItem(
        value: 'reqRatchetReset',
        child: Text('Request Ratchet Reset'),
      ),
      PopupMenuItem(
        value: 'subscribe',
        child: Text('Subscribe to Posts'),
      ),
      PopupMenuItem(
        value: 'rename',
        child: Text('Rename User'),
      ),
      PopupMenuItem(
        value: 'suggestToKX',
        child: Text('Suggest User to KX'),
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
    if (targetUserChat == null) {
      if (postFrom != null && targetUserId != null) {
        return ContextMenu(
          disabled: disabled,
          handleItemTap: _handleItemTap(context),
          items: _buildUserNotKXedMenu(),
          child: child,
        );
      }
      return child;
    }
    return ContextMenu(
      disabled: disabled,
      handleItemTap: _handleItemTap(context),
      items: _buildUserMenu(),
      child: child,
    );
  }
}
