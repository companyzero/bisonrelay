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
                color: theme.getTheme().focusColor),
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
              contentPadding:
                  const EdgeInsets.only(left: 10, right: 10, top: 5, bottom: 5),
              errorBorder: const OutlineInputBorder(
                borderRadius: BorderRadius.all(Radius.circular(30.0)),
                borderSide: BorderSide(color: Colors.red, width: 2.0),
              ),
              focusedBorder: OutlineInputBorder(
                borderRadius: const BorderRadius.all(Radius.circular(30.0)),
                borderSide:
                    BorderSide(color: theme.getTheme().focusColor, width: 2.0),
              ),
              border: OutlineInputBorder(
                borderRadius: const BorderRadius.all(Radius.circular(30.0)),
                borderSide:
                    BorderSide(color: theme.getTheme().cardColor, width: 2.0),
              ),
              hintText: "0.00",
              hintStyle: TextStyle(
                  fontSize: theme.getMediumFont(context),
                  letterSpacing: 0.5,
                  fontWeight: FontWeight.w300,
                  color: theme.getTheme().dividerColor),
              filled: true,
              fillColor: theme.getTheme().cardColor,
              suffixText: "DCR",
            )));
