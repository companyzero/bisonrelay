import 'package:collection/collection.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';

class FileUploadModel extends ChatEvent with ChangeNotifier {
  final String uid;
  final String filepath;
  FileUploadModel({required this.uid, required this.filepath})
      : super(uid, 'Upload of $filepath');

  int _sentChunks = 0;
  int get sentChunks => _sentChunks;

  int _totalChunks = 0;
  int get totalChunks => _totalChunks;

  double get progress =>
      _totalChunks == 0 ? 0 : _sentChunks.toDouble() / _totalChunks.toDouble();

  String? _error;
  String? get error => _error;
  void _setError(String error) {
    if (_error != null) {
      _error = error;
      notifyListeners();
    }
  }

  bool _sent = false;
  bool get sent => _sent;
  void _markSent() {
    if (!_sent) {
      _sent = true;
      notifyListeners();
    }
  }

  void _updateProgress(SendProgress progress) {
    _sentChunks = progress.sent;
    _totalChunks = progress.total;
    _error ??= progress.error;
    notifyListeners();
  }
}

class UploadsModel extends ChangeNotifier {
  static UploadsModel of(BuildContext context, {bool listen = false}) =>
      Provider.of<UploadsModel>(context, listen: listen);

  UploadsModel() {
    _handleSendProgress();
  }

  final List<FileUploadModel> _uploads = [];
  FileUploadModel? _findUploadByArgs(SendFileArgs args) {
    return _uploads.firstWhereOrNull(
        (fu) => fu.uid == args.uid && fu.filepath == args.filepath);
  }

  FileUploadModel sendFile(String uid, String filepath) {
    var model = FileUploadModel(uid: uid, filepath: filepath);
    _uploads.add(model);
    (() async {
      try {
        await Golib.sendFile(uid, filepath);
      } catch (exception) {
        model._setError("$exception");
      } finally {
        model._markSent();
      }
    })();
    return model;
  }

  void _handleSendProgress() async {
    await for (var update in Golib.sendFileProgress()) {
      var fu = _findUploadByArgs(update.args);
      if (fu == null) {
        continue;
      }

      fu._updateProgress(update.progress);
    }
  }
}
