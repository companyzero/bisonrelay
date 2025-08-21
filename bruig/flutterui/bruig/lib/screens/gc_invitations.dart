import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:bruig/theme_manager.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';

class GCInvitationsScreen extends StatefulWidget {
  static String routeName = "/gcInvitations";
  const GCInvitationsScreen({super.key});

  @override
  State<GCInvitationsScreen> createState() => _GCInvitationsScreenState();
}

class _GCInvitationsScreenState extends State<GCInvitationsScreen> {
  List<GCInvitation> invites = [];

  void updateList() async {
    try {
      var newInvites = await Golib.listGCInvitations();
      if (mounted) {
        var invCountModel =
            ClientModel.of(context, listen: false).gcInviteCount;
        invCountModel.value = invCountModel.countPendingInvites(newInvites);
        setState(() {
          invites = newInvites;
        });
      }
    } catch (exception) {
      showErrorSnackbar(this, "Unable to load list of invitations: $exception");
    }
  }

  void acceptInvite(int iid) async {
    var snackbar = SnackBarModel.of(context);
    try {
      await Golib.acceptGCInvite(iid);
      updateList();
    } catch (exception) {
      snackbar.error("Unable to accept invite: $exception");
    }
  }

  void declineInvite(int iid) async {
    var snackbar = SnackBarModel.of(context);
    try {
      await Golib.declineGCInvite(iid);
      updateList();
    } catch (exception) {
      snackbar.error("Unable to decline invite: $exception");
    }
  }

  @override
  void initState() {
    super.initState();
    updateList();
  }

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => StartupScreen([
              const Txt.H("Received GC Invitations"),
              const SizedBox(height: 20),
              ...(invites.map((i) => Container(
                    //width: 200,
                    //height: 100,
                    margin: const EdgeInsets.symmetric(vertical: 20),
                    // color: Colors.amber,
                    child: Column(children: [
                      Text("Name: ${i.name}"),
                      Text("Inviter: ${i.inviter.nick}"),
                      Text(
                          "Expires: ${DateTime.fromMillisecondsSinceEpoch(i.invite.expires * 1000)}"),
                      Text("GC ID ${i.invite.id}"),
                      const SizedBox(height: 5),
                      i.accepted
                          ? const Txt(
                              "Invite Accepted! Waiting for admin to add to GC.",
                              color: TextColor.successOnSurface)
                          : Wrap(spacing: 5, children: [
                              OutlinedButton(
                                  onPressed: () => acceptInvite(i.iid),
                                  child: const Text("Accept Invite")),
                              CancelButton(
                                  onPressed: () => declineInvite(i.iid),
                                  label: "Decline"),
                            ]),
                    ]),
                  ))),
              if (invites.isEmpty)
                const Txt("No invitations", color: TextColor.onSurfaceVariant),
              const SizedBox(height: 10),
              ElevatedButton(
                  onPressed: () => Navigator.pop(context),
                  child: const Text("Done")),
            ]));
  }
}
