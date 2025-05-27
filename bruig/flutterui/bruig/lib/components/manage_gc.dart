import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/confirmation_dialog.dart';
import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/icons.dart';
import 'package:bruig/components/info_grid.dart';
import 'package:bruig/components/inputs.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/components/usersearch/user_search_model.dart';
import 'package:bruig/components/usersearch/user_search_panel.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/realtimechat.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/theme_manager.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:tuple/tuple.dart';

class ManageGCScreen extends StatefulWidget {
  final ClientModel client;
  final RealtimeChatModel rtc;
  final ChatModel chat;
  const ManageGCScreen(this.client, this.rtc, this.chat, {super.key});

  @override
  State<ManageGCScreen> createState() => _ManageGCScreenState();
}

class _InviteUserPanel extends StatefulWidget {
  final ClientModel client;
  final ChatModel gc;
  final List<String> memberUIDs;
  final VoidCallback goBack;
  const _InviteUserPanel(this.client, this.gc, this.memberUIDs, this.goBack);

  @override
  State<_InviteUserPanel> createState() => _InviteUserPanelState();
}

class _InviteUserPanelState extends State<_InviteUserPanel> {
  bool loading = false;
  ClientModel get client => widget.client;
  ChatModel get gc => widget.gc;
  UserSelectionModel userSel = UserSelectionModel(allowMultiple: true);

  void inviteUsers() async {
    if (loading) return;
    if (userSel.selected.isEmpty) {
      widget.goBack();
      return;
    }

    setState(() => loading = true);
    var snackbar = SnackBarModel.of(context);

    try {
      var selected = userSel.selected;
      for (var user in selected) {
        await Golib.inviteToGC(InviteToGC(gc.id, user.id));
      }
      if (selected.length == 1) {
        snackbar.success('Sent invitation to ${selected[0].nick}');
      } else {
        snackbar.success('Sent invitations to ${selected.length} users');
      }
      widget.goBack();
    } catch (exception) {
      snackbar.error('Unable to invite: $exception');
      setState(() => loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(10),
      child: Column(children: [
        Txt.L("Invite to GC ${gc.nick}"),
        const SizedBox(height: 10),
        Expanded(
            child: UserSearchPanel(
          client,
          userSelModel: userSel,
          targets: UserSearchPanelTargets.users,
          searchInputHintText: "Search for users",
          confirmLabel: "Confirm Invitation",
          excludeUIDs: widget.memberUIDs,
          onCancel: widget.goBack,
          onConfirm: !loading ? inviteUsers : null,
        ))
      ]),
    );
  }
}

class _ChangeGCOwnerPanel extends StatefulWidget {
  final ClientModel client;
  final ChatModel gc;
  final List<ChatModel> users;
  final VoidCallback goBack;
  const _ChangeGCOwnerPanel(this.client, this.gc, this.users, this.goBack);

  @override
  State<_ChangeGCOwnerPanel> createState() => __ChangeGCOwnerPanelState();
}

class __ChangeGCOwnerPanelState extends State<_ChangeGCOwnerPanel> {
  bool loading = false;
  ClientModel get client => widget.client;
  ChatModel get gc => widget.gc;
  UserSelectionModel userSel = UserSelectionModel(allowMultiple: false);
  ChatModel? get newOwner =>
      userSel.selected.isNotEmpty ? userSel.selected[0] : null;

  void doChangeOwner() async {
    setState(() => loading = true);
    try {
      await Golib.modifyGCOwner(gc.id, newOwner!.id);
      widget.goBack();
    } catch (exception) {
      showErrorSnackbar(this, "Unable to modify GC Owner: $exception");
      setState(() => loading = false);
    }
  }

  void confirmNewOwner() async {
    if (newOwner == null) {
      return;
    }

    showModalBottomSheet(
        context: context,
        builder: (BuildContext context) => Container(
              padding: const EdgeInsets.all(30),
              child: Row(children: [
                Text(
                    "Really set '${newOwner!.nick}' as new GC owner? This CANNOT be undone"),
                const SizedBox(width: 10, height: 10),
                OutlinedButton(
                  onPressed: () => Navigator.pop(context),
                  child: const Text("No"),
                ),
                const SizedBox(width: 10, height: 10),
                OutlinedButton(
                    onPressed: () {
                      Navigator.pop(context);
                      // widget.changeOwner(newOwner);
                      doChangeOwner();
                    },
                    child: const Text("Yes")),
              ]),
            ));
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(10),
      child: Column(children: [
        Txt.L("Change GC ${gc.nick} Owner"),
        const SizedBox(height: 10),
        Expanded(
            child: UserSearchPanel(
          client,
          userSelModel: userSel,
          targets: UserSearchPanelTargets.users,
          sourceChats: widget.users,
          searchInputHintText: "Search for user",
          confirmLabel: "Confirm Change Owner",
          onCancel: widget.goBack,
          onConfirm: !loading && newOwner != null ? confirmNewOwner : null,
          onChatTapped: (c) => setState(() {}),
        ))
      ]),
    );
  }
}

enum _ScreenState {
  managing,
  inviting,
  changingOwner,
}

class _ManageGCScreenState extends State<ManageGCScreen> {
  // This must be updated every time a new GC version is deployed and its features
  // implemented in bruig.
  // ignore: non_constant_identifier_names
  final MAXGCVERSION = 1;

  bool loading = false;
  String get gcID => widget.chat.id;
  String get gcName => widget.chat.nick;
  String gcOwner = "";
  String inviteNick = "";
  int gcVersion = 0;
  int gcGeneration = 0;
  DateTime gcTimestamp = DateTime.fromMillisecondsSinceEpoch(0);
  List<ChatModel> users = [];
  Map<String, dynamic> blockedUsers = {};
  List<String> memberUIDs = [];
  ChatModel? userToInvite;
  bool localIsAdmin = false;
  bool localIsOwner = false;
  bool firstLoading = true;
  Map<String, bool> admins = {};
  List<String> unkxdMembers = [];
  _ScreenState state = _ScreenState.managing;
  String rtdtRV = "";

  @override
  void initState() {
    super.initState();
    reloadUsers();
  }

  @override
  void didUpdateWidget(ManageGCScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    reloadUsers();
    if (widget.chat != oldWidget.chat) {
      state = _ScreenState.managing;
    }
  }

  bool isBlocked(String uid) {
    var v = blockedUsers[uid];
    return v != null;
  }

  bool isGCAdmin(String uid) => admins[uid] ?? false;

  bool isGCOwner(String uid) => uid == gcOwner;

  void blockUser(String uid) async {
    var snackbar = SnackBarModel.of(context);
    try {
      await Golib.addToGCBlockList(gcID, uid);
      reloadUsers();
    } catch (exception) {
      snackbar.error('Unable to add to GC block list: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  void unblockUser(String uid) async {
    var snackbar = SnackBarModel.of(context);
    try {
      await Golib.removeFromGCBlockList(gcID, uid);
      reloadUsers();
    } catch (exception) {
      snackbar.error('Unable to remove from GC block list: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  void reloadUsers() async {
    var snackbar = SnackBarModel.of(context);
    try {
      var gcdb = await Golib.getGC(gcID);
      var gc = gcdb.metadata;
      var cli = widget.client;
      List<ChatModel> newUsers = [];
      var newBlocked = await Golib.getGCBlockList(gcID);
      List<String> newUnkxd = [];
      for (var memberID in gc.members) {
        if (memberID == widget.client.publicID) {
          continue;
        }
        var chat = cli.getExistingChat(memberID);
        if (chat != null) {
          newUsers.add(chat);
        } else {
          newUnkxd.add(memberID);
        }
      }
      Map<String, bool> newAdmins = {};
      gc.extraAdmins?.forEach((e) => newAdmins[e] = true);
      var myID = widget.client.publicID;
      setState(() {
        memberUIDs = gc.members;
        gcOwner = gc.members[0];
        users = newUsers;
        admins = newAdmins;
        localIsOwner = gcOwner == myID;
        gcVersion = gc.version;
        gcGeneration = gc.generation;
        gcTimestamp = DateTime.fromMillisecondsSinceEpoch(gc.timestamp * 1000);
        localIsAdmin = localIsOwner ||
            (gc.extraAdmins != null && gc.extraAdmins!.contains(myID));
        blockedUsers = newBlocked;
        unkxdMembers = newUnkxd;
        rtdtRV = gcdb.rtdtSessionRV;
      });
    } catch (exception) {
      snackbar.error('Unable to reload gc: $exception');
    } finally {
      firstLoading = false;
    }
  }

  void removeUser(ChatModel user) async {
    var snackbar = SnackBarModel.of(context);
    setState(() => loading = true);
    try {
      await Golib.removeGcUser(gcID, user.id);
      reloadUsers();
    } catch (exception) {
      snackbar.error('Unable to remove user: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  void confirmRemoveUser(context, index) {
    if (loading) return;

    showModalBottomSheet(
        context: context,
        builder: (BuildContext context) => Container(
              padding: const EdgeInsets.all(30),
              child: Wrap(runSpacing: 10, children: [
                Txt("Really remove '${users[index].nick}'?"),
                const SizedBox(width: 10, height: 10),
                CancelButton(
                  onPressed: () => Navigator.pop(context),
                ),
                const SizedBox(width: 10, height: 10),
                OutlinedButton(
                    onPressed: () {
                      Navigator.pop(context);
                      removeUser(users[index]);
                    },
                    child: const Text("Yes")),
              ]),
            ));
  }

  void addAsAdmin(ChatModel user) async {
    var snackbar = SnackBarModel.of(context);
    List<String> newAdmins = admins.keys.toList();
    newAdmins.add(user.id);
    setState(() => loading = true);
    try {
      await Golib.modifyGCAdmins(gcID, newAdmins);
      snackbar.success("Added ${user.nick} as admin");
    } catch (exception) {
      snackbar.error("Unable add ${user.nick} as admin: $exception");
    } finally {
      setState(() => loading = false);
      reloadUsers();
    }
  }

  void removeAsAdmin(ChatModel user) async {
    var snackbar = SnackBarModel.of(context);
    List<String> newAdmins = admins.keys.toList();
    newAdmins.remove(user.id);
    setState(() => loading = true);
    try {
      await Golib.modifyGCAdmins(gcID, newAdmins);
      snackbar.success("Removed ${user.nick} as admin");
    } catch (exception) {
      snackbar.error("Unable remove ${user.nick} as admin: $exception");
    } finally {
      setState(() => loading = false);
      reloadUsers();
    }
  }

  Future<void> doKillGC() async {
    var snackbar = SnackBarModel.of(context);
    setState(() => loading = true);
    try {
      await Golib.killGC(gcID);
      if (rtdtRV != "") {
        widget.rtc.refreshSessions();
      }
      widget.client.removeChat(widget.chat);
    } catch (exception) {
      snackbar.error('Unable to kill GC: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  void killGC() {
    showConfirmDialog(context,
        title: "Confirm Kill GC",
        child: Text(
            "Really dissolve GC ${widget.chat.nick}? This action cannot be undone."),
        confirmButtonText: "Kill GC",
        onConfirm: doKillGC);
  }

  Future<void> doPartFromGC() async {
    var snackbar = SnackBarModel.of(context);
    setState(() => loading = true);
    try {
      await Golib.partFromGC(gcID);
      widget.client.removeChat(widget.chat);
    } catch (exception) {
      snackbar.error('Unable to part from GC: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  void partFromGC() {
    showConfirmDialog(context,
        title: "Confirm Part GC",
        onConfirm: doPartFromGC,
        child: Text(
            "Really exit from GC ${widget.chat.nick}? Joining back will require a new invitation from a GC admin."));
  }

  Future<void> hideGC() async {
    var snackbar = SnackBarModel.of(context);
    setState(() => loading = true);
    try {
      widget.client.hideChat(widget.chat);
      widget.client.active = null;
    } catch (exception) {
      snackbar.error("Unable to hide GC: $exception");
    } finally {
      setState(() => loading = false);
    }
  }

  Future<void> upgradeGC() async {
    var snackbar = SnackBarModel.of(context);
    setState(() => loading = true);
    try {
      await Golib.upgradeGC(gcID);
      snackbar.success("Upgraded GC!");
    } catch (exception) {
      snackbar.error("Unable to upgrade GC: $exception");
    } finally {
      setState(() => loading = false);
      reloadUsers();
    }
  }

  void toStateChangeOwner() =>
      setState(() => state = _ScreenState.changingOwner);
  void toStateInvite() => setState(() => state = _ScreenState.inviting);
  void toStateManage() {
    reloadUsers;
    setState(() => state = _ScreenState.managing);
  }

  Future<void> doCreateRTDTSession(int extraSize) async {
    if (extraSize < 0) {
      showErrorSnackbar(this, "Cannot use additional size < 0");
      return;
    }

    setState(() => loading = true);
    try {
      await widget.rtc.createSessionFromGC(gcID, extraSize);
      showSuccessSnackbar(this, "Realtime chat session created!");
    } catch (exception) {
      showErrorSnackbar(this, "Unable to create RTDT session: $exception");
    } finally {
      setState(() => loading = false);
    }
  }

  Future<void> createRTDTSession() async {
    int extraSize = 0;
    showConfirmDialog(context,
        title: "Confirm Creating",
        child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Txt(
                  "Confirm creation of a realtime chat session associated with the GC"),
              const SizedBox(height: 10),
              Row(children: [
                const Txt("Additional Member Capacity:"),
                const SizedBox(width: 10),
                SizedBox(
                    width: 100,
                    child: intInput(onChanged: (amount) => extraSize = amount)),
              ]),
              const SizedBox(height: 5),
              const Txt.S(
                  "Note: additional member capacity is needed if new members will be added to the GC."
                  "This is needed because a relatime chat session size cannot be changed after creation."),
            ]),
        confirmButtonText: "Create",
        onConfirm: () => doCreateRTDTSession(extraSize));
  }

  Widget buildAdminAction(ChatModel user, int index) {
    if (!localIsAdmin || gcVersion == 0) {
      if (isGCOwner(user.id)) {
        return Tooltip(
            message: "User is owner of this GC",
            child: Icon(
              Icons.shield,
              color: Colors.yellowAccent.shade400,
            ));
      }
      if (isGCAdmin(user.id)) {
        return const Tooltip(
            message: "User is admin of this GC", child: Icon(Icons.shield));
      }
      return const SizedBox(width: 23);
    }

    if (isGCAdmin(user.id)) {
      return IconButton(
        icon: const Icon(Icons.remove_moderator),
        tooltip: "Remove as admin",
        onPressed: !loading ? () => removeAsAdmin(user) : null,
      );
    }

    if (!isGCOwner(users[index].id)) {
      return IconButton(
        icon: const Icon(Icons.add_moderator),
        tooltip: "Add as admin",
        onPressed: !loading ? () => addAsAdmin(user) : null,
      );
    }

    return const SizedBox(width: 40);
  }

  @override
  Widget build(BuildContext context) {
    if (firstLoading) {
      return const Scaffold(body: Center(child: Text("Loading...")));
    }

    if (state == _ScreenState.inviting) {
      return _InviteUserPanel(
          widget.client, widget.chat, memberUIDs, toStateManage);
    } else if (state == _ScreenState.changingOwner) {
      return _ChangeGCOwnerPanel(
          widget.client, widget.chat, users, toStateManage);
    }

    return Align(
        alignment: Alignment.topLeft,
        child: Container(
            padding:
                const EdgeInsets.only(left: 15, right: 15, top: 8, bottom: 12),
            child: SingleChildScrollView(
                child: Column(children: [
              Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                const Txt.L("Managing GC - "),
                Txt.L(gcName),
              ]),
              const SizedBox(height: 20),
              Wrap(runSpacing: 10, spacing: 10, children: [
                localIsOwner
                    ? OutlinedButton(
                        onPressed: !loading ? killGC : null,
                        child: const Text("Kill GC"))
                    : OutlinedButton(
                        onPressed: !loading ? partFromGC : null,
                        child: const Text("Part from GC")),
                localIsAdmin && gcVersion < MAXGCVERSION
                    ? OutlinedButton(
                        onPressed: !loading ? upgradeGC : null,
                        child: const Text("Upgrade Version"))
                    : const Empty(),
                localIsOwner
                    ? OutlinedButton(
                        onPressed: !loading ? toStateChangeOwner : null,
                        child: const Text("Change Owner"))
                    : const Empty(),
                if (localIsAdmin && rtdtRV == "")
                  OutlinedButton(
                      onPressed: !loading ? createRTDTSession : null,
                      child: const Text("Create Realtime Session")),
                OutlinedButton(
                    onPressed: !loading ? hideGC : null,
                    child: const Text("Hide GC"))
              ]),
              const SizedBox(height: 20),
              SimpleInfoGrid(colLabelSize: 85, separatorWidth: 0, [
                Tuple2(const Txt.S("ID:"),
                    Copyable.txt(Txt.S(gcID, overflow: TextOverflow.ellipsis))),
                Tuple2(const Txt.S("Version:"), Txt.S(gcVersion.toString())),
                Tuple2(
                    const Txt.S("Generation:"), Txt.S(gcGeneration.toString())),
                Tuple2(const Txt.S("Timestamp:"),
                    Txt.S(gcTimestamp.toIso8601String())),
              ]),
              const Divider(),
              Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                const Txt.L("GC Members"),
                const SizedBox(width: 5),
                if (localIsAdmin)
                  OutlinedButton(
                      onPressed: !loading ? toStateInvite : null,
                      child: const Text("Invite to GC")),
              ]),
              ListView.builder(
                  shrinkWrap: true,
                  itemCount: users.length,
                  itemBuilder: (context, index) => ListTile(
                        title: Text(users[index].nick),
                        trailing:
                            Row(mainAxisSize: MainAxisSize.min, children: [
                          isBlocked(users[index].id)
                              ? IconButton(
                                  tooltip: "Un-ignore user messages in this GC",
                                  icon: const Icon(Icons.volume_off_outlined),
                                  onPressed: () => unblockUser(users[index].id),
                                )
                              : IconButton(
                                  tooltip: "Ignore user messages in this GC",
                                  icon: const Icon(Icons.volume_mute),
                                  onPressed: () => blockUser(users[index].id),
                                ),
                          localIsAdmin
                              ? IconButton(
                                  icon: const Icon(Icons.remove_circle),
                                  tooltip: "Remove user from GC",
                                  onPressed: !loading
                                      ? () => confirmRemoveUser(context, index)
                                      : null,
                                )
                              : const Empty(),
                          buildAdminAction(users[index], index),
                        ]),
                      )),
              const SizedBox(height: 20),
              ...(unkxdMembers.isEmpty
                  ? []
                  : [
                      const Row(
                          mainAxisAlignment: MainAxisAlignment.center,
                          children: [
                            Txt.L("Unkx'd Members"),
                            SizedBox(width: 10),
                            InfoTooltipIcon(
                                size: 14,
                                tooltip:
                                    "These are members of the GC that your client hasn't "
                                    "exchanged keys yet.\nThe client will automatically "
                                    "attempt to KX with them and will complete "
                                    "this process when both the mediator from this GC "
                                    "and the other party are online.")
                          ]),
                      const SizedBox(height: 10),
                      ...unkxdMembers.map((id) => Copyable.txt(Txt.S(id,
                          style: ThemeNotifier.of(context, listen: false)
                              .extraTextStyles
                              .monospaced))),
                      const SizedBox(height: 20),
                    ]),
              ElevatedButton(
                  onPressed: !loading
                      ? () => widget.client.ui.showProfile.val = false
                      : null,
                  child: const Text("Done"))
            ]))));
  }
}
