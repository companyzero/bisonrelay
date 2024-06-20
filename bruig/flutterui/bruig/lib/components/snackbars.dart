import 'package:bruig/components/copyable.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:flutter/material.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

class SnackbarDisplayer extends StatefulWidget {
  final SnackBarModel snackBar;
  final Widget child;
  const SnackbarDisplayer(this.snackBar, this.child, {super.key});

  @override
  State<SnackbarDisplayer> createState() => _SnackbarDisplayerState();
}

class _SnackbarDisplayerState extends State<SnackbarDisplayer> {
  SnackBarModel get snackBar => widget.snackBar;
  SnackBarMessage snackBarMsg = SnackBarMessage.empty();

  void snackBarChanged() {
    if (snackBar.snackBars.isNotEmpty) {
      var newSnackbarMessage =
          snackBar.snackBars[snackBar.snackBars.length - 1];
      if (newSnackbarMessage.msg != snackBarMsg.msg ||
          newSnackbarMessage.error != snackBarMsg.error ||
          newSnackbarMessage.timestamp != snackBarMsg.timestamp) {
        setState(() {
          snackBarMsg = newSnackbarMessage;
        });
        var bgColor = snackBarMsg.error
            ? SurfaceColor.errorContainer
            : SurfaceColor.primaryContainer;
        var textColor = textColorForSurfaceColor[bgColor];
        var theme = Provider.of<ThemeNotifier>(context, listen: false);
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
            backgroundColor: theme.surfaceColor(bgColor),
            content: Copyable(snackBarMsg.msg,
                textStyle:
                    theme.textStyleFor(context, TextSize.small, textColor),
                showSnackbar: false)));
      }
    }
  }

  @override
  void initState() {
    super.initState();
    widget.snackBar.addListener(snackBarChanged);
  }

  @override
  void didUpdateWidget(SnackbarDisplayer oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.snackBar.removeListener(snackBarChanged);
    widget.snackBar.addListener(snackBarChanged);
  }

  @override
  void dispose() {
    widget.snackBar.removeListener(snackBarChanged);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return widget.child;
  }
}

void showErrorSnackbar(BuildContext context, String msg) {
  if (!context.mounted) {
    debugPrint("Unmounted snackbar error message: $msg");
  }
  var snackBar = Provider.of<SnackBarModel>(context, listen: false);

  snackBar.error(msg);
}

void showSuccessSnackbar(BuildContext context, String msg) {
  if (!context.mounted) {
    debugPrint("Unmounted snackbar success message: $msg");
    return;
  }
  var snackBar = Provider.of<SnackBarModel>(context, listen: false);

  snackBar.success(msg);
}
