import 'dart:async';

import 'package:bruig/components/dcr_input.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:bruig/models/snackbar.dart';

class LNPaymentsPage extends StatefulWidget {
  final SnackBarModel snackBar;
  const LNPaymentsPage(this.snackBar, {Key? key}) : super(key: key);

  @override
  State<LNPaymentsPage> createState() => _LNPaymentsPageState();
}

class _DecodedInvoice extends StatelessWidget {
  final LNDecodedInvoice inv;
  final AmountEditingController payAmountCtrl;
  const _DecodedInvoice(this.inv, this.payAmountCtrl, {Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var darkTextColor = theme.indicatorColor;
    var dividerColor = theme.highlightColor;
    var secondaryTextColor = theme.dividerColor;
    return Column(
      children: [
        Row(children: [
          Text("Decoded Invoice to Pay",
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
              width: 80,
              child: Text("Description:",
                  style: TextStyle(fontSize: 11, color: secondaryTextColor))),
          SizedBox(
              width: 500,
              child: Text(
                inv.description,
                style: TextStyle(fontSize: 11, color: secondaryTextColor),
              ))
        ]),
        const SizedBox(height: 8),
        Row(children: [
          SizedBox(
              width: 80,
              child: Text("Destination:",
                  style: TextStyle(fontSize: 11, color: secondaryTextColor))),
          SizedBox(
              width: 500,
              child: Text(
                inv.destination,
                style: TextStyle(fontSize: 11, color: secondaryTextColor),
              ))
        ]),
        const SizedBox(height: 8),
        Row(children: [
          SizedBox(
              width: 80,
              child: Text("Amount:",
                  style: TextStyle(fontSize: 11, color: secondaryTextColor))),
          SizedBox(
              width: 100,
              child: inv.amount == 0
                  ? dcrInput(controller: payAmountCtrl)
                  : Text("${inv.amount.toStringAsFixed(8)} DCR",
                      style:
                          TextStyle(fontSize: 11, color: secondaryTextColor)))
        ]),
        const SizedBox(height: 21),
      ],
    );
  }
}

class _LNPaymentsPageState extends State<LNPaymentsPage> {
  SnackBarModel get snackBar => widget.snackBar;
  TextEditingController memoCtrl = TextEditingController();
  TextEditingController payCtrl = TextEditingController();
  AmountEditingController genAmountCtrl = AmountEditingController();
  AmountEditingController payAmountCtrl = AmountEditingController();

  String generatedInvoice = "";
  String invoiceToDecode = "";
  Timer? decodeTimer;
  LNDecodedInvoice? decoded;
  bool paying = false;

  void generateInvoice() async {
    try {
      var res = await Golib.lnGenInvoice(genAmountCtrl.amount, memoCtrl.text);
      setState(() {
        generatedInvoice = res.paymentRequest;
        memoCtrl.clear();
        genAmountCtrl.clear();
      });
    } catch (exception) {
      snackBar.error("Unable to generate invoice: $exception");
    }
  }

  void copyInvoiceToClipboard() async {
    Clipboard.setData(ClipboardData(text: generatedInvoice));
    snackBar.success("Copied generated invoice to clipboard");
  }

  void decodeInvoice() async {
    decodeTimer = null;
    try {
      var newDecoded = await Golib.lnDecodeInvoice(invoiceToDecode);
      setState(() {
        decoded = newDecoded;
      });
    } catch (exception) {
      snackBar.error("Unable to decode invoice: $exception");
    }
  }

  void onPayInvoiceChanged() {
    var invoice = payCtrl.text.trim();
    if (invoice == invoiceToDecode) return;
    if (decodeTimer != null) {
      decodeTimer!.cancel();
      decodeTimer = null;
    }
    invoiceToDecode = invoice;
    setState(() {
      decoded = null;
    });
    if (invoice == "") {
      return;
    }
    decodeTimer = Timer(const Duration(seconds: 1), decodeInvoice);
  }

  void payInvoice() async {
    try {
      var amount = payAmountCtrl.amount;
      setState(() {
        decoded = null;
        paying = true;
        payAmountCtrl.clear();
      });
      await Golib.lnPayInvoice(invoiceToDecode, amount);
      snackBar.success("Invoice Paid!");
    } catch (exception) {
      snackBar.error("Unable to pay invoice: $exception");
    } finally {
      setState(() {
        paying = false;
      });
    }
  }

  @override
  void initState() {
    super.initState();
    payCtrl.addListener(onPayInvoiceChanged);
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
    return Container(
      margin: const EdgeInsets.all(1),
      decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(3), color: backgroundColor),
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(children: [
            Text("Generate Invoice",
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
                width: 80,
                child: Text("Memo:",
                    style: TextStyle(fontSize: 11, color: secondaryTextColor))),
            SizedBox(
                width: 500,
                child: TextField(
                    style: TextStyle(fontSize: 11, color: secondaryTextColor),
                    controller: memoCtrl,
                    decoration: InputDecoration(
                        hintText: "Type an invoice description",
                        hintStyle:
                            TextStyle(fontSize: 11, color: secondaryTextColor),
                        filled: true,
                        fillColor: inputFill)))
          ]),
          const SizedBox(height: 8),
          Row(children: [
            SizedBox(
                width: 80,
                child: Text("Amount:",
                    style: TextStyle(fontSize: 11, color: secondaryTextColor))),
            SizedBox(width: 100, child: dcrInput(controller: genAmountCtrl))
          ]),
          const SizedBox(height: 17),
          ElevatedButton(
            onPressed: generateInvoice,
            style: ElevatedButton.styleFrom(
                textStyle: TextStyle(color: textColor, fontSize: 11)),
            child: const Text("Generate Invoice"),
          ),
          const SizedBox(height: 21),
          generatedInvoice != ""
              ? Column(children: [
                  Row(children: [
                    Text("Generated Invoice",
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
                  Text(generatedInvoice,
                      style:
                          TextStyle(color: secondaryTextColor, fontSize: 11)),
                  const SizedBox(height: 10),
                  ElevatedButton(
                    onPressed: copyInvoiceToClipboard,
                    style: ElevatedButton.styleFrom(
                      textStyle: TextStyle(color: textColor, fontSize: 11),
                    ),
                    child: const Text("Copy Invoice"),
                  ),
                  const SizedBox(height: 21),
                ])
              : const Empty(),
          Row(children: [
            Text("Pay Invoice",
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
                width: 80,
                child: Text("Invoice ID:",
                    style: TextStyle(fontSize: 11, color: secondaryTextColor))),
            SizedBox(
                width: 500,
                child: TextField(
                    style: TextStyle(fontSize: 11, color: secondaryTextColor),
                    controller: payCtrl,
                    decoration: InputDecoration(
                        hintText: "Type an invoice hash",
                        hintStyle:
                            TextStyle(fontSize: 11, color: secondaryTextColor),
                        filled: true,
                        fillColor: inputFill)))
          ]),
          const SizedBox(height: 20),
          //TextField(controller: payCtrl),
          decoded != null
              ? _DecodedInvoice(decoded!, payAmountCtrl)
              : const Empty(),
          decoded != null && decodeTimer == null
              ? ElevatedButton.icon(
                  icon: Icon(
                      !paying ? Icons.credit_score : Icons.hourglass_bottom),
                  label: Text("Pay Invoice",
                      style: TextStyle(fontSize: 11, color: textColor)),
                  onPressed: !paying ? payInvoice : null,
                )
              : const Empty(),
          paying ? const Icon(Icons.hourglass_bottom) : const Empty(),
        ],
      ),
    );
  }
}
