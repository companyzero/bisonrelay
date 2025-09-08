import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/realtimechat.dart';
import 'package:bruig/screens/overview.dart';
import 'package:bruig/screens/realtimechat/creatertc.dart';
import 'package:bruig/theme_manager.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

void showInstantCallModal(
    BuildContext parentContext, RealtimeChatModel rtc, ChatModel chat) {
  showModalBottomSheet(
    context: parentContext,
    builder: (BuildContext context) => InstantCallModal(rtc, chat,
        OverviewNavigatorModel.of(parentContext).navKey.currentState!),
  );
}

class InstantCallModal extends StatefulWidget {
  final RealtimeChatModel rtc;
  final ChatModel chat;
  final NavigatorState parentNav;
  const InstantCallModal(this.rtc, this.chat, this.parentNav, {super.key});

  @override
  State<InstantCallModal> createState() => _InstantCallModalState();
}

class _InstantCallModalState extends State<InstantCallModal> {
  RealtimeChatModel get rtc => widget.rtc;
  ChatModel get chat => widget.chat;
  bool loading = false;

  void makeInstantCall() async {
    setState(() => loading = true);
    try {
      var currentInstantSession = await rtc.createInstantSession([chat.id]);
      // Starting instant call
      chat.startInstantCall(currentInstantSession);
      if (mounted) {
        Navigator.of(context).pop();
        //widget.parentNav.pushReplacementNamed(RealtimeChatScreen.routeName);
      }
    } catch (exception) {
      showErrorSnackbar(this, "Unable to join session: $exception");
    }
  }

  void gotoCreateGroupInstantCall() async {
    Navigator.of(context).pop();
    Navigator.of(context, rootNavigator: true).pushNamed(
        CreateRealtimeChatScreen.routeName,
        arguments:
            CreateRealtimeChatScreenArgs(isInstant: true, initial: chat));
  }

  @override
  Widget build(BuildContext context) {
    return Container(
        padding: const EdgeInsets.all(30),
        child: Wrap(
            runSpacing: 10,
            spacing: 10,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              Consumer<ThemeNotifier>(
                  builder: (context, theme, child) => FilledButton(
                      style: FilledButton.styleFrom(
                          backgroundColor: theme.extraColors.successOnSurface),
                      onPressed: loading ? null : makeInstantCall,
                      child: Row(
                        children: [
                          Icon(Icons.call,
                              color: theme.colors.onPrimaryContainer),
                          const SizedBox(width: 10),
                          Txt.S("Call ${chat.nick}",
                              style: theme.textStyleFor(
                                  context, null, TextColor.onPrimaryContainer))
                        ],
                      ))),
              OutlinedButton(
                  onPressed: loading ? null : gotoCreateGroupInstantCall,
                  child: Row(
                    children: [
                      Icon(Icons.add),
                      const SizedBox(width: 10),
                      Txt.S("Make group call")
                    ],
                  )),
              Consumer<ThemeNotifier>(
                  builder: (context, theme, child) => ElevatedButton(
                      style: ElevatedButton.styleFrom(
                          backgroundColor: theme.colors.errorContainer),
                      onPressed: () {
                        Navigator.of(context).pop();
                      },
                      child: Row(children: [
                        Icon(Icons.cancel_outlined,
                            color: theme.colors.onErrorContainer),
                        const SizedBox(width: 10),
                        Txt.S("Cancel",
                            style: theme.textStyleFor(
                                context, null, TextColor.onErrorContainer))
                      ]))),
            ]));
  }
}
