import 'dart:collection';

import 'package:emoji_picker_flutter/emoji_picker_flutter.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:retrieval/key_value_trie.dart';

final KeyValueTrie<List<Emoji>> emojiTrie = _initEmojiTrie();

KeyValueTrie<List<Emoji>> _initEmojiTrie() {
  final mp = <String, List<Emoji>>{};

  // Gather map of full words to list of emojis.
  for (var cat in defaultEmojiSet) {
    for (var e in cat.emoji) {
      var name = e.name.toLowerCase();
      var words = name.split(" ");

      // Add the emoji code replacing " " with "_".
      words.add(name.replaceAll(" ", "_"));

      for (var w in words) {
        if (mp.containsKey(w)) {
          mp[w]!.add(e);
        } else {
          mp[w] = [e];
        }
      }
    }
  }

  // Convert to trie for faster prefix search.
  final kv = KeyValueTrie<List<Emoji>>();
  for (var k in mp.keys) {
    kv.insert(k, mp[k]!);
  }

  return kv;
}

List<Emoji> findEmojis(String word) {
  try {
    var lists = emojiTrie.find(word);
    return lists.fold([], (res, v) {
      for (var e in v) {
        if (!res.contains(e)) {
          res.add(e);
        }
      }
      return res;
    });
  } catch (exception) {
    return [];
  }
}

final _lastEmojiRegExp = RegExp(r':(\b\w{2,})$');

String lastEmojiCodeFrom(String s) {
  var lastMatchIndex = s.lastIndexOf(_lastEmojiRegExp);
  if (lastMatchIndex == -1) {
    return "";
  }

  var match = _lastEmojiRegExp.firstMatch(s.substring(lastMatchIndex));
  if (match == null) return ""; // Should not happen.
  return match.group(1)!;
}

class TypingEmojiSelModel extends ChangeNotifier {
  static TypingEmojiSelModel of(BuildContext context, {listen = true}) =>
      Provider.of<TypingEmojiSelModel>(context, listen: listen);

  List<Emoji> _selectionList = [];
  Iterable<Emoji> get selectionList => UnmodifiableListView(_selectionList);
  bool get isTypingEmoji => _selectionList.isNotEmpty;

  ValueNotifier<int> selected = ValueNotifier(-1);
  String _lastEmojiCode = "";
  String get lastEmojiCode => _lastEmojiCode;

  Emoji? get selectedEmoji =>
      selected.value > -1 && selected.value < _selectionList.length
          ? _selectionList[selected.value]
          : null;

  void clearSelection() {
    if (_selectionList.isEmpty) {
      return;
    }

    _selectionList.clear();
    selected.value = -1;
    _lastEmojiCode = "";
    notifyListeners();
  }

  void maybeSelectEmojis(TextEditingController controller) {
    var sel = controller.selection;
    if (sel.start != sel.end) {
      clearSelection();
      return;
    }

    var before = controller.selection.textBefore(controller.text);
    var emojiCode = lastEmojiCodeFrom(before);

    // Require 2 chars to start searching.
    if (emojiCode.length < 2) {
      clearSelection();
      return;
    }

    var emojis = findEmojis(emojiCode);
    if (emojis.isEmpty) {
      clearSelection();
      return;
    }

    _selectionList = emojis;
    _lastEmojiCode = emojiCode;
    notifyListeners();
    selected.value = 0;
  }

  void changeSelection(int delta) {
    int newSel = selected.value + delta;
    if (newSel < 0 || newSel >= _selectionList.length) {
      return;
    }

    selected.value = newSel;
  }

  String replaceTypedEmojiCode(TextEditingController controller) {
    if (selectedEmoji == null) {
      return "";
    }

    var emoji = selectedEmoji!;

    var sel = controller.selection;
    if (sel.start != sel.end) {
      clearSelection();
      return "";
    }

    var before = controller.selection.textBefore(controller.text);
    var after = controller.selection.textAfter(controller.text);
    var emojiCode = lastEmojiCodeFrom(before);

    var emojiCodeStartIndex = before.lastIndexOf(":$emojiCode");
    if (emojiCodeStartIndex == -1) return ""; // Should not happen.

    before = before.substring(0, emojiCodeStartIndex);
    var newText = "$before${emoji.emoji}$after";
    return newText;
  }
}
