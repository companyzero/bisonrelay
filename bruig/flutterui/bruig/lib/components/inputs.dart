import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

// Text field with default app styling.
class TextInput extends StatelessWidget {
  final TextEditingController? controller;
  final String? hintText;
  final TextSize textSize;
  final ValueChanged<String>? onSubmitted;
  const TextInput(
      {this.textSize = TextSize.medium,
      this.controller,
      this.hintText,
      this.onSubmitted,
      super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => TextField(
              onSubmitted: onSubmitted,
              style: theme.textStyleFor(context, textSize, null),
              controller: controller,
              decoration: hintText != null
                  ? InputDecoration(hintText: hintText!)
                  : null,
            ));
  }
}

class IntEditingController extends TextEditingController {
  int get intvalue => text != "" ? int.parse(text) : 0;
  set intvalue(int v) => text = v.toString();
}

class _LimitIntTextInputFormatter extends TextInputFormatter {
  @override
  TextEditingValue formatEditUpdate(
      TextEditingValue oldValue, TextEditingValue newValue) {
    if (newValue.text == "") return newValue;
    try {
      int.parse(newValue.text);
      return newValue;
    } catch (exception) {
      return oldValue;
    }
  }
}

Widget intInput({
  void Function(int amount)? onChanged,
  IntEditingController? controller,
}) =>
    Consumer<ThemeNotifier>(
        builder: (context, theme, _) => TextField(
              style: theme.textStyleFor(context, TextSize.small, null),
              controller: controller,
              onChanged: (String v) {
                try {
                  int val = v != "" ? int.parse(v) : 0;
                  if (onChanged != null) onChanged(val);
                } catch (exception) {
                  // ignore.
                }
              },
              keyboardType:
                  const TextInputType.numberWithOptions(decimal: true),
              inputFormatters: [_LimitIntTextInputFormatter()],
            ));
