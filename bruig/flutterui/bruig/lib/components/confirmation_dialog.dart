import 'package:flutter/material.dart';
import 'package:bruig/screens/overview.dart';

void confirmationDialog(
    BuildContext context,
    VoidCallback confirm,
    String title,
    String content,
    String confirmButtonText,
    String cancelButtonText) {
  showDialog(
      context: context,
      builder: (BuildContext ctx) {
        return AlertDialog(
          title: Text(title),
          content: Text(content),
          actions: [
            // The "Yes" button
            TextButton(
                onPressed: () {
                  confirm();
                  Navigator.of(scaffoldKey.currentContext!).pop();
                },
                child: Text(
                    confirmButtonText != "" ? confirmButtonText : "Confirm")),
            TextButton(
                onPressed: () =>
                    Navigator.of(scaffoldKey.currentContext!).pop(),
                child:
                    Text(cancelButtonText != "" ? cancelButtonText : "Cancel"))
          ],
        );
      });
}
