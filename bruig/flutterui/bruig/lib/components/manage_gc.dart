import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/users_dropdown.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';
import 'package:bruig/models/snackbar.dart';

class ManageGCScreen extends StatelessWidget {
  const ManageGCScreen({Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    return Consumer2<ClientModel, SnackBarModel>(
        builder: (context, client, snackBar, child) {
      var activeHeading = client.active;
      if (activeHeading == null) return Container();
      var chat = client.getExistingChat(activeHeading.id);
      if (chat == null) {
        return ElevatedButton(
            child: const Text("Done"), onPressed: () => Navigator.pop(context));
      }

      return ManageGCScreenForChat(chat, client, snackBar);
    });
  }
}

class ManageGCScreenForChat extends StatefulWidget {
  final ChatModel chat;
  final ClientModel client;
  final SnackBarModel snackBar;
  const ManageGCScreenForChat(this.chat, this.client, this.snackBar, {Key? key})
      : super(key: key);

  @override
  State<ManageGCScreenForChat> createState() =>
      ManageGCScreenState(chat.id, chat.nick);
}

class _InviteUserPanel extends StatefulWidget {
  final String gcID;
  final SnackBarModel snackBar;
  const _InviteUserPanel(this.gcID, this.snackBar, {super.key});

  @override
  State<_InviteUserPanel> createState() => _InviteUserPanelState();
}

class _InviteUserPanelState extends State<_InviteUserPanel> {
  SnackBarModel get snackBar => widget.snackBar;
  bool loading = false;
  ChatModel? userToInvite;

  void inviteUser(BuildContext context) async {
    if (loading) return;
    if (userToInvite == null) return;
    setState(() => loading = true);

    try {
      await Golib.inviteToGC(InviteToGC(widget.gcID, userToInvite!.id));
      snackBar.success('Sent invitation to "${userToInvite!.nick}"');
    } catch (exception) {
      snackBar.error('Unable to invite: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;

    return Column(children: [
      Divider(height: 10, color: textColor),
      const SizedBox(height: 10),
      Text("Invite to GC", style: TextStyle(color: textColor, fontSize: 20)),
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

class ManageGCScreenState extends State<ManageGCScreenForChat> {
  SnackBarModel get snackBar => widget.snackBar;
  // This must be updated every time a new GC version is deployed and its features
  // implemented in bruig.
  final MAXGCVERSION = 1;

  bool loading = false;
  final String gcID;
  final String gcName;
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

  ManageGCScreenState(this.gcID, this.gcName);

  @override
  void initState() {
    super.initState();
    reloadUsers();
  }

  void onDone(BuildContext context) {
    //Navigator.pushReplacementNamed(context, "/chats");
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
      snackBar.error('Unable to add to GC block list: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  void unblockUser(String uid) async {
    try {
      await Golib.removeFromGCBlockList(gcID, uid);
      reloadUsers();
    } catch (exception) {
      snackBar.error('Unable to remove from GC block list: $exception');
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
      snackBar.error('Unable to reload gc: $exception');
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
      snackBar.error('Unable to remove user: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  void confirmRemoveUser(context, index) {
    if (loading) return;
    var theme = Theme.of(context);
    var textColor = theme.focusColor;

    showModalBottomSheet(
        context: context,
        builder: (BuildContext context) => Container(
              padding: const EdgeInsets.all(30),
              child: Row(children: [
                Text("Really remove '${users[index].nick}'?",
                    style: TextStyle(color: textColor)),
                const SizedBox(width: 10, height: 10),
                ElevatedButton(
                  onPressed: () => Navigator.pop(context),
                  style: ElevatedButton.styleFrom(backgroundColor: Colors.grey),
                  child: const Text("No"),
                ),
                const SizedBox(width: 10, height: 10),
                ElevatedButton(
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
      snackBar.success("Added ${user.nick} as admin");
    } catch (exception) {
      snackBar.error("Unable add ${user.nick} as admin: $exception");
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
      snackBar.success("Removed ${user.nick} as admin");
    } catch (exception) {
      snackBar.error("Unable remove ${user.nick} as admin: $exception");
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
      widget.client.active = null;
    } catch (exception) {
      snackBar.error('Unable to kill GC: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  Future<void> partFromGC() async {
    setState(() => loading = true);
    try {
      await Golib.partFromGC(gcID);
      widget.client.removeChat(widget.chat);
      widget.client.active = null;
    } catch (exception) {
      snackBar.error('Unable to part from GC: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  Future<void> upgradeGC() async {
    setState(() => loading = true);
    try {
      await Golib.upgradeGC(gcID);
      snackBar.success("Upgraded GC!");
    } catch (exception) {
      snackBar.error("Unable to upgrade GC: $exception");
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
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    if (firstLoading) {
      return const Scaffold(body: Center(child: Text("Loading...")));
    }
    return Container(
      padding: const EdgeInsets.all(40),
      constraints: const BoxConstraints(maxWidth: 500),
      child: Column(
        children: [
          Row(children: [
            Text("Managing GC - ",
                style: TextStyle(fontSize: 20, color: textColor)),
            Text(gcName,
                style: TextStyle(
                    fontSize: 20,
                    fontWeight: FontWeight.bold,
                    color: textColor)),
            const SizedBox(width: 20),
            localIsOwner
                ? ElevatedButton(
                    onPressed: !loading ? killGC : null,
                    child: const Text("Kill GC"))
                : ElevatedButton(
                    onPressed: !loading ? partFromGC : null,
                    child: const Text("Part from GC")),
            const SizedBox(width: 10),
            localIsAdmin && gcVersion < MAXGCVERSION
                ? ElevatedButton(
                    onPressed: !loading ? upgradeGC : null,
                    child: const Text("Upgrade Version"))
                : const Empty(),
          ]),
          const SizedBox(height: 10),
          Row(children: [
            Text("ID: ",
                style: TextStyle(
                    color: textColor,
                    fontWeight: FontWeight.w100,
                    fontSize: 10)),
            Copyable(
                gcID,
                TextStyle(
                    color: textColor,
                    fontWeight: FontWeight.w100,
                    fontSize: 10))
          ]),
          const SizedBox(height: 3),
          Row(children: [
            Text(
                "Version: $gcVersion, Generation: $gcGeneration, Timestamp: ${gcTimestamp.toIso8601String()}",
                style: TextStyle(
                    color: textColor,
                    fontWeight: FontWeight.w100,
                    fontSize: 10))
          ]),
          const SizedBox(height: 10),
          localIsAdmin
              ? Container(
                  margin: const EdgeInsets.only(top: 10, bottom: 10),
                  child: _InviteUserPanel(gcID, snackBar))
              : const Empty(),
          Divider(height: 10, color: textColor),
          const SizedBox(height: 10),
          Text("GC Members", style: TextStyle(color: textColor, fontSize: 20)),
          Expanded(
              child: ListView.builder(
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
                      ))),
          ElevatedButton(
              //style: ElevatedButton.styleFrom(primary: Colors.grey),
              onPressed: !loading ? () => widget.client.profile = null : null,
              child: const Text("Done"))
        ],
      ),
    );
  }
}
