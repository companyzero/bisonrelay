import 'dart:async';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/dcr_input.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/notifications.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

class NeedsOutChannelScreen extends StatefulWidget {
  static const routeName = "/needsOutChannel";
  final AppNotifications ntfns;
  final ClientModel client;
  const NeedsOutChannelScreen(this.ntfns, this.client, {Key? key})
      : super(key: key);

  @override
  State<NeedsOutChannelScreen> createState() => _NeedsOutChannelScreenState();
}

class _NeedsOutChannelScreenState extends State<NeedsOutChannelScreen> {
  ClientModel get client => widget.client;

  String addr = "";
  int initialMaxOutAmount = -1;
  int maxOutAmount = 0;
  int walletBalance = 0;
  int numPendingChannels = 0;
  int numChannels = 0;
  Timer? updateTimer;
  bool loading = false;
  TextEditingController peerCtrl = TextEditingController();
  AmountEditingController amountCtrl = AmountEditingController();
  String preventMsg = "foo";
  bool showAdvanced = false;

  void getNewAddress() async {
    try {
      var res = await Golib.lnGetDepositAddr("");
      setState(() {
        addr = res;
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to load deposit address: $exception");
    }
  }

  void updateBalance() async {
    try {
      var res = await Golib.lnGetBalances();
      var resInfo = await Golib.lnGetInfo();
      var resPending = await Golib.lnListPendingChannels();
      setState(() {
        maxOutAmount = res.channel.maxOutboundAmount;
        walletBalance = res.wallet.totalBalance;
        numPendingChannels = resPending.pendingOpen.length;
        numChannels = resInfo.numActiveChannels;

        if (numPendingChannels > 0) {
          preventMsg =
              '''Cannot open new outbound channels while the local client has pending
channels. Wait until all pending channels have been confirmed before
attempting to open a new one.''';
        } else if (res.wallet.confirmedBalance == 0 && walletBalance > 0) {
          preventMsg =
              '''Cannot open new outbound channels while the local client doesn't
have a confirmed wallet balance. Wait until any recent transactions have been
confirmed on-chain before attempting to open a new channel.''';
        } else if (walletBalance == 0) {
          preventMsg =
              '''Cannot open a new outbound channel while the local client doesn't
have any funds in its wallet. Send funds on-chain to the wallet so that it can
open channels to other LN nodes.''';
        } else {
          preventMsg = "";
        }
      });

      if (initialMaxOutAmount == -1) {
        initialMaxOutAmount = res.channel.maxOutboundAmount;
      }

      if (res.channel.maxOutboundAmount > 0) {
        widget.ntfns.delType(AppNtfnType.walletNeedsChannels);
      }
      var needsInbound = res.channel.maxInboundAmount == 0;
      if (res.channel.maxOutboundAmount > initialMaxOutAmount) {
        Navigator.of(context).pop();
        if (needsInbound) {
          Navigator.of(context).pushNamed("/needsInChannel");
        }
      }
    } catch (exception) {
      showErrorSnackbar(context, "Unable to update wallet balance: $exception");
    } finally {
      updateTimer = Timer(const Duration(seconds: 5), updateBalance);
    }
  }

  Future<void> openChannel() async {
    var peer = peerCtrl.text.trim();
    var amount = amountCtrl.amount;

    setState(() => loading = true);
    try {
      // Connect to peer first.
      var p = peer.indexOf("@");
      if (p > -1) {
        try {
          await Golib.lnConnectToPeer(peer);
        } catch (exception) {
          // Ignore "already connected" exceptions.
          if (!exception.toString().contains("already connected")) rethrow;
        }
        peer = peer.substring(0, p);
      }

      await Golib.lnOpenChannel(peer, amount, 0);
      setState(() {
        peerCtrl.clear();
        amountCtrl.clear();
      });
      showSuccessSnackbar(context, "Opening channel...");
    } catch (exception) {
      showErrorSnackbar(context, "Unable to open channel: $exception");
      return;
    } finally {
      setState(() => loading = false);
    }
  }

  void verifyNetwork() async {
    try {
      var res = await Golib.lnGetInfo();
      if (res.chains[0].network == "mainnet") {
        setState(() {
          peerCtrl.text =
              "03bd03386d7b2efe80ae46d6c8cfcfdfcf9c9297a465ac0d48c110d11ae58ed509@hub0.bisonrelay.org:9735";
        });
      }
    } catch (exception) {
      showErrorSnackbar(context, "Unable to verify network: $exception");
    }
  }

  void showAdvancedArea() {
    setState(() {
      showAdvanced = true;
    });
  }

  void hideAdvancedArea() {
    setState(() {
      showAdvanced = false;
    });
  }

  @override
  void initState() {
    super.initState();
    verifyNetwork();
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
    return Consumer<ThemeNotifier>(
      builder: (context, theme, child) => StartupScreen(
        [
          Center(
            child: Text("Setting up Bison Relay",
                style: TextStyle(
                    color: theme.getTheme().dividerColor,
                    fontSize: theme.getHugeFont(context),
                    fontWeight: FontWeight.w200)),
          ),
          const SizedBox(height: 20),
          Center(
            child: Text("Add Outbound Capacity",
                style: TextStyle(
                    color: theme.getTheme().focusColor,
                    fontSize: theme.getLargeFont(context),
                    fontWeight: FontWeight.w300)),
          ),
          const SizedBox(height: 20),
          Center(
            child: SizedBox(
                width: 650,
                child: Text(
                  '''
The wallet requires LN channels with outbound capacity to send funds ("bandwidth") in order to pay for messages to and from the server and to pay other users for their content.
                      ''',
                  style: TextStyle(
                      color: theme.getTheme().focusColor,
                      fontSize: theme.getMediumFont(context),
                      fontWeight: FontWeight.w300),
                )),
          ),
          const SizedBox(height: 10),
          ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 400),
              child: Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(
                        textAlign: TextAlign.left,
                        "Wallet Balance:",
                        style: TextStyle(
                            color: theme.getTheme().indicatorColor,
                            fontSize: theme.getSmallFont(context),
                            fontWeight: FontWeight.w300)),
                    Text(
                        textAlign: TextAlign.right,
                        formatDCR(atomsToDCR(walletBalance)),
                        style: TextStyle(
                            color: theme.getTheme().indicatorColor,
                            fontSize: theme.getSmallFont(context),
                            fontWeight: FontWeight.w300)),
                  ])),
          const SizedBox(height: 3),
          ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 400),
              child: Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(
                        textAlign: TextAlign.left,
                        "Outbound Channel Capacity:",
                        style: TextStyle(
                            color: theme.getTheme().indicatorColor,
                            fontSize: theme.getSmallFont(context),
                            fontWeight: FontWeight.w300)),
                    Text(
                        textAlign: TextAlign.right,
                        formatDCR(atomsToDCR(maxOutAmount)),
                        style: TextStyle(
                            color: theme.getTheme().indicatorColor,
                            fontSize: theme.getSmallFont(context),
                            fontWeight: FontWeight.w300))
                  ])),
          ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 400),
              child: Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(
                        textAlign: TextAlign.left,
                        "Pending Channels:",
                        style: TextStyle(
                            color: theme.getTheme().indicatorColor,
                            fontSize: theme.getSmallFont(context),
                            fontWeight: FontWeight.w300)),
                    Text(
                        textAlign: TextAlign.right,
                        "$numPendingChannels",
                        style: TextStyle(
                            color: theme.getTheme().indicatorColor,
                            fontSize: theme.getSmallFont(context),
                            fontWeight: FontWeight.w300))
                  ])),
          ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 400),
              child: Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(
                        textAlign: TextAlign.left,
                        "Active Channels:",
                        style: TextStyle(
                            color: theme.getTheme().indicatorColor,
                            fontSize: theme.getSmallFont(context),
                            fontWeight: FontWeight.w300)),
                    Text(
                        textAlign: TextAlign.right,
                        "$numChannels",
                        style: TextStyle(
                            color: theme.getTheme().indicatorColor,
                            fontSize: theme.getSmallFont(context),
                            fontWeight: FontWeight.w300))
                  ])),
          const SizedBox(height: 10),
          preventMsg == ""
              ? Column(children: [
                  Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                    Text("Amount",
                        style: TextStyle(
                            color: theme.getTheme().indicatorColor,
                            fontSize: theme.getMediumFont(context),
                            fontWeight: FontWeight.w300)),
                    const SizedBox(
                      width: 10,
                    ),
                    SizedBox(
                      width: 150,
                      child: dcrInput(controller: amountCtrl),
                    )
                  ]),
                  const SizedBox(height: 10),
                  LoadingScreenButton(
                    empty: true,
                    onPressed:
                        showAdvanced ? hideAdvancedArea : showAdvancedArea,
                    text: showAdvanced ? "Hide Advanced" : "Show Advanced",
                  ),
                  const SizedBox(height: 10),
                  LoadingScreenButton(
                    onPressed: !loading ? openChannel : null,
                    text: "Request Outbound Channel",
                  ),
                  showAdvanced
                      ? Column(children: [
                          const SizedBox(height: 10),
                          Row(
                              mainAxisAlignment: MainAxisAlignment.center,
                              children: [
                                Text("Peer ID and Address",
                                    style: TextStyle(
                                        color: theme.getTheme().indicatorColor,
                                        fontSize: theme.getSmallFont(context),
                                        fontWeight: FontWeight.w300)),
                                const SizedBox(width: 10),
                                Expanded(
                                    child: TextField(
                                  style: TextStyle(
                                      color: theme.getTheme().focusColor,
                                      fontSize: theme.getSmallFont(context),
                                      fontWeight: FontWeight.w300),
                                  controller: peerCtrl,
                                  decoration: const InputDecoration(
                                      hintText: "node-pub-key@addr:port"),
                                ))
                              ]),
                          const SizedBox(height: 10),
                        ])
                      : const SizedBox(height: 10),
                ])
              : Text(preventMsg,
                  style: TextStyle(color: theme.getTheme().dividerColor)),
          LoadingScreenButton(
            onPressed: () => Navigator.of(context).pop(),
            text: "Skip",
          ),
        ],
      ),
    );
  }
}
