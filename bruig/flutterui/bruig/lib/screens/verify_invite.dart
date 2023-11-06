import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

class VerifyInviteScreen extends StatefulWidget {
  const VerifyInviteScreen({Key? key}) : super(key: key);

  @override
  State<VerifyInviteScreen> createState() => _VerifyInviteScreenState();
}

class _VerifyInviteScreenState extends State<VerifyInviteScreen> {
  bool _loading = false;
  bool redeeming = false;
  RedeemedInviteFunds? redeemed;

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

  void redeemFunds(BuildContext context, InviteFunds funds) async {
    setState(() => redeeming = true);
    try {
      var res = await Golib.redeemInviteFunds(funds);
      setState(() {
        redeemed = res;
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to redeem funds: $exception");
      setState(() => redeeming = false);
    }
  }

  Widget buildFundsWidget(BuildContext context, InviteFunds funds) {
    var textColor = const Color(0xFF8E8D98);
    var ts = TextStyle(color: textColor);

    if (redeemed != null) {
      var total = formatDCR(atomsToDCR(redeemed!.total));
      return Column(children: [
        Text("Redeemed $total on the following tx:", style: ts),
        Copyable(redeemed!.txid, ts),
        Text("The funds will be available after the tx is mined.", style: ts),
      ]);
    }

    if (redeeming) {
      return Text("Attempting to redeem funds...", style: ts);
    }

    return Column(children: [
      Column(children: [
        Text("This invite contains funds stored in the following UTXO:",
            style: ts),
        Copyable("${funds.txid}:${funds.index}", ts),
        Text("Attempt to redeem funds?", style: ts),
      ]),
      const SizedBox(height: 10),
      ElevatedButton(
          onPressed: () => redeemFunds(context, funds),
          child: const Text("Redeem Funds")),
    ]);
  }

  @override
  Widget build(BuildContext context) {
    var invite = ModalRoute.of(context)!.settings.arguments as Invitation;

    var backgroundColor = const Color(0xFF19172C);
    var cardColor = const Color(0xFF05031A);
    var textColor = const Color(0xFF8E8D98);
    var errorColor = Colors.red;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Scaffold(
            body: Container(
                color: backgroundColor,
                child: Stack(children: [
                  Container(
                      decoration: const BoxDecoration(
                          image: DecorationImage(
                              fit: BoxFit.fill,
                              image:
                                  AssetImage("assets/images/loading-bg.png")))),
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
                        const Expanded(child: Empty()),
                        Text("Accept Invite",
                            style: TextStyle(
                                color: textColor,
                                fontSize: theme.getHugeFont(context),
                                fontWeight: FontWeight.w200)),
                        const SizedBox(height: 34),
                        invite.invite.funds != null
                            ? buildFundsWidget(context, invite.invite.funds!)
                            : const Empty(),
                        const SizedBox(height: 20),
                        Text("Name: ${invite.invite.public.name}",
                            style: TextStyle(
                                color: textColor,
                                fontSize: theme.getMediumFont(context),
                                fontWeight: FontWeight.w300)),
                        Text("Nick: ${invite.invite.public.nick}",
                            style: TextStyle(
                                color: textColor,
                                fontSize: theme.getMediumFont(context),
                                fontWeight: FontWeight.w300)),
                        Text("Identity: ${invite.invite.public.identity}",
                            style: TextStyle(
                                color: textColor,
                                fontSize: theme.getMediumFont(context),
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
                                backgroundColor: errorColor),
                            onPressed: () => onDenyInvite(context),
                            child: const Text("Deny")),
                        const Expanded(child: Empty()),
                      ]))
                ]))));
  }
}
