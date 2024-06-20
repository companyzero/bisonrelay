import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import 'package:provider/provider.dart';
import 'package:bruig/screens/startupscreen.dart';

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
    var snackbar = SnackBarModel.of(context);

    try {
      var res = await Golib.redeemInviteFunds(funds);
      setState(() {
        redeemed = res;
      });
    } catch (exception) {
      snackbar.error("Unable to redeem funds: $exception");
      setState(() => redeeming = false);
    }
  }

  List<Widget> buildFundsWidget(BuildContext context, InviteFunds funds) {
    if (redeemed != null) {
      var total = formatDCR(atomsToDCR(redeemed!.total));
      return [
        Text("Redeemed $total on the following tx:"),
        Copyable(redeemed!.txid),
        const Text("The funds will be available after the tx is mined."),
      ];
    }

    if (redeeming) {
      return [const Text("Attempting to redeem funds...")];
    }

    return [
      const Text("This invite contains funds stored in the following UTXO:"),
      Copyable("${funds.txid}:${funds.index}"),
      const Text("Attempt to redeem funds?"),
      const SizedBox(height: 10),
      OutlinedButton(
          onPressed: () => redeemFunds(context, funds),
          child: const Text("Redeem Funds")),
    ];
  }

  @override
  Widget build(BuildContext context) {
    var invite = ModalRoute.of(context)!.settings.arguments as Invitation;

    return StartupScreen([
      const Txt.H("Accept Invite"),
      const SizedBox(height: 34),
      ...(invite.invite.funds != null
          ? buildFundsWidget(context, invite.invite.funds!)
          : []),
      const SizedBox(height: 20),
      Text("Name: ${invite.invite.public.name}"),
      Text("Nick: ${invite.invite.public.nick}"),
      Copyable(invite.invite.public.identity,
          child: Text("Identity: ${invite.invite.public.identity}")),
      const SizedBox(height: 34),
      SizedBox(
          width: 600,
          child:
              Row(mainAxisAlignment: MainAxisAlignment.spaceBetween, children: [
            FilledButton.tonal(
                onPressed:
                    !_loading ? () => onAcceptInvite(context, invite) : null,
                child: const Text("Accept")),
            Container(height: 10),
            CancelButton(onPressed: () => onDenyInvite(context), label: "Deny"),
          ])),
    ]);
  }
}
