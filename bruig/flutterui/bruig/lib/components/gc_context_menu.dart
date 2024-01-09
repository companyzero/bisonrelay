import 'package:flutter/material.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/components/context_menu.dart';
import 'package:bruig/components/rename_chat.dart';

void resendList(targetGcChat) async {
  var msg = SynthChatEvent("Resending GC list to members");
  msg.state = SCE_sending;
  targetGcChat.append(ChatEventModel(msg, null));
  try {
    await targetGcChat.resendGCList();
    msg.state = SCE_sent;
  } catch (exception) {
    msg.error = Exception("Unable to resend GC list: $exception");
  }
}

class GcContexMenu extends StatelessWidget {
  const GcContexMenu(
      {super.key,
      required this.child,
      this.client,
      this.targetGcChat,
      this.disabled,
      this.targetUserId,
      this.mobile});

  final bool? disabled;
  final ClientModel? client;
  final ChatModel? targetGcChat;
  final Widget child;
  final String? targetUserId;
  final void Function(BuildContext)? mobile;

  void Function(dynamic) _handleItemTap(context) {
    return (result) {
      switch (result) {
        case 'manage':
          client!.profile = targetGcChat;
          break;
        case 'rename':
          showRenameModalBottom(context, targetGcChat!);
          break;
        case 'resend':
          resendList(targetGcChat!);
          break;
      }
    };
  }

  PopMenuList _buildUserMenu() {
    return const [
      PopupMenuItem(
        value: 'manage',
        child: Text('Manage GC'),
      ),
      PopupMenuItem(
        value: 'rename',
        child: Text('Rename GC'),
      ),
      PopupMenuItem(
        value: 'resend',
        child: Text('Resend GC List'),
      ),
    ];
  }

  @override
  Widget build(BuildContext context) {
    return ContextMenu(
      gc: true,
      disabled: disabled,
      handleItemTap: _handleItemTap(context),
      items: _buildUserMenu(),
      mobile: (context) => mobile!(context),
      child: child,
    );
  }
}
