import 'package:bruig/models/client.dart';
import 'package:collection/collection.dart';
import 'package:flutter/material.dart';

class UserSelectionModel extends ChangeNotifier {
  final List<ChatModel> _selected = [];
  bool allowMultiple;
  UnmodifiableListView<ChatModel> get selected =>
      UnmodifiableListView(_selected);

  UserSelectionModel({this.allowMultiple = false});

  bool toggle(ChatModel chat) {
    if (_selected.contains(chat)) {
      del(chat);
      return false;
    } else {
      add(chat);
      return true;
    }
  }

  bool contains(ChatModel chat) => _selected.contains(chat);

  void add(ChatModel chat) {
    if (!_selected.contains(chat)) {
      if (!allowMultiple) _selected.clear();
      _selected.add(chat);
      notifyListeners();
    }
  }

  void del(ChatModel chat) {
    if (_selected.contains(chat)) {
      _selected.remove(chat);
      notifyListeners();
    }
  }
}
