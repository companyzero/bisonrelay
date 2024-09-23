import 'dart:collection';

import 'package:bruig/util.dart';
import 'package:collection/collection.dart';
import 'package:flutter/cupertino.dart';
import 'package:bruig/notification_service.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';

class FeedCommentModel extends ChangeNotifier {
  String _nick;
  String get nick => _nick;
  set nick(String v) {
    _nick = nick;
    notifyListeners();
  }

  bool _unreadComment = false;
  bool get unreadComment => _unreadComment;
  set unreadComment(bool v) {
    _unreadComment = v;
    notifyListeners();
  }

  int _level = 0;
  int get level => _level;
  set level(int v) {
    _level = v;
    notifyListeners();
  }

  final List<FeedCommentModel> _children = [];
  UnmodifiableListView<FeedCommentModel> get children =>
      UnmodifiableListView(_children);

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

  DateTime _lastStatusTS = DateTime.fromMillisecondsSinceEpoch(0);
  DateTime get lastStatusTS => _lastStatusTS;
  set lastStatusTS(DateTime ts) {
    _lastStatusTS = ts;
    notifyListeners();
  }

  bool _hasUnreadComments = false;
  bool get hasUnreadComments => _hasUnreadComments;
  set hasUnreadComments(bool b) {
    _hasUnreadComments = b;
    notifyListeners();
  }

  bool _hasUnreadPost = false;
  bool get hasUnreadPost => _hasUnreadPost;
  set hasUnreadPost(bool b) {
    _hasUnreadPost = b;
    notifyListeners();
  }

  bool _active = false;
  bool get active => _active;
  void _setActive(bool b) {
    _active = b;
    _hasUnreadPost = false;
    _hasUnreadComments = false;
    notifyListeners();
  }

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
    var cmap = {};
    var children = {};
    for (var c in newComments) {
      cmap[c.id] = c;
      if (c.parentID == "") {
        roots.add(c);
        continue;
      }
      var pc = cmap[c.parentID];
      if (pc == null) {
        // Comment without knowing parent.
        roots.add(c);
        continue;
      }
      c.level = pc.level + 1;
      if (children.containsKey(c.parentID)) {
        children[c.parentID]!.add(c);
      } else {
        children[c.parentID] = [c];
      }
    }

    // Process comment threads, starting with top level comments.
    List<FeedCommentModel> sorted = [];
    var stack = roots;
    for (stack = roots.reversed.toList(); stack.isNotEmpty;) {
      var el = stack.removeLast();
      if (el.level == 0) sorted.add(el);
      var cs = children[el.id];
      if (cs == null) {
        continue;
      }
      el._children.addAll(cs);

      stack.addAll(cs.reversed);
    }
    _comments = sorted;
    notifyListeners();
  }

  final List<String> _newComments = [];
  Iterable<String> get newComments => UnmodifiableListView(_newComments);

  void addNewComment(String comment) {
    _newComments.add(comment);
    notifyListeners();
  }

  FeedCommentModel? _findParent(String id, List<FeedCommentModel> comments) {
    var idx = comments.where((e) => e.id == id);
    if (idx.isNotEmpty) {
      return idx.first;
    }
    for (FeedCommentModel el in comments) {
      var parent = _findParent(id, el.children);
      if (parent != null) {
        return parent;
      }
    }
    return null;
  }

  Future<void> addReceivedStatus(
      PostMetadataStatus ps, bool mine, PostSummary post) async {
    if (ps.attributes[RMPSComment] == "") {
      // Not a comment. Nothing to do.
      return;
    }
    _hasUnreadComments = true;

    // Figure out where to insert the comment or add a new top-level comment.
    var c = await _statusToComment(ps);
    c.unreadComment = true;
    if (c.parentID == "") {
      _comments.add(c);
    } else {
      var parent = _findParent(c.parentID, _comments);
      if (parent != null) {
        c.level = parent.level + 1;
        parent._children.add(c);
      } else {
        _comments.add(c);
      }
    }
    NotificationService().showPostCommentNotification(post, c.nick, c.comment);
    /*
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
    */

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

class NewPostModel {
  Map<String, String> embedContents = {};
  String content = "";

  void clear() {
    content = "";
    embedContents = {};
  }

  // Returns the new ID of the tracked embed.
  String trackEmbed(String data) {
    var id = generateRandomString(12);
    while (embedContents.containsKey(id)) {
      id = generateRandomString(12);
    }
    embedContents[id] = data;
    return id;
  }

  // Returns the actual full content that will be included in the post.
  String getFullContent() {
    // Replace embedded content with actual content.
    var fc = content;
    final pattern = RegExp(r"(--embed\[.*data=)\[content ([a-zA-Z0-9]{12})]");
    fc = fc.replaceAllMapped(pattern, (match) {
      var embed = embedContents[match.group(2)];
      if (embed == null) {
        throw "Content not found: ${match.group(2)}";
      }
      return match.group(1)! + embed;
    });
    return fc;
  }
}

class FeedModel extends ChangeNotifier {
  final List<FeedPostModel> _posts = [];
  Iterable<FeedPostModel> get posts => UnmodifiableListView(_posts);

  bool _hasUnreadPostsComments = false;
  bool get hasUnreadPostsComments => _hasUnreadPostsComments;
  set hasUnreadPostsComments(bool b) {
    _hasUnreadPostsComments = b;
    notifyListeners();
  }

  final List<String> _downloadingUserPosts = [];
  bool dowloadingUserPost(String pid) => _downloadingUserPosts.contains(pid);

  Future<void> getUserPost(String authorId, String postId) async {
    if (!_downloadingUserPosts.contains(postId)) {
      _downloadingUserPosts.add(postId);
      notifyListeners();
    }
    await Golib.getUserPost(authorId, postId);
  }

  FeedPostModel? _active;
  FeedPostModel? get active => _active;

  set active(FeedPostModel? f) {
    _active?._setActive(false);
    _active = f;
    f?._setActive(true);

    // Check for unreadPostsAndComments so we can turn off sidebar notification
    bool unread = false;
    for (int i = 0; i < _posts.length; i++) {
      if (_posts[i].hasUnreadComments || _posts[i]._hasUnreadPost) {
        unread = true;
      }
    }
    _hasUnreadPostsComments = unread;
    notifyListeners();
  }

  void _handleFeedPosts() async {
    // List existing posts before listening for new posts.
    var oldPosts = await Golib.listPosts();
    oldPosts.sort((PostSummary a, b) => b.date.compareTo(a.date));
    for (var p in oldPosts) {
      var newPost = FeedPostModel(p);
      await newPost.readComments();
      newPost.lastStatusTS = newPost.summ.lastStatusTS;
      await newPost.readPost();
      _posts.add(newPost);
    }
    _posts.sort(sortFeedPosts);
    notifyListeners();

    var stream = Golib.postsFeed();
    await for (var msg in stream) {
      // Add at the start of the feed so it appears at the top of the feed page.
      var newPost = FeedPostModel(msg);
      newPost.hasUnreadPost = true;
      hasUnreadPostsComments = true;
      newPost.lastStatusTS = newPost.summ.lastStatusTS;
      await newPost.readPost();
      _posts.insert(0, newPost);
      if (_downloadingUserPosts.contains(newPost.summ.id)) {
        _downloadingUserPosts.remove(newPost.summ.id);
      }

      NotificationService().showPostNotification(newPost.summ);
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
        hasUnreadPostsComments = true;
        post.lastStatusTS = DateTime.now();
        await post.addReceivedStatus(msg.status, true, post.summ);
      }
      _posts.sort(sortFeedPosts);
    }
    notifyListeners();
  }

  final NewPostModel newPost = NewPostModel();

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

  // Sorting algo to attempt to organize posts
  int sortFeedPosts(FeedPostModel a, FeedPostModel b) {
    var idt = a.lastStatusTS.millisecondsSinceEpoch;
    if (idt <= 0) {
      idt = a.summ.date.toLocal().millisecondsSinceEpoch;
    }
    var jdt = b.lastStatusTS.millisecondsSinceEpoch;
    if (jdt <= 0) {
      jdt = b.summ.date.toLocal().millisecondsSinceEpoch;
    }
    return jdt.compareTo(idt);
  }

  FeedModel() {
    _handleFeedPosts();
    _handlePostStatus();
  }
}
