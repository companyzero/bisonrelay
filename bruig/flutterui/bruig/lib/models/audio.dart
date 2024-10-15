import 'dart:io';

import 'package:bruig/storage_manager.dart';
import 'package:bruig/util.dart';
import 'package:flutter/foundation.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:just_audio/just_audio.dart';
import 'package:permission_handler/permission_handler.dart';

dynamic globalAudioPlayerInitError;

class _MemAudioSource extends StreamAudioSource {
  final Uint8List bytes;
  final String contentType;
  _MemAudioSource(this.contentType, this.bytes);

  @override
  Future<StreamAudioResponse> request([int? start, int? end]) async {
    start ??= 0;
    end ??= bytes.length;
    return StreamAudioResponse(
      sourceLength: bytes.length,
      contentLength: end - start,
      offset: start,
      stream: Stream.value(bytes.sublist(start, end)),
      contentType: contentType,
    );
  }
}

class AudioPlayerEventsModel extends ChangeNotifier {
  PlaybackEvent _lastEvent = PlaybackEvent();
  PlaybackEvent get lastEvent => _lastEvent;
  void _update(PlaybackEvent event) {
    _lastEvent = event;
    notifyListeners();
  }
}

class AudioPositionModel extends ChangeNotifier {
  Duration _length = const Duration();
  Duration get length => _length;
  Duration _position = const Duration();
  Duration get position => _position;

  void _setLength(Duration d) {
    _length = d;
    notifyListeners();
  }

  void _setPosition(Duration d) {
    _position = d;
    notifyListeners();
  }

  void _reset(Duration newLength) {
    _length = newLength;
    _position = const Duration();
    notifyListeners();
  }
}

class AudioModel extends ChangeNotifier {
  AudioModel() {
    if (globalAudioPlayerInitError == null) _initPlayer();
    _loadDefaultDeviceIds();
  }

  String _captureDeviceId = "";
  String get captureDeviceId => _captureDeviceId;
  set captureDeviceId(String v) {
    _captureDeviceId = v;
    notifyListeners();
    StorageManager.saveString(StorageManager.audioCaptureDeviceIdKey, v);
  }

  String _playbackDeviceId = "";
  String get playbackDeviceId => _playbackDeviceId;
  set playbackDeviceId(String v) {
    _playbackDeviceId = v;
    notifyListeners();
    StorageManager.saveString(StorageManager.audioPlaybackDeviceIdKey, v);
  }

  void _loadDefaultDeviceIds() async {
    var captureId =
        await StorageManager.readString(StorageManager.audioCaptureDeviceIdKey);
    var playbackId = await StorageManager.readString(
        StorageManager.audioPlaybackDeviceIdKey);

    // Select the devices based on: 1. user preference (if device exists),
    // 2: default ID (if any device exists), 3: leave empty (use whatever is
    // default at the time of capture/playback).
    var devs = await Golib.listAudioDevices();
    if (devs.capture.any((d) => d.id == captureId)) {
      _captureDeviceId = captureId;
    } else {
      var i = devs.capture.indexWhere((d) => d.isDefault);
      if (i > -1) {
        _captureDeviceId = devs.capture[i].id;
      }
    }
    if (devs.playback.any((d) => d.id == playbackId)) {
      _playbackDeviceId = playbackId;
    } else {
      var i = devs.playback.indexWhere((d) => d.isDefault);
      if (i > -1) {
        _playbackDeviceId = devs.playback[i].id;
      }
    }

    notifyListeners();
  }

  bool _recording = false;
  bool get recording => _recording;

  bool? _hasMicPermission;

  bool _lockedRecording = false;
  bool get lockedRecording => _lockedRecording;
  set lockedRecording(bool v) {
    _lockedRecording = v;
    notifyListeners();
  }

  bool _playing = false;
  bool get playing => _playing;

  bool _hasRecord = false;
  bool get hasRecord => _hasRecord;

  RecordedAudioNote? _lastRecord;
  RecordedAudioNote? get lastRecord => _lastRecord;

  void clearRecorded() {
    _hasRecord = false;
    _lastRecord = null;
    _lockedRecording = false;
    notifyListeners();
  }

  DateTime _startRecordTime = DateTime.now();
  DateTime get startRecordTime => _startRecordTime;

  Future<void> recordNote() async {
    if (recording) {
      throw "Already recording";
    }
    if (playing) {
      await stop();
    }

    if (_hasMicPermission == null) {
      if (Platform.isAndroid || Platform.isIOS) {
        _hasMicPermission = await Permission.microphone.request().isGranted;
      } else {
        _hasMicPermission = true;
      }
    }
    if (_hasMicPermission != null && !_hasMicPermission!) {
      throw "App denied microphone permission";
    }

    _hasRecord = false;
    _recording = true;
    _startRecordTime = DateTime.now();
    _lastRecord = null;
    _lockedRecording = false;
    notifyListeners();

    var args = AudioRecordNoteArgs(captureDeviceId);
    try {
      await Golib.startAudioNoteRecord(args);
      var newHasRecord =
          DateTime.now().difference(startRecordTime).inSeconds > 0;
      if (newHasRecord) {
        _lastRecord = await Golib.audioNoteEmbed();
      }

      _recording = false;
      _hasRecord = newHasRecord;
      _lockedRecording = false;
      notifyListeners();
    } catch (exception) {
      _recording = false;
      _lockedRecording = false;
      notifyListeners();
      rethrow;
    }
  }

  Future<void> playbackNote() async {
    if (recording) {
      throw "Cannot playback while recording";
    }
    if (playing) {
      await stop();
    }

    _playing = true;
    _playingSource = null;
    notifyListeners();
    try {
      await Golib.startAudioNotePlayback(playbackDeviceId);
      _playing = false;
      notifyListeners();
    } catch (exception) {
      _playing = false;
      notifyListeners();
      rethrow;
    }
  }

  Future<void> stop() async {
    if (player.playing) {
      await player
          .pause(); // Pause instead of stop() because it reduces latency in next play().
      await player.seek(const Duration());
    } else {
      _lockedRecording = false;
      await Golib.stopAudioNote();
    }
  }

  final playlist = ConcatenatingAudioSource(children: []);
  final AudioPlayer player = AudioPlayer();
  dynamic _playingSource;
  dynamic get playingSource => _playingSource;
  final AudioPlayerEventsModel playerEvents = AudioPlayerEventsModel();
  final AudioPositionModel audioPosition = AudioPositionModel();

  void _initPlayer() async {
    player.playingStream.listen(_handlePlayingEvents);
    player.playbackEventStream.listen(_handlePlayerEvents);
    player.createPositionStream().listen(_handlePositionEvents);
    await playlist
        .add(SilenceAudioSource(duration: const Duration(milliseconds: 1)));
    await player.setAudioSource(playlist);
    await player.load();
    await player.play();
    (() async {
      // Needed on windows because SilenceAudioSource fails to stop automatically.
      await sleep(const Duration(seconds: 1));
      stop();
    })(); // Force stop player.
  }

  void _handlePlayingEvents(bool newPlaying) {
    if (newPlaying != _playing) {
      _playing = newPlaying;
      notifyListeners();
    }
  }

  void _handlePlayerEvents(PlaybackEvent event) async {
    // if (event.processingState != ProcessingState.ready) {
    //   debugPrint("XXXXX audio event ${event.toString()}");
    // }

    // var idleAfterCompleted =
    //     playerEvents.lastEvent.processingState == ProcessingState.completed &&
    //         event.processingState == ProcessingState.idle;

    playerEvents._update(event);

    if (Platform.isAndroid &&
        _playing &&
        event.processingState == ProcessingState.completed) {
      // Workaround android bug where playing=false event is never received.
      //_playing = false;
      //notifyListeners();
      (() async {
        await sleep(const Duration(milliseconds: 1));
        stop();
      })();
    }

    if (event.duration != null) {
      audioPosition._setLength(event.duration!);
    }

    // if (idleAfterCompleted) {
    //   (() async {
    //     // await player.stop();
    //     // await sleep(const Duration(milliseconds: 1));
    //     await player.seek(Duration.zero, index: 0);
    //   })();
    // }
  }

  void _handlePositionEvents(Duration d) {
    audioPosition._setPosition(d);
  }

  bool _playlistItemIs(Uint8List data) {
    if (playlist.children.isEmpty) {
      return false;
    }
    if (playlist.children[0] is! _MemAudioSource) {
      return false;
    }
    return (playlist.children[0] as _MemAudioSource).bytes == data;
  }

  Future<void> playMemAudio(String contentType, Uint8List data) async {
    if (globalAudioPlayerInitError != null) {
      throw "Audio player init error: $globalAudioPlayerInitError";
    }

    if (playing) {
      await stop();
    }

    _playingSource = data;
    notifyListeners();

    try {
      if (!_playlistItemIs(data)) {
        await playlist.clear();
        await playlist.add(_MemAudioSource(contentType, data));
        var length = await player.load();
        audioPosition._reset(length ?? const Duration());
      } else {
        // Already loaded this audio file.
        audioPosition._setPosition(const Duration());
        // await player.seek(const Duration(), index: 0);
      }
      await player.play();
    } catch (exception) {
      _playingSource = null;
      notifyListeners();
      rethrow;
    }
  }
}
