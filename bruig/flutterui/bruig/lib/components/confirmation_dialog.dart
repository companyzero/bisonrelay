import 'package:flutter/material.dart';

void confirmationDialog(
    BuildContext context,
    VoidCallback confirm,
    VoidCallback cancel,
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
                onPressed: confirm,
                child: Text(
                    confirmButtonText != "" ? confirmButtonText : "Confirm")),
            TextButton(
                onPressed: cancel,
                child:
                    Text(cancelButtonText != "" ? cancelButtonText : "Cancel"))
          ],
        );
      });
}
