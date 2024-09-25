import 'dart:math';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/dcr_input.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';

void showPayTipModalBottom(BuildContext context, ChatModel chat) {
  showModalBottomSheet(
    context: context,
    builder: (BuildContext context) => PayTip(chat),
  );
}

class PayTip extends StatefulWidget {
  final ChatModel chat;

  const PayTip(this.chat, {super.key});

  @override
  State<PayTip> createState() => _PayTipState();
}

class _PayTipState extends State<PayTip> {
  double amount = 0;
  ChatModel get chat => widget.chat;

  void payTip() {
    if (amount <= 0) return;
    Navigator.pop(context);
    chat.payTip(amount);
  }

  @override
  Widget build(BuildContext context) {
    var chat = widget.chat;

    return Container(
      padding: const EdgeInsets.all(30),
      child: Wrap(
          runSpacing: 10,
          spacing: 10,
          alignment: WrapAlignment.center,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            Text(
                "Pay tip to '${chat.nick.substring(0, min(chat.nick.length, 100))}'"),
            SizedBox(width: 150, child: dcrInput(onChanged: (v) => amount = v)),
            CancelButton(onPressed: () => Navigator.pop(context)),
            OutlinedButton(onPressed: payTip, child: const Text("Pay")),
          ]),
    );
  }
}
