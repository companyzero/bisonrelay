import 'dart:async';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/components/collapsable.dart';
import 'package:bruig/components/copyable.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/uistate.dart';
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
import 'package:bruig/theme_manager.dart';

class LNScreenTitle extends StatelessWidget {
  const LNScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer2<MainMenuModel, ThemeNotifier>(
        builder: (context, menu, theme, child) {
      if (menu.activePageTab <= 0) {
        return const Txt.L("LN");
      }
      var idx = lnScreenSub.indexWhere((e) => e.pageTab == menu.activePageTab);

      return Txt.L("LN / ${lnScreenSub[idx].label}");
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
        return Consumer<ClientModel>(
            builder: (context, client, child) => LNOnChainPage(client));
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
    bool isScreenSmall = checkIsScreenSmall(context);
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
  static String routeName = "/ln/confirmRecvChannelPay";

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

    var chanSize = formatDCR(atomsToDCR(est.request.chanSize));
    var amount = formatDCR(atomsToDCR(est.amount));
    var second = 1000000000;
    var hour = second * 60 * 60;
    var day = hour * 24;
    String minLifetime;
    if (est.serverPolicy.minChanLifetime < hour) {
      minLifetime =
          "${(est.serverPolicy.minChanLifetime / second).truncate()} seconds";
    } else if (est.serverPolicy.minChanLifetime < day) {
      minLifetime =
          "${(est.serverPolicy.minChanLifetime / hour).truncate()} hours";
    } else {
      minLifetime =
          "${(est.serverPolicy.minChanLifetime / day).truncate()} days";
    }

    return Scaffold(
        body: Container(
            padding: const EdgeInsets.all(10),
            child: Column(
                crossAxisAlignment: CrossAxisAlignment.center,
                children: [
                  const Txt.L("Confirm LN Payment to Open Receive Channel"),
                  const SizedBox(height: 20),
                  Text("Requested channel size: $chanSize"),
                  Text("Minimum channel lifetime: $minLifetime"),
                  Text("Payment amount: $amount"),
                  const SizedBox(height: 20),
                  Collapsable("Additional Information",
                      child: Column(children: [
                        Text("Server Address: ${est.request.server}"),
                        Wrap(alignment: WrapAlignment.center, children: [
                          const Text("Server Node ID: "),
                          Copyable(
                            est.serverPolicy.node,
                            textOverflow: TextOverflow.ellipsis,
                          )
                        ]),
                        Text(
                            "Server node addresses: ${est.serverPolicy.addresses.join(", ")}"),
                        Text(
                            "Max number of channels: ${est.serverPolicy.maxNbChannels}"),
                      ])),
                  const SizedBox(height: 20),
                  Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                    OutlinedButton(
                        onPressed: () => pay(context),
                        child: const Text("Pay")),
                    const SizedBox(width: 20),
                    CancelButton(onPressed: () => cancel(context)),
                  ])
                ])));
  }
}
