import 'dart:collection';

import 'package:flutter/cupertino.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';

class FeedCommentModel extends ChangeNotifier {
  String _nick;
  String get nick => _nick;
  void set nick(String v) {
    this._nick = nick;
    notifyListeners();
  }

  int _level = 0;
  int get level => _level;
  void set level(int v) {
    _level = v;
    notifyListeners();
  }

  final String comment;
  final String uid;
  final String parentID;
  final String id;
  final String timestamp;
  FeedCommentModel(this.id, this.uid, this.comment,
      {nick = "", this.parentID = "", this.timestamp = ""})
      : _nick = nick;
}

class FeedPostModel extends ChangeNotifier {
  final PostSummary summ;
  FeedPostModel(this.summ);

  String content = "";

  Future<void> readPost() async {
    var pm = await Golib.readPost(summ.from, summ.id);
    content = pm.attributes[RMPMain] ?? "";
    notifyListeners();
  }

  Future<FeedCommentModel> _statusToComment(PostMetadataStatus pms) async {
    var nick = pms.attributes[RMPFromNick] ?? "[${pms.from}]";
    var parentID = pms.attributes[RMPParent] ?? "";
    var timestamp = pms.attributes[RMPTimestamp] ?? "";

    return FeedCommentModel(
        pms.hash(), pms.from, pms.attributes[RMPSComment] ?? "",
        nick: nick, parentID: parentID, timestamp: timestamp);
  }

  List<FeedCommentModel> _comments = [];
  UnmodifiableListView<FeedCommentModel> get comments =>
      UnmodifiableListView(_comments);
  Future<void> readComments() async {
    var status = await Golib.listPostStatus(summ.from, summ.id);

    // Fetch the comments.
    var newComments = await Future.wait(status
        .where((e) => e.attributes[RMPSComment] != "")
        .map(_statusToComment)
        .toList());

    // Sort by thread. First, build a map of comment by parent.
    List<FeedCommentModel> roots = [];
    var cmap = Map<String, FeedCommentModel>();
    var children = Map<String, List<FeedCommentModel>>();
    newComments.forEach((c) {
      cmap[c.id] = c;
      if (c.parentID == "") {
        roots.add(c);
        return;
      }
      var pc = cmap[c.parentID];
      if (pc == null) {
        // Comment without knowing parent.
        roots.add(c);
        return;
      }
      c.level = pc.level + 1;
      if (children.containsKey(c.parentID)) {
        children[c.parentID]!.add(c);
      } else {
        children[c.parentID] = [c];
      }
    });

    // Process comment threads, starting with top level comments.
    List<FeedCommentModel> sorted = [];
    var stack = roots;
    for (stack = roots.reversed.toList(); stack.isNotEmpty;) {
      var el = stack.removeLast();
      sorted.add(el);
      var cs = children[el.id];
      if (cs == null) {
        continue;
      }
      stack.addAll(cs.reversed);
    }

    _comments = sorted;
    notifyListeners();
  }

  List<String> _newComments = [];
  Iterable<String> get newComments => UnmodifiableListView(_newComments);

  void addNewComment(String comment) {
    _newComments.add(comment);
    notifyListeners();
  }

  void addReceivedStatus(PostMetadataStatus ps, bool mine) async {
    if (ps.attributes[RMPSComment] == "") {
      // Not a comment. Nothing to do.
      return;
    }

    // Figure out where to insert the comment or add a new top-level comment.
    var c = await _statusToComment(ps);
    var idx = _comments.indexWhere((e) => e.id == c.parentID);
    if (idx < 0) {
      _comments.add(c);
    } else {
      // Find where to insert. Need to insert before the next comment that is at
      // the same level as the parent (or lower).
      var level = _comments[idx].level;
      c.level = level + 1;
      int insertIdx;
      for (insertIdx = idx + 1; insertIdx < _comments.length; insertIdx++) {
        if (_comments[insertIdx].level <= level) {
          break;
        }
      }
      _comments.insert(insertIdx, c);
    }

    // Drop from list of unreplicated comments if this status update is mine.
    if (mine) {
      var idx = _newComments.indexWhere((e) => e == ps.attributes[RMPSComment]);
      if (idx > -1) {
        _newComments.removeAt(idx);
      }
    }

    notifyListeners();
  }

  // Whether this post was replaced by the author version of the post in the client.
  bool _replacedByAuthorVersion = false;
  bool get replacedByAuthorVersion => _replacedByAuthorVersion;
  void _replaceByAuthorVersion() {
    _replacedByAuthorVersion = true;
    notifyListeners();
  }
}

class FeedModel extends ChangeNotifier {
  List<FeedPostModel> _posts = [];
  Iterable<FeedPostModel> get posts => UnmodifiableListView(_posts);

  void _handleFeedPosts() async {
    // List existing posts before listening for new posts.
    var oldPosts = await Golib.listPosts();
    oldPosts.sort((PostSummary a, b) => b.date.compareTo(a.date));
    oldPosts.forEach((p) {
      _posts.add(FeedPostModel(p));
    });
    notifyListeners();

    var stream = Golib.postsFeed();
    await for (var msg in stream) {
      // Add at the start of the feed so it appears at the top of the feed page.
      _posts.insert(0, FeedPostModel(msg));

      // Handle posts that replace a previously relayed post: the client removes
      // the relayed post in favor of the one by the author, so remove such posts
      // from the list.
      if (msg.from == msg.authorID) {
        _posts.removeWhere((e) {
          var remove = e.summ.id == msg.id && e.summ.from != msg.authorID;
          if (remove) {
            e._replaceByAuthorVersion();
          }
          return remove;
        });
      }

      notifyListeners();
    }
  }

  void _handlePostStatus() async {
    var stream = Golib.postStatusFeed();
    await for (var msg in stream) {
      // Find the post.
      var postIdx = _posts.indexWhere(
          (p) => p.summ.from == msg.postFrom && p.summ.id == msg.pid);
      if (postIdx > -1) {
        var post = _posts[postIdx];
        post.addReceivedStatus(msg.status, true);
      }
    }
  }

  Future<void> createPost(String content) async {
    var newPost = await Golib.createPost(content);
    _posts.insert(0, FeedPostModel(newPost));
    notifyListeners();
  }

  FeedPostModel? getPost(String fromID, String pid) {
    var idx =
        _posts.indexWhere((e) => e.summ.from == fromID && e.summ.id == pid);
    return idx == -1 ? null : _posts[idx];
  }

  FeedModel() {
    _handleFeedPosts();
    _handlePostStatus();
  }
}
