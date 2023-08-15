import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/buttons.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import 'package:qr_flutter/qr_flutter.dart';
import 'package:bruig/components/empty_widget.dart';

class LNInfoPage extends StatefulWidget {
  const LNInfoPage({Key? key}) : super(key: key);

  @override
  State<LNInfoPage> createState() => _LNInfoPageState();
}

class _LNInfoPageState extends State<LNInfoPage> {
  bool loading = true;
  LNInfo info = LNInfo.empty();
  LNBalances balances = LNBalances.empty();
  String depositAddr = "";

  void loadInfo() async {
    setState(() => loading = true);
    try {
      var newInfo = await Golib.lnGetInfo();
      var newBalances = await Golib.lnGetBalances();
      setState(() {
        info = newInfo;
        balances = newBalances;
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to load LN info: $exception");
    } finally {
      setState(() => loading = false);
    }
  }

  void getDepositAddr() async {
    try {
      var newAddr = await Golib.lnGetDepositAddr("");
      setState(() {
        depositAddr = newAddr;
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to fetch deposit address: $exception");
    }
  }

  @override
  void initState() {
    super.initState();
    loadInfo();
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var secondaryTextColor = theme.dividerColor;
    var darkTextColor = theme.indicatorColor;
    var dividerColor = theme.highlightColor;
    var backgroundColor = theme.backgroundColor;
    if (loading) {
      return Text("Loading...", style: TextStyle(color: textColor));
    }

    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    var onChainBalance = formatDCR(atomsToDCR(balances.wallet.totalBalance));
    var maxReceive = formatDCR(atomsToDCR(balances.channel.maxInboundAmount));
    var maxSend = formatDCR(atomsToDCR(balances.channel.maxOutboundAmount));

    return isScreenSmall
        ? Container(
            margin: const EdgeInsets.all(1),
            decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(3), color: backgroundColor),
            padding: const EdgeInsets.all(16),
            child: ListView(children: [
              Container(
                  decoration: BoxDecoration(
                    color: dividerColor,
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Container(
                            padding: const EdgeInsets.only(
                                left: 10, top: 10, bottom: 5),
                            child: Text("Balances",
                                textAlign: TextAlign.left,
                                style:
                                    TextStyle(color: textColor, fontSize: 18))),
                        Divider(
                          color: backgroundColor, //color of divider
                          height: 10, //height spacing of divider
                          thickness: 2, //thickness of divier line
                          indent: 0, //spacing at the start of divider
                          endIndent: 0, //spacing at the end of divider
                        ),
                        Container(
                            padding: const EdgeInsets.only(
                                left: 10, top: 5, bottom: 10),
                            child: Row(children: [
                              Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text("Max Receivable:",
                                        style: TextStyle(
                                            fontSize: 15,
                                            color: secondaryTextColor)),
                                    const SizedBox(height: 8),
                                    Text("Max Sendable:",
                                        style: TextStyle(
                                            fontSize: 15,
                                            color: secondaryTextColor)),
                                    const SizedBox(height: 8),
                                    Text("On-chain Balance:",
                                        style: TextStyle(
                                            fontSize: 15,
                                            color: secondaryTextColor))
                                  ]),
                              const SizedBox(width: 10),
                              Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text(maxReceive,
                                        style: TextStyle(
                                            fontWeight: FontWeight.w300,
                                            fontSize: 15,
                                            color: textColor)),
                                    const SizedBox(height: 8),
                                    Text(maxSend,
                                        style: TextStyle(
                                            fontWeight: FontWeight.w300,
                                            fontSize: 15,
                                            color: textColor)),
                                    const SizedBox(height: 8),
                                    Text(onChainBalance,
                                        style: TextStyle(
                                            fontWeight: FontWeight.w300,
                                            fontSize: 15,
                                            color: textColor))
                                  ]),
                            ])),
                      ])),
              const SizedBox(height: 20),
              Container(
                  decoration: BoxDecoration(
                    color: dividerColor,
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Container(
                          padding: const EdgeInsets.only(
                              left: 10, top: 10, bottom: 5),
                          child: Text("Wallet",
                              textAlign: TextAlign.left,
                              style:
                                  TextStyle(color: textColor, fontSize: 18))),
                      Divider(
                        color: backgroundColor, //color of divider
                        height: 10, //height spacing of divider
                        thickness: 2,
                      ),
                      Container(
                          padding: const EdgeInsets.only(
                              left: 10, top: 5, bottom: 10),
                          child: Row(children: [
                            Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text("Chain Height",
                                      style: TextStyle(
                                          fontSize: 15,
                                          color: secondaryTextColor)),
                                  const SizedBox(height: 8),
                                  Text("Synced to Chain",
                                      style: TextStyle(
                                          fontSize: 15,
                                          color: secondaryTextColor)),
                                  const SizedBox(height: 8),
                                  Text("Synced to Graph:",
                                      style: TextStyle(
                                          fontSize: 15,
                                          color: secondaryTextColor)),
                                  const SizedBox(height: 8),
                                  Text("Pending Channels:",
                                      style: TextStyle(
                                          fontSize: 15,
                                          color: secondaryTextColor)),
                                  const SizedBox(height: 8),
                                  Text("Inactive Channels:",
                                      style: TextStyle(
                                          fontSize: 15,
                                          color: secondaryTextColor)),
                                  const SizedBox(height: 8),
                                  Text("Active Channels:",
                                      style: TextStyle(
                                          fontSize: 15,
                                          color: secondaryTextColor)),
                                ]),
                            const SizedBox(width: 10),
                            Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(info.blockHeight.toString(),
                                      style: TextStyle(
                                          fontWeight: FontWeight.w300,
                                          fontSize: 15,
                                          color: textColor)),
                                  const SizedBox(height: 8),
                                  Text(info.syncedToChain.toString(),
                                      style: TextStyle(
                                          fontWeight: FontWeight.w300,
                                          fontSize: 15,
                                          color: textColor)),
                                  const SizedBox(height: 8),
                                  Text(info.syncedToGraph.toString(),
                                      style: TextStyle(
                                          fontWeight: FontWeight.w300,
                                          fontSize: 15,
                                          color: textColor)),
                                  const SizedBox(height: 8),
                                  Text(info.numPendingChannels.toString(),
                                      style: TextStyle(
                                          fontWeight: FontWeight.w300,
                                          fontSize: 15,
                                          color: textColor)),
                                  const SizedBox(height: 8),
                                  Text(info.numInactiveChannels.toString(),
                                      style: TextStyle(
                                          fontWeight: FontWeight.w300,
                                          fontSize: 15,
                                          color: textColor)),
                                  const SizedBox(height: 8),
                                  Text(info.numActiveChannels.toString(),
                                      style: TextStyle(
                                          fontWeight: FontWeight.w300,
                                          fontSize: 15,
                                          color: textColor)),
                                ]),
                          ])),
                      Divider(
                        color: backgroundColor, //color of divider
                        height: 10, //height spacing of divider
                        thickness: 2,
                      ),
                      Container(
                          padding: const EdgeInsets.only(
                              left: 10, top: 5, bottom: 10),
                          child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text("Version:",
                                    style: TextStyle(
                                        fontSize: 15,
                                        color: secondaryTextColor)),
                                const SizedBox(height: 5),
                                Copyable(
                                    info.version.trim(),
                                    TextStyle(
                                        letterSpacing: 1,
                                        fontWeight: FontWeight.w300,
                                        fontSize: 15,
                                        color: textColor))
                              ])),
                      Divider(
                        color: backgroundColor, //color of divider
                        height: 10, //height spacing of divider
                        thickness: 2,
                      ),
                      Container(
                          padding: const EdgeInsets.only(
                              left: 10, top: 5, bottom: 10, right: 20),
                          child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text("Node ID:",
                                    style: TextStyle(
                                        fontSize: 15,
                                        color: secondaryTextColor)),
                                const SizedBox(height: 5),
                                Copyable(
                                    info.identityPubkey.trim(),
                                    TextStyle(
                                        letterSpacing: 1,
                                        fontWeight: FontWeight.w300,
                                        color: textColor,
                                        fontSize: 15)),
                              ])),
                      Divider(
                        color: backgroundColor, //color of divider
                        height: 10, //height spacing of divider
                        thickness: 2,
                      ),
                      Container(
                          padding: const EdgeInsets.only(
                              left: 10, top: 5, bottom: 10, right: 20),
                          child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text("Chain Hash:",
                                    style: TextStyle(
                                        fontSize: 15,
                                        color: secondaryTextColor)),
                                const SizedBox(height: 5),
                                Container(
                                    decoration: BoxDecoration(
                                      color: backgroundColor,
                                      borderRadius: BorderRadius.circular(10),
                                    ),
                                    padding: const EdgeInsets.all(8),
                                    child: Copyable(
                                        info.blockHash.toString(),
                                        TextStyle(
                                            letterSpacing: 1,
                                            color: textColor,
                                            fontSize: 15)))
                              ])),
                      Container(
                          padding: const EdgeInsets.only(
                              left: 10, top: 5, bottom: 10, right: 10),
                          child: Column(children: [
                            MobileScreenButton(
                              minSize: MediaQuery.of(context).size.width,
                              onPressed: getDepositAddr,
                              text: "New Deposit Address",
                            ),
                            const SizedBox(height: 10),
                            Copyable(depositAddr,
                                TextStyle(color: textColor, fontSize: 15)),
                          ])),
                    ],
                  ))
            ]))
        : Container(
            margin: const EdgeInsets.all(1),
            decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(3), color: backgroundColor),
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(children: [
                  Text("Balances",
                      textAlign: TextAlign.left,
                      style: TextStyle(color: darkTextColor, fontSize: 15)),
                  Expanded(
                      child: Divider(
                    color: dividerColor, //color of divider
                    height: 10, //height spacing of divider
                    thickness: 1, //thickness of divier line
                    indent: 8, //spacing at the start of divider
                    endIndent: 5, //spacing at the end of divider
                  )),
                ]),
                const SizedBox(height: 21),
                Row(children: [
                  Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text("Max Receivable:",
                            style: TextStyle(
                                fontSize: 11, color: secondaryTextColor)),
                        const SizedBox(height: 8),
                        Text("Max Sendable:",
                            style: TextStyle(
                                fontSize: 11, color: secondaryTextColor)),
                        const SizedBox(height: 8),
                        Text("On-chain Balance:",
                            style: TextStyle(
                                fontSize: 11, color: secondaryTextColor))
                      ]),
                  const SizedBox(width: 10),
                  Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(maxReceive,
                            style: TextStyle(fontSize: 11, color: textColor)),
                        const SizedBox(height: 8),
                        Text(maxSend,
                            style: TextStyle(fontSize: 11, color: textColor)),
                        const SizedBox(height: 8),
                        Text(onChainBalance,
                            style: TextStyle(fontSize: 11, color: textColor))
                      ]),
                ]),
                const SizedBox(height: 34),
                Row(children: [
                  Text("Wallet",
                      textAlign: TextAlign.left,
                      style: TextStyle(color: darkTextColor, fontSize: 15)),
                  Expanded(
                      child: Divider(
                    color: dividerColor, //color of divider
                    height: 10, //height spacing of divider
                    thickness: 1, //thickness of divier line
                    indent: 8, //spacing at the start of divider
                    endIndent: 5, //spacing at the end of divider
                  )),
                ]),
                const SizedBox(height: 21),
                Row(children: [
                  Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text("Chain Height",
                            style: TextStyle(
                                fontSize: 11, color: secondaryTextColor)),
                        const SizedBox(height: 8),
                        Text("Synced to Chain",
                            style: TextStyle(
                                fontSize: 11, color: secondaryTextColor)),
                        const SizedBox(height: 8),
                        Text("Synced to Graph:",
                            style: TextStyle(
                                fontSize: 11, color: secondaryTextColor)),
                        const SizedBox(height: 8),
                        Text("Pending Channels:",
                            style: TextStyle(
                                fontSize: 11, color: secondaryTextColor)),
                        const SizedBox(height: 8),
                        Text("Inactive Channels:",
                            style: TextStyle(
                                fontSize: 11, color: secondaryTextColor)),
                        const SizedBox(height: 8),
                        Text("Active Channels:",
                            style: TextStyle(
                                fontSize: 11, color: secondaryTextColor)),
                        const SizedBox(height: 8),
                        Text("Version:",
                            style: TextStyle(
                                fontSize: 11, color: secondaryTextColor)),
                        const SizedBox(height: 8),
                        Text("Node ID:",
                            style: TextStyle(
                                fontSize: 11, color: secondaryTextColor)),
                        const SizedBox(height: 8),
                        Text("Chain Hash:",
                            style: TextStyle(
                                fontSize: 11, color: secondaryTextColor))
                      ]),
                  const SizedBox(width: 10),
                  Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(info.blockHeight.toString(),
                            style: TextStyle(fontSize: 11, color: textColor)),
                        const SizedBox(height: 8),
                        Text(info.syncedToChain.toString(),
                            style: TextStyle(fontSize: 11, color: textColor)),
                        const SizedBox(height: 8),
                        Text(info.syncedToGraph.toString(),
                            style: TextStyle(fontSize: 11, color: textColor)),
                        const SizedBox(height: 8),
                        Text(info.numPendingChannels.toString(),
                            style: TextStyle(fontSize: 11, color: textColor)),
                        const SizedBox(height: 8),
                        Text(info.numInactiveChannels.toString(),
                            style: TextStyle(fontSize: 11, color: textColor)),
                        const SizedBox(height: 8),
                        Text(info.numActiveChannels.toString(),
                            style: TextStyle(fontSize: 11, color: textColor)),
                        const SizedBox(height: 8),
                        Copyable(info.version.trim(),
                            TextStyle(fontSize: 11, color: textColor)),
                        const SizedBox(height: 8),
                        Copyable(info.identityPubkey.trim(),
                            TextStyle(color: textColor, fontSize: 11)),
                        const SizedBox(height: 8),
                        Copyable(info.blockHash.toString(),
                            TextStyle(color: textColor, fontSize: 11))
                      ]),
                ]),
                const SizedBox(height: 21),
                Row(children: [
                  ElevatedButton(
                      onPressed: getDepositAddr,
                      child: Text("New Deposit Address",
                          style: TextStyle(fontSize: 11, color: textColor))),
                  const SizedBox(width: 20),
                  Copyable(
                      depositAddr, TextStyle(color: textColor, fontSize: 15)),
                ]),
                const SizedBox(width: 21),
                depositAddr.isNotEmpty
                    ? Container(
                        margin: const EdgeInsets.all(10),
                        color: Colors.white,
                        child: QrImageView(
                          data: depositAddr,
                          version: QrVersions.auto,
                          size: 200.0,
                        ))
                    : Empty(),
              ],
            ));
  }
}
