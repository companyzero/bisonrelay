import 'dart:io';
import 'dart:typed_data';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/audio.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:just_audio/just_audio.dart';
import 'package:path_provider/path_provider.dart';
import 'package:path/path.dart' as path;
import 'package:share_plus/share_plus.dart';

class AudioPlayerTracker extends StatefulWidget {
  final Uint8List audioBytes;
  final AudioModel audio;
  const AudioPlayerTracker(
      {required this.audioBytes, required this.audio, super.key});

  @override
  State<AudioPlayerTracker> createState() => _AudioPlayerTrackerState();
}

class _AudioPlayerTrackerState extends State<AudioPlayerTracker> {
  AudioModel get audio => widget.audio;
  bool get playing => audio.playing && audio.playingSource == widget.audioBytes;
  bool listeningPosition = false;

  double progress = 0;

  void onPositionChanged() {
    var length = audio.audioPosition.length.inMilliseconds;
    var value = audio.audioPosition.position.inMilliseconds;

    // print("YYYYY onPositionChanged tracker $value $length");

    if (length == 0) {
      return;
    }
    setState(() {
      progress = value / length;
    });
  }

  void update() {
    if (!playing) {
      if (listeningPosition) {
        audio.audioPosition.removeListener(onPositionChanged);
        listeningPosition = false;
      }
      return;
    }

    if (!listeningPosition) {
      audio.audioPosition.addListener(onPositionChanged);
      listeningPosition = true;
    }

    if (audio.playerEvents.lastEvent.processingState ==
        ProcessingState.completed) {
      if (listeningPosition) {
        audio.audioPosition.removeListener(onPositionChanged);
        listeningPosition = false;
      }
      setState(() {
        progress = 1;
      });
    }
  }

  @override
  void initState() {
    super.initState();
    audio.playerEvents.addListener(update);
    update();
  }

  @override
  void didUpdateWidget(covariant AudioPlayerTracker oldWidget) {
    super.didUpdateWidget(oldWidget);
    update();
  }

  @override
  void dispose() {
    audio.playerEvents.removeListener(update);
    if (listeningPosition) {
      audio.audioPosition.removeListener(onPositionChanged);
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return LinearProgressIndicator(value: progress);
  }
}

class AudioElement extends StatefulWidget {
  final Uint8List audioBytes;
  final String mimeType;
  final AudioModel audio;
  const AudioElement(
      {super.key,
      required this.audioBytes,
      required this.mimeType,
      required this.audio});

  @override
  State<AudioElement> createState() => _AudioElementState();
}

class _AudioElementState extends State<AudioElement> {
  AudioModel get audio => widget.audio;
  bool playing = false;
  double playSpeed = 1.0;

  static const List<double> speeds = [0.5, 0.75, 1.0, 1.25, 1.5, 1.75, 2];

  void playStop() async {
    if (playing) {
      try {
        audio.stop();
      } catch (exception) {
        showErrorSnackbar(this, "Unable to stop audio: $exception");
      }
    } else {
      try {
        await audio.player.setSpeed(playSpeed);
        await audio.playMemAudio(widget.mimeType, widget.audioBytes);
      } catch (exception) {
        showErrorSnackbar(this, "Unable to play audio: $exception");
      }
    }
  }

  void setPlaySpeed(double? v) {
    if (v == null) {
      return;
    }
    setState(() => playSpeed = v);
    if (playing) {
      audio.player.setSpeed(v);
    }
  }

  void updated() {
    var newPlaying = audio.playingSource == widget.audioBytes && audio.playing;
    if (playing != newPlaying) {
      setState(() {
        playing = newPlaying;
      });
    }
  }

  void saveOgg() async {
    var fname = await FilePicker.platform.saveFile(
          dialogTitle: "Select filename",
          fileName: "audio.ogg",
          bytes: widget.audioBytes,
        ) ??
        "";

    if (fname == "") {
      return;
    }

    // File(fname).writeAsBytesSync(widget.audioBytes);
    if (Platform.isAndroid) {
      showSuccessSnackbar(this, "Saved audio file");
    } else {
      showSuccessSnackbar(this, "Saved audio file $fname");
    }
  }

  Future<String> tempOggDir() async {
    bool isMobile = Platform.isIOS || Platform.isAndroid;
    String base = isMobile
        ? (await getApplicationCacheDirectory()).path
        : (await getDownloadsDirectory())?.path ?? "";
    return path.join(base, "audio");
  }

  void shareOgg() async {
    var fname = "audio.ogg";
    var dir = await tempOggDir();
    if (!Directory(dir).existsSync()) {
      Directory(dir).createSync(recursive: true);
    }
    fname = path.join(dir, fname);
    File(fname).writeAsBytesSync(widget.audioBytes);
    Share.shareXFiles([XFile(fname)]);
  }

  @override
  void initState() {
    super.initState();
    audio.addListener(updated);
    updated();
  }

  @override
  void didUpdateWidget(covariant AudioElement oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.audio != audio) {
      oldWidget.audio.removeListener(updated);
      audio.addListener(updated);
      updated();
    }
  }

  @override
  void dispose() {
    audio.removeListener(updated);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return (Row(children: [
      CircularProgressButton(
          active: playing,
          inactiveIcon: Icons.play_arrow,
          activeIcon: Icons.stop,
          onTapDown: playStop),
      const SizedBox(width: 20),
      Expanded(
          child:
              AudioPlayerTracker(audio: audio, audioBytes: widget.audioBytes)),
      const SizedBox(width: 20),
      DropdownButton(
        value: playSpeed,
        items: speeds
            .map((v) =>
                DropdownMenuItem<double>(value: v, child: Txt.S("${v}x")))
            .toList(),
        onChanged: setPlaySpeed,
      ),
      IconButton(onPressed: saveOgg, icon: Icon(Icons.download)),
      if (Platform.isAndroid)
        IconButton(onPressed: shareOgg, icon: Icon(Icons.share)),
    ]));
  }
}
