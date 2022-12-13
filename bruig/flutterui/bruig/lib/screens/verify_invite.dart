import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:provider/provider.dart';

class VerifyInviteScreen extends StatefulWidget {
  const VerifyInviteScreen({Key? key}) : super(key: key);

  @override
  State<VerifyInviteScreen> createState() => _VerifyInviteScreenState();
}

class _VerifyInviteScreenState extends State<VerifyInviteScreen> {
  bool _loading = false;

  @override
  void initState() {
    super.initState();
  }

  void onAcceptInvite(BuildContext context, Invitation invite) async {
    if (_loading) return;
    setState(() {
      _loading = true;
    });

    var client = Provider.of<ClientModel>(context, listen: false);
    client.acceptInvite(invite);
    Navigator.pop(context);
  }

  void onDenyInvite(BuildContext context) {
    Navigator.pop(context);
  }

  @override
  Widget build(BuildContext context) {
    var invite = ModalRoute.of(context)!.settings.arguments as Invitation;

    var theme = Theme.of(context);
    return Scaffold(
        body: Center(
            child: Container(
                padding: const EdgeInsets.all(40),
                constraints: const BoxConstraints(maxWidth: 500),
                child: Column(children: [
                  Text('''Name: ${invite.user.name}
Nick: ${invite.user.nick}
Identity: ${invite.user.uid}'''),
                  Container(height: 50),
                  ElevatedButton(
                      onPressed: !_loading
                          ? () => onAcceptInvite(context, invite)
                          : null,
                      child: const Text("Accept")),
                  Container(height: 10),
                  ElevatedButton(
                      style: ElevatedButton.styleFrom(
                          backgroundColor: theme.errorColor),
                      onPressed: () => onDenyInvite(context),
                      child: const Text("Deny"))
                ]))));
  }
}
