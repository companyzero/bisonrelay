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
    var backgroundColor = const Color(0xFF19172C);
    var cardColor = const Color(0xFF05031A);
    var textColor = const Color(0xFF8E8D98);
    var secondaryTextColor = const Color(0xFFE4E3E6);
    var darkTextColor = const Color(0xFF5A5968);
    return Scaffold(
        body: Container(
            color: backgroundColor,
            child: Stack(children: [
              Container(
                  decoration: const BoxDecoration(
                      image: DecorationImage(
                          fit: BoxFit.fill,
                          image: AssetImage("assets/images/loading-bg.png")))),
              Container(
                  decoration: BoxDecoration(
                      gradient: LinearGradient(
                          begin: Alignment.bottomLeft,
                          end: Alignment.topRight,
                          colors: [
                        cardColor,
                        const Color(0xFF07051C),
                        backgroundColor.withOpacity(0.34),
                      ],
                          stops: const [
                        0,
                        0.17,
                        1
                      ])),
                  padding: const EdgeInsets.all(10),
                  child: Column(children: [
                    const SizedBox(height: 258),
                    Text("Accept Invite",
                        style: TextStyle(
                            color: textColor,
                            fontSize: 34,
                            fontWeight: FontWeight.w200)),
                    const SizedBox(height: 34),
                    Text("Name: ${invite.user.name}",
                        style: TextStyle(
                            color: textColor,
                            fontSize: 15,
                            fontWeight: FontWeight.w300)),
                    Text("Nick: ${invite.user.nick}",
                        style: TextStyle(
                            color: textColor,
                            fontSize: 15,
                            fontWeight: FontWeight.w300)),
                    Text("Identity: ${invite.user.uid}",
                        style: TextStyle(
                            color: textColor,
                            fontSize: 15,
                            fontWeight: FontWeight.w300)),
                    const SizedBox(height: 34),
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
                  ]))
            ])));
  }
}
