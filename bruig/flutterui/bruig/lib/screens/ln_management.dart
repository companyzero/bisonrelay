import 'package:bruig/components/buttons.dart';
import 'package:bruig/screens/ln/accounts.dart';
import 'package:bruig/screens/ln/backups.dart';
import 'package:bruig/screens/ln/channels.dart';
import 'package:bruig/screens/ln/info.dart';
import 'package:bruig/screens/ln/network.dart';
import 'package:bruig/screens/ln/onchain.dart';
import 'package:bruig/screens/ln/payments.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import 'package:bruig/components/ln_management_bar.dart';
import 'package:bruig/models/snackbar.dart';

class LNScreenTitle extends StatelessWidget {
  const LNScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return Text("Bison Relay / LN",
        style: TextStyle(fontSize: 15, color: Theme.of(context).focusColor));
  }
}

class LNScreen extends StatefulWidget {
  static String routeName = "/ln";
  final SnackBarModel snackBar;
  const LNScreen(this.snackBar, {Key? key}) : super(key: key);

  @override
  State<LNScreen> createState() => _LNScreenState();
}

class _LNScreenState extends State<LNScreen> {
  SnackBarModel get snackBar => widget.snackBar;
  int tabIndex = 0;

  Widget activeTab() {
    switch (tabIndex) {
      case 0:
        return LNInfoPage(snackBar);
      case 1:
        return LNAccountsPage(snackBar);
      case 2:
        return LNOnChainPage(snackBar);
      case 3:
        return LNChannelsPage(snackBar);
      case 4:
        return LNPaymentsPage(snackBar);
      case 5:
        return LNNetworkPage(snackBar);
      case 6:
        return LNBackupsPage(snackBar);
    }
    return Text("Active is $tabIndex");
  }

  void onItemChanged(int index) {
    setState(() => tabIndex = index);
  }

  @override
  void initState() {
    super.initState();
  }

  @override
  void didUpdateWidget(LNScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
  }

  @override
  void dispose() {
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Row(children: [
      LNManagementBar(onItemChanged, tabIndex),
      Expanded(child: activeTab())
    ]);
  }
}

class LNConfirmRecvChanPaymentScreen extends StatelessWidget {
  const LNConfirmRecvChanPaymentScreen({Key? key}) : super(key: key);

  void cancel(BuildContext context) {
    Golib.lnConfirmPayReqRecvChan(false);
    Navigator.of(context).pop();
  }

  void pay(BuildContext context) {
    Golib.lnConfirmPayReqRecvChan(true);
    Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    final LNReqChannelEstValue est =
        ModalRoute.of(context)!.settings.arguments as LNReqChannelEstValue;

    var amount = formatDCR(atomsToDCR(est.amount));

    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    return Scaffold(
        body: Container(
            padding: const EdgeInsets.all(10),
            child: Column(children: [
              Text("Confirm LN Payment to Open Receive Channel",
                  style: TextStyle(color: textColor, fontSize: 20)),
              const SizedBox(height: 20),
              Text("Amount: $amount", style: TextStyle(color: textColor)),
              const SizedBox(height: 20),
              Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                ElevatedButton(
                    onPressed: () => pay(context),
                    child: Text("Pay", style: TextStyle(color: textColor))),
                const SizedBox(width: 20),
                CancelButton(onPressed: () => cancel(context)),
              ])
            ])));
  }
}
