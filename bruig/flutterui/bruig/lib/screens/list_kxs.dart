import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/info_grid.dart';
import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:collection/collection.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';

class ListKXsScreen extends StatefulWidget {
  static const routeName = "/listKXs";

  const ListKXsScreen({super.key});

  @override
  State<ListKXsScreen> createState() => _ListKXsScreenState();
}

class _ListKXsScreenState extends State<ListKXsScreen> {
  List<KXData> kxs = [];

  void listKXs() async {
    try {
      var newKXs = await Golib.listKXs();
      newKXs.sortBy((v) => v.timestamp);
      newKXs.reverseRange(0, newKXs.length);
      setState(() => kxs = newKXs);
    } catch (exception) {
      showErrorSnackbar(this, "Unable to list KXs: $exception");
    }
  }

  @override
  void initState() {
    super.initState();
    listKXs();
  }

  @override
  Widget build(BuildContext context) {
    var client = Provider.of<ClientModel>(context, listen: false);

    return StartupScreen([
      kxs.isNotEmpty
          ? const Txt.H("List of Ongoing KX Attempts")
          : const Txt.L("No KX attempt in progress"),
      const SizedBox(height: 30),
      ...kxs.map<Widget>((kx) => Container(
          padding: const EdgeInsets.symmetric(horizontal: 50, vertical: 10),
          child: SimpleInfoGridAdv(items: [
            ["For Reset?", "${kx.isForReset}"],
            ["Stage", "${kx.stage}"],
            ["Updated", "${kx.timestamp}"],
            ["Initial RV", Copyable(kx.initialRV)],
            ["Target Nick", kx.invitee?.nick ?? ""],
            ["Target ID", Copyable(kx.invitee?.identity ?? "")],
            ["Mediator ID", Copyable(kx.mediatorID ?? "")],
            [
              "Mediator",
              kx.mediatorID != null && kx.mediatorID != ""
                  ? Row(children: [
                      UserAvatarFromID(client, kx.mediatorID!, radius: 10),
                      const SizedBox(width: 10),
                      UserNickFromID(kx.mediatorID!)
                    ])
                  : ""
            ],
            ["My Reset RV", Copyable(kx.myResetRV)],
          ]))),
      const SizedBox(height: 10),
      TextButton(
          onPressed: () {
            Navigator.of(context).pop();
          },
          child: const Text("Done"))
    ]);
  }
}
