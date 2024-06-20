import 'dart:async';

import 'package:bruig/components/collapsable.dart';
import 'package:bruig/components/dcr_input.dart';
import 'package:bruig/components/info_grid.dart';
import 'package:bruig/components/inputs.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/notifications.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:bruig/util.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import 'package:bruig/theme_manager.dart';
import 'package:tuple/tuple.dart';

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

  void getNewAddress() async {
    try {
      var res = await Golib.lnGetDepositAddr("");
      setState(() {
        addr = res;
      });
    } catch (exception) {
      showErrorSnackbar(this, "Unable to load deposit address: $exception");
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
        popNavigatorFromState(this);
        if (needsInbound) {
          pushNavigatorFromState(this, "/needsInChannel");
        }
      }
    } catch (exception) {
      showErrorSnackbar(this, "Unable to update wallet balance: $exception");
    } finally {
      updateTimer = Timer(const Duration(seconds: 5), updateBalance);
    }
  }

  Future<void> openChannel() async {
    var snackbar = SnackBarModel.of(context);

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
      snackbar.success("Opening channel...");
    } catch (exception) {
      snackbar.error("Unable to open channel: $exception");
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
      } else if (res.chains[0].network == "simnet") {
        setState(() {
          peerCtrl.text =
              "03bb9246b8eaacde90c3b9e7a0539b0b70cde514ec0d2571c68063ac15edac5534@127.0.0.1:20102";
        });
      }
    } catch (exception) {
      showErrorSnackbar(this, "Unable to verify network: $exception");
    }
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
    return StartupScreen([
      const Center(child: Txt.H("Setting up Bison Relay")),
      const SizedBox(height: 20),
      const Txt.L("Add Outbound Capacity"),
      const SizedBox(height: 20),
      const SizedBox(
          width: 650,
          child: Text(
            '''
The wallet requires LN channels with outbound capacity to send funds ("bandwidth") in order to pay for messages to and from the server and to pay other users for their content.
                      ''',
          )),
      SizedBox(
          width: 350,
          child: SimpleInfoGrid(
              colLabelSize: 200,
              separatorWidth: 0,
              useListBuilder: false,
              rowAlignment: MainAxisAlignment.spaceBetween,
              [
                Tuple2(
                    const Txt.S("Wallet Balance:",
                        color: TextColor.onSurfaceVariant),
                    Txt.S(formatDCR(atomsToDCR(walletBalance)),
                        color: TextColor.onSurfaceVariant)),
                Tuple2(
                    const Txt.S("Outbound Channel Capacity:",
                        color: TextColor.onSurfaceVariant),
                    Txt.S(formatDCR(atomsToDCR(maxOutAmount)),
                        color: TextColor.onSurfaceVariant)),
                Tuple2(
                    const Txt.S("Pending Channels:",
                        color: TextColor.onSurfaceVariant),
                    Txt.S(numPendingChannels.toString(),
                        color: TextColor.onSurfaceVariant)),
                Tuple2(
                    const Txt.S("Active Channels:",
                        color: TextColor.onSurfaceVariant),
                    Txt.S(numChannels.toString(),
                        color: TextColor.onSurfaceVariant)),
              ])),
      const SizedBox(height: 10),
      ...(preventMsg != ""
          ? [Text(preventMsg)]
          : [
              Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                const Text("Amount:"),
                const SizedBox(width: 10),
                SizedBox(width: 200, child: dcrInput(controller: amountCtrl))
              ]),
              const SizedBox(height: 10),
              SizedBox(
                  width: 600,
                  child: Collapsable("Advanced Channel Config",
                      child: Column(children: [
                        Row(
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              const Txt.S("Peer ID and Address"),
                              const SizedBox(width: 10),
                              Expanded(
                                  child: TextInput(
                                      controller: peerCtrl,
                                      hintText: "node-pub-key@addr:port"))
                            ]),
                        const SizedBox(height: 10),
                      ]))),
              const SizedBox(height: 20),
              FilledButton.tonal(
                onPressed: !loading ? openChannel : null,
                child: const Txt.S("Request Outbound Channel"),
              ),
            ]),
      const SizedBox(height: 20),
      OutlinedButton(
        onPressed: () => Navigator.of(context).pop(),
        child: const Text("Skip"),
      ),
    ]);
  }
}
