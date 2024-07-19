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

void showConfirmDialog(
  BuildContext context, {
  String title = "Confirmation",
  String? content,
  Widget? child,
  String confirmButtonText = "Confirm",
  String cancelButtonText = "Cancel",
  VoidCallback? onConfirm,
  VoidCallback? onCancel,
}) {
  pop() {
    if (scaffoldKey.currentContext != null) {
      Navigator.of(scaffoldKey.currentContext!).pop();
    }
  }

  showDialog(
      context: context,
      builder: (BuildContext ctx) {
        return AlertDialog(
          title: Text(title),
          content: child ?? Text(content ?? ""),
          actions: [
            // The "Yes" button
            TextButton(
                onPressed: () {
                  pop();
                  onConfirm != null ? onConfirm() : null;
                },
                child: Text(confirmButtonText)),
            TextButton(
                onPressed: () {
                  pop();
                  onCancel != null ? onCancel() : null;
                },
                child: Text(cancelButtonText))
          ],
        );
      });
}
