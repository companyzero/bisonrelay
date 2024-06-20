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

// NOTE: this only shows the snackbar error if the contextOrState is still
// mounted. Prefer saving the SnackBarModel across the async gap when the error
// should be displayed even if the component is unmounted:
//
// Future<void> foo(BuildContext) async {
//   var snackbar = SnackBarModel.of(context);
//   try {
//     await bar();
//   } catch(exception) {
//     snackbar.error("Unable to bar(): $exception");
//   }
// }
//
// This function is useful in situations where it would be an error to display
// the error snackbar message if the component has been unmounted.
void showErrorSnackbar(dynamic contextOrState, String msg) {
  BuildContext context;
  if (contextOrState is BuildContext) {
    context = contextOrState;
  } else if (contextOrState is State) {
    if (!contextOrState.mounted) {
      debugPrint(
          "Tried showing error snackbar but state $contextOrState was not mounted: $msg");
      return;
    }

    context = contextOrState.context;
  } else {
    throw "Passed arg not a State or BuildContext: $contextOrState";
  }

  if (!context.mounted) {
    debugPrint("Context is umounted for snackback error msg: $msg");
    return;
  }

  var snackBar = Provider.of<SnackBarModel>(context, listen: false);
  snackBar.error(msg);
}

// See showErrorSnackbar for warning.
void showSuccessSnackbar(dynamic contextOrState, String msg) {
  BuildContext context;
  if (contextOrState is BuildContext) {
    context = contextOrState;
  } else if (contextOrState is State) {
    if (!contextOrState.mounted) {
      debugPrint(
          "Tried showing error snackbar but state $contextOrState was not mounted: $msg");
      return;
    }

    context = contextOrState.context;
  } else {
    throw "Passed arg not a State or BuildContext: $contextOrState";
  }

  if (!context.mounted) {
    debugPrint("Context is umounted for snackback error msg: $msg");
    return;
  }

  var snackBar = Provider.of<SnackBarModel>(context, listen: false);
  snackBar.success(msg);
}
