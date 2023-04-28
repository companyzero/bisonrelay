import 'dart:async';

import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/models/notifications.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import 'package:qr_flutter/qr_flutter.dart';

class NeedsFundsScreen extends StatefulWidget {
  final AppNotifications ntfns;
  final SnackBarModel snackBar;
  const NeedsFundsScreen(this.ntfns, this.snackBar, {Key? key})
      : super(key: key);

  @override
  State<NeedsFundsScreen> createState() => _NeedsFundsScreenState();
}

class _NeedsFundsScreenState extends State<NeedsFundsScreen> {
  SnackBarModel get snackBar => widget.snackBar;
  String addr = "";
  int confirmedBalance = 0;
  int unconfirmedBalance = 0;
  Timer? updateTimer;
  bool createdNeedsChanNtf = false;
  bool redeeming = false;
  RedeemedInviteFunds? redeemed;
  bool? forwardIfBalance;

  void getNewAddress() async {
    try {
      var res = await Golib.lnGetDepositAddr("");
      setState(() {
        addr = res;
      });
    } catch (exception) {
      snackBar.error("Unable to load deposit address: $exception");
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

          if (forwardIfBalance ?? false) {
            Navigator.of(context, rootNavigator: true).pop();
            Navigator.of(context, rootNavigator: true)
                .pushNamed("/needsOutChannel");
            done = true;
          }
          forwardIfBalance = false;
        }
      } else {
        forwardIfBalance = true;
      }
    } catch (exception) {
      snackBar.error("Unable to update wallet balance: $exception");
    } finally {
      if (!done) {
        updateTimer = Timer(const Duration(seconds: 5), updateBalance);
      }
    }
  }

  void redeemFunds() async {
    try {
      // Decode the invite and send to the user verification screen.
      var filePickRes = await FilePicker.platform.pickFiles();
      if (filePickRes == null) return;
      var filePath = filePickRes.files.first.path;
      if (filePath == null) return;
      filePath = filePath.trim();
      if (filePath == "") return;
      var invite = await Golib.decodeInvite(filePath);
      if (invite.invite.funds == null) {
        throw "Invite does not include funds";
      }
      setState(() => redeeming = true);
      var res = await Golib.redeemInviteFunds(invite.invite.funds!);
      setState(() => redeemed = res);
    } catch (exception) {
      setState(() => redeeming = false);
      snackBar.error("Unable to redeem invite funds: $exception");
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

  Widget buildFundsWidget(BuildContext context) {
    var secondaryTextColor = const Color(0xFFE4E3E6);
    var ts = TextStyle(
        color: secondaryTextColor, fontSize: 13, fontWeight: FontWeight.w300);

    if (redeemed != null) {
      var total = formatDCR(atomsToDCR(redeemed!.total));
      return Column(children: [
        Text("Redeemed $total on the following tx:", style: ts),
        Copyable(redeemed!.txid, ts),
        Text("The funds will be available after the tx is mined.", style: ts),
      ]);
    }

    if (redeeming) {
      return Column(children: [
        Text("Attempting to redeem funds from invite file.\n", style: ts),
      ]);
    }

    return Row(mainAxisAlignment: MainAxisAlignment.center, children: [
      Text('''
If someone sent you an invite file with funds, you  may also attempt
to redeem it by clicking in the button.
''', style: ts),
      const SizedBox(width: 18),
      ElevatedButton(onPressed: redeemFunds, child: const Text("Redeem Funds")),
    ]);
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
                      const Expanded(child: Empty()),
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
                      const SizedBox(height: 20),
                      Text('''
The wallet requires on-chain DCR funds to be able to open Lightning Network (LN) channels
and perform payments to the server and other users of the Bison Relay network.

Send DCR funds to the following address to receive funds in your wallet. Note that the
wallet seed will be needed to recover these funds if the wallet data in this computer is
corrupted or lost.
''',
                          style: TextStyle(
                              color: secondaryTextColor,
                              fontSize: 13,
                              fontWeight: FontWeight.w300)),
                      buildFundsWidget(context),
                      const SizedBox(height: 13),
                      Container(
                          margin: const EdgeInsets.all(10),
                          color: Colors.white,
                          child: QrImage(
                            data: addr,
                            version: QrVersions.auto,
                            size: 200.0,
                          )),
                      const SizedBox(height: 13),
                      Container(
                          color: cardColor,
                          padding: const EdgeInsets.only(
                              left: 22, top: 18, right: 22, bottom: 18),
                          child: Copyable(
                              addr, TextStyle(color: textColor, fontSize: 15))),
                      const SizedBox(height: 9),
                      Container(
                          padding: const EdgeInsets.only(
                              left: 324 + 22, right: 324 + 20),
                          child: Row(children: [
                            Text(
                                textAlign: TextAlign.left,
                                "Unconfirmed wallet balance:",
                                style: TextStyle(
                                    color: darkTextColor,
                                    fontSize: 13,
                                    fontWeight: FontWeight.w300)),
                            Text(
                                textAlign: TextAlign.right,
                                formatDCR(atomsToDCR(unconfirmedBalance)),
                                style: TextStyle(
                                    color: darkTextColor,
                                    fontSize: 13,
                                    fontWeight: FontWeight.w300)),
                          ])),
                      const SizedBox(height: 3),
                      Container(
                          padding: const EdgeInsets.only(
                              left: 324 + 22, right: 324 + 20),
                          child: Row(children: [
                            Text(
                                textAlign: TextAlign.left,
                                "Confirmed wallet balance:",
                                style: TextStyle(
                                    color: darkTextColor,
                                    fontSize: 13,
                                    fontWeight: FontWeight.w300)),
                            Text(
                                textAlign: TextAlign.right,
                                formatDCR(atomsToDCR(confirmedBalance)),
                                style: TextStyle(
                                    color: darkTextColor,
                                    fontSize: 13,
                                    fontWeight: FontWeight.w300))
                          ])),
                      const SizedBox(height: 20),
                      LoadingScreenButton(
                        onPressed: () => Navigator.of(context).pop(),
                        text: "Finish",
                      ),
                      const Expanded(child: Empty()),
                    ],
                  )),
            ])));
  }
}
