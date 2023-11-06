import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

class AmountEditingController extends TextEditingController {
  double get amount => text != "" ? double.parse(text) : 0;
  set amount(double v) => text = v.toStringAsFixed(8);
}

Widget dcrInput(
        {void Function(double amount)? onChanged,
        AmountEditingController? controller}) =>
    Consumer<ThemeNotifier>(
        builder: (context, theme, _) => TextField(
            style: TextStyle(
                fontSize: theme.getSmallFont(context),
                color: Color(0xFF8E8D98)),
            controller: controller,
            onChanged: (String v) {
              double amount = v != "" ? double.parse(v) : 0;
              if (onChanged != null) onChanged(amount);
            },
            keyboardType: const TextInputType.numberWithOptions(decimal: true),
            inputFormatters: [
              FilteringTextInputFormatter.allow(RegExp(r'[0-9]+\.?[0-9]*'))
            ],
            decoration: InputDecoration(
              hintStyle: TextStyle(
                  fontSize: theme.getSmallFont(context),
                  color: Color(0xFF8E8D98)),
              filled: true,
              fillColor: const Color.fromARGB(255, 84, 84, 89),
              hintText: "0.00",
              suffixText: "DCR",
            )));
