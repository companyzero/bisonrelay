import 'package:bruig/components/inputs.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/screens/ln/components.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

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
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Container(
              padding: const EdgeInsets.all(16),
              child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const LNInfoSectionHeader("Wallet Accounts"),
                    Expanded(
                      child: ListView.builder(
                          itemCount: accounts.length,
                          itemBuilder: (BuildContext context, int index) {
                            var acc = accounts[index];
                            var confirmed =
                                formatDCR(atomsToDCR(acc.confirmedBalance));
                            var unconf =
                                formatDCR(atomsToDCR(acc.unconfirmedBalance));
                            return ListTile(
                              contentPadding: EdgeInsets.zero,
                              minVerticalPadding: 2,
                              title: Txt.S(acc.name),
                              subtitle: Txt.S(
                                  "$confirmed confirmed\n$unconf unconfirmed\nKeys: ${acc.internalKeyCount} internal, ${acc.externalKeyCount} external\n"),
                            );
                          }),
                    ),
                    const LNInfoSectionHeader("Create New Account"),
                    const SizedBox(height: 21),
                    Row(children: [
                      const Txt.S("Account Name:"),
                      const SizedBox(width: 10),
                      Expanded(
                          child: TextInput(
                        textSize: TextSize.small,
                        controller: nameCtrl,
                        hintText: "Name of the new account",
                      )),
                      ElevatedButton(
                          onPressed: createAccount, child: const Text("Create"))
                    ]),
                  ]),
            ));
  }
}
