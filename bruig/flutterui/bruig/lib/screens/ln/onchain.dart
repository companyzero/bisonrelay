import 'package:bruig/components/accounts_dropdown.dart';
import 'package:bruig/components/confirmation_dialog.dart';
import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/dcr_input.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/indicator.dart';
import 'package:bruig/components/inputs.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/screens/ln/components.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';

class LNOnChainPage extends StatefulWidget {
  final ClientModel client;
  const LNOnChainPage(this.client, {super.key});

  @override
  State<LNOnChainPage> createState() => _LNOnChainPageState();
}

class _LNOnChainPageState extends State<LNOnChainPage> {
  String? recvAddr;
  String? recvAccount;
  String? sendAccount;
  TextEditingController sendAddrCtrl = TextEditingController();
  AmountEditingController amountCtrl = AmountEditingController();
  int rescanProgress = -1;
  int rescanTarget = 0;
  List<Transaction> transactions = [];
  bool loadingTxs = false;

  void generateRecvAddr() async {
    var snackbar = SnackBarModel.of(context);
    if (recvAccount == null) {
      return;
    }
    try {
      var newAddr = await Golib.lnGetDepositAddr(recvAccount!);
      setState(() {
        recvAddr = newAddr;
      });
    } catch (exception) {
      snackbar.error("Unable to generate address: $exception");
    }
  }

  void doSend(double amount, String addr, String fromAccount) async {
    var snackbar = SnackBarModel.of(context);
    setState(() {
      sendAddrCtrl.clear();
      amountCtrl.clear();
    });
    try {
      await Golib.sendOnChain(addr, amount, fromAccount);
      snackbar.success("Sent on-chain transaction");
      listTransactions();
    } catch (exception) {
      snackbar.error("Unable to send coins: $exception");
    }
  }

  void confirmSend() async {
    var snackbar = SnackBarModel.of(context);
    if (sendAddrCtrl.text.isEmpty) {
      snackbar.error("Address cannot be empty");
      return;
    }
    if (amountCtrl.amount <= 0) {
      snackbar.error("Amount must be positive");
      return;
    }
    if (sendAccount == null) {
      snackbar.error("Source account cannot be empty");
      return;
    }

    var amount = amountCtrl.amount;
    var sendAddr = sendAddrCtrl.text;
    var account = sendAccount!;
    confirmationDialog(context, () {
      doSend(amount, sendAddr, account);
    }, "Confirm Send", "Send $amount DCR to $sendAddr?", "Send", "Cancel");
  }

  void rescanProgressed() {
    setState(() {
      rescanProgress = widget.client.rescanNotifier.progressHeight;
    });
  }

  void rescan() async {
    var snackbar = SnackBarModel.of(context);
    var tipHeight = (await Golib.lnGetInfo()).blockHeight;

    var rescanNtfn = widget.client.rescanNotifier;
    rescanNtfn.addListener(rescanProgressed);

    setState(() {
      rescanProgress = 0;
      rescanTarget = tipHeight;
    });

    try {
      await Golib.rescanWallet(0);
    } catch (exception) {
      snackbar.error("Unable to rescan wallet: $exception");
    }

    rescanNtfn.removeListener(rescanProgressed);
    setState(() {
      rescanProgress = -1;
    });
  }

  void listTransactions() async {
    var snackbar = SnackBarModel.of(context);
    setState(() {
      loadingTxs = true;
    });
    try {
      var txs = await Golib.listTransactions(0, 0);
      setState(() {
        transactions = txs;
      });
    } catch (exception) {
      snackbar.error("Unable to list transactions: $exception");
    } finally {
      setState(() {
        loadingTxs = false;
      });
    }
  }

  @override
  void initState() {
    super.initState();
    listTransactions();
  }

  @override
  void dispose() {
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Container(
        alignment: Alignment.topLeft,
        padding: const EdgeInsets.all(16),
        child: SingleChildScrollView(
            child:
                Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          const LNInfoSectionHeader("Receive On-Chain"),
          const SizedBox(height: 10),
          Row(children: [
            const Text("Account: "),
            const SizedBox(width: 10),
            Expanded(
                child: AccountsDropDown(
                    onChanged: (value) => setState(() {
                          recvAccount = value;
                          recvAddr = null;
                        }))),
          ]),
          const SizedBox(height: 10),
          recvAddr == null
              ? OutlinedButton(
                  onPressed: generateRecvAddr,
                  child: const Text("Generate Address"))
              : Copyable.txt(Txt(recvAddr!)),
          const SizedBox(height: 40),
          const LNInfoSectionHeader("Send On-Chain"),
          const SizedBox(height: 10),
          Row(
            children: [
              const SizedBox(width: 110, child: Text("From Account: ")),
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
              const SizedBox(width: 110, child: Text("To Address: ")),
              const SizedBox(width: 10),
              Expanded(
                  child: TextInput(
                controller: sendAddrCtrl,
                hintText: "Destination Address",
              )),
            ],
          ),
          const SizedBox(height: 10),
          Row(
            children: [
              const SizedBox(width: 110, child: Text("Amount (DCR): ")),
              const SizedBox(width: 10),
              Expanded(child: dcrInput(controller: amountCtrl)),
            ],
          ),
          const SizedBox(height: 15),
          OutlinedButton(
              onPressed: confirmSend, child: const Text("Send On-Chain")),
          const SizedBox(height: 10),
          const LNInfoSectionHeader("Rescan"),
          const SizedBox(height: 10),
          OutlinedButton(
              onPressed: rescanProgress == -1 ? rescan : null,
              child: const Text("Rescan Wallet")),
          const SizedBox(height: 10),
          rescanProgress < 0
              ? const Empty()
              : Txt.S(
                  "Rescanned through $rescanProgress / $rescanTarget blocks (${(rescanProgress / rescanTarget * 100).toStringAsFixed(2)}%)"),
          Row(children: [
            const Txt.S("On-Chain Transactions"),
            if (loadingTxs) ...[
              const SizedBox(width: 8),
              const SizedBox(
                  width: 15,
                  height: 15,
                  child: IndeterminateIndicator(strokeWidth: 2.0)),
            ],
            const SizedBox(width: 8),
            const Expanded(child: Divider()),
          ]),
          const SizedBox(height: 10),
          if (transactions.isNotEmpty)
            ...transactions
                .map((tx) => Row(children: [
                      Flexible(
                          flex: 4,
                          child: Align(
                              alignment: Alignment.topRight,
                              child: Txt.S(formatDCR(atomsToDCR(tx.amount))))),
                      const SizedBox(width: 10),
                      Flexible(
                          flex: 2,
                          child: Align(
                              alignment: Alignment.topRight,
                              child: Txt.S("${tx.blockHeight}"))),
                      const SizedBox(width: 10),
                      Flexible(flex: 15, child: Copyable.txt(Txt.S(tx.txHash))),
                    ]))
                .toList(),
        ])));
  }
}
