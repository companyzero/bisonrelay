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
        TextSize textSize = TextSize.medium,
        AmountEditingController? controller}) =>
    Consumer<ThemeNotifier>(
        builder: (context, theme, _) => TextField(
            style: theme.textStyleFor(context, textSize, null),
            controller: controller,
            onChanged: (String v) {
              double amount = v != "" ? double.parse(v) : 0;
              if (onChanged != null) onChanged(amount);
            },
            keyboardType: const TextInputType.numberWithOptions(decimal: true),
            inputFormatters: [
              FilteringTextInputFormatter.allow(RegExp(r'[0-9]+\.?[0-9]*'))
            ],
            decoration: const InputDecoration(
              contentPadding: EdgeInsets.zero,
              // errorBorder: OutlineInputBorder(
              //   borderRadius: const BorderRadius.all(Radius.circular(30.0)),
              //   borderSide: BorderSide(color: theme.colors.error, width: 2.0),
              // ),
              // focusedBorder: OutlineInputBorder(
              //   borderRadius: const BorderRadius.all(Radius.circular(30.0)),
              //   borderSide: BorderSide(color: theme.colors.outline, width: 2.0),
              // ),
              // border: OutlineInputBorder(
              //   borderRadius: const BorderRadius.all(Radius.circular(30.0)),
              //   borderSide: BorderSide(color: theme.colors.outline, width: 2.0),
              // ),
              hintText: "0.00",
              // filled: true,
              // fillColor: theme.colors.surfaceContainer,
              suffixText: "DCR",
            )));
