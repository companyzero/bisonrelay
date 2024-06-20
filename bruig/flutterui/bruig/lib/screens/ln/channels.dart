import 'dart:async';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/confirmation_dialog.dart';
import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/info_grid.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/screens/needs_out_channel.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';
import 'package:tuple/tuple.dart';

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
    var themeNtf = Provider.of<ThemeNotifier>(context);
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var secondaryTextColor = theme.dividerColor;
    var capacity = atomsToDCR(chan.capacity).toStringAsFixed(8);
    var localBalance = atomsToDCR(chan.localBalance).toStringAsFixed(8);
    var remoteBalance = atomsToDCR(chan.remoteBalance).toStringAsFixed(8);
    var state = chan.active ? "Active" : "Inactive";
    var labelTs = TextStyle(
        fontSize: themeNtf.getSmallFont(context), color: secondaryTextColor);
    var valTs =
        TextStyle(fontSize: themeNtf.getSmallFont(context), color: textColor);
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SizedBox(height: 13),
                SimpleInfoGrid(
                    colLabelSize: 110,
                    colValueFlex: 8,
                    separatorWidth: 10,
                    useListBuilder: false,
                    [
                      Tuple2(Text("State:", style: labelTs),
                          Text(state, style: valTs)),
                      Tuple2(
                          Text("Remote Node:", style: labelTs),
                          Copyable(
                              textOverflow: TextOverflow.ellipsis,
                              chan.remotePubkey,
                              textStyle: valTs)),
                      Tuple2(
                          Text("Channel Point:", style: labelTs),
                          Copyable(
                              textOverflow: TextOverflow.ellipsis,
                              chan.channelPoint,
                              textStyle: valTs)),
                      Tuple2(Text("Channel ID:", style: labelTs),
                          Copyable(chan.shortChanID, textStyle: valTs)),
                      Tuple2(Text("Channel Capacity:", style: labelTs),
                          Text("$capacity DCR", style: valTs)),
                      ...(isScreenSmall
                          ? [
                              Tuple2(Text("Local Balance:", style: labelTs),
                                  Text("$localBalance DCR", style: valTs)),
                              Tuple2(Text("Remote Balance:", style: labelTs),
                                  Text("$remoteBalance DCR", style: valTs)),
                            ]
                          : [
                              Tuple2(
                                  Text("Relative Balances:", style: labelTs),
                                  Wrap(children: [
                                    Text("$localBalance DCR", style: valTs),
                                    const SizedBox(width: 8),
                                    Text("Local <--> Remote", style: labelTs),
                                    const SizedBox(width: 8),
                                    Text("$remoteBalance DCR", style: valTs),
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
            ));
  }
}

Widget pendingChanSummary(BuildContext context, LNPendingChannel chan,
    String state, Color textColor, Color secondaryTextColor,
    {String? closingTx}) {
  var themeNtf = Provider.of<ThemeNotifier>(context);
  var capacity = atomsToDCR(chan.capacity).toStringAsFixed(8);
  var localBalance = atomsToDCR(chan.localBalance).toStringAsFixed(8);
  var remoteBalance = atomsToDCR(chan.remoteBalance).toStringAsFixed(8);
  var labelTs = TextStyle(
      fontSize: themeNtf.getSmallFont(context), color: secondaryTextColor);
  var valTs =
      TextStyle(fontSize: themeNtf.getSmallFont(context), color: textColor);
  bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
  return SimpleInfoGrid(
      colLabelSize: 110,
      colValueFlex: 8,
      separatorWidth: 10,
      useListBuilder: false,
      [
        Tuple2(Text("State:", style: labelTs), Text(state, style: valTs)),
        Tuple2(
            Text("Remote Node:", style: labelTs),
            Copyable(
                textOverflow: TextOverflow.ellipsis,
                chan.remoteNodePub,
                textStyle: valTs)),
        Tuple2(
            Text("Channel Point:", style: labelTs),
            Copyable(
                textOverflow: TextOverflow.ellipsis,
                chan.channelPoint,
                textStyle: valTs)),
        Tuple2(Text("Channel ID:", style: labelTs),
            Copyable(chan.shortChanID, textStyle: valTs)),
        Tuple2(Text("Channel Capacity:", style: labelTs),
            Text("$capacity DCR", style: valTs)),
        ...(isScreenSmall
            ? [
                Tuple2(Text("Local Balance:", style: labelTs),
                    Text("$localBalance DCR", style: valTs)),
                Tuple2(Text("Remote Balance:", style: labelTs),
                    Text("$remoteBalance DCR", style: valTs)),
              ]
            : [
                Tuple2(
                    Text("Relative Balances:", style: labelTs),
                    Wrap(children: [
                      Text("$localBalance DCR", style: valTs),
                      const SizedBox(width: 8),
                      Text("Local <--> Remote", style: labelTs),
                      const SizedBox(width: 8),
                      Text("$remoteBalance DCR", style: valTs),
                    ])),
              ]),
        ...(closingTx != null
            ? [
                Tuple2(Text("Closing Tx:", style: labelTs),
                    Copyable(closingTx, textStyle: valTs))
              ]
            : []),
      ]);
}

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
    return pendingChanSummary(
        context, chan, state, textColor, secondaryTextColor);
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
    return pendingChanSummary(
        context, chan, state, textColor, secondaryTextColor);
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
    return pendingChanSummary(
        context, chan, state, textColor, secondaryTextColor,
        closingTx: pending.closingTxid);
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
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var darkTextColor = theme.indicatorColor;
    var dividerColor = theme.highlightColor;
    var backgroundColor = theme.backgroundColor;
    if (loading && channels.isEmpty) {
      return Text("Loading...", style: TextStyle(color: textColor));
    }

    return Container(
        margin: const EdgeInsets.all(1),
        decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(3), color: backgroundColor),
        padding: const EdgeInsets.all(16),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Row(children: [
            Consumer<ThemeNotifier>(
                builder: (context, theme, _) => Text("Channels",
                    textAlign: TextAlign.left,
                    style: TextStyle(
                        color: darkTextColor,
                        fontSize: theme.getMediumFont(context)))),
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
              return Text("Unknown channel type $chan",
                  style: TextStyle(color: textColor));
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
