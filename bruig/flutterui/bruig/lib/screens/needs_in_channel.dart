import 'dart:async';

import 'package:bruig/components/collapsable.dart';
import 'package:bruig/components/dcr_input.dart';
import 'package:bruig/components/info_grid.dart';
import 'package:bruig/components/inputs.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/notifications.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import 'package:bruig/theme_manager.dart';
import 'package:tuple/tuple.dart';

class NeedsInChannelScreen extends StatefulWidget {
  final AppNotifications ntfns;
  final ClientModel client;
  const NeedsInChannelScreen(this.ntfns, this.client, {Key? key})
      : super(key: key);

  @override
  State<NeedsInChannelScreen> createState() => _NeedsInChannelScreenState();
}

class _NeedsInChannelScreenState extends State<NeedsInChannelScreen> {
  ClientModel get client => widget.client;

  String addr = "";
  int initialMaxInAmount = -1;
  int maxOutAmount = 0;
  int maxInAmount = 0;
  int walletBalance = 0;
  int numPendingChannels = 0;
  int numChannels = 0;
  Timer? updateTimer;
  bool loading = false;
  TextEditingController serverCtrl = TextEditingController();
  TextEditingController certCtrl = TextEditingController();
  AmountEditingController amountCtrl = AmountEditingController();
  String preventMsg = "";

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

  void updateBalance(bool resetTimer) async {
    try {
      var res = await Golib.lnGetBalances();
      var resInfo = await Golib.lnGetInfo();
      var resPending = await Golib.lnListPendingChannels();
      setState(() {
        maxOutAmount = res.channel.maxOutboundAmount;
        maxInAmount = res.channel.maxInboundAmount;
        walletBalance = res.wallet.totalBalance;
        numPendingChannels = resPending.pendingOpen.length;
        numChannels = resInfo.numActiveChannels;
        if (maxOutAmount == 0) {
          preventMsg =
              '''The client cannot open an inbound channel without having channels
with outbound capacity. Please open new outbound channels before
requesting  inbound capacity.''';
        } else if (numPendingChannels > 0) {
          preventMsg =
              '''The client cannot open an inbound channel while it still
has pending channels being opened. Wait until the pending
channel is confirmed to request a new inbound channel''';
        } else {
          preventMsg = "";
        }
      });

      if (initialMaxInAmount == -1) {
        initialMaxInAmount = res.channel.maxInboundAmount;
      }

      if (res.channel.maxInboundAmount > 0) {
        widget.ntfns.delType(AppNtfnType.walletNeedsInChannels);

        if (res.channel.maxInboundAmount > initialMaxInAmount) {
          Navigator.of(context).pop();
        }
      }
    } catch (exception) {
      showErrorSnackbar(context, "Unable to update wallet balance: $exception");
    } finally {
      if (resetTimer) {
        updateTimer =
            Timer(const Duration(seconds: 5), () => updateBalance(true));
      }
    }
  }

  void requestRecvCapacity() async {
    if (serverCtrl.text == "") {
      showErrorSnackbar(context, "Liquidity provider server cannot be empty");
      return;
    }

    if (amountCtrl.amount < 0.00001) {
      showErrorSnackbar(
          context, "Channel size to request liquidity is too low");
      return;
    }

    setState(() => loading = true);
    try {
      await Golib.lnRequestRecvCapacity(
          serverCtrl.text, "", amountCtrl.amount, certCtrl.text);
      setState(() {
        serverCtrl.clear();
        amountCtrl.clear();
      });
    } catch (exception) {
      showErrorSnackbar(
          context, "Unable to request receive capacity: $exception");
    } finally {
      setState(() => loading = false);
      updateBalance(false);
    }
  }

  void verifyNetwork() async {
    try {
      var res = await Golib.lnGetInfo();
      if (res.chains[0].network == "mainnet") {
        setState(() {
          serverCtrl.text = "https://lp0.bisonrelay.org:9130";
          certCtrl.text = """-----BEGIN CERTIFICATE-----
MIIBwjCCAWmgAwIBAgIQA78YKmDt+ffFJmAN5EZmejAKBggqhkjOPQQDAjAyMRMw
EQYDVQQKEwpiaXNvbnJlbGF5MRswGQYDVQQDExJscDAuYmlzb25yZWxheS5vcmcw
HhcNMjIwOTE4MTMzNjA4WhcNMzIwOTE2MTMzNjA4WjAyMRMwEQYDVQQKEwpiaXNv
bnJlbGF5MRswGQYDVQQDExJscDAuYmlzb25yZWxheS5vcmcwWTATBgcqhkjOPQIB
BggqhkjOPQMBBwNCAASF1StlsfdDUaCXMiZvDBhhMZMdvAUoD6wBdS0tMBN+9y91
UwCBu4klh+VmpN1kCzcR6HJHSx5Cctxn7Smw/w+6o2EwXzAOBgNVHQ8BAf8EBAMC
AoQwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUqqlcDx8e+XgXXU9cXAGQEhS8
59kwHQYDVR0RBBYwFIISbHAwLmJpc29ucmVsYXkub3JnMAoGCCqGSM49BAMCA0cA
MEQCIGtLFLIVMnU2EloN+gI+uuGqqqeBIDSNhP9+bznnZL/JAiABsLKKtaTllCSM
cNPr8Y+sSs2MHf6xMNBQzV4KuIlPIg==
-----END CERTIFICATE-----""";
        });
      } else if (res.chains[0].network == "simnet") {
        setState(() {
          serverCtrl.text = "https://127.0.0.1:29130";
        });
      }
    } catch (exception) {
      showErrorSnackbar(context, "Unable to verify network: $exception");
    }
  }

  @override
  void initState() {
    super.initState();
    verifyNetwork();
    getNewAddress();
    updateBalance(true);
  }

  @override
  void dispose() {
    updateTimer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return StartupScreen([
      const Txt.H("Setting up Bison Relay"),
      const SizedBox(height: 20),
      const Txt.L("Add Inbound Capacity"),
      const SizedBox(height: 20),
      const SizedBox(
          width: 650,
          child: Text(
            '''
The wallet requires LN channels with inbound capacity to receive funds to be able to receive payments from other users.
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
                    const Txt.S("Outbound Channel Capacity:",
                        color: TextColor.onSurfaceVariant),
                    Txt.S(formatDCR(atomsToDCR(maxOutAmount)),
                        color: TextColor.onSurfaceVariant)),
                Tuple2(
                    const Txt.S("Inbound Channel Capacity:",
                        color: TextColor.onSurfaceVariant),
                    Txt.S(formatDCR(atomsToDCR(maxInAmount)),
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
      ...(preventMsg != ""
          ? [Text(preventMsg)]
          : [
              Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                const Text("Amount:"),
                const SizedBox(width: 10),
                SizedBox(width: 250, child: dcrInput(controller: amountCtrl)),
              ]),
              const SizedBox(height: 20),
              SizedBox(
                  width: 600,
                  child: Collapsable("Advanced Channel Config",
                      child: Column(children: [
                        Row(children: [
                          const SizedBox(
                              width: 140,
                              child: Text("LP Server Address:",
                                  textAlign: TextAlign.right)),
                          const SizedBox(width: 10),
                          Expanded(
                              child: TextInput(
                                  controller: serverCtrl,
                                  hintText: "https://lpd-server:port"))
                        ]),
                        const SizedBox(height: 20),
                        Row(children: [
                          const SizedBox(
                              width: 140,
                              child: Text("LP Server Cert:",
                                  textAlign: TextAlign.right)),
                          const SizedBox(width: 10),
                          Expanded(
                              child: TextField(
                            controller: certCtrl,
                            maxLines: 10,
                            keyboardType: TextInputType.multiline,
                          ))
                        ])
                      ]))),
              const SizedBox(height: 20),
              FilledButton.tonal(
                onPressed: !loading ? requestRecvCapacity : null,
                child: const Text("Request Inbound Channel"),
              ),
            ]),
      const SizedBox(height: 20),
      OutlinedButton(
        onPressed: () => Navigator.of(context).pop(),
        child: const Text("Skip"),
      ),
      const SizedBox(height: 30),
      const SizedBox(width: 650, child: Text('''
Explanation of Inbound Channels:

One way of opening a channel with inbound capacity is to pay for a node to open a channel back to your LN wallet. This is done through a "Liquidity Provider" service.

Note that having a channel with inbound capacity is not for sending or receiving messages. It is only required in order to receive payments from other users.

After the channel is opened, it may take up to 6 confirmations for it to be broadcast through the network. Individual peers may take longer to detect and to consider the channel to send payments.
                ''')),
    ]);
  }
}
