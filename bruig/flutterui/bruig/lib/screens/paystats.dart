import 'package:bruig/components/confirmation_dialog.dart';
import 'package:bruig/components/info_grid.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import 'package:tuple/tuple.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/theme_manager.dart';

class PayStatsScreenTitle extends StatelessWidget {
  const PayStatsScreenTitle({Key? key}) : super(key: key);
  @override
  Widget build(BuildContext context) {
    return const Txt.L("Payment Stats");
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
      showErrorSnackbar(this, "Unable to list payment stats: $exception");
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
      showErrorSnackbar(this, "Unable to fetch user pay stats: $exception");
    }
  }

  void delete(int index) async {
    var nick = stats[index].item2;
    if (nick == "") {
      nick = stats[index].item1;
    }
    confirmationDialog(context, () async {
      try {
        await Golib.clearPayStats(stats[index].item1);
        listPayStats();
      } catch (exception) {
        showErrorSnackbar(this, "Unable to clear stats: $exception");
      }
    }, "Clear data?", "Really clear data for user $nick?", "Clear", "Cancel");
  }

  @override
  void initState() {
    super.initState();
    listPayStats();
  }

  @override
  Widget build(BuildContext context) {
    var theme = ThemeNotifier.of(context);

    var evenBgColor = theme.colors.surfaceDim;
    var oddBgColor = theme.colors.surfaceBright;
    var evenTxtStyle =
        theme.textStyleFor(context, TextSize.small, TextColor.onSurface);
    var oddTxtStyle =
        theme.textStyleFor(context, TextSize.small, TextColor.onSurface);

    return Container(
      padding: const EdgeInsets.all(16),
      child: Column(children: [
        const Row(children: [
          SizedBox(width: 100, child: Txt.S("User")),
          SizedBox(width: 105, child: Txt.S("Sent (atoms)")),
          SizedBox(width: 130, child: Txt.S(" Received (atoms) ")),
        ]),
        const SizedBox(height: 5),
        Expanded(
          flex: 5,
          child: ListView.builder(
              itemCount: stats.length,
              padding: const EdgeInsets.all(0),
              itemBuilder: (context, index) => ListTile(
                    horizontalTitleGap: 0,
                    minVerticalPadding: 0,
                    contentPadding: const EdgeInsets.all(3),
                    tileColor: index.isEven ? evenBgColor : oddBgColor,
                    selectedColor: index.isEven ? evenBgColor : oddBgColor,
                    onTap: () => select(index),
                    shape: index == selectedIndex
                        ? Border.all(color: theme.colors.primary)
                        : null,
                    title: Row(children: [
                      SizedBox(
                          width: 100,
                          child: Text(
                              stats[index].item2.isNotEmpty
                                  ? stats[index].item2
                                  : "User fees",
                              style: index.isOdd ? oddTxtStyle : evenTxtStyle)),
                      SizedBox(
                          width: 110,
                          child: Text("${stats[index].item3.totalSent}",
                              style: index.isOdd ? oddTxtStyle : evenTxtStyle)),
                      SizedBox(
                          width: 130,
                          child: Text("${stats[index].item3.totalReceived}",
                              style: index.isOdd ? oddTxtStyle : evenTxtStyle)),
                      const Expanded(child: Empty()),
                      IconButton(
                          iconSize: 18,
                          padding: const EdgeInsets.all(0),
                          onPressed: () {
                            delete(index);
                          },
                          icon: const Icon(Icons.delete)),
                    ]),
                  )),
        ),
        const Divider(),
        userStats.isNotEmpty
            ? Expanded(
                flex: 2,
                child: Container(
                    color: theme.colors.surface,
                    child: Row(children: [
                      Expanded(
                        flex: 2,
                        child: Column(children: [
                          Row(children: [
                            const Text("Total Sent"),
                            const SizedBox(width: 50),
                            Text(
                                textAlign: TextAlign.right,
                                formatDCR(milliatomsToDCR(userStatsTotalSent))),
                          ]),
                          const Divider(),
                          Expanded(
                              child: SimpleInfoGrid(
                            userStats
                                .map<Tuple2<Widget, Widget>>((e) => Tuple2(
                                    e.total < 0
                                        ? Text(e.prefix)
                                        : const Empty(),
                                    e.total < 0
                                        ? Text(
                                            formatDCR(milliatomsToDCR(e.total)))
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
                                const Text("Total Received"),
                                const SizedBox(width: 50),
                                Text(
                                    textAlign: TextAlign.right,
                                    formatDCR(milliatomsToDCR(
                                        userStatsTotalReceived))),
                              ]),
                              const Divider(),
                              Expanded(
                                  child: SimpleInfoGrid(
                                userStats
                                    .map<Tuple2<Widget, Widget>>((e) => Tuple2(
                                        e.total > 0
                                            ? Text(e.prefix)
                                            : const Empty(),
                                        e.total > 0
                                            ? Text(formatDCR(
                                                milliatomsToDCR(e.total)))
                                            : const Empty()))
                                    .toList(),
                                controller: userStatsReceivedCtrl,
                              ))
                            ]),
                      )
                    ])))
            : const Empty(),
      ]),
    );
  }
}
