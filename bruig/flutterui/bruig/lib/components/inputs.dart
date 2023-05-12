import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

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
    TextField(
        style: const TextStyle(fontSize: 11, color: Color(0xFF8E8D98)),
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
        decoration: const InputDecoration(
          hintStyle: TextStyle(fontSize: 11, color: Color(0xFF8E8D98)),
          filled: true,
          fillColor: Color(0xFF121026),
        ));
