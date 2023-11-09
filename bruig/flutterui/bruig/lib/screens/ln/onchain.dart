import 'package:bruig/components/accounts_dropdown.dart';
import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/dcr_input.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

class LNOnChainPage extends StatefulWidget {
  const LNOnChainPage({super.key});

  @override
  State<LNOnChainPage> createState() => _LNOnChainPageState();
}

class _LNOnChainPageState extends State<LNOnChainPage> {
  String? recvAddr;
  String? recvAccount;
  String? sendAccount;
  TextEditingController sendAddrCtrl = TextEditingController();
  AmountEditingController amountCtrl = AmountEditingController();

  void generateRecvAddr() async {
    if (recvAccount == null) {
      return;
    }
    try {
      var newAddr = await Golib.lnGetDepositAddr(recvAccount!);
      setState(() {
        recvAddr = newAddr;
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to generate address: $exception");
    }
  }

  void doSend(double amount, String addr, String fromAccount) async {
    setState(() {
      sendAddrCtrl.clear();
      amountCtrl.clear();
    });
    try {
      await Golib.sendOnChain(addr, amount, fromAccount);
      showSuccessSnackbar(context, "Sent on-chain transaction");
    } catch (exception) {
      showErrorSnackbar(context, "Unable to send coins: $exception");
    }
  }

  void confirmSend() async {
    if (sendAddrCtrl.text.isEmpty) {
      showErrorSnackbar(context, "Address cannot be empty");
      return;
    }
    if (amountCtrl.amount <= 0) {
      showErrorSnackbar(context, "Amount must be positive");
      return;
    }
    if (sendAccount == null) {
      showErrorSnackbar(context, "Source account cannot be empty");
      return;
    }

    var amount = amountCtrl.amount;
    var sendAddr = sendAddrCtrl.text;
    var account = sendAccount!;
    showModalBottomSheet(
        context: context,
        builder: (BuildContext context) => Container(
            padding: const EdgeInsets.all(30),
            child: Row(children: [
              Text("Send $amount DCR to $sendAddr?",
                  style: TextStyle(color: Theme.of(context).focusColor)),
              IconButton(
                onPressed: () => Navigator.pop(context),
                icon: const Icon(Icons.cancel),
                tooltip: "Cancel",
              ),
              const Expanded(child: Empty()),
              IconButton(
                onPressed: () {
                  Navigator.pop(context);
                  doSend(amount, sendAddr, account);
                },
                icon: const Icon(Icons.attach_money),
                tooltip: "Send (cannot be undone)",
              ),
            ])));
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var secondaryTextColor = theme.dividerColor;
    var darkTextColor = theme.indicatorColor;
    var dividerColor = theme.highlightColor;
    var backgroundColor = theme.backgroundColor;
    var inputFill = theme.hoverColor;

    return Consumer<ThemeNotifier>(
      builder: (context, theme, _) => Container(
          margin: const EdgeInsets.all(1),
          decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(3), color: backgroundColor),
          padding: const EdgeInsets.all(16),
          child:
              Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Row(children: [
              Text("Receive On-Chain",
                  textAlign: TextAlign.left,
                  style: TextStyle(
                      color: darkTextColor,
                      fontSize: theme.getMediumFont(context))),
              Expanded(
                  child: Divider(
                color: dividerColor, //color of divider
                height: 10, //height spacing of divider
                thickness: 1, //thickness of divier line
                indent: 8, //spacing at the start of divider
                endIndent: 5, //spacing at the end of divider
              )),
            ]),
            const SizedBox(height: 10),
            Row(
              children: [
                Text("Account: ", style: TextStyle(color: darkTextColor)),
                const SizedBox(width: 10),
                Expanded(
                    child: AccountsDropDown(
                        onChanged: (value) => setState(() {
                              recvAccount = value;
                              recvAddr = null;
                            }))),
              ],
            ),
            const SizedBox(height: 10),
            recvAddr == null
                ? ElevatedButton(
                    onPressed: generateRecvAddr,
                    child: const Text("Generate Address"))
                : Copyable(recvAddr!, TextStyle(color: textColor)),
            const SizedBox(height: 40),
            Row(children: [
              Text("Send On-Chain",
                  textAlign: TextAlign.left,
                  style: TextStyle(
                      color: darkTextColor,
                      fontSize: theme.getMediumFont(context))),
              Expanded(
                  child: Divider(
                color: dividerColor, //color of divider
                height: 10, //height spacing of divider
                thickness: 1, //thickness of divier line
                indent: 8, //spacing at the start of divider
                endIndent: 5, //spacing at the end of divider
              )),
            ]),
            const SizedBox(height: 10),
            Row(
              children: [
                SizedBox(
                    width: 100,
                    child: Text("From Account: ",
                        style: TextStyle(color: darkTextColor))),
                const SizedBox(width: 10),
                Expanded(
                    child: AccountsDropDown(
                        onChanged: (value) => setState(() {
                              sendAccount = value;
                            }))),
              ],
            ),
            const SizedBox(height: 10),
            Row(
              children: [
                SizedBox(
                    width: 100,
                    child: Text("To Address: ",
                        style: TextStyle(color: darkTextColor))),
                const SizedBox(width: 10),
                Expanded(
                    child: TextField(
                        style: TextStyle(
                            fontSize: theme.getSmallFont(context),
                            color: secondaryTextColor),
                        controller: sendAddrCtrl,
                        decoration: InputDecoration(
                            hintText: "Destination Address",
                            hintStyle: TextStyle(
                                fontSize: theme.getSmallFont(context),
                                color: secondaryTextColor),
                            filled: true,
                            fillColor: inputFill))),
              ],
            ),
            const SizedBox(height: 10),
            Row(
              children: [
                SizedBox(
                    width: 100,
                    child: Text("Amount (DCR): ",
                        style: TextStyle(color: darkTextColor))),
                const SizedBox(width: 10),
                Expanded(child: dcrInput(controller: amountCtrl)),
              ],
            ),
            const SizedBox(height: 10),
            ElevatedButton(
                onPressed: confirmSend, child: const Text("Send On-Chain")),
          ])),
    );
  }
}
