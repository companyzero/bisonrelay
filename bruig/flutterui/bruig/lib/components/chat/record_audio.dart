import 'dart:async';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/chat/types.dart';
import 'package:bruig/components/containers.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/audio.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/theme_manager.dart';
import 'package:bruig/util.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/util.dart';
import 'package:provider/provider.dart';

const _smallScreenSizeMultiplier = 1.2;

class RecordAudioInputButton extends StatefulWidget {
  const RecordAudioInputButton({super.key});

  @override
  State<RecordAudioInputButton> createState() => _RecordAudioInputButtonState();
}

class _RecordAudioInputButtonState extends State<RecordAudioInputButton> {
  bool hoveringLockRecording = false;

  // Used to confirm re-record when already have record.
  void startRecording(AudioModel audio, SnackBarModel snackbar) async {
    try {
      await audio.recordNote();
    } catch (exception) {
      snackbar.error("Unable to record audio note: $exception");
    }
  }

  @override
  Widget build(BuildContext context) {
    var isScreenSmall = checkIsScreenSmall(context);
    double iconSizeMult = isScreenSmall ? _smallScreenSizeMultiplier : 1;
    return TooltipExcludingMobile(
        message: "Hold to record an audio note",
        child: Consumer3<AudioModel, SnackBarModel, ThemeNotifier>(
            builder: (context, audio, snackbar, theme, child) => Listener(
                onPointerUp: (event) {
                  if (audio.recording && hoveringLockRecording) {
                    audio.lockedRecording = true;
                  } else {
                    audio.stop();
                  }
                },
                child: Row(children: [
                  if (audio.recording)
                    Container(
                        margin: const EdgeInsets.only(right: 10),
                        padding: const EdgeInsets.all(10),
                        decoration: BoxDecoration(
                            color: hoveringLockRecording
                                ? theme.colors.surfaceBright
                                    .withAlpha(127) // Colors.white10
                                : null,
                            shape: BoxShape.circle),
                        child: Tooltip(
                            message: "Lock recording",
                            child: DragTarget(
                              onWillAcceptWithDetails: (details) {
                                setState(() => hoveringLockRecording = true);
                                return true;
                              },
                              onLeave: (data) {
                                setState(() => hoveringLockRecording = false);
                              },
                              builder: (context, candidateData, rejectedData) =>
                                  hoveringLockRecording || audio.lockedRecording
                                      ? Icon(
                                          size: isScreenSmall ? 40 : 30,
                                          color: theme.colors.onSurfaceVariant,
                                          Icons.lock_outline)
                                      : Icon(
                                          size: isScreenSmall ? 40 : 30,
                                          color: theme.colors.onSurfaceVariant,
                                          Icons.lock_open_outlined),
                            ))),
                  LongPressDraggable(
                      data: true,
                      dragAnchorStrategy: pointerDragAnchorStrategy,
                      feedback: const Empty(),
                      child: CircularProgressButton(
                        sizeMultiplier: iconSizeMult,
                        active: audio.recording,
                        inactiveIcon: Icons.mic,
                        activeIcon: Icons.mic_off_rounded,
                        holdDuration: audio.hasRecord
                            ? const Duration(milliseconds: 1000)
                            : null,
                        onHold: audio.hasRecord && !audio.recording
                            ? () => startRecording(audio, snackbar)
                            : null,
                        onTapDown: audio.hasRecord || audio.recording
                            ? null
                            : () => startRecording(audio, snackbar),
                        onTapUp: () async {
                          try {
                            await audio.stop();
                            setState(() => hoveringLockRecording = false);
                          } catch (exception) {
                            snackbar.error(
                                "Unable to stop recording audio note: $exception");
                          }
                        },
                      )),
                ]))));
  }
}

class SmallScreenRecordInfoPanel extends StatefulWidget {
  final AudioModel audio;
  const SmallScreenRecordInfoPanel({super.key, required this.audio});

  @override
  State<SmallScreenRecordInfoPanel> createState() =>
      _SmallScreenRecordInfoPanelState();
}

class _SmallScreenRecordInfoPanelState
    extends State<SmallScreenRecordInfoPanel> {
  AudioModel get audio => widget.audio;

  DateTime? recordStartTime;
  String recordTime = "";
  Timer? recordTimer;

  void updateRecordTime(_) {
    var elapsed = DateTime.now().difference(audio.startRecordTime);
    setState(() {
      recordStartTime = audio.startRecordTime;
      recordTime = formatSmallDuration(elapsed);
    });
  }

  @override
  void didUpdateWidget(covariant SmallScreenRecordInfoPanel oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (audio.startRecordTime != recordStartTime) {
      updateRecordTime(null);
    }
    if (audio.recording && recordTimer == null) {
      recordTimer =
          Timer.periodic(const Duration(seconds: 1), updateRecordTime);
    } else if (!audio.recording && recordTimer != null) {
      recordTimer?.cancel();
      recordTimer = null;
    }
  }

  @override
  Widget build(BuildContext context) {
    if (!audio.recording && !audio.hasRecord) {
      return const Empty();
    }

    return Box(
        padding: const EdgeInsets.all(5),
        color: SurfaceColor.tertiaryContainer,
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          if (widget.audio.recording || widget.audio.hasRecord)
            Txt.L(recordTime,
                style: ThemeNotifier.of(context).extraTextStyles.monospaced),
          ...(widget.audio.lastRecord != null
              ? [
                  Txt.L(
                      "Size: ${humanReadableSize(widget.audio.lastRecord!.size)}   "),
                  Txt.L(
                      "Est. Cost: ${formatDCR(milliatomsToDCR(widget.audio.lastRecord!.cost))}")
                ]
              : [const SizedBox(height: 46)]),
        ]));
  }
}

class RecordAudioInputPanel extends StatefulWidget {
  final VoidCallback? onRecordingStarted;
  final VoidCallback? onRecordingEnded;
  final VoidCallback? onRecordingDone;
  final SendMsg sendMsg;
  final AudioModel audio;
  const RecordAudioInputPanel(
      {this.onRecordingStarted,
      this.onRecordingEnded,
      this.onRecordingDone,
      required this.sendMsg,
      required this.audio,
      super.key});

  @override
  State<RecordAudioInputPanel> createState() => _RecordAudioInputPanelState();
}

class _RecordAudioInputPanelState extends State<RecordAudioInputPanel> {
  AudioModel get audio => widget.audio;
  bool get recording => audio.recording;
  bool get playing => audio.playing && audio.playingSource == null;
  RecordedAudioNote? recordedNote;

  String recordTime = "";
  Timer? recordTimer;

  void updateRecordTime(_) {
    var elapsed = DateTime.now().difference(audio.startRecordTime);
    setState(() {
      recordTime = formatSmallDuration(elapsed);
    });
  }

  void playback() async {
    try {
      await audio.playbackNote();
    } catch (exception) {
      showErrorSnackbar(this, "Unable to playback audio note: $exception");
    }
  }

  void stopPlayback() async {
    try {
      await audio.stop();
    } catch (exception) {
      showErrorSnackbar(this, "Unable to stop playback: $exception");
    }
  }

  void playbackStop() {
    playing ? stopPlayback() : playback();
  }

  void updateState() async {
    if (!audio.hasRecord) {
      // Cleared recording.
      recordTime = "";
      recordedNote = null;
    } else if (recordedNote == null && audio.lastRecord != null) {
      // Finished successful recording.
      var note = audio.lastRecord!;

      // await sleep(Duration(milliseconds: 100));
      setState(() {
        recordTime =
            formatSmallDuration(Duration(milliseconds: note.durationMs));
        recordedNote = note;
      });
    }

    if (!audio.recording && recordTimer != null) {
      // Stopped recording.
      recordTimer!.cancel();
      recordTimer = null;
    }
    if (audio.recording && recordTimer == null) {
      // Started recording.
      recordTimer =
          Timer.periodic(const Duration(seconds: 1), updateRecordTime);
    }

    setState(() {});
  }

  void cancel() {
    setState(() {
      recordTime = "";
      recordedNote = null;
    });
    audio.clearRecorded();
  }

  void accept() {
    if (recordedNote != null) {
      widget.sendMsg(recordedNote!.embed);
    }
    audio.clearRecorded();
    recordTime = "";
  }

  @override
  void initState() {
    super.initState();
    audio.addListener(updateState);
    updateState();
  }

  @override
  void didUpdateWidget(RecordAudioInputPanel oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.audio.removeListener(updateState);
    audio.addListener(updateState);
  }

  @override
  void dispose() {
    if (recording) {
      audio.stop();
    }
    audio.removeListener(updateState);
    super.dispose();
  }

  Widget buildRecordInfo() {
    return Expanded(
        child: Wrap(spacing: 10, children: [
      if (recordedNote != null)
        Txt.S("Size: ${humanReadableSize(recordedNote!.size)}   "),
      if (recordedNote != null)
        Txt.S("Est. Cost: ${formatDCR(milliatomsToDCR(recordedNote!.cost))}"),
    ]));
  }

  Widget buildPlayStopBtn(bool isScreenSmall) {
    return CircularProgressButton(
        sizeMultiplier: isScreenSmall ? _smallScreenSizeMultiplier : 1.0,
        active: playing,
        inactiveIcon: Icons.play_arrow,
        activeIcon: Icons.stop_sharp,
        onTapDown: playbackStop);
  }

  Widget buildCancelBtn(bool isScreenSmall) {
    return TooltipExcludingMobile(
        message: "Hold to discard audio note",
        child: CircularProgressButton(
            sizeMultiplier: isScreenSmall ? _smallScreenSizeMultiplier : 1.0,
            inactiveIcon: Icons.cancel_outlined,
            holdDuration: const Duration(milliseconds: 1500),
            onHold: cancel));
  }

  Widget buildAcceptBtn(bool isScreenSmall) {
    return TooltipExcludingMobile(
        message: "Send audio note",
        child: IconButton(
            onPressed: accept,
            iconSize: isScreenSmall ? 30 : null,
            icon: const Icon(Icons.send)));
  }

  @override
  Widget build(BuildContext context) {
    bool hasRecordToSend = audio.hasRecord;
    var isSmallScreen = checkIsScreenSmall(context);

    if (isSmallScreen) {
      return Row(crossAxisAlignment: CrossAxisAlignment.center, children: [
        if (hasRecordToSend) ...[
          buildPlayStopBtn(true),
          buildCancelBtn(true),
          buildAcceptBtn(true),
        ],
      ]);

      // Info about recording is shown on SmallScreenRecordInfoPanel, which is
      // overlayed on top of Messages (on ActiveChat).
    }

    return Row(crossAxisAlignment: CrossAxisAlignment.center, children: [
      const SizedBox(width: 10, height: 40),
      if (recording || hasRecordToSend)
        Txt.S(recordTime,
            style: ThemeNotifier.of(context).extraTextStyles.monospaced),
      const SizedBox(width: 5),
      if (hasRecordToSend) buildPlayStopBtn(false),
      if (hasRecordToSend) buildCancelBtn(false),
      const SizedBox(width: 5),
      buildRecordInfo(),
      if (hasRecordToSend) buildAcceptBtn(false),
    ]);
  }
}
