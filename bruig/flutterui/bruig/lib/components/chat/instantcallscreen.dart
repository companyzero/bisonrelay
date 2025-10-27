import 'dart:io';
import 'dart:async';

import 'package:bruig/components/confirmation_dialog.dart';
import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/audio.dart';
import 'package:bruig/models/realtimechat.dart';
import 'package:bruig/models/uistate.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/theme_manager.dart';
import 'package:bruig/util.dart';
import 'package:flutter/material.dart';
import 'package:just_audio/just_audio.dart';
import 'package:provider/provider.dart';
import 'package:proximity_sensor/proximity_sensor.dart';

class InstantCallScreen extends StatefulWidget {
  final RealtimeChatModel rtc;
  final RTDTSessionModel session;
  final AudioModel audio;
  final ClientModel client;
  final ChatModel chat;
  const InstantCallScreen(
      this.rtc, this.session, this.audio, this.client, this.chat,
      {super.key});

  @override
  State<InstantCallScreen> createState() => _InstantCallScreenState();
}

class _InstantCallScreenState extends State<InstantCallScreen> {
  RTDTSessionModel get session => widget.session;
  RealtimeChatModel get rtc => widget.rtc;
  AudioModel get audio => widget.audio;
  ClientModel get client => widget.client;
  ChatModel get chat => widget.chat;
  List<RMRTDTSessionPublisher> publishers = [];
  RTDTLivePeerModel? livePeer;
  Timer? timerRefresh;
  bool livePeerConnected = false;

  late AudioPlayer player = AudioPlayer();

  void leaveLiveSession() async {
    try {
      await rtc.leaveLiveSession(session);
      setState(() {
        if (session.isInstant) {
          // Leave instant 1v1 session
          var cm = client.getExistingChat(session.info.metadata.owner);
          cm?.finishInstantCall();
        }
      });
    } catch (exception) {
      showErrorSnackbar(this, "Unable to leave session: $exception");
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

      setState(() {
        if (session.isInstant) {
          // Leave instant 1v1 session
          var cm = client.getExistingChat(session.info.metadata.owner);
          cm?.finishInstantCall();
        }
      });
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
      setState(() {
        if (session.isInstant) {
          for (var m in session.info.members) {
            var cm = client.getExistingChat(m.uid);
            cm?.finishInstantCall();
          }
        }
      });
      showSuccessSnackbar(this, "Dissolved session ${session.sessionShortRV}");
    } catch (exception) {
      showErrorSnackbar(this, "Unable to dissolve session: $exception");
    }
  }

  void sessionUpdated() async {
    bool finishCall = false;
    setState(() {
      publishers = session.info.metadata.publishers;
      ChatModel? peerChat;
      for (var pub in publishers) {
        peerChat = client.getExistingChat(pub.publisherID);
        if (peerChat != null) {
          livePeer = session.livePeer(pub.peerID);
          if (!livePeerConnected && livePeer != null) {
            // Call is initially connected. Set connected true and stop ringing.
            player.stop();
            livePeerConnected = true;
          } else if (livePeerConnected && livePeer == null) {
            // Peer disconnected from a previously live session.  Set peer connected false and finish call.
            finishCall = true;
            livePeerConnected = false;
          }
        }
      }
    });
    if (finishCall) {
      await rtc.dissolveSession(session.sessionRV);
      chat.finishInstantCall();
    }
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

  void refreshIfLive(Timer t) async {
    if (session.inLiveSession) {
      await session.refreshFromLive();
    }
  }

  @override
  void initState() {
    super.initState();
    listenSensor();
    session.addListener(sessionUpdated);
    publishers = session.info.metadata.publishers;
    for (var pub in publishers) {
      var peerChat = client.getExistingChat(pub.publisherID);
      if (peerChat != null) {
        if (!livePeerConnected) {
          livePeerConnected = true;
        }
        livePeer = session.livePeer(pub.peerID);
        break;
      }
    }
    // Create a timer to refresh details every 1 second (bufferCount, etc).
    timerRefresh = Timer.periodic(Duration(seconds: 1), refreshIfLive);

    WidgetsBinding.instance.addPostFrameCallback((_) async {
      // Do ringing when owner is shown instant call page (and is attempting
      // to connect with peer).
      if (session.info.metadata.owner == client.publicID) {
        await player.setAsset('assets/br_instant_call.mp3');
        await player.setLoopMode(LoopMode.all);
        await player.play();
      }
    });
  }

  @override
  void didUpdateWidget(InstantCallScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.session != session) {
      oldWidget.session.removeListener(sessionUpdated);
      session.addListener(sessionUpdated);
      publishers = session.info.metadata.publishers;
    }
  }

  @override
  void dispose() {
    session.removeListener(sessionUpdated);
    timerRefresh?.cancel();
    player.dispose();
    super.dispose();
  }

  Future<void> listenSensor() async {
    FlutterError.onError = (FlutterErrorDetails details) {
      showErrorSnackbar(this, "Unable to exit session: $details");
    };

    // -------------------------------------------------- <ANDROID ONLY>
    // NOTE: The following calls only work on Android. Otherwise, nothing happens.
    // You only need to make this call if you want to turn off the screen.
    // Add below permission in your AndroidManifest.xml file.
    //     <uses-permission android:name="android.permission.WAKE_LOCK"/>
    await ProximitySensor.setProximityScreenOff(true)
        .onError((error, stackTrace) {
      showErrorSnackbar(
          this, "could not enable screen off functionality: $error");

      return null;
    });
    // -------------------------------------------------- <ANDROID ONLY>
  }

  @override
  Widget build(BuildContext context) {
    var isSmallScreen = checkIsScreenSmall(context);
    // Helper to show an icon button or elevated button depending on screen size.
    Widget basicButton(IconData icon, VoidCallback? onPressed,
        {ButtonStyle? style}) {
      if (isSmallScreen) {
        return IconButton(onPressed: onPressed, style: style, icon: Icon(icon));
      } else {
        return IconButton(icon: Icon(icon), onPressed: onPressed, style: style);
      }
    }

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Container(
            padding:
                const EdgeInsets.only(left: 15, right: 15, top: 8, bottom: 0),
            child: Column(
                mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Txt.H(chat.nick),
                    ],
                  ),
                  ChatAvatar(
                    chat,
                    radius: 100,
                  ),
                  Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                    if (session.inLiveSession && livePeer == null)
                      Txt.S("Connecting...")
                    else
                      Txt.S(
                          "${timeDifference(chat.instantCallStart, DateTime.now())}s"),
                    const SizedBox(width: 20),
                    Consumer<RealtimeChatRTTModel>(
                        builder: (context, rtt, child) => rtt.lastRTTNano > 0
                            ? Txt.S("RTT: ${rtt.lastRTTNanoStr}")
                            : const Empty()),
                    if (session.inLiveSession &&
                        livePeer != null &&
                        (livePeer?.bufferCount ?? 0) > 0) ...[
                      const SizedBox(width: 20),
                      Txt.S(
                          "Buffer: ${formatMsDuration(Duration(milliseconds: (livePeer?.bufferCount ?? 0) * 20))}")
                    ],
                  ]),
                  Align(
                      alignment: AlignmentDirectional.bottomCenter,
                      child: Row(
                          mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                          children: [
                            if (session.inLiveSession && !session.hasHotAudio)
                              basicButton(Icons.mic_off_sharp, makeAudioHot,
                                  style: IconButton.styleFrom(
                                      iconSize: 60,
                                      hoverColor: theme.colors.primary
                                          .withValues(alpha: 10.0),
                                      backgroundColor: theme.colors.primary,
                                      foregroundColor:
                                          theme.colors.primaryContainer)),
                            if (session.hasHotAudio)
                              basicButton(Icons.mic_sharp, disableHotAudio,
                                  style: IconButton.styleFrom(
                                      iconSize: 60,
                                      hoverColor: theme.colors.primaryContainer
                                          .withValues(alpha: 10.0),
                                      backgroundColor:
                                          theme.colors.primaryContainer,
                                      foregroundColor: theme.colors.primary)),
                            if (Platform.isAndroid &&
                                audio.androidFoundPlaybackDevices &&
                                session.inLiveSession) ...[
                              basicButton(
                                  audio.playbackDeviceId ==
                                          audio.androidSpeakerDeviceID
                                      ? Icons.volume_up
                                      : Icons.phone_android_sharp,
                                  toggleAndroidSpeaker,
                                  style: IconButton.styleFrom(
                                      iconSize: 60,
                                      hoverColor: theme.colors.primaryContainer
                                          .withValues(alpha: 10.0),
                                      backgroundColor:
                                          theme.colors.primaryContainer,
                                      foregroundColor: theme.colors.primary)),
                            ],
                            if (session.inLiveSession)
                              basicButton(
                                  Icons.phone_rounded,
                                  !session.leavingLiveSession
                                      ? session.isAdmin
                                          ? doDissolveSess
                                          : doExitSess
                                      : null,
                                  style: IconButton.styleFrom(
                                    iconSize: 60,
                                    foregroundColor: theme.colors.error,
                                    backgroundColor:
                                        theme.colors.errorContainer,
                                  )),
                          ]))
                ])));
  }
}
