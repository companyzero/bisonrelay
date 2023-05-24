import 'package:bruig/components/dcr_input.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import 'package:bruig/components/copyable.dart';

class LNNetworkPage extends StatefulWidget {
  const LNNetworkPage({Key? key}) : super(key: key);

  @override
  State<LNNetworkPage> createState() => _LNNetworkPageState();
}

class _PeerW extends StatelessWidget {
  final LNPeer peer;
  const _PeerW(this.peer, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var dividerColor = theme.highlightColor;
    var secondaryTextColor = theme.dividerColor;
    return Column(children: [
      const SizedBox(height: 8),
      Row(children: [
        SizedBox(
            width: 100,
            child: Text("Peer ID:",
                style: TextStyle(fontSize: 11, color: secondaryTextColor))),
        Text(peer.pubkey, style: TextStyle(fontSize: 11, color: textColor)),
      ]),
      const SizedBox(height: 8),
      Row(children: [
        SizedBox(
            width: 100,
            child: Text("Address:",
                style: TextStyle(fontSize: 11, color: secondaryTextColor))),
        Text(peer.address, style: TextStyle(fontSize: 11, color: textColor)),
      ]),
      const SizedBox(height: 10),
      Row(children: [
        Expanded(
            child: Divider(
          color: dividerColor, //color of divider
          height: 10, //height spacing of divider
          thickness: 1, //thickness of divier line
          endIndent: 5, //spacing at the end of divider
        ))
      ]),
    ]);
  }
}

class _QueriedRouteW extends StatelessWidget {
  final LNQueryRouteResponse res;
  final String node;
  final ScrollController scrollCtrl = ScrollController();
  _QueriedRouteW(this.node, this.res, {Key? key}) : super(key: key);

  Widget buildHop(
      int route, int hop, Color textColor, Color secondaryTextColor) {
    var h = res.routes[route].hops[hop];
    var chanID = shortChanIDToStr(h.chanId);
    return Row(children: [
      Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Text("Hop:", style: TextStyle(fontSize: 11, color: secondaryTextColor)),
        const SizedBox(height: 8),
        Text("Node:",
            style: TextStyle(fontSize: 11, color: secondaryTextColor)),
        const SizedBox(height: 8),
        Text("Channel ID:",
            style: TextStyle(fontSize: 11, color: secondaryTextColor)),
        const SizedBox(height: 8),
      ]),
      const SizedBox(width: 8),
      Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Text("$hop", style: TextStyle(fontSize: 11, color: textColor)),
        const SizedBox(height: 8),
        Copyable(h.pubkey, TextStyle(fontSize: 11, color: textColor)),
        const SizedBox(height: 8),
        Text(chanID, style: TextStyle(fontSize: 11, color: textColor)),
        const SizedBox(height: 8)
      ])
    ]);
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var secondaryTextColor = theme.dividerColor;
    var successProb = (res.successProb * 100).toStringAsFixed(2);
    return Column(children: [
      Text(node, style: TextStyle(fontSize: 13, color: textColor)),
      Text("Success Probability: $successProb%",
          style: TextStyle(fontSize: 13, color: textColor)),
      const SizedBox(height: 8),
      res.routes.isEmpty
          ? Text("No routes to node",
              style: TextStyle(fontSize: 13, color: textColor))
          : Expanded(
              child: ListView.separated(
              separatorBuilder: (context, index) => Divider(
                height: 5,
                thickness: 1,
                color: secondaryTextColor,
              ),
              controller: scrollCtrl,
              itemCount: res.routes[0].hops.length,
              itemBuilder: (context, index) =>
                  buildHop(0, index, textColor, secondaryTextColor),
            ))
    ]);
  }
}

class _NodeInfo extends StatelessWidget {
  final LNGetNodeInfoResponse nodeInfo;
  final ScrollController scrollCtrl = ScrollController();
  _NodeInfo(this.nodeInfo, {Key? key}) : super(key: key);

  Widget buildChannel(int channel, Color textColor, Color secondaryTextColor) {
    var chan = nodeInfo.channels[channel];
    var chanID = shortChanIDToStr(chan.channelID);
    var capacity = formatDCR(atomsToDCR(chan.capacity));
    return Row(children: [
      Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Text("Channel ID:",
            style: TextStyle(fontSize: 11, color: secondaryTextColor)),
        const SizedBox(height: 8),
        Text("Last Channel Update:",
            style: TextStyle(fontSize: 11, color: secondaryTextColor)),
        const SizedBox(height: 8),
        Text("Channel Point:",
            style: TextStyle(fontSize: 11, color: secondaryTextColor)),
        const SizedBox(height: 8),
        Text("Channel Capacity:",
            style: TextStyle(fontSize: 11, color: secondaryTextColor)),
        const SizedBox(height: 8),
        Text("Node 1:",
            style: TextStyle(fontSize: 11, color: secondaryTextColor)),
        const SizedBox(height: 8),
        Text("Node 2:",
            style: TextStyle(fontSize: 11, color: secondaryTextColor))
      ]),
      const SizedBox(width: 10),
      Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Text(chanID, style: TextStyle(fontSize: 11, color: textColor)),
        const SizedBox(height: 8),
        Text(
            DateTime.fromMillisecondsSinceEpoch(chan.lastUpdate * 1000)
                .toIso8601String(),
            style: TextStyle(fontSize: 11, color: textColor)),
        const SizedBox(height: 8),
        Copyable(chan.channelPoint, TextStyle(fontSize: 11, color: textColor)),
        const SizedBox(height: 8),
        Text(capacity, style: TextStyle(fontSize: 11, color: textColor)),
        const SizedBox(height: 8),
        Text("${chan.node1Pub} disabled: ${chan.node1Policy.disabled}",
            style: TextStyle(fontSize: 11, color: textColor)),
        const SizedBox(height: 8),
        Text("${chan.node2Pub} disabled: ${chan.node2Policy.disabled}",
            style: TextStyle(fontSize: 11, color: textColor)),
      ])
    ]);
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var secondaryTextColor = theme.dividerColor;
    return Column(children: [
      Copyable(nodeInfo.node.pubkey, TextStyle(fontSize: 13, color: textColor)),
      Copyable(nodeInfo.node.alias, TextStyle(fontSize: 13, color: textColor)),
      Text("Number of channels: ${nodeInfo.numChannels}",
          style: TextStyle(fontSize: 13, color: textColor)),
      Text("Total Capacity: ${formatDCR(atomsToDCR(nodeInfo.totalCapacity))}",
          style: TextStyle(fontSize: 13, color: textColor)),
      const SizedBox(height: 8),
      nodeInfo.channels.isEmpty
          ? Text("No channels for node", style: TextStyle(color: textColor))
          : Expanded(
              child: ListView.separated(
              separatorBuilder: (context, index) => Divider(
                height: 5,
                thickness: 1,
                color: secondaryTextColor,
              ),
              controller: scrollCtrl,
              itemCount: nodeInfo.channels.length,
              itemBuilder: (context, index) =>
                  buildChannel(index, textColor, secondaryTextColor),
            ))
    ]);
  }
}

class _LNNetworkPageState extends State<LNNetworkPage> {
  bool loading = true;
  bool connecting = false;
  bool querying = false;
  bool closed = true;
  String serverNode = "";
  List<LNPeer> peers = [];
  String lastQueriedNode = "";
  LNQueryRouteResponse queryRouteRes = LNQueryRouteResponse.empty();
  LNGetNodeInfoResponse nodeInfo = LNGetNodeInfoResponse.empty();
  final TextEditingController connectCtrl = TextEditingController();
  final TextEditingController queryRouteCtrl = TextEditingController();
  final AmountEditingController queryAmountCtrl = AmountEditingController();

  void closeNodeInfo() async {
    setState(() {
      queryRouteRes = LNQueryRouteResponse.empty();
      nodeInfo = LNGetNodeInfoResponse.empty();
    });
  }

  void loadInfo() async {
    setState(() {
      loading = true;
    });
    try {
      var newPeers = await Golib.lnListPeers();
      var newServerNode = await Golib.lnGetServerNode();
      setState(() {
        peers = newPeers;
        serverNode = newServerNode;
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to load network info: $exception");
    } finally {
      setState(() {
        loading = false;
      });
    }
  }

  void connectToPeer() async {
    setState(() {
      connecting = true;
    });
    try {
      await Golib.lnConnectToPeer(connectCtrl.text);
      var newPeers = await Golib.lnListPeers();
      setState(() {
        connectCtrl.clear();
        peers = newPeers;
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to connect to peer: $exception");
    } finally {
      setState(() {
        connecting = false;
      });
    }
  }

  Future<void> queryRouteToNode(String node, double amount) async {
    setState(() {
      querying = true;
    });
    try {
      if (amount < 0.00000001) {
        amount = 0.00000001;
      }

      var newNodeInfo = await Golib.lnGetNodeInfo(node);
      var newQueryRouteRes = await Golib.lnQueryRoute(amount, node);
      setState(() {
        nodeInfo = newNodeInfo;
        lastQueriedNode = node;
        queryRouteRes = newQueryRouteRes;
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to query route: $exception");
    } finally {
      setState(() {
        querying = false;
        closed = false;
      });
    }
  }

  void queryRoute() async {
    await queryRouteToNode(queryRouteCtrl.text, queryAmountCtrl.amount);
  }

  void queryRouteToServer() async {
    await queryRouteToNode(serverNode, 0);
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
    var darkTextColor = theme.indicatorColor;
    var dividerColor = theme.highlightColor;
    var backgroundColor = theme.backgroundColor;
    var secondaryTextColor = theme.dividerColor;
    var inputFill = theme.hoverColor;
    if (loading) {
      return Text("Loading...", style: TextStyle(color: textColor));
    }
    if (nodeInfo.node.pubkey != "" &&
        lastQueriedNode != "" &&
        queryRouteRes.routes.isNotEmpty) {
      return Container(
          margin: const EdgeInsets.all(1),
          decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(3), color: backgroundColor),
          padding: const EdgeInsets.all(16),
          child: Stack(alignment: Alignment.topRight, children: [
            Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Row(children: [
                Text("Node Info",
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
              Expanded(child: _NodeInfo(nodeInfo)),
              const SizedBox(height: 21),
              Row(children: [
                Text("Queried Route Results",
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
              Expanded(child: _QueriedRouteW(lastQueriedNode, queryRouteRes))
            ]),
            Positioned(
                top: 5,
                right: 5,
                child: Material(
                    color: dividerColor.withOpacity(0),
                    child: IconButton(
                        tooltip: "Close",
                        hoverColor: dividerColor,
                        splashRadius: 15,
                        iconSize: 15,
                        onPressed: () => closeNodeInfo(),
                        icon:
                            Icon(color: darkTextColor, Icons.close_outlined))))
          ]));
    }
    return Container(
      margin: const EdgeInsets.all(1),
      decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(3), color: backgroundColor),
      padding: const EdgeInsets.all(16),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Row(children: [
          Text("Server Node",
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
          SizedBox(
              width: 100,
              child: Text("Node ID:",
                  style: TextStyle(fontSize: 11, color: secondaryTextColor))),
          Text(serverNode, style: TextStyle(fontSize: 11, color: textColor)),
        ]),
        const SizedBox(height: 21),
        ElevatedButton(
            onPressed: !querying ? queryRouteToServer : null,
            child: Text("Query Route",
                style: TextStyle(fontSize: 11, color: textColor))),
        const SizedBox(height: 34),
        Row(children: [
          Text("Peers",
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
            flex: 10,
            child: ListView.builder(
              itemCount: peers.length,
              itemBuilder: (context, index) => _PeerW(peers[index]),
            )),
        const SizedBox(height: 21),
        Row(children: [
          SizedBox(
              width: 100,
              child: Text("Connect to Peer:",
                  style: TextStyle(fontSize: 11, color: secondaryTextColor))),
          SizedBox(
              width: 500,
              child: TextField(
                  style: TextStyle(fontSize: 11, color: secondaryTextColor),
                  controller: connectCtrl,
                  decoration: InputDecoration(
                      hintText: "ID",
                      hintStyle:
                          TextStyle(fontSize: 11, color: secondaryTextColor),
                      filled: true,
                      fillColor: inputFill)))
        ]),
        ElevatedButton(
            onPressed: !connecting ? connectToPeer : null,
            child: Text("Connect",
                style: TextStyle(fontSize: 11, color: textColor))),
        const SizedBox(height: 34),
        Row(children: [
          Text("Query Route",
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
        Row(children: [
          SizedBox(
              width: 100,
              child: Text("Node ID:",
                  style: TextStyle(fontSize: 11, color: secondaryTextColor))),
          SizedBox(
              width: 500,
              child: TextField(
                  style: TextStyle(fontSize: 11, color: secondaryTextColor),
                  controller: queryRouteCtrl,
                  decoration: InputDecoration(
                      hintText: "Node ID",
                      hintStyle:
                          TextStyle(fontSize: 11, color: secondaryTextColor),
                      filled: true,
                      fillColor: inputFill)))
        ]),
        const SizedBox(height: 8),
        Row(children: [
          SizedBox(
              width: 100,
              child: Text("Amount:",
                  style: TextStyle(fontSize: 11, color: secondaryTextColor))),
          SizedBox(width: 100, child: dcrInput(controller: queryAmountCtrl))
        ]),
        ElevatedButton(
            onPressed: !querying ? queryRoute : null,
            child: Text("Search",
                style: TextStyle(fontSize: 11, color: textColor))),
      ]),
    );
  }
}
