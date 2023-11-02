import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

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
            style: TextStyle(
                fontSize: theme.getSmallFont(), color: const Color(0xFF8E8D98)),
            controller: controller,
            onChanged: (String v) {
              try {
                int val = v != "" ? int.parse(v) : 0;
                if (onChanged != null) onChanged(val);
              } catch (exception) {
                // ignore.
              }
            },
            keyboardType: const TextInputType.numberWithOptions(decimal: true),
            inputFormatters: [_LimitIntTextInputFormatter()],
            decoration: InputDecoration(
              hintStyle: TextStyle(
                  fontSize: theme.getSmallFont(),
                  color: const Color(0xFF8E8D98)),
              filled: true,
              fillColor: const Color(0xFF121026),
            )));
