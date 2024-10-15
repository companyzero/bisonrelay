import 'dart:async';
import 'dart:math';

import 'package:bruig/components/empty_widget.dart';
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

class CircularProgressButton extends StatefulWidget {
  final bool active;
  final IconData? activeIcon;
  final IconData inactiveIcon;
  final VoidCallback? onTapDown;
  final VoidCallback? onTapUp;
  final VoidCallback? onHold;
  final Duration? holdDuration;
  final double sizeMultiplier;
  const CircularProgressButton(
      {this.active = false,
      required this.inactiveIcon,
      this.activeIcon,
      this.onTapDown,
      this.onTapUp,
      this.onHold,
      this.holdDuration,
      this.sizeMultiplier = 1.0,
      super.key});

  @override
  State<CircularProgressButton> createState() => _CircularProgressButtonState();
}

class _CircularProgressButtonState extends State<CircularProgressButton> {
  double? progress;
  Timer? progressTimer;
  DateTime? progressStart;

  void updateProgress(_) {
    var now = DateTime.now();
    var elapsedMs = DateTime.now()
        .difference(progressStart ?? now)
        .inMilliseconds
        .toDouble();
    var totalMs = (widget.holdDuration?.inMilliseconds ?? 0).toDouble();
    if (totalMs == 0) {
      return;
    }
    var newProgress = min(elapsedMs / totalMs, 1.0);
    setState(() {
      progress = newProgress;
    });

    if (progress == 1) {
      progressTimer?.cancel();
      progressTimer = null;
      if (widget.onHold != null) {
        widget.onHold!();
      }
    }
  }

  void tapDown(_) {
    if (widget.onTapDown != null) {
      widget.onTapDown!();
    }
    if (widget.holdDuration != null && progressTimer == null) {
      progressStart = DateTime.now();
      progressTimer =
          Timer.periodic(const Duration(milliseconds: 80), updateProgress);
    }
  }

  void tapUp(_) {
    if (widget.onTapUp != null) {
      widget.onTapUp!();
    }

    if (progressTimer != null) {
      progressTimer?.cancel();
      progressTimer = null;
      setState(() {
        progress = null;
      });
    }
  }

  @override
  void initState() {
    super.initState();
  }

  @override
  void didUpdateWidget(covariant CircularProgressButton oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.active != widget.active ||
        oldWidget.activeIcon != widget.activeIcon ||
        oldWidget.inactiveIcon != widget.inactiveIcon ||
        oldWidget.holdDuration != widget.holdDuration) {
      setState(() {
        if (widget.holdDuration == null) {
          progress = null;
        }
      });
    }
  }

  @override
  void dispose() {
    progressTimer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    IconData icon = widget.active
        ? widget.activeIcon ?? widget.inactiveIcon
        : widget.inactiveIcon;
    return Stack(alignment: Alignment.center, children: [
      SizedBox(
          width: 40 * widget.sizeMultiplier,
          height: 40 * widget.sizeMultiplier,
          child: widget.active || progress != null
              ? CircularProgressIndicator(value: progress, strokeWidth: 2)
              : const Empty()),
      InkResponse(
          radius: 17 * widget.sizeMultiplier,
          containedInkWell: false,
          onTapDown: tapDown,
          onTapUp: tapUp,
          child: Consumer<ThemeNotifier>(
              builder: (context, theme, child) => Icon(icon,
                  size: 25 * widget.sizeMultiplier,
                  color: theme.textColor(TextColor.onSurfaceVariant))))
    ]);
  }
}
