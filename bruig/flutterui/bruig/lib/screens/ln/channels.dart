import 'dart:async';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/confirmation_dialog.dart';
import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/info_grid.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/ln/components.dart';
import 'package:bruig/screens/needs_out_channel.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import 'package:tuple/tuple.dart';

typedef CloseChanCB = Future<void> Function(LNChannel chan);

class LNChannelsPage extends StatefulWidget {
  const LNChannelsPage({super.key});

  @override
  State<LNChannelsPage> createState() => _LNChannelsPageState();
}

class _ChanW extends StatelessWidget {
  final LNChannel chan;
  final CloseChanCB closeChan;
  const _ChanW(this.chan, this.closeChan);

  @override
  Widget build(BuildContext context) {
    var capacity = atomsToDCR(chan.capacity).toStringAsFixed(8);
    var localBalance = atomsToDCR(chan.localBalance).toStringAsFixed(8);
    var remoteBalance = atomsToDCR(chan.remoteBalance).toStringAsFixed(8);
    var state = chan.active ? "Active" : "Inactive";

    bool isScreenSmall = checkIsScreenSmall(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SizedBox(height: 13),
        SimpleInfoGrid(
            colLabelSize: 110,
            colValueFlex: 8,
            separatorWidth: 10,
            useListBuilder: false,
            [
              Tuple2(const Txt.S("State:"), Txt.S(state)),
              Tuple2(
                  const Txt.S("Remote Node:"),
                  Copyable.txt(Txt.S(chan.remotePubkey,
                      overflow: TextOverflow.ellipsis))),
              Tuple2(
                  const Txt.S("Channel Point:"),
                  Copyable.txt(Txt.S(
                    chan.channelPoint,
                    overflow: TextOverflow.ellipsis,
                  ))),
              Tuple2(const Txt.S("Channel ID:"),
                  Copyable.txt(Txt.S(chan.shortChanID))),
              Tuple2(const Txt.S("Channel Capacity:"), Txt.S("$capacity DCR")),
              ...(isScreenSmall
                  ? [
                      Tuple2(const Txt.S("Local Balance:"),
                          Txt.S("$localBalance DCR")),
                      Tuple2(const Txt.S("Remote Balance:"),
                          Txt.S("$remoteBalance DCR")),
                    ]
                  : [
                      Tuple2(
                          const Txt.S("Relative Balances:"),
                          Wrap(children: [
                            Txt.S("$localBalance DCR"),
                            const SizedBox(width: 8),
                            const Txt.S("Local <--> Remote"),
                            const SizedBox(width: 8),
                            Txt.S("$remoteBalance DCR"),
                          ])),
                    ]),
            ]),
        const SizedBox(height: 13),
        CancelButton(
          label: "Close Channel",
          onPressed: () => closeChan(chan),
        ),
        const SizedBox(height: 13),
      ],
    );
  }
}

Widget pendingChanSummary(
    BuildContext context, LNPendingChannel chan, String state,
    {String? closingTx}) {
  var capacity = atomsToDCR(chan.capacity).toStringAsFixed(8);
  var localBalance = atomsToDCR(chan.localBalance).toStringAsFixed(8);
  var remoteBalance = atomsToDCR(chan.remoteBalance).toStringAsFixed(8);

  bool isScreenSmall = checkIsScreenSmall(context);
  return SimpleInfoGrid(
      colLabelSize: 110,
      colValueFlex: 8,
      separatorWidth: 10,
      useListBuilder: false,
      [
        Tuple2(const Txt.S("State:"), Txt.S(state)),
        Tuple2(
            const Txt.S("Remote Node:"),
            Copyable.txt(
                Txt.S(chan.remoteNodePub, overflow: TextOverflow.ellipsis))),
        Tuple2(
            const Txt.S("Channel Point:"),
            Copyable.txt(
                Txt.S(chan.channelPoint, overflow: TextOverflow.ellipsis))),
        Tuple2(const Txt.S("Channel ID:"), Copyable(chan.shortChanID)),
        Tuple2(const Txt.S("Channel Capacity:"), Txt.S("$capacity DCR")),
        ...(isScreenSmall
            ? [
                Tuple2(
                    const Txt.S("Local Balance:"), Txt.S("$localBalance DCR")),
                Tuple2(const Txt.S("Remote Balance:"),
                    Txt.S("$remoteBalance DCR")),
              ]
            : [
                Tuple2(
                    const Txt.S("Relative Balances:"),
                    Wrap(children: [
                      Txt.S("$localBalance DCR"),
                      const SizedBox(width: 8),
                      const Txt.S("Local <--> Remote"),
                      const SizedBox(width: 8),
                      Txt.S("$remoteBalance DCR"),
                    ])),
              ]),
        ...(closingTx != null
            ? [Tuple2(const Txt.S("Closing Tx:"), Copyable(closingTx))]
            : []),
      ]);
}

class _PendingOpenChanW extends StatelessWidget {
  final LNPendingOpenChannel pending;
  LNPendingChannel get chan => pending.channel;
  const _PendingOpenChanW(this.pending);

  @override
  Widget build(BuildContext context) {
    var state = "Pending Open";
    return pendingChanSummary(context, chan, state);
  }
}

class _WaitingCloseChanW extends StatelessWidget {
  final LNWaitingCloseChannel pending;
  LNPendingChannel get chan => pending.channel;
  const _WaitingCloseChanW(this.pending);

  @override
  Widget build(BuildContext context) {
    var state = "Waiting Close";
    return pendingChanSummary(context, chan, state);
  }
}

class _PendingForceCloseChanW extends StatelessWidget {
  final LNPendingForceClosingChannel pending;
  LNPendingChannel get chan => pending.channel;
  const _PendingForceCloseChanW(this.pending);

  @override
  Widget build(BuildContext context) {
    var state = "Pending Force Close";
    return pendingChanSummary(context, chan, state,
        closingTx: pending.closingTxid);
  }
}

class _LNChannelsPageState extends State<LNChannelsPage> {
  bool loading = true;
  List<dynamic> channels = List.empty();
  ScrollController channelsCtrl = ScrollController();
  Timer? loadInfoTimer;

  void loadInfo() async {
    var snackbar = SnackBarModel.of(context);
    setState(() => loading = true);
    try {
      var newChans = await Golib.lnListChannels();
      var newPending = await Golib.lnListPendingChannels();
      var newChannels = [
        ...newChans,
        ...newPending.pendingOpen,
        ...newPending.waitingClose,
        ...newPending.pendingForceClose
      ];

      // Check if any channel changed that needs to be updated.
      var needsUpdate = newChannels.length != channels.length;
      for (var i = 0; !needsUpdate && i < newChannels.length; i++) {
        var c1 = channels[i];
        var c2 = newChannels[i];
        if (c1 is LNChannel && c2 is LNChannel) {
          needsUpdate = c1.active != c2.active ||
              c1.chanID != c2.chanID ||
              c1.localBalance != c2.localBalance;
        } else if (c1 is LNPendingOpenChannel && c2 is LNPendingOpenChannel) {
          needsUpdate = c1.channel.channelPoint != c2.channel.channelPoint;
        } else if (c1 is LNWaitingCloseChannel && c2 is LNWaitingCloseChannel) {
          needsUpdate = c1.channel.channelPoint != c2.channel.channelPoint;
        } else if (c1 is LNPendingForceClosingChannel &&
            c2 is LNPendingForceClosingChannel) {
          needsUpdate = c1.channel.channelPoint != c2.channel.channelPoint ||
              c1.blocksTilMaturity != c2.blocksTilMaturity;
        } else {
          needsUpdate = true;
        }
      }

      if (!needsUpdate) {
        return;
      }

      setState(() {
        channels = newChannels;
      });
    } catch (exception) {
      snackbar.error("Unable to load LN channels: $exception");
    } finally {
      setState(() => loading = false);
    }
  }

  Future<void> closeChan(LNChannel chan) async {
    var snackbar = SnackBarModel.of(context);
    setState(() => loading = true);
    try {
      await Golib.lnCloseChannel(chan.channelPoint, !chan.active);
    } catch (exception) {
      snackbar.error("Unable to close channel: $exception");
      return;
    } finally {
      setState(() => loading = false);
    }

    loadInfo();
  }

  Future<void> confirmClose(LNChannel chan) async {
    confirmationDialog(
      context,
      () => closeChan(chan),
      "Close Channel?",
      "Really close the channel ${chan.shortChanID}?",
      "Close",
      "Cancel",
    );
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
    if (loading && channels.isEmpty) {
      return const Text("Loading...");
    }

    return Container(
        padding: const EdgeInsets.all(16),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          const LNInfoSectionHeader("Channels"),
          Expanded(
              child: ListView.separated(
            controller: channelsCtrl,
            separatorBuilder: (context, index) => const Divider(),
            itemCount: channels.length,
            itemBuilder: (context, index) {
              var chan = channels[index];
              if (chan is LNChannel) {
                return _ChanW(chan, confirmClose);
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
              return Text("Unknown channel type $chan");
            },
          )),
          const SizedBox(height: 10),
          Wrap(runSpacing: 10, spacing: 20, children: [
            ElevatedButton(
                onPressed: () => Navigator.of(context, rootNavigator: true)
                    .pushNamed(NeedsOutChannelScreen.routeName),
                child: const Text("Open Outbound Channel")),
            ElevatedButton(
                onPressed: () => Navigator.of(context, rootNavigator: true)
                    .pushNamed("/needsInChannel"),
                child: const Text("Request Inbound Channel"))
          ]),
        ]));
  }
}
