import 'package:bruig/components/copyable.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

void showErrorSnackbar(BuildContext context, String msg) {
  var snackBar = Provider.of<SnackBarModel>(context, listen: false);

  snackBar.error(msg);
}

void showSuccessSnackbar(BuildContext context, String msg) {
  var snackBar = Provider.of<SnackBarModel>(context, listen: false);

  snackBar.success(msg);
}
