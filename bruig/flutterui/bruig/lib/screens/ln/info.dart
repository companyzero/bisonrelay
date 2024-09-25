import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/info_grid.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/screens/ln/components.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import 'package:tuple/tuple.dart';

class LNInfoPage extends StatefulWidget {
  const LNInfoPage({super.key});

  @override
  State<LNInfoPage> createState() => _LNInfoPageState();
}

class _LNInfoPageState extends State<LNInfoPage> {
  bool loading = true;
  LNInfo info = LNInfo.empty();
  LNBalances balances = LNBalances.empty();
  String depositAddr = "";

  void loadInfo() async {
    var snackbar = SnackBarModel.of(context);
    setState(() => loading = true);
    try {
      var newInfo = await Golib.lnGetInfo();
      var newBalances = await Golib.lnGetBalances();
      setState(() {
        info = newInfo;
        balances = newBalances;
      });
    } catch (exception) {
      snackbar.error("Unable to load LN info: $exception");
    } finally {
      setState(() => loading = false);
    }
  }

  @override
  void initState() {
    super.initState();
    loadInfo();
  }

  @override
  Widget build(BuildContext context) {
    if (loading) {
      return const Text("Loading...");
    }

    var onChainBalance = formatDCR(atomsToDCR(balances.wallet.totalBalance));
    var maxReceive = formatDCR(atomsToDCR(balances.channel.maxInboundAmount));
    var maxSend = formatDCR(atomsToDCR(balances.channel.maxOutboundAmount));

    return Container(
        alignment: Alignment.topLeft,
        padding: const EdgeInsets.all(16),
        child: SingleChildScrollView(
            child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const LNInfoSectionHeader("Balances"),
            const SizedBox(height: 21),
            SimpleInfoGrid(colLabelSize: 110, [
              Tuple2(const Txt.S("Max Receivable:"), Txt.S(maxReceive)),
              Tuple2(const Txt.S("Max Sendable:"), Txt.S(maxSend)),
              Tuple2(const Txt.S("On-chain Balance:"), Txt.S(onChainBalance)),
            ]),
            const SizedBox(height: 34),
            const LNInfoSectionHeader("Balances"),
            const SizedBox(height: 21),
            SimpleInfoGrid(colLabelSize: 110, [
              Tuple2(const Txt.S("Chain Height"),
                  Txt.S(info.blockHeight.toString())),
              Tuple2(const Txt.S("Synced to Chain:"),
                  Txt.S(info.syncedToChain.toString())),
              Tuple2(const Txt.S("Synced to Graph:"),
                  Txt.S(info.syncedToGraph.toString())),
              Tuple2(const Txt.S("Pending Channels:"),
                  Txt.S(info.numPendingChannels.toString())),
              Tuple2(const Txt.S("Inactive Channels:"),
                  Txt.S(info.numInactiveChannels.toString())),
              Tuple2(const Txt.S("Active Channels:"),
                  Txt.S(info.numActiveChannels.toString())),
              Tuple2(const Txt.S("Version:"),
                  Copyable.txt(Txt.S(info.version.trim()))),
              Tuple2(const Txt.S("Node ID:"),
                  Copyable.txt(Txt.S(info.identityPubkey.trim()))),
              Tuple2(const Txt.S("Chain Hash:"),
                  Copyable.txt(Txt.S(info.blockHash.toString()))),
            ]),
          ],
        )));
  }
}
