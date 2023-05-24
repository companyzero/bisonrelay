import 'dart:async';

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
import 'package:bruig/screens/overview.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/models/menus.dart';
import 'package:provider/provider.dart';

class LNScreenTitle extends StatelessWidget {
  const LNScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer<MainMenuModel>(builder: (context, menu, child) {
      if (menu.activePageTab <= 0) {
        return Text("Bison Relay / LN",
            style:
                TextStyle(fontSize: 15, color: Theme.of(context).focusColor));
      }
      var idx = LnScreenSub.indexWhere((e) => e.pageTab == menu.activePageTab);

      return Text("Bison Relay / LN / ${LnScreenSub[idx].label}",
          style: TextStyle(fontSize: 15, color: Theme.of(context).focusColor));
    });
  }
}

class LNScreen extends StatefulWidget {
  static String routeName = "/ln";
  final MainMenuModel mainMenu;
  const LNScreen(this.mainMenu, {Key? key}) : super(key: key);

  @override
  State<LNScreen> createState() => _LNScreenState();
}

class _LNScreenState extends State<LNScreen> {
  int tabIndex = 0;

  Widget activeTab() {
    switch (tabIndex) {
      case 0:
        return const LNInfoPage();
      case 1:
        return const LNAccountsPage();
      case 2:
        return const LNOnChainPage();
      case 3:
        return const LNChannelsPage();
      case 4:
        return const LNPaymentsPage();
      case 5:
        return const LNNetworkPage();
      case 6:
        return const LNBackupsPage();
    }
    return Text("Active is $tabIndex");
  }

  void onItemChanged(int index) {
    setState(() => tabIndex = index);
    Timer(const Duration(milliseconds: 1),
        () async => widget.mainMenu.activePageTab = index);
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
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    if (ModalRoute.of(context)!.settings.arguments != null) {
      final args = ModalRoute.of(context)!.settings.arguments as PageTabs;
      tabIndex = args.tabIndex;
    }

    return Row(children: [
      ModalRoute.of(context)!.settings.arguments == null
          ? isScreenSmall
              ? const Empty()
              : LNManagementBar(onItemChanged, tabIndex)
          : const Empty(),
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
