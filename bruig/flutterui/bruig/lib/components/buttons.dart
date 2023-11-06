import 'package:flutter/material.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

class CancelButton extends StatelessWidget {
  final VoidCallback? onPressed;
  final bool loading;
  final String label;
  const CancelButton(
      {required this.onPressed,
      this.loading = false,
      this.label = "Cancel",
      Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    var errorColor = Theme.of(context).errorColor; // ERROR COLOR;
    return ElevatedButton(
        style: ElevatedButton.styleFrom(backgroundColor: errorColor),
        onPressed: !loading ? onPressed : null,
        child: Text(label));
  }
}

ButtonStyle raisedButtonStyle(ThemeData theme) {
  return ElevatedButton.styleFrom(
    padding: const EdgeInsets.only(left: 34, top: 10, right: 34, bottom: 10),
    minimumSize: const Size(150, 55),
    foregroundColor: theme.focusColor,
    backgroundColor: theme.highlightColor,
    //padding: EdgeInsets.symmetric(horizontal: 16),
    shape: const RoundedRectangleBorder(
      borderRadius: BorderRadius.all(Radius.circular(30)),
    ),
  );
}

ButtonStyle emptyButtonStyle(ThemeData theme) {
  return ElevatedButton.styleFrom(
    padding: const EdgeInsets.only(left: 34, top: 10, right: 34, bottom: 10),
    minimumSize: const Size(150, 55),
    foregroundColor: theme.focusColor,
    //padding: EdgeInsets.symmetric(horizontal: 16),
    shape: RoundedRectangleBorder(
        borderRadius: const BorderRadius.all(Radius.circular(30)),
        side: BorderSide(color: theme.indicatorColor, width: 2)),
  );
}

ButtonStyle readMoreButton(ThemeData theme) {
  return ElevatedButton.styleFrom(
    padding: const EdgeInsets.only(left: 10, top: 10, right: 10, bottom: 10),
    foregroundColor: const Color(0xFF8E8D98),
    //padding: EdgeInsets.symmetric(horizontal: 16),
    shape: RoundedRectangleBorder(
        borderRadius: const BorderRadius.all(Radius.circular(30)),
        side: BorderSide(color: theme.indicatorColor, width: 1)),
  );
}

class LoadingScreenButton extends StatelessWidget {
  final VoidCallback? onPressed;
  final bool loading;
  final String text;
  final bool empty;
  final double minSize;
  const LoadingScreenButton(
      {required this.onPressed,
      required this.text,
      this.loading = false,
      this.empty = false,
      this.minSize = 0,
      Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => TextButton(
            style: minSize != 0
                ? ElevatedButton.styleFrom(
                    padding: const EdgeInsets.only(
                        left: 34, top: 10, right: 34, bottom: 10),
                    minimumSize: Size(minSize - 30, 55),
                    foregroundColor: theme.getTheme().focusColor,
                    backgroundColor: theme.getTheme().highlightColor,
                    //padding: EdgeInsets.symmetric(horizontal: 16),
                    shape: const RoundedRectangleBorder(
                      borderRadius: BorderRadius.all(Radius.circular(30)),
                    ),
                  )
                : empty
                    ? emptyButtonStyle(theme.getTheme())
                    : raisedButtonStyle(theme.getTheme()),
            onPressed: !loading ? onPressed : null,
            child: Text(text,
                style: TextStyle(
                    fontSize: theme.getLargeFont(context),
                    fontWeight: FontWeight.normal))));
  }
}

class MobileScreenButton extends StatelessWidget {
  final VoidCallback? onPressed;
  final bool loading;
  final String text;
  final bool empty;
  final double minSize;
  const MobileScreenButton(
      {required this.onPressed,
      required this.text,
      this.loading = false,
      this.empty = false,
      this.minSize = 0,
      Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var buttonForeground = theme.backgroundColor;
    var buttonBackground = theme.bottomAppBarColor;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => TextButton(
            style: minSize != 0
                ? ElevatedButton.styleFrom(
                    padding: const EdgeInsets.only(
                        left: 34, top: 13, right: 34, bottom: 13),
                    minimumSize: Size(minSize - 46, 20),
                    foregroundColor: buttonForeground,
                    backgroundColor: buttonBackground,
                    shape: const RoundedRectangleBorder(
                      borderRadius: BorderRadius.all(Radius.circular(30)),
                    ),
                  )
                : empty
                    ? emptyButtonStyle(theme.getTheme())
                    : raisedButtonStyle(theme.getTheme()),
            onPressed: !loading ? onPressed : null,
            child: Text(text,
                style: TextStyle(
                    letterSpacing: 1,
                    fontSize: theme.getMediumFont(context),
                    fontWeight: FontWeight.w500))));
  }
}

class FeedReadMoreButton extends StatelessWidget {
  final VoidCallback? onPressed;
  final bool loading;
  final String text;
  final bool empty;
  final double minSize;
  const FeedReadMoreButton(
      {required this.onPressed,
      required this.text,
      this.loading = false,
      this.empty = false,
      this.minSize = 0,
      Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => TextButton(
            style: readMoreButton(theme.getTheme()),
            onPressed: !loading ? onPressed : null,
            child: Text(text,
                style: TextStyle(
                    letterSpacing: 1,
                    fontSize: theme.getMediumFont(context)))));
  }
}
