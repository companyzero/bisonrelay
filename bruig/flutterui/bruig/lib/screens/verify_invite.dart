import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';
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

  List<Widget> buildFundsWidget(
      BuildContext context, InviteFunds funds, ThemeNotifier theme) {
    if (redeemed != null) {
      var total = formatDCR(atomsToDCR(redeemed!.total));
      return [
        Text("Redeemed $total on the following tx:",
            style: TextStyle(color: theme.getTheme().dividerColor)),
        Copyable(redeemed!.txid,
            textStyle: TextStyle(color: theme.getTheme().dividerColor)),
        Text("The funds will be available after the tx is mined.",
            style: TextStyle(color: theme.getTheme().dividerColor)),
      ];
    }

    if (redeeming) {
      return [
        Text("Attempting to redeem funds...",
            style: TextStyle(color: theme.getTheme().dividerColor))
      ];
    }

    return [
      Text("This invite contains funds stored in the following UTXO:",
          style: TextStyle(color: theme.getTheme().dividerColor)),
      Copyable("${funds.txid}:${funds.index}",
          textStyle: TextStyle(color: theme.getTheme().dividerColor)),
      Text("Attempt to redeem funds?",
          style: TextStyle(color: theme.getTheme().dividerColor)),
      const SizedBox(height: 10),
      ElevatedButton(
          onPressed: () => redeemFunds(context, funds),
          child: const Text("Redeem Funds")),
    ];
  }

  @override
  Widget build(BuildContext context) {
    var invite = ModalRoute.of(context)!.settings.arguments as Invitation;

    var errorColor = Colors.red;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => StartupScreen([
              Text("Accept Invite",
                  style: TextStyle(
                      color: theme.getTheme().dividerColor,
                      fontSize: theme.getHugeFont(context),
                      fontWeight: FontWeight.w200)),
              const SizedBox(height: 34),
              ...(invite.invite.funds != null
                  ? buildFundsWidget(context, invite.invite.funds!, theme)
                  : []),
              const SizedBox(height: 20),
              Text("Name: ${invite.invite.public.name}",
                  style: TextStyle(
                      color: theme.getTheme().dividerColor,
                      fontSize: theme.getMediumFont(context),
                      fontWeight: FontWeight.w300)),
              Text("Nick: ${invite.invite.public.nick}",
                  style: TextStyle(
                      color: theme.getTheme().dividerColor,
                      fontSize: theme.getMediumFont(context),
                      fontWeight: FontWeight.w300)),
              Text("Identity: ${invite.invite.public.identity}",
                  style: TextStyle(
                      color: theme.getTheme().dividerColor,
                      fontSize: theme.getMediumFont(context),
                      fontWeight: FontWeight.w300)),
              const SizedBox(height: 34),
              ElevatedButton(
                  onPressed:
                      !_loading ? () => onAcceptInvite(context, invite) : null,
                  child: const Text("Accept")),
              Container(height: 10),
              ElevatedButton(
                  style: ElevatedButton.styleFrom(backgroundColor: errorColor),
                  onPressed: () => onDenyInvite(context),
                  child: const Text("Deny")),
            ]));
  }
}
