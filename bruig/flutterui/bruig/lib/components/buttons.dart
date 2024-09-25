import 'package:bruig/components/text.dart';
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
      super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => ElevatedButton(
            style: ElevatedButton.styleFrom(
                backgroundColor: theme.colors.errorContainer),
            onPressed: !loading ? onPressed : null,
            child: Text(label,
                style: theme.textStyleFor(
                    context, null, TextColor.onErrorContainer))));
  }
}

ButtonStyle raisedButtonStyle(ThemeNotifier theme) {
  return ElevatedButton.styleFrom(
    padding: const EdgeInsets.only(left: 34, top: 10, right: 34, bottom: 10),
    minimumSize: const Size(150, 55),
    foregroundColor: theme.colors.onPrimaryContainer,
    backgroundColor: theme.colors.primaryContainer,
    //padding: EdgeInsets.symmetric(horizontal: 16),
    shape: const RoundedRectangleBorder(
      borderRadius: BorderRadius.all(Radius.circular(30)),
    ),
  );
}

ButtonStyle emptyButtonStyle(ThemeNotifier theme) {
  return ElevatedButton.styleFrom(
    padding: const EdgeInsets.only(left: 34, top: 10, right: 34, bottom: 10),
    minimumSize: const Size(150, 55),
    shape: RoundedRectangleBorder(
        borderRadius: const BorderRadius.all(Radius.circular(30)),
        side: BorderSide(color: theme.colors.outlineVariant, width: 2)),
  );
}

ButtonStyle readMoreButton(ThemeNotifier theme) {
  return ElevatedButton.styleFrom(
    padding: const EdgeInsets.only(left: 10, top: 10, right: 10, bottom: 10),
    // foregroundColor: theme.dividerColor,
    //padding: EdgeInsets.symmetric(horizontal: 16),
    shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.all(Radius.circular(30)),
        side: BorderSide(/*color: theme.indicatorColor,*/ width: 1)),
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
      super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => TextButton(
            style: minSize != 0
                ? ElevatedButton.styleFrom(
                    padding: const EdgeInsets.only(
                        left: 34, top: 10, right: 34, bottom: 10),
                    minimumSize: Size(minSize - 30, 55),
                    //padding: EdgeInsets.symmetric(horizontal: 16),
                    shape: const RoundedRectangleBorder(
                      borderRadius: BorderRadius.all(Radius.circular(30)),
                    ),
                  )
                : empty
                    ? emptyButtonStyle(theme)
                    : raisedButtonStyle(theme),
            onPressed: !loading ? onPressed : null,
            child: Txt.L(text, textAlign: TextAlign.center)));
  }
}

// Generic about button.
class AboutButton extends StatelessWidget {
  const AboutButton({super.key});
  @override
  Widget build(BuildContext context) {
    return IconButton(
        tooltip: "About Bison Relay",
        onPressed: () {
          Navigator.of(context).pushNamed("/about");
        },
        icon: Image.asset(
          fit: BoxFit.contain,
          "assets/images/icon.png",
        ));
  }
}
