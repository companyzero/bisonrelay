import 'package:flutter/material.dart';
import 'package:bruig/screens/overview.dart';

void confirmationDialog(
    BuildContext context,
    VoidCallback confirm,
    String title,
    String content,
    String confirmButtonText,
    String cancelButtonText,
    {VoidCallback? onCancel}) {
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
                  if (scaffoldKey.currentContext != null) {
                    Navigator.of(scaffoldKey.currentContext!).pop();
                  }
                },
                child: Text(
                    confirmButtonText != "" ? confirmButtonText : "Confirm")),
            TextButton(
                onPressed: () {
                  if (scaffoldKey.currentContext != null) {
                    Navigator.of(scaffoldKey.currentContext!).pop();
                  }
                  if (onCancel != null) {
                    onCancel();
                  }
                },
                child:
                    Text(cancelButtonText != "" ? cancelButtonText : "Cancel"))
          ],
        );
      });
}
