import 'package:bruig/components/confirmation_dialog.dart';
import 'package:bruig/components/containers.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/equalizer_icon.dart';
import 'package:bruig/components/inputs.dart';
import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/components/volume_control.dart';
import 'package:bruig/models/audio.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/emoji.dart';
import 'package:bruig/models/realtimechat.dart';
import 'package:bruig/screens/chats.dart';
import 'package:bruig/screens/realtimechat/activertc.dart';
import 'package:bruig/screens/realtimechat/creatertc.dart';
import 'package:bruig/theme_manager.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:provider/provider.dart';

class RealtimeChatTitle extends StatelessWidget {
  const RealtimeChatTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return Row(children: [
      const Txt.L("Realtime Chat"),
      IconButton(
          onPressed: () {
            Navigator.of(context).pushNamed(CreateRealtimeChatScreen.routeName);
          },
          icon: const Icon(Icons.add_box),
          tooltip: "Create new session"),
    ]);
  }
}

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
        SizedBox(
            width: 24,
            height: 24,
            child: Icon(Icons.mic_off_outlined, color: theme.colors.primary))
      else if (session.inLiveSession && livePeer != null && livePeer.isLive)
        InkWell(
            onTap: () => setState(() => changingVolume = !changingVolume),
            child: EqualizerIcon(isActive: livePeer.hasSound))
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

class _RTDTSessionW extends StatefulWidget {
  final RealtimeChatModel rtc;
  final RTDTSessionModel session;
  final ClientModel client;
  const _RTDTSessionW(this.rtc, this.client, this.session);

  @override
  State<_RTDTSessionW> createState() => __RTDTSessionWState();
}

class __RTDTSessionWState extends State<_RTDTSessionW> {
  RTDTSessionModel get session => widget.session;
  ClientModel get client => widget.client;
  RealtimeChatModel get rtc => widget.rtc;
  List<RMRTDTSessionPublisher> publishers = [];
  bool isActive = false;

  void sessionUpdated() {
    setState(() {
      publishers = session.info.metadata.publishers;
    });
  }

  void activeSessionChanged() {
    var newIsActive = rtc.active.active == session;
    if (newIsActive != isActive) {
      setState(() {
        isActive = newIsActive;
      });
    }
  }

  @override
  void initState() {
    super.initState();
    publishers = session.info.metadata.publishers;
    session.addListener(sessionUpdated);
    rtc.active.addListener(activeSessionChanged);
    isActive = rtc.active.active == session;
  }

  @override
  void didUpdateWidget(_RTDTSessionW oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.session != session) {
      oldWidget.session.removeListener(sessionUpdated);
      session.addListener(sessionUpdated);
      publishers = session.info.metadata.publishers;
      isActive = rtc.active.active == session;
    }
  }

  @override
  void dispose() {
    session.removeListener(sessionUpdated);
    rtc.active.removeListener(activeSessionChanged);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return ListTile(
        onTap: () => !isActive ? rtc.active.active = session : null,
        selected: isActive,
        trailing: session.hasHotAudio
            ? const Icon(Icons.mic)
            : session.inLiveSession
                ? const Icon(Icons.headphones)
                : null,
        title: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Txt.L(session.info.metadata.description),
          Txt.S(session.info.metadata.rv.substring(0, 10)),
        ]));
  }
}

class _RTDTSessionsList extends StatefulWidget {
  final RealtimeChatModel rtc;
  final ClientModel client;
  const _RTDTSessionsList(this.rtc, this.client);

  @override
  State<_RTDTSessionsList> createState() => __RTDTSessionsListState();
}

class __RTDTSessionsListState extends State<_RTDTSessionsList> {
  RealtimeChatModel get rtc => widget.rtc;
  ClientModel get client => widget.client;

  void sessionsUpdated() async {
    setState(() {});
  }

  @override
  void initState() {
    super.initState();
    rtc.addListener(sessionsUpdated);
    rtc.refreshSessions();
  }

  @override
  void didUpdateWidget(_RTDTSessionsList oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.rtc != rtc) {
      oldWidget.rtc.removeListener(sessionsUpdated);
      rtc.addListener(sessionsUpdated);
    }
  }

  @override
  void dispose() {
    rtc.removeListener(sessionsUpdated);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var sessions = rtc.sessions;
    return ListView(
        shrinkWrap: true,
        children:
            sessions.map((sess) => _RTDTSessionW(rtc, client, sess)).toList());
  }
}

class RealtimeChatScreen extends StatefulWidget {
  static String routeName = "/realtimechat";
  final TypingEmojiSelModel typingEmoji;
  const RealtimeChatScreen(this.typingEmoji, {super.key});

  @override
  State<RealtimeChatScreen> createState() => _RealtimeChatScreenState();
}

class _RealtimeChatScreenState extends State<RealtimeChatScreen> {
  late CustomInputFocusNode inputFocusNode;

  @override
  void initState() {
    super.initState();
    inputFocusNode = CustomInputFocusNode(widget.typingEmoji);
  }

  @override
  Widget build(BuildContext context) {
    var rtc = RealtimeChatModel.of(context, listen: false);
    var client = ClientModel.of(context, listen: false);
    var audio = AudioModel.of(context, listen: false);
    return Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
      SecondarySideMenu(width: 200, child: _RTDTSessionsList(rtc, client)),
      Expanded(
          child: Consumer<ActiveRealTimeSessionChatModel>(
              builder: (context, activeModel, child) =>
                  activeModel.active != null
                      ? ActiveRealtimeChatScreen(
                          rtc, activeModel.active!, audio, inputFocusNode)
                      : const Empty())),
    ]);
  }
}
