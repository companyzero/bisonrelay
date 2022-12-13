import 'package:flutter/material.dart';

void showErrorSnackbar(BuildContext context, String msg) {
  ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      backgroundColor: Theme.of(context).errorColor,
      content: Text(msg, style: const TextStyle(color: Color(0xFFE4E3E6)))));
}

void showSuccessSnackbar(BuildContext context, String msg) {
  ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      backgroundColor: Colors.green[300],
      content: Text(msg, style: const TextStyle(color: Color(0xFFE4E3E6)))));
}
