// ignore_for_file: avoid_return_types_on_setters

import 'dart:collection';

import 'package:flutter/foundation.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';

class FileDownloadModel extends ChangeNotifier {
  final String uid;
  final String fid;
  ReceivedFile _rf;
  FileDownloadModel(this.uid, this.fid, this._rf, this._diskPath);

  ReceivedFile get rf => _rf;
  set rf(ReceivedFile v) {
    _rf = v;
    notifyListeners();
  }

  String _diskPath = "";
  String get diskPath => _diskPath;
  set diskPath(String v) {
    _diskPath = v;
    notifyListeners();
  }

  double _progress = 0;
  double get progress => _progress;
  void set progress(double v) {
    _progress = v;
    notifyListeners();
  }
}

class UnknownDownload {
  final String uid;
  final String fid;

  UnknownDownload(this.uid, this.fid);
}

class DownloadsModel extends ChangeNotifier {
  DownloadsModel() {
    _loadDownloads();
    _handleCompletedDownloads();
    _handleDownloadProgress();
  }

  final List<FileDownloadModel> _downloads = [];
  Iterable<FileDownloadModel> get downloads => UnmodifiableListView(_downloads);

  int _findDownload(String uid, String fid) {
    return _downloads.indexWhere((e) => e.uid == uid && e.fid == fid);
  }

  FileDownloadModel? getDownload(String uid, String fid) {
    var idx = _findDownload(uid, fid);
    if (idx < 0) {
      return null;
    }
    return _downloads[idx];
  }

  void ensureDownloadExists(String uid, String fid, FileMetadata fm) {
    var idx = _findDownload(uid, fid);
    if (idx > -1) {
      return;
    }

    var fdm = FileDownloadModel(uid, fid, ReceivedFile(fid, uid, "", fm), "");
    _downloads.add(fdm);
    notifyListeners();
  }

  // unknownDownloads are the downloads for which the local client doesn't have
  // metadata information yet.
  List<UnknownDownload> unknownDownloads = [];

  Future<FileDownloadModel> getUserFile(ReceivedFile f) async {
    int idx = _findDownload(f.uid, f.fid);
    final FileDownloadModel res;
    if (idx == -1) {
      res = FileDownloadModel(f.uid, f.fid, f, f.diskPath);
    } else {
      res = _downloads[idx];
    }
    await Golib.getUserContent(f.uid, f.fid);
    if (idx == -1) {
      _downloads.add(res);
      notifyListeners();
    }
    return res;
  }

  // getUnknownUserFile starts the download process for a file for which the
  // local client does not yet have metadata information.
  Future<void> getUnknownUserFile(String uid, String fid) async {
    if (unknownDownloads.indexWhere((v) => v.uid == uid && v.fid == fid) ==
        -1) {
      unknownDownloads.add(UnknownDownload(uid, fid));
    }
    await Golib.getUserContent(uid, fid);
  }

  // _removeDownload locally removes the download info.
  void _removeDownload(String uid, String fid) {
    unknownDownloads.removeWhere((v) => v.uid == uid && v.fid == fid);
    _downloads.removeWhere((v) => v.uid == uid && v.fid == fid);
    notifyListeners();
  }

  Future<void> confirmFileDownload(String uid, String fid, bool confirm) async {
    if (!confirm) {
      _removeDownload(uid, fid);
    }
    await Golib.confirmFileDownload(fid, confirm);
  }

  void _loadDownloads() async {
    var res = await Golib.listDownloads();
    for (var e in res) {
      var rf = ReceivedFile(e.fid, e.uid, e.diskPath, e.metadata);
      var f = FileDownloadModel(e.uid, e.fid, rf, e.diskPath);
      _downloads.add(f);
    }
    notifyListeners();
  }

  void _handleCompletedDownloads() async {
    var stream = Golib.downloadsCompleted();
    await for (var update in stream) {
      var idx = _findDownload(update.uid, update.fid);
      if (idx > -1) {
        _downloads[idx].diskPath = update.diskPath;
      }
    }
  }

  void _handleDownloadProgress() async {
    var stream = Golib.downloadProgress();
    await for (var update in stream) {
      var idx = _findDownload(update.uid, update.fid);
      if (idx < 0) {
        continue;
      }
      var f = _downloads[idx];
      var nbChunks = update.metadata.manifest.length;
      if (f.rf.metadata == null) {
        // Received metadata. Update in the model.
        f.rf = f.rf.cloneWithMeta(f.rf.metadata);
      }
      f.progress = (nbChunks - update.nbMissingChunks) / nbChunks;
    }
  }

  Future<void> cancelDownload(String uid, String fid) async {
    await Golib.cancelDownload(fid);
    final idx = _findDownload(uid, fid);
    if (idx > -1) {
      _downloads.removeAt(idx);
      notifyListeners();
    }
  }
}
