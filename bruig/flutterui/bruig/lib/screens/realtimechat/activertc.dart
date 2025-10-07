import 'dart:async';

import 'package:bruig/components/chat/input.dart';
import 'package:bruig/components/chat/messages.dart';
import 'package:bruig/components/chat/rtc_session_header.dart';
import 'package:bruig/components/confirmation_dialog.dart';
import 'package:bruig/components/containers.dart';
import 'package:bruig/components/equalizer_icon.dart';
import 'package:bruig/components/inputs.dart';
import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/components/typing_emoji_panel.dart';
import 'package:bruig/components/volume_control.dart';
import 'package:bruig/models/audio.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/emoji.dart';
import 'package:bruig/models/realtimechat.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/screens/chats.dart';
import 'package:bruig/screens/ln/components.dart';
import 'package:bruig/theme_manager.dart';
import 'package:bruig/util.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:provider/provider.dart';
import 'package:scrollable_positioned_list/scrollable_positioned_list.dart';

class _RealtimeSessionPublisherW extends StatefulWidget {
  final ClientModel client;
  final RTDTSessionModel session;
  final RTDTLivePeerModel? peer;
  final RMRTDTSessionPublisher publisher;
  const _RealtimeSessionPublisherW(
      {required this.client,
      required this.session,
      required this.peer,
      required this.publisher});

  @override
  State<_RealtimeSessionPublisherW> createState() =>
      __RealtimeSessionPublisherWState();
}

class __RealtimeSessionPublisherWState
    extends State<_RealtimeSessionPublisherW> {
  ClientModel get client => widget.client;
  RTDTSessionModel get session => widget.session;
  RTDTLivePeerModel? get peer => widget.peer;
  RMRTDTSessionPublisher get publisher => widget.publisher;
  ChatModel? get peerChat => client.getExistingChat(publisher.publisherID);

  bool changingVolume = false;

  void update() {
    setState(() {});
  }

  void confirmKick() {
    if (peerChat == null) {
      return;
    }

    int banSeconds = 0;
    showConfirmDialog(
      context,
      title: "Confirm temporary kick",
      onConfirm: () async {
        await session.kickMember(peer?.peerID ?? 0, banSeconds);
        peerChat!.append(
            ChatEventModel(
                SynthChatEvent(
                    "Kicked ${peerChat?.nick} from live realtime session "
                    "(temp banned for $banSeconds seconds)"),
                null),
            false);
      },
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Txt.L("Really kick user ${peerChat?.nick} from session?"),
          const SizedBox(height: 10),
          Row(children: [
            const Txt("Temporary ban duration (seconds): "),
            SizedBox(
                width: 75, child: intInput(onChanged: (v) => banSeconds = v)),
          ]),
        ],
      ),
    );
  }

  void confirmRemove() {
    if (peerChat == null) {
      return;
    }

    showConfirmDialog(
      context,
      title: "Confirm permanent removal?",
      onConfirm: () async {
        await session.removeMember(peerChat!.id);
        peerChat!.append(
            ChatEventModel(
                SynthChatEvent(
                    "Permanently removed ${peerChat?.nick} from live realtime session"),
                null),
            false);
      },
      content:
          "Really remove user ${peerChat?.nick} from session? This cannot be undone.",
    );
  }

  @override
  void initState() {
    super.initState();
    if (peer != null) {
      peer!.addListener(update);
    }
  }

  @override
  void didUpdateWidget(_RealtimeSessionPublisherW oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget != widget) {
      if (oldWidget.peer != null) {
        oldWidget.peer!.removeListener(update);
      }
      if (peer != null) {
        peer!.addListener(update);
      }
    }
  }

  @override
  void dispose() {
    if (peer != null) {
      peer!.removeListener(update);
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var theme = ThemeNotifier.of(context, listen: false);
    String pubNick = publisher.alias;
    var knownNick = client.getNick(publisher.publisherID);
    if (knownNick != "") pubNick = knownNick;
    var livePeer = session.livePeer(publisher.peerID);
    return Row(children: [
      if (session.inLiveSession &&
          livePeer != null &&
          livePeer.isLive &&
          !livePeer.hasSoundStream)
        Tooltip(
            message: "User is online but not sending voice data",
            child: SizedBox(
                width: 24,
                height: 24,
                child:
                    Icon(Icons.mic_off_outlined, color: theme.colors.primary)))
      else if (session.inLiveSession && livePeer != null && livePeer.isLive)
        Tooltip(
            message: "Click to change user volume",
            child: InkWell(
                onTap: () => setState(() => changingVolume = !changingVolume),
                child: EqualizerIcon(isActive: livePeer.hasSound)))
      else
        const SizedBox(width: 24, height: 24),
      if (session.inLiveSession &&
          livePeer != null &&
          livePeer.isLive &&
          changingVolume)
        VolumeGainControl(
            initialValue: livePeer.gain,
            onChangedDelta: (delta) async {
              await livePeer.modifyGain(delta);
            }),
      const SizedBox(width: 8),
      UserAvatarFromID(
        client,
        publisher.publisherID,
        radius: 10,
      ),
      const SizedBox(width: 5, height: 30),
      Txt.S(pubNick),
      if (session.inLiveSession &&
          peer != null &&
          (peer?.bufferCount ?? 0) > 0) ...[
        const SizedBox(width: 5, height: 30),
        Txt.S(
            "buf: ${formatMsDuration(Duration(milliseconds: (peer?.bufferCount ?? 0) * 20))}")
      ],
      const SizedBox(width: 5),
      if (session.inLiveSession &&
          session.isAdmin &&
          peerChat != null &&
          peer != null &&
          peer!.isLive)
        IconButton(
          onPressed: confirmKick,
          icon: const Icon(Icons.remove_circle),
          tooltip: "Temporarily kick from session",
        ),
      const SizedBox(width: 5),
      if (session.isAdmin && peerChat != null)
        IconButton(
          onPressed: confirmRemove,
          icon: const Icon(Icons.person_remove),
          tooltip: "Permanently remove from session",
        ),
    ]);
  }
}

class ActiveRealtimeChatScreen extends StatefulWidget {
  final RealtimeChatModel rtc;
  final RTDTSessionModel session;
  final AudioModel audio;
  final CustomInputFocusNode inputFocusNode;
  const ActiveRealtimeChatScreen(
      this.rtc, this.session, this.audio, this.inputFocusNode,
      {super.key});

  @override
  State<ActiveRealtimeChatScreen> createState() =>
      _ActiveRealtimeChatScreenState();
}

class _ActiveRealtimeChatScreenState extends State<ActiveRealtimeChatScreen> {
  RTDTSessionModel get session => widget.session;
  RealtimeChatModel get rtc => widget.rtc;
  ClientModel get client => rtc.client;
  List<RMRTDTSessionPublisher> publishers = [];
  ChatModel get sessionChat => session.info.gc == ""
      ? session.chat
      : client.getExistingChat(session.info.gc) ?? session.chat;

  late ItemScrollController _itemScrollController;
  late ItemPositionsListener _itemPositionsListener;

  Timer? timerRefresh;

  void sessionUpdated() {
    setState(() {
      publishers = session.info.metadata.publishers;
    });
  }

  void sendMsg(String msg) async {
    var snackbar = SnackBarModel.of(context);
    try {
      if (sessionChat == session.chat) {
        // Ephemeral chat.
        await session.sendMsg(rtc.client, msg);
      } else {
        // GC chat.
        await sessionChat.sendMsg(msg);
      }
    } catch (exception) {
      snackbar.error("Unable to send message: $exception");
    }
  }

  void refreshIfLive(Timer t) async {
    if (session.inLiveSession) {
      await session.refreshFromLive();
    }
  }

  @override
  void initState() {
    super.initState();
    _itemScrollController = ItemScrollController();
    _itemPositionsListener = ItemPositionsListener.create();
    publishers = session.info.metadata.publishers;
    session.addListener(sessionUpdated);

    // Create a timer to refresh details every 1 second (bufferCount, etc).
    timerRefresh = Timer.periodic(Duration(seconds: 1), refreshIfLive);
  }

  @override
  void didUpdateWidget(ActiveRealtimeChatScreen oldWidget) {
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
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var ownerNick = rtc.client.getNick(session.info.metadata.owner);

    return SizedBox(
        child: Column(children: [
      Box(
        // Info panel
        color: SurfaceColor.primaryContainer,
        padding: const EdgeInsets.all(10),
        margin: const EdgeInsets.only(left: 10, right: 12, bottom: 10),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          RTCSessionHeader(rtc, session, widget.audio),
          const SizedBox(height: 10),
          if (session.isInstant)
            Box(
                padding: EdgeInsets.symmetric(horizontal: 10),
                margin: EdgeInsets.only(bottom: 5),
                color: SurfaceColor.tertiary,
                child: Txt.S("Instant Call - will be removed once left")),
          Txt.S("RV: ${session.info.metadata.rv}"),
          Txt.S("Size: ${session.info.metadata.size}"),
          Txt.S("Description: ${session.info.metadata.description}"),
          Txt.S("Local Peer ID: ${session.info.localPeerID.toRadixString(16)}"),
          Row(
            children: [
              const Txt.S("Owner: "),
              UserAvatarFromID(
                rtc.client,
                session.info.metadata.owner,
                radius: 10,
              ),
              const SizedBox(width: 5),
              Txt.S(ownerNick)
            ],
          ),
          const SizedBox(height: 10),
          const LNInfoSectionHeader("Session Members"),
          ...publishers.map((pub) => _RealtimeSessionPublisherW(
              client: rtc.client,
              publisher: pub,
              session: session,
              peer: session.livePeer(pub.peerID))),
        ]),
      ),
      if (session.inLiveSession) ...[
        Expanded(
            child: Stack(children: [
          Messages(sessionChat, rtc.client, _itemScrollController,
              _itemPositionsListener),
          Positioned(
              bottom: 10,
              left: 10,
              right: 10,
              child: Consumer<TypingEmojiSelModel>(
                  builder: (context, typingEmoji, child) => TypingEmojiPanel(
                        model: typingEmoji,
                        focusNode: widget.inputFocusNode,
                      ))),
        ])),
        ChatInput(sendMsg, sessionChat, widget.inputFocusNode,
            allowAudio: false),
        const SizedBox(height: 5),
      ],
    ]));
  }
}
