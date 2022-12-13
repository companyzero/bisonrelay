import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

void showPayTipModalBottom(BuildContext context, ChatModel chat) {
  showModalBottomSheet(
    context: context,
    builder: (BuildContext context) => PayTip(chat),
  );
}

class PayTip extends StatefulWidget {
  final ChatModel chat;

  const PayTip(this.chat, {Key? key}) : super(key: key);

  @override
  State<PayTip> createState() => _PayTipState();
}

class _PayTipState extends State<PayTip> {
  void payTip(ChatModel chat, double amount) {
    if (amount <= 0) return;
    chat.payTip(amount);
  }

  @override
  Widget build(BuildContext context) {
    var chat = widget.chat;
    double amount = 0;

    return Container(
      padding: const EdgeInsets.all(30),
      child: Row(children: [
        Text("Pay tip to '${chat.nick}'",
            style: TextStyle(color: Theme.of(context).focusColor)),
        const SizedBox(width: 10),
        Container(
            width: 150,
            margin: const EdgeInsets.only(right: 10),
            child: TextField(
              onChanged: (String v) => amount = v != "" ? double.parse(v) : 0,
              keyboardType:
                  const TextInputType.numberWithOptions(decimal: true),
              inputFormatters: [
                FilteringTextInputFormatter.allow(RegExp(r'[0-9]+\.?[0-9]*'))
              ],
              decoration: const InputDecoration(
                hintText: "0.00",
                suffixText: "DCR",
              ),
            )),
        ElevatedButton(
          onPressed: () => Navigator.pop(context),
          style: ElevatedButton.styleFrom(backgroundColor: Colors.grey),
          child: const Text("Cancel"),
        ),
        const SizedBox(width: 10),
        ElevatedButton(
            onPressed: () {
              Navigator.pop(context);
              payTip(chat, amount);
            },
            child: const Text("Pay")),
      ]),
    );
  }
}
