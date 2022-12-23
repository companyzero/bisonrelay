import 'package:bruig/components/copyable.dart';
import 'package:flutter/material.dart';

void showErrorSnackbar(BuildContext context, String msg) {
  ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      backgroundColor: Theme.of(context).errorColor,
      content: Copyable(msg, const TextStyle(color: Color(0xFFE4E3E6)),
          showSnackbar: false)));
}

void showSuccessSnackbar(BuildContext context, String msg) {
  ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      backgroundColor: Colors.green[300],
      content: Copyable(msg, const TextStyle(color: Color(0xFFE4E3E6)),
          showSnackbar: false)));
}
