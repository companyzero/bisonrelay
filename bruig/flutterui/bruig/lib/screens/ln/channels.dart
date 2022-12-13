import 'dart:async';

import 'package:bruig/components/snackbars.dart';
import 'package:bruig/screens/needs_out_channel.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';

typedef CloseChanCB = Future<void> Function(LNChannel chan);

class LNChannelsPage extends StatefulWidget {
  const LNChannelsPage({Key? key}) : super(key: key);

  @override
  State<LNChannelsPage> createState() => _LNChannelsPageState();
}

class _ChanW extends StatelessWidget {
  final LNChannel chan;
  final CloseChanCB closeChan;
  const _ChanW(this.chan, this.closeChan, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var secondaryTextColor = theme.dividerColor;
    var capacity = atomsToDCR(chan.capacity).toStringAsFixed(8);
    var localBalance = atomsToDCR(chan.localBalance).toStringAsFixed(8);
    var remoteBalance = atomsToDCR(chan.remoteBalance).toStringAsFixed(8);
    var state = chan.active ? "Active" : "Inactive";
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SizedBox(height: 13),
        Row(children: [
          Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Text("State:",
                style: TextStyle(fontSize: 11, color: secondaryTextColor)),
            const SizedBox(height: 8),
            Text("Remote Node:",
                style: TextStyle(fontSize: 11, color: secondaryTextColor)),
            const SizedBox(height: 8),
            Text("Channel Point:",
                style: TextStyle(fontSize: 11, color: secondaryTextColor)),
            const SizedBox(height: 8),
            Text("Channel Capacity:",
                style: TextStyle(fontSize: 11, color: secondaryTextColor)),
            const SizedBox(height: 8),
            Text("Relative Balances:",
                style: TextStyle(fontSize: 11, color: secondaryTextColor))
          ]),
          const SizedBox(width: 10),
          Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Text(state, style: TextStyle(fontSize: 11, color: textColor)),
            const SizedBox(height: 8),
            Text(chan.remotePubkey,
                style: TextStyle(fontSize: 11, color: textColor)),
            const SizedBox(height: 8),
            Text(chan.channelPoint,
                style: TextStyle(fontSize: 11, color: textColor)),
            const SizedBox(height: 8),
            Text("$capacity DCR",
                style: TextStyle(fontSize: 11, color: textColor)),
            const SizedBox(height: 8),
            Row(children: [
              Text("$localBalance DCR",
                  style: TextStyle(fontSize: 11, color: textColor)),
              const SizedBox(width: 8),
              Text("Local <--> Remote",
                  style: TextStyle(fontSize: 11, color: secondaryTextColor)),
              const SizedBox(width: 8),
              Text("$remoteBalance DCR",
                  style: TextStyle(fontSize: 11, color: textColor)),
            ])
          ]),
        ]),
        const SizedBox(height: 13),
        ElevatedButton(
          onPressed: () => closeChan(chan),
          style: ElevatedButton.styleFrom(
              textStyle: TextStyle(color: textColor, fontSize: 11),
              backgroundColor: theme.errorColor),
          child: const Text("Close Channel"),
        ),
        const SizedBox(height: 13),
      ],
    );
  }
}

Widget pendingChanSummary(LNPendingChannel chan, String state, Color textColor,
    Color secondaryTextColor) {
  var capacity = atomsToDCR(chan.capacity).toStringAsFixed(8);
  var localBalance = atomsToDCR(chan.localBalance).toStringAsFixed(8);
  var remoteBalance = atomsToDCR(chan.remoteBalance).toStringAsFixed(8);
  return Row(children: [
    Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      Text("State:", style: TextStyle(fontSize: 11, color: secondaryTextColor)),
      const SizedBox(height: 8),
      Text("Remote Node:",
          style: TextStyle(fontSize: 11, color: secondaryTextColor)),
      const SizedBox(height: 8),
      Text("Channel Point:",
          style: TextStyle(fontSize: 11, color: secondaryTextColor)),
      const SizedBox(height: 8),
      Text("Channel Capacity:",
          style: TextStyle(fontSize: 11, color: secondaryTextColor)),
      const SizedBox(height: 8),
      Text("Relative Balances:",
          style: TextStyle(fontSize: 11, color: secondaryTextColor))
    ]),
    const SizedBox(width: 10),
    Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      Text(state, style: TextStyle(fontSize: 11, color: textColor)),
      const SizedBox(height: 8),
      Text(chan.remoteNodePub,
          style: TextStyle(fontSize: 11, color: textColor)),
      const SizedBox(height: 8),
      Text(chan.channelPoint, style: TextStyle(fontSize: 11, color: textColor)),
      const SizedBox(height: 8),
      Text("$capacity DCR", style: TextStyle(fontSize: 11, color: textColor)),
      const SizedBox(height: 8),
      Row(children: [
        Text("$localBalance DCR",
            style: TextStyle(fontSize: 11, color: textColor)),
        const SizedBox(width: 8),
        Text("Local <--> Remote",
            style: TextStyle(fontSize: 11, color: secondaryTextColor)),
        const SizedBox(width: 8),
        Text("$remoteBalance DCR",
            style: TextStyle(fontSize: 11, color: textColor)),
      ])
    ]),
  ]);
}
/*
  return [
    Text("Remote Node", style: TextStyle(color: textColor)),
    Text(chan.remoteNodePub, style: TextStyle(color: textColor)),
    Text("Channel Point", style: TextStyle(color: textColor)),
    Text(chan.channelPoint, style: TextStyle(color: textColor)),
    Text("Channel Capacity", style: TextStyle(color: textColor)),
    Text("$capacity DCR", style: TextStyle(color: textColor)),
    Text("Relative Balances", style: TextStyle(color: textColor)),
    Text("Local $localBalance DCR <--> $remoteBalance DCR Remote",
        style: TextStyle(color: textColor)),
  ];
}
*/

class _PendingOpenChanW extends StatelessWidget {
  final LNPendingOpenChannel pending;
  LNPendingChannel get chan => pending.channel;
  const _PendingOpenChanW(this.pending, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var state = "Pending Open";
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var secondaryTextColor = theme.dividerColor;
    return pendingChanSummary(chan, state, textColor, secondaryTextColor);
  }
}

class _WaitingCloseChanW extends StatelessWidget {
  final LNWaitingCloseChannel pending;
  LNPendingChannel get chan => pending.channel;
  const _WaitingCloseChanW(this.pending, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var state = "Waiting Close";
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var secondaryTextColor = theme.dividerColor;
    return pendingChanSummary(chan, state, textColor, secondaryTextColor);
  }
}

class _PendingForceCloseChanW extends StatelessWidget {
  final LNPendingForceClosingChannel pending;
  LNPendingChannel get chan => pending.channel;
  const _PendingForceCloseChanW(this.pending, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var state = "Pending Force Close";
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var secondaryTextColor = theme.dividerColor;
    return Column(
      children: [
        pendingChanSummary(chan, state, textColor, secondaryTextColor),
        Text("Closing TX", style: TextStyle(color: textColor)),
        SelectableText(pending.closingTxid, style: TextStyle(color: textColor)),
      ],
    );
  }
}

class _LNChannelsPageState extends State<LNChannelsPage> {
  bool loading = true;
  List<dynamic> channels = List.empty();
  ScrollController channelsCtrl = ScrollController();
  Timer? loadInfoTimer;

  void loadInfo() async {
    setState(() => loading = true);
    try {
      var newChans = await Golib.lnListChannels();
      var newPending = await Golib.lnListPendingChannels();
      setState(() {
        channels = [
          ...newChans,
          ...newPending.pendingOpen,
          ...newPending.waitingClose,
          ...newPending.pendingForceClose
        ];
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to load LN channels: $exception");
    } finally {
      setState(() => loading = false);
    }
  }

  Future<void> closeChan(LNChannel chan) async {
    setState(() => loading = true);
    try {
      await Golib.lnCloseChannel(chan.channelPoint, !chan.active);
    } catch (exception) {
      showErrorSnackbar(context, "Unable to close channel: $exception");
      return;
    } finally {
      setState(() => loading = false);
    }

    loadInfo();
  }

  @override
  void initState() {
    super.initState();
    loadInfo();
    loadInfoTimer =
        Timer.periodic(const Duration(seconds: 5), (t) => loadInfo());
  }

  @override
  void dispose() {
    loadInfoTimer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var darkTextColor = theme.indicatorColor;
    var dividerColor = theme.highlightColor;
    var backgroundColor = theme.backgroundColor;
    if (loading) {
      return Text("Loading...", style: TextStyle(color: textColor));
    }

    return Container(
        margin: const EdgeInsets.all(1),
        decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(3), color: backgroundColor),
        padding: const EdgeInsets.all(16),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Row(children: [
            Text("Channels",
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
          Expanded(
              child: ListView.separated(
            controller: channelsCtrl,
            separatorBuilder: (context, index) => Divider(
              height: 5,
              thickness: 1,
              color: dividerColor,
            ),
            itemCount: channels.length,
            itemBuilder: (context, index) {
              var chan = channels[index];
              if (chan is LNChannel) {
                return _ChanW(chan, closeChan);
              }
              if (chan is LNPendingOpenChannel) {
                return _PendingOpenChanW(chan);
              }
              if (chan is LNWaitingCloseChannel) {
                return _WaitingCloseChanW(chan);
              }
              if (chan is LNPendingForceClosingChannel) {
                return _PendingForceCloseChanW(chan);
              }
              return Text("Unknown channel type $chan",
                  style: TextStyle(color: textColor));
            },
          )),
          Row(children: [
            ElevatedButton(
                onPressed: () => Navigator.of(context, rootNavigator: true)
                    .pushNamed(NeedsOutChannelScreen.routeName),
                child: const Text("Open Outbound Channel")),
            const SizedBox(width: 20),
            ElevatedButton(
                onPressed: () => Navigator.of(context, rootNavigator: true)
                    .pushNamed("/needsInChannel"),
                child: const Text("Request Inbound Channel"))
          ]),
        ]));
  }
}
