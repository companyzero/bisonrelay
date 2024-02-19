import 'package:bruig/components/info_grid.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import 'package:tuple/tuple.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

class PayStatsScreenTitle extends StatelessWidget {
  const PayStatsScreenTitle({Key? key}) : super(key: key);
  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Text("Payment Stats",
            style: TextStyle(
                fontSize: theme.getLargeFont(context),
                color: Theme.of(context).focusColor)));
  }
}

class PayStatsScreen extends StatefulWidget {
  static String routeName = "/payStats";
  final ClientModel client;
  const PayStatsScreen(this.client, {Key? key}) : super(key: key);

  @override
  State<PayStatsScreen> createState() => _PayStatsScreenState();
}

class _PayStatsScreenState extends State<PayStatsScreen> {
  ClientModel get client => widget.client;
  List<Tuple3<String, String, UserPayStats>> stats = []; // UID,nick,stat
  int selectedIndex = -1;
  List<PayStatsSummary> userStats = [];
  ScrollController userStatsSentCtrl = ScrollController();
  ScrollController userStatsReceivedCtrl = ScrollController();
  int userStatsTotalReceived = 0;
  int userStatsTotalSent = 0;

  void listPayStats() async {
    try {
      var statsMap = await Golib.listPaymentStats();
      var newStats = statsMap.entries
          .map((e) => Tuple3<String, String, UserPayStats>(
              e.key, client.getNick(e.key), e.value))
          .toList();
      newStats.sort((a, b) {
        var ta = a.item3.totalSent + a.item3.totalReceived;
        var tb = b.item3.totalSent + b.item3.totalReceived;
        return tb - ta;
      });
      setState(() {
        stats = newStats;
        if (selectedIndex >= stats.length) {
          selectedIndex = -1;
        }
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to list payment stats: $exception");
    }
  }

  void select(int index) async {
    setState(() {
      selectedIndex = index;
    });
    try {
      var newUserStats = await Golib.summarizeUserPayStats(stats[index].item1);
      setState(() {
        userStats = newUserStats;
        userStatsTotalReceived = 0;
        userStatsTotalSent = 0;
        for (int i = 0; i < userStats.length; i++) {
          if (userStats[i].total > 0) {
            userStatsTotalReceived += userStats[i].total;
          } else {
            userStatsTotalSent += userStats[i].total;
          }
        }
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to fetch user pay stats: $exception");
    }
  }

  void delete(int index) async {
    try {
      //var newUserStats = await Golib.clearPayStats(stats[index].item1);
      listPayStats();
    } catch (exception) {
      showErrorSnackbar(context, "Unable to clear stats: $exception");
    }
  }

  @override
  void initState() {
    super.initState();
    listPayStats();
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var backgroundColor = theme.backgroundColor;
    var otherBackgroundColor = theme.indicatorColor;
    var otherTextColor = theme.highlightColor;
    var highlightColor = theme.highlightColor;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Container(
              margin: const EdgeInsets.all(1),
              decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(3),
                  color: backgroundColor),
              padding: const EdgeInsets.all(16),
              child: Column(children: [
                ListTile(
                  contentPadding: const EdgeInsets.all(0),
                  title: Container(
                      margin: const EdgeInsets.only(top: 0, bottom: 0),
                      padding: const EdgeInsets.only(
                          left: 8, top: 10, right: 8, bottom: 10),
                      color: backgroundColor,
                      child: Row(children: [
                        SizedBox(
                            width: 100,
                            child: Text("User",
                                style: TextStyle(
                                    color: textColor,
                                    fontSize: theme.getSmallFont(context)))),
                        SizedBox(
                          width: 105,
                          child: Text("Sent (atoms)",
                              style: TextStyle(
                                  color: textColor,
                                  fontSize: theme.getSmallFont(context))),
                        ),
                        SizedBox(
                            width: 110,
                            child: Text(" Received (atoms) ",
                                style: TextStyle(
                                    color: textColor,
                                    fontSize: theme.getSmallFont(context)))),
                      ])),
                ),
                Expanded(
                  flex: 5,
                  child: ListView.builder(
                      itemCount: stats.length,
                      padding: const EdgeInsets.all(0),
                      itemBuilder: (context, index) => ListTile(
                            contentPadding: const EdgeInsets.all(0),
                            title: Container(
                                margin:
                                    const EdgeInsets.only(top: 0, bottom: 0),
                                padding: const EdgeInsets.only(
                                    left: 8, top: 0, right: 8, bottom: 0),
                                color: index.isOdd
                                    ? backgroundColor
                                    : otherBackgroundColor,
                                child: Row(children: [
                                  SizedBox(
                                      width: 100,
                                      child: Text(
                                          stats[index].item2.isNotEmpty
                                              ? stats[index].item2
                                              : "User fees",
                                          style: TextStyle(
                                              color: index.isOdd
                                                  ? textColor
                                                  : otherTextColor,
                                              fontSize: theme
                                                  .getSmallFont(context)))),
                                  SizedBox(
                                      width: 110,
                                      child: Text(
                                          "${stats[index].item3.totalSent}",
                                          style: TextStyle(
                                              color: index.isOdd
                                                  ? textColor
                                                  : otherTextColor,
                                              fontSize: theme
                                                  .getSmallFont(context)))),
                                  SizedBox(
                                      width: 110,
                                      child: Text(
                                          "${stats[index].item3.totalReceived}",
                                          style: TextStyle(
                                              color: index.isOdd
                                                  ? textColor
                                                  : otherTextColor,
                                              fontSize: theme
                                                  .getSmallFont(context)))),
                                  Expanded(
                                      child: IconButton(
                                          alignment: Alignment.centerRight,
                                          iconSize: 18,
                                          padding: const EdgeInsets.all(0),
                                          onPressed: () {
                                            delete(index);
                                          },
                                          icon: const Icon(Icons.delete)))
                                ])),
                            selectedColor:
                                index.isEven ? highlightColor : otherTextColor,
                            textColor: textColor,
                            hoverColor:
                                index.isEven ? highlightColor : otherTextColor,
                            selected: index == selectedIndex,
                            onTap: () => select(index),
                          )),
                ),
                const Divider(),
                userStats.isNotEmpty
                    ? Expanded(
                        flex: 2,
                        child: Row(children: [
                          Expanded(
                            flex: 2,
                            child: Column(children: [
                              Row(children: [
                                Text("Total Sent",
                                    style: TextStyle(color: textColor)),
                                const SizedBox(width: 50),
                                Text(
                                    textAlign: TextAlign.right,
                                    formatDCR(
                                        milliatomsToDCR(userStatsTotalSent)),
                                    style: TextStyle(color: textColor)),
                              ]),
                              const Divider(),
                              Expanded(
                                  child: SimpleInfoGrid(
                                userStats
                                    .map<Tuple2<Widget, Widget>>((e) => Tuple2(
                                        e.total < 0
                                            ? Text(e.prefix,
                                                style:
                                                    TextStyle(color: textColor))
                                            : const Empty(),
                                        e.total < 0
                                            ? Text(
                                                formatDCR(
                                                    milliatomsToDCR(e.total)),
                                                style:
                                                    TextStyle(color: textColor))
                                            : const Empty()))
                                    .toList(),
                                controller: userStatsSentCtrl,
                              ))
                            ]),
                          ),
                          Expanded(
                            flex: 2,
                            child: Column(
                                mainAxisAlignment: MainAxisAlignment.start,
                                children: [
                                  Row(children: [
                                    Text("Total Received",
                                        style: TextStyle(color: textColor)),
                                    const SizedBox(width: 50),
                                    Text(
                                        textAlign: TextAlign.right,
                                        formatDCR(milliatomsToDCR(
                                            userStatsTotalReceived)),
                                        style: TextStyle(color: textColor)),
                                  ]),
                                  const Divider(),
                                  Expanded(
                                      child: SimpleInfoGrid(
                                    userStats
                                        .map<Tuple2<Widget, Widget>>((e) =>
                                            Tuple2(
                                                e.total > 0
                                                    ? Text(e.prefix,
                                                        style: TextStyle(
                                                            color: textColor))
                                                    : const Empty(),
                                                e.total > 0
                                                    ? Text(
                                                        formatDCR(
                                                            milliatomsToDCR(
                                                                e.total)),
                                                        style: TextStyle(
                                                            color: textColor))
                                                    : const Empty()))
                                        .toList(),
                                    controller: userStatsReceivedCtrl,
                                  ))
                                ]),
                          )
                        ]))
                    : const Empty(),
              ]),
            ));
  }
}
