import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/info_grid.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/components/users_dropdown.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:tuple/tuple.dart';

class ManageGCScreen extends StatefulWidget {
  final ClientModel client;
  final ChatModel chat;
  const ManageGCScreen(this.client, this.chat, {Key? key}) : super(key: key);

  @override
  State<ManageGCScreen> createState() => _ManageGCScreenState();
}

class _InviteUserPanel extends StatefulWidget {
  final String gcID;
  const _InviteUserPanel(this.gcID);

  @override
  State<_InviteUserPanel> createState() => _InviteUserPanelState();
}

class _InviteUserPanelState extends State<_InviteUserPanel> {
  bool loading = false;
  ChatModel? userToInvite;

  void inviteUser(BuildContext context) async {
    if (loading) return;
    if (userToInvite == null) return;
    setState(() => loading = true);

    try {
      await Golib.inviteToGC(InviteToGC(widget.gcID, userToInvite!.id));
      showSuccessSnackbar(
          context, 'Sent invitation to "${userToInvite!.nick}"');
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to invite: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Column(children: [
      const Divider(),
      const SizedBox(height: 10),
      const Text("Invite to GC"),
      const SizedBox(height: 10),
      Row(children: [
        Expanded(child: UsersDropdown(cb: (ChatModel? chat) {
          userToInvite = chat;
        })),
        Container(width: 20),
        ElevatedButton(
            onPressed: !loading ? () => inviteUser(context) : null,
            child: const Text('Invite User')),
      ])
    ]);
  }
}

class _ChangeGCOwnerPanel extends StatefulWidget {
  final String gcID;
  final List<ChatModel> users;
  final Function changeOwner;
  const _ChangeGCOwnerPanel(this.gcID, this.users, this.changeOwner);

  @override
  State<_ChangeGCOwnerPanel> createState() => __ChangeGCOwnerPanelState();
}

class __ChangeGCOwnerPanelState extends State<_ChangeGCOwnerPanel> {
  bool loading = false;
  ChatModel? newOwner;

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
                      widget.changeOwner(newOwner);
                    },
                    child: const Text("Yes")),
              ]),
            ));
  }

  @override
  Widget build(BuildContext context) {
    var userIDs = widget.users.map((e) => e.id).toList();
    return Column(children: [
      const SizedBox(height: 10),
      const Text("Change GC Owner"),
      const SizedBox(height: 10),
      Row(children: [
        Expanded(
            child: UsersDropdown(
                limitUIDs: userIDs,
                cb: (ChatModel? chat) {
                  newOwner = chat;
                })),
        Container(width: 20),
        ElevatedButton(
            onPressed: !loading ? confirmNewOwner : null,
            child: const Text('Change Owner')),
      ])
    ]);
  }
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
  ChatModel? userToInvite;
  bool localIsAdmin = false;
  bool localIsOwner = false;
  bool firstLoading = true;
  Map<String, bool> admins = {};

  @override
  void initState() {
    super.initState();
    reloadUsers();
  }

  @override
  void didUpdateWidget(ManageGCScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    reloadUsers();
  }

  void onDone(BuildContext context) {
    Navigator.pop(context);
  }

  bool isBlocked(String uid) {
    var v = blockedUsers[uid];
    return v != null;
  }

  bool isGCAdmin(String uid) => admins[uid] ?? false;

  bool isGCOwner(String uid) => uid == gcOwner;

  void blockUser(String uid) async {
    try {
      await Golib.addToGCBlockList(gcID, uid);
      reloadUsers();
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to add to GC block list: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  void unblockUser(String uid) async {
    try {
      await Golib.removeFromGCBlockList(gcID, uid);
      reloadUsers();
    } catch (exception) {
      showErrorSnackbar(
          context, 'Unable to remove from GC block list: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  void reloadUsers() async {
    try {
      var gc = await Golib.getGC(gcID);
      var cli = widget.client;
      List<ChatModel> newUsers = [];
      var newBlocked = await Golib.getGCBlockList(gcID);
      gc.members.map((v) => cli.getExistingChat(v)).forEach((v) {
        if (v != null) {
          newUsers.add(v);
        }
      });
      Map<String, bool> newAdmins = {};
      gc.extraAdmins?.forEach((e) => newAdmins[e] = true);
      var myID = widget.client.publicID;
      setState(() {
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
      });
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to reload gc: $exception');
    } finally {
      firstLoading = false;
    }
  }

  void removeUser(ChatModel user) async {
    setState(() => loading = true);
    try {
      await Golib.removeGcUser(gcID, user.id);
      reloadUsers();
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to remove user: $exception');
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
    List<String> newAdmins = admins.keys.toList();
    newAdmins.add(user.id);
    setState(() => loading = true);
    try {
      await Golib.modifyGCAdmins(gcID, newAdmins);
      showSuccessSnackbar(context, "Added ${user.nick} as admin");
    } catch (exception) {
      showErrorSnackbar(
          context, "Unable add ${user.nick} as admin: $exception");
    } finally {
      setState(() => loading = false);
      reloadUsers();
    }
  }

  void removeAsAdmin(ChatModel user) async {
    List<String> newAdmins = admins.keys.toList();
    newAdmins.remove(user.id);
    setState(() => loading = true);
    try {
      await Golib.modifyGCAdmins(gcID, newAdmins);
      showSuccessSnackbar(context, "Removed ${user.nick} as admin");
    } catch (exception) {
      showErrorSnackbar(
          context, "Unable remove ${user.nick} as admin: $exception");
    } finally {
      setState(() => loading = false);
      reloadUsers();
    }
  }

  Future<void> killGC() async {
    setState(() => loading = true);
    try {
      await Golib.killGC(gcID);
      widget.client.removeChat(widget.chat);
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to kill GC: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  Future<void> partFromGC() async {
    setState(() => loading = true);
    try {
      await Golib.partFromGC(gcID);
      widget.client.removeChat(widget.chat);
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to part from GC: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  Future<void> hideGC() async {
    setState(() => loading = true);
    try {
      widget.client.hideChat(widget.chat);
      widget.client.active = null;
    } catch (exception) {
      showErrorSnackbar(context, "Unable to hide GC: $exception");
    } finally {
      setState(() => loading = false);
    }
  }

  Future<void> upgradeGC() async {
    setState(() => loading = true);
    try {
      await Golib.upgradeGC(gcID);
      showSuccessSnackbar(context, "Upgraded GC!");
    } catch (exception) {
      showErrorSnackbar(context, "Unable to upgrade GC: $exception");
    } finally {
      setState(() => loading = false);
      reloadUsers();
    }
  }

  Future<void> changeOwner(ChatModel newOwner) async {
    if (loading) return;
    setState(() => loading = true);
    try {
      await Golib.modifyGCOwner(gcID, newOwner.id);
    } catch (exception) {
      showErrorSnackbar(context, "Unable to modify GC Owner: $exception");
    } finally {
      setState(() => loading = false);
      reloadUsers();
    }
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
              const SizedBox(height: 10),
              localIsAdmin
                  ? Container(
                      margin: const EdgeInsets.only(top: 10, bottom: 10),
                      child: _InviteUserPanel(gcID))
                  : const Empty(),
              const Divider(),
              if (localIsOwner) ...[
                Container(
                    margin: const EdgeInsets.only(top: 10, bottom: 10),
                    child: _ChangeGCOwnerPanel(gcID, users, changeOwner)),
                const Divider(),
              ],
              const Text("GC Members"),
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
              const SizedBox(height: 10),
              ElevatedButton(
                  onPressed: !loading
                      ? () => widget.client.ui.showProfile.val = false
                      : null,
                  child: const Text("Done"))
            ]))));
  }
}
