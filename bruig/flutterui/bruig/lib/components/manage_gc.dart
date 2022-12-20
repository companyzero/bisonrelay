import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/users_dropdown.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';

class ManageGCScreen extends StatelessWidget {
  const ManageGCScreen({Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    return Consumer<ClientModel>(builder: (context, client, child) {
      var activeHeading = client.active;
      if (activeHeading == null) return Container();
      var chat = client.getExistingChat(activeHeading.id);
      if (chat == null) {
        return ElevatedButton(
            child: const Text("Done"), onPressed: () => Navigator.pop(context));
      }

      return ManageGCScreenForChat(chat, client);
    });
  }
}

class ManageGCScreenForChat extends StatefulWidget {
  final ChatModel chat;
  final ClientModel client;
  const ManageGCScreenForChat(this.chat, this.client, {Key? key})
      : super(key: key);

  @override
  State<ManageGCScreenForChat> createState() =>
      ManageGCScreenState(chat.id, chat.nick);
}

class ManageGCScreenState extends State<ManageGCScreenForChat> {
  final _formKey = GlobalKey<FormState>();
  bool loading = false;
  final String gcID;
  final String gcName;
  String inviteNick = "";
  List<ChatModel> users = [];
  Map<String, dynamic> blockedUsers = {};
  ChatModel? userToInvite;
  bool isAdmin = false;
  bool firstLoading = true;

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
      setState(() {
        users = newUsers;
        isAdmin = gc.members[0] == widget.client.publicID;
        blockedUsers = newBlocked;
      });
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to reload gc: $exception');
    } finally {
      firstLoading = false;
    }
  }

  void inviteUser(BuildContext context) async {
    if (loading) return;
    if (!_formKey.currentState!.validate()) return;
    _formKey.currentState!.save();
    if (userToInvite == null) return;
    setState(() => loading = true);

    try {
      await Golib.inviteToGC(InviteToGC(gcID, userToInvite!.id));
      showSuccessSnackbar(
          context, 'Sent invitation to "${userToInvite!.nick}"');
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to invite: $exception');
    } finally {
      setState(() => loading = false);
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
              child: Row(children: [
                Text("Really remove '${users[index].nick}'?"),
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

  Future<void> killGC() async {
    setState(() => loading = true);
    try {
      await Golib.killGC(gcID);
      widget.client.removeChat(widget.chat);
      Navigator.pop(context);
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
      Navigator.pop(context);
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to part from GC: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    if (firstLoading) {
      return const Scaffold(body: Center(child: Text("Loading...")));
    }
    return Scaffold(
      body: Center(
        child: Container(
            padding: const EdgeInsets.all(40),
            constraints: const BoxConstraints(maxWidth: 500),
            child: Form(
                key: _formKey,
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
                      isAdmin
                          ? ElevatedButton(
                              onPressed: !loading ? killGC : null,
                              child: const Text("Kill GC"))
                          : ElevatedButton(
                              onPressed: !loading ? partFromGC : null,
                              child: const Text("Part from GC"))
                    ]),
                    isAdmin
                        ? Row(children: [
                            Expanded(
                                child: UsersDropdown(cb: (ChatModel? chat) {
                              userToInvite = chat;
                            })),
                            Container(width: 20),
                            ElevatedButton(
                                onPressed:
                                    !loading ? () => inviteUser(context) : null,
                                child: const Text('Invite User')),
                          ])
                        : const Empty(),
                    Container(height: 10),
                    Expanded(
                        child: ListView.builder(
                            itemCount: users.length,
                            itemBuilder: (context, index) => ListTile(
                                  title: Text(users[index].nick),
                                  trailing: Row(
                                      mainAxisSize: MainAxisSize.min,
                                      children: [
                                        isBlocked(users[index].id)
                                            ? IconButton(
                                                tooltip:
                                                    "Un-ignore user messages in this GC",
                                                icon: const Icon(
                                                    Icons.volume_off_outlined),
                                                onPressed: () => unblockUser(
                                                    users[index].id),
                                              )
                                            : IconButton(
                                                tooltip:
                                                    "Ignore user messages in this GC",
                                                icon: const Icon(
                                                    Icons.volume_mute),
                                                onPressed: () =>
                                                    blockUser(users[index].id),
                                              ),
                                        isAdmin
                                            ? IconButton(
                                                icon: const Icon(
                                                    Icons.remove_circle),
                                                onPressed: !loading
                                                    ? () => confirmRemoveUser(
                                                        context, index)
                                                    : null,
                                              )
                                            : const Empty()
                                      ]),
                                ))),
                    ElevatedButton(
                        //style: ElevatedButton.styleFrom(primary: Colors.grey),
                        onPressed: !loading
                            ? () => widget.client.profile = null
                            : null,
                        child: const Text("Done"))
                  ],
                ))),
      ),
    );
  }
}
