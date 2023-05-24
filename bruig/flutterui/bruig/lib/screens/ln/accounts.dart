import 'package:bruig/components/snackbars.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';

class LNAccountsPage extends StatefulWidget {
  const LNAccountsPage({super.key});

  @override
  State<LNAccountsPage> createState() => _LNAccountsPageState();
}

class _LNAccountsPageState extends State<LNAccountsPage> {
  TextEditingController nameCtrl = TextEditingController();
  List<Account> accounts = [];

  void reloadAccounts() async {
    try {
      var newAccounts = await Golib.listAccounts();
      setState(() {
        accounts = newAccounts;
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to load accounts: $exception");
    }
  }

  void createAccount() async {
    var name = nameCtrl.text.trim();
    if (name.isEmpty) {
      showErrorSnackbar(context, "New account name cannot be empty");
      return;
    }

    try {
      setState(() => nameCtrl.clear());
      await Golib.createAccount(name);
      reloadAccounts();
    } catch (exception) {
      showErrorSnackbar(context, "Unable to create new account: $exception");
    }
  }

  @override
  void initState() {
    super.initState();
    reloadAccounts();
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var secondaryTextColor = theme.dividerColor;
    var darkTextColor = theme.indicatorColor;
    var dividerColor = theme.highlightColor;
    var backgroundColor = theme.backgroundColor;
    var inputFill = theme.hoverColor;

    return Container(
      margin: const EdgeInsets.all(1),
      decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(3), color: backgroundColor),
      padding: const EdgeInsets.all(16),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Row(children: [
          Text("Wallet Accounts",
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
        Expanded(
          child: ListView.builder(
              itemCount: accounts.length,
              itemBuilder: (BuildContext context, int index) {
                var acc = accounts[index];
                var confirmed = formatDCR(atomsToDCR(acc.confirmedBalance));
                var unconf = formatDCR(atomsToDCR(acc.unconfirmedBalance));
                return ListTile(
                  title: Text(acc.name),
                  subtitle: Text(
                      "$confirmed confirmed\n$unconf unconfirmed\nKeys: ${acc.internalKeyCount} internal, ${acc.externalKeyCount} external\n"),
                );
              }),
        ),
        Row(children: [
          Text("Create New Account",
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
          Text("Account Name:",
              style: TextStyle(fontSize: 11, color: secondaryTextColor)),
          const SizedBox(width: 10),
          Expanded(
              child: TextField(
                  style: TextStyle(fontSize: 11, color: secondaryTextColor),
                  controller: nameCtrl,
                  decoration: InputDecoration(
                      hintText: "Name of the new account",
                      hintStyle:
                          TextStyle(fontSize: 11, color: secondaryTextColor),
                      filled: true,
                      fillColor: inputFill))),
          const SizedBox(width: 10),
          ElevatedButton(onPressed: createAccount, child: const Text("Create"))
        ]),
      ]),
    );
  }
}
