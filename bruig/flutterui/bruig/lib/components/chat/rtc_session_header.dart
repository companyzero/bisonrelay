import 'dart:io';

import 'package:bruig/components/confirmation_dialog.dart';
import 'package:bruig/components/context_menu.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/audio.dart';
import 'package:bruig/models/realtimechat.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/realtimechat/invitetortc.dart';
import 'package:bruig/screens/realtimechat/rtclist.dart';
import 'package:bruig/theme_manager.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

class RTCSessionHeader extends StatefulWidget {
  final RealtimeChatModel rtc;
  final RTDTSessionModel session;
  final AudioModel audio;
  const RTCSessionHeader(this.rtc, this.session, this.audio, {super.key});

  @override
  State<RTCSessionHeader> createState() => _RTCSessionHeaderState();
}

class _RTCSessionHeaderState extends State<RTCSessionHeader> {
  RTDTSessionModel get session => widget.session;
  RealtimeChatModel get rtc => widget.rtc;
  AudioModel get audio => widget.audio;

  void leaveLiveSession() async {
    try {
      await rtc.leaveLiveSession(session);
    } catch (exception) {
      showErrorSnackbar(this, "Unable to leave session: $exception");
    }
  }

  void joinLiveSession() async {
    try {
      await rtc.joinLiveSession(session);
    } catch (exception) {
      showErrorSnackbar(this, "Unable to join session: $exception");
    }
  }

  void makeAudioHot() async {
    try {
      await rtc.switchHotAudio(session);
    } catch (exception) {
      showErrorSnackbar(this, "Unable to make audio hot: $exception");
    }
  }

  void disableHotAudio() async {
    try {
      await rtc.disableHotAudio();
    } catch (exception) {
      showErrorSnackbar(this, "Unable to disable hot audio: $exception");
    }
  }

  void doExitSess() async {
    try {
      await rtc.exitSession(session.sessionRV);
      showSuccessSnackbar(this, "Exited session ${session.sessionShortRV}");
    } catch (exception) {
      showErrorSnackbar(this, "Unable to exit session: $exception");
    }
  }

  void confirmExitSess() {
    showConfirmDialog(context,
        title: "Confirm exit session?",
        content:
            "Really exit this realtime chat session? You can only come back if invited again.",
        onConfirm: doExitSess);
  }

  void doDissolveSess() async {
    try {
      await rtc.dissolveSession(session.sessionRV);
      showSuccessSnackbar(this, "Dissolved session ${session.sessionShortRV}");
    } catch (exception) {
      showErrorSnackbar(this, "Unable to dissolve session: $exception");
    }
  }

  void confirmDissolveSess() {
    showConfirmDialog(context,
        title: "Confirm dissolve session?",
        content:
            "Really dissolve this realtime chat session? The session cannot be recreated.",
        onConfirm: doDissolveSess);
  }

  void doRotateSessCookies() async {
    try {
      await session.rotateCookies();
      showSuccessSnackbar(
          this, "Rotate session ${session.sessionShortRV} cookies");
    } catch (exception) {
      showErrorSnackbar(this, "Unable to rotate session cookies: $exception");
    }
  }

  void rotateSessCookies() {
    showConfirmDialog(context,
        title: "Rotate session cookies?",
        content:
            "This will prevent any members that were kicked from rejoining. In rare cases, it may disrupt live peers.",
        onConfirm: doRotateSessCookies);
  }

  void sessionUpdated() {
    setState(() {});
  }

  void toggleAndroidSpeaker() async {
    if (audio.playbackDeviceId == audio.androidEarpieceDeviceID) {
      if (audio.androidPrevPlaybackDeviceID != "") {
        audio.playbackDeviceId = audio.androidPrevPlaybackDeviceID;
      } else {
        audio.playbackDeviceId = audio.androidSpeakerDeviceID;
      }
    } else if (audio.playbackDeviceId == audio.androidPrevPlaybackDeviceID) {
      audio.playbackDeviceId = audio.androidSpeakerDeviceID;
    } else {
      audio.playbackDeviceId = audio.androidEarpieceDeviceID;
    }
    setState(() {});
  }

  @override
  void initState() {
    super.initState();
    session.addListener(sessionUpdated);
  }

  @override
  void didUpdateWidget(RTCSessionHeader oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.session != session) {
      oldWidget.session.removeListener(sessionUpdated);
      session.addListener(sessionUpdated);
    }
  }

  @override
  void dispose() {
    session.removeListener(sessionUpdated);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var isSmallScreen = checkIsScreenSmall(context);

    // Helper to show an icon button or elevated button depending on screen size.
    Widget button(IconData icon, String label, VoidCallback? onPressed,
        {ButtonStyle? style}) {
      if (isSmallScreen) {
        return ElevatedButton(
            onPressed: onPressed, style: style, child: Icon(icon));
      } else {
        return ElevatedButton.icon(
            icon: Icon(icon),
            label: Txt(label),
            onPressed: onPressed,
            style: style);
      }
    }

    var theme = ThemeNotifier.of(context, listen: false);

    return Row(children: [
      Expanded(
          child: Wrap(
              runSpacing: 10,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
            if (session.inLiveSession)
              button(Icons.keyboard_return, "Leave Live Session",
                  !session.leavingLiveSession ? leaveLiveSession : null)
            else
              ElevatedButton.icon(
                  icon: const Icon(Icons.join_right),
                  label: const Txt("Join Live Session"),
                  onPressed:
                      !session.joiningLiveSession ? joinLiveSession : null),
            SizedBox(width: isSmallScreen ? 5 : 20),
            if (session.inLiveSession && !session.hasHotAudio)
              button(Icons.mic_sharp, "Enable mic", makeAudioHot,
                  style: ElevatedButton.styleFrom(
                      backgroundColor: theme.colors.surface,
                      textStyle: theme.textStyleFor(
                          context, TextSize.medium, TextColor.onPrimary))),
            if (session.hasHotAudio)
              button(Icons.mic_off_sharp, "Disable Mic", disableHotAudio,
                  style: ElevatedButton.styleFrom(
                      backgroundColor: theme.colors.errorContainer,
                      textStyle: theme.textStyleFor(context, TextSize.medium,
                          TextColor.onErrorContainer))),
            if (Platform.isAndroid &&
                audio.androidFoundPlaybackDevices &&
                session.inLiveSession) ...[
              SizedBox(width: isSmallScreen ? 5 : 20),
              button(
                  audio.playbackDeviceId == audio.androidSpeakerDeviceID
                      ? Icons.speaker
                      : Icons.volume_up,
                  "",
                  toggleAndroidSpeaker),
            ],
            if (session.inLiveSession) ...[
              const SizedBox(width: 10),
              Consumer<RealtimeChatRTTModel>(
                  builder: (context, rtt, child) => rtt.lastRTTNano > 0
                      ? Txt.S("RTT ${rtt.lastRTTNanoStr}")
                      : const Empty())
            ],
          ])),
      ContextMenu(
        handleItemTap: (v) {
          switch (v) {
            case "gotosess":
              rtc.active.active = session;
              Navigator.of(context).pushNamed(RealtimeChatScreen.routeName);
              break;
            case "invite":
              Navigator.of(context, rootNavigator: true).pushNamed(
                  InviteToRealtimeChatScreen.routeName,
                  arguments: session);
              break;
            case "exit":
              confirmExitSess();
              break;
            case "dissolve":
              confirmDissolveSess();
              break;
            case "rotcookies":
              rotateSessCookies();
            case null:
              break;
            default:
              showErrorSnackbar(this, "Unknown key in menu: '$v'");
          }
        },
        items: [
          const PopupMenuItem(
              value: "gotosess", child: Text("View Session Info")),
          if (session.isAdmin)
            const PopupMenuItem(
                value: "invite", child: Text("Invite to Session")),
          if (!session.isAdmin)
            const PopupMenuItem(
                value: "exit", child: Text("Permanently exit Session")),
          if (session.isAdmin)
            const PopupMenuItem(
                value: "rotcookies", child: Text("Rotate Cookies")),
          if (session.isAdmin)
            const PopupMenuItem(
                value: "dissolve", child: Text("Dissolve Session")),
        ],
        child: const Icon(Icons.menu),
      ),
    ]);
  }
}
