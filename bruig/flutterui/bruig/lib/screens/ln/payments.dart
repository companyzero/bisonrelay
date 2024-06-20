import 'dart:async';

import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/dcr_input.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/inputs.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/screens/ln/components.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart' as services;
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:bruig/theme_manager.dart';

class LNPaymentsPage extends StatefulWidget {
  const LNPaymentsPage({Key? key}) : super(key: key);

  @override
  State<LNPaymentsPage> createState() => _LNPaymentsPageState();
}

class _LNPaymentsPageState extends State<LNPaymentsPage> {
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
    var snackbar = SnackBarModel.of(context);
    try {
      var res = await Golib.lnGenInvoice(genAmountCtrl.amount, memoCtrl.text);
      setState(() {
        generatedInvoice = res.paymentRequest;
        memoCtrl.clear();
        genAmountCtrl.clear();
      });
    } catch (exception) {
      snackbar.error("Unable to generate invoice: $exception");
    }
  }

  void copyInvoiceToClipboard() async {
    services.Clipboard.setData(services.ClipboardData(text: generatedInvoice));
    showSuccessSnackbar(this, "Copied generated invoice to clipboard");
  }

  void decodeInvoice() async {
    var snackbar = SnackBarModel.of(context);
    decodeTimer = null;
    try {
      var newDecoded = await Golib.lnDecodeInvoice(invoiceToDecode);
      setState(() {
        decoded = newDecoded;
      });
    } catch (exception) {
      snackbar.error("Unable to decode invoice: $exception");
    }
  }

  void onPayInvoiceChanged() {
    var invoice = payCtrl.text.trim();
    if (invoice.startsWith("lnpay://")) {
      invoice = invoice.substring(8);
    }
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
    var snackbar = SnackBarModel.of(context);
    try {
      var amount = payAmountCtrl.amount;
      setState(() {
        decoded = null;
        paying = true;
        payAmountCtrl.clear();
      });
      await Golib.lnPayInvoice(invoiceToDecode, amount);
      snackbar.success("Invoice Paid!");
    } catch (exception) {
      snackbar.error("Unable to pay invoice: $exception");
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

  List<Widget> _buildDecodedInvoice(BuildContext context) {
    if (decoded == null) {
      return [];
    }

    var inv = decoded!;
    return [
      const LNInfoSectionHeader("Decoded Invoice to Pay"),
      const SizedBox(height: 8),
      Row(children: [
        const SizedBox(width: 80, child: Txt.S("Description:")),
        Expanded(child: Txt.S(inv.description))
      ]),
      const SizedBox(height: 8),
      Row(children: [
        const SizedBox(width: 80, child: Txt.S("Destination:")),
        Expanded(
            child: Copyable.txt(Txt.S(
          inv.destination,
        )))
      ]),
      const SizedBox(height: 8),
      Row(children: [
        const SizedBox(width: 80, child: Txt.S("Amount:")),
        Expanded(
            child: inv.amount == 0
                ? dcrInput(controller: payAmountCtrl, textSize: TextSize.small)
                : Txt.S("${inv.amount.toStringAsFixed(8)} DCR"))
      ]),
      const SizedBox(height: 10),
    ];
  }

  @override
  Widget build(BuildContext context) {
    return Container(
        padding: const EdgeInsets.all(16),
        alignment: Alignment.topLeft,
        child: SingleChildScrollView(
            child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const LNInfoSectionHeader("Generate Invoice"),
            Row(children: [
              const SizedBox(width: 80, child: Text("Memo:")),
              Expanded(
                  child: TextInput(
                      controller: memoCtrl,
                      hintText: "Type an invoice description")),
            ]),
            Row(children: [
              const SizedBox(width: 80, child: Text("Amount:")),
              SizedBox(width: 150, child: dcrInput(controller: genAmountCtrl))
            ]),
            const SizedBox(height: 17),
            OutlinedButton(
              onPressed: generateInvoice,
              child: const Text("Generate Invoice"),
            ),
            const SizedBox(height: 21),
            ...(generatedInvoice == ""
                ? []
                : [
                    const LNInfoSectionHeader("Generated Invoice"),
                    const SizedBox(height: 8),
                    Copyable.txt(Txt.S(generatedInvoice)),
                    const SizedBox(height: 21),
                  ]),
            const LNInfoSectionHeader("Pay Invoice"),
            const SizedBox(height: 8),
            Row(children: [
              const SizedBox(width: 80, child: Text("Invoice:")),
              Expanded(
                  child: TextInput(
                      controller: payCtrl, hintText: "Type an LN invoice"))
            ]),
            const SizedBox(height: 20),
            ..._buildDecodedInvoice(context),
            decoded != null && decodeTimer == null
                ? OutlinedButton.icon(
                    icon: Icon(
                        !paying ? Icons.credit_score : Icons.hourglass_bottom),
                    label: const Text("Pay Invoice"),
                    onPressed: !paying ? payInvoice : null,
                  )
                : const Empty(),
            paying ? const Icon(Icons.hourglass_bottom) : const Empty(),
          ],
        )));
  }
}
