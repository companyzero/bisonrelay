import 'dart:async';

import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/notifications.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';

class NeedsFundsScreen extends StatefulWidget {
  final AppNotifications ntfns;
  const NeedsFundsScreen(this.ntfns, {Key? key}) : super(key: key);

  @override
  State<NeedsFundsScreen> createState() => _NeedsFundsScreenState();
}

class _NeedsFundsScreenState extends State<NeedsFundsScreen> {
  String addr = "";
  int confirmedBalance = 0;
  int unconfirmedBalance = 0;
  Timer? updateTimer;
  bool createdNeedsChanNtf = false;

  void getNewAddress() async {
    try {
      var res = await Golib.lnGetDepositAddr();
      setState(() {
        addr = res;
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to load deposit address: $exception");
    }
  }

  void updateBalance() async {
    var done = false;
    try {
      var res = await Golib.lnGetBalances();
      setState(() {
        confirmedBalance = res.wallet.confirmedBalance;
        unconfirmedBalance = res.wallet.unconfirmedBalance;
      });

      if (res.wallet.confirmedBalance > 0) {
        widget.ntfns.delType(AppNtfnType.walletNeedsFunds);

        if (res.channel.maxOutboundAmount == 0 && !createdNeedsChanNtf) {
          widget.ntfns.addNtfn(AppNtfn(AppNtfnType.walletNeedsChannels));
          createdNeedsChanNtf = true;

          Navigator.of(context, rootNavigator: true).pop();
          Navigator.of(context, rootNavigator: true)
              .pushNamed("/needsOutChannel");
          done = true;
        }
      }
    } catch (exception) {
      showErrorSnackbar(context, "Unable to update wallet balance: $exception");
    } finally {
      if (!done) {
        updateTimer = Timer(const Duration(seconds: 5), updateBalance);
      }
    }
  }

  @override
  void initState() {
    super.initState();
    getNewAddress();
    updateBalance();
  }

  @override
  void dispose() {
    updateTimer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
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
                  child: Column(
                    children: [
                      const SizedBox(height: 89),
                      Text("Setting up Bison Relay",
                          style: TextStyle(
                              color: textColor,
                              fontSize: 34,
                              fontWeight: FontWeight.w200)),
                      const SizedBox(height: 20),
                      Text("Receive Wallet Funds",
                          style: TextStyle(
                              color: secondaryTextColor,
                              fontSize: 21,
                              fontWeight: FontWeight.w300)),
                      const SizedBox(height: 34),
                      Text('''
The wallet requires on-chain DCR funds to be able to open Lightning Network (LN)
channels and perform payments to the server and other users of the Bison Relay
network.

Send DCR funds to the folowing address to receive funds in your wallet. Note that
the wallet seed will be needed to recover these funds if the wallet data in this
computer is corrupted or lost.
''',
                          style: TextStyle(
                              color: secondaryTextColor,
                              fontSize: 13,
                              fontWeight: FontWeight.w300)),
                      const SizedBox(height: 21),
                      Container(
                          margin: const EdgeInsets.only(left: 324, right: 324),
                          color: cardColor,
                          padding: const EdgeInsets.only(
                              left: 22, top: 18, right: 22, bottom: 18),
                          child: Copyable(
                              addr, TextStyle(color: textColor, fontSize: 15))),
                      const SizedBox(height: 9),
                      Container(
                          padding: const EdgeInsets.only(
                              left: 324 + 22, right: 324 + 20),
                          child: Column(children: [
                            Row(children: [
                              Text(
                                  textAlign: TextAlign.left,
                                  "Unconfirmed wallet balance:",
                                  style: TextStyle(
                                      color: darkTextColor,
                                      fontSize: 13,
                                      fontWeight: FontWeight.w300)),
                              Text(
                                  textAlign: TextAlign.right,
                                  "${formatDCR(atomsToDCR(unconfirmedBalance))}",
                                  style: TextStyle(
                                      color: darkTextColor,
                                      fontSize: 13,
                                      fontWeight: FontWeight.w300)),
                            ]),
                            const SizedBox(height: 3),
                            Row(children: [
                              Text(
                                  textAlign: TextAlign.left,
                                  "Confirmed wallet balance:",
                                  style: TextStyle(
                                      color: darkTextColor,
                                      fontSize: 13,
                                      fontWeight: FontWeight.w300)),
                              Text(
                                  textAlign: TextAlign.right,
                                  "${formatDCR(atomsToDCR(confirmedBalance))}",
                                  style: TextStyle(
                                      color: darkTextColor,
                                      fontSize: 13,
                                      fontWeight: FontWeight.w300))
                            ])
                          ])),
                      const SizedBox(height: 34),
                      LoadingScreenButton(
                        onPressed: () => Navigator.of(context).pop(),
                        text: "Finish",
                      )
                    ],
                  )),
            ])));
  }
}
