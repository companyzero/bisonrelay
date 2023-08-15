import 'package:flutter/material.dart';

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

final ButtonStyle raisedButtonStyle = ElevatedButton.styleFrom(
  padding: const EdgeInsets.only(left: 34, top: 10, right: 34, bottom: 10),
  minimumSize: const Size(150, 55),
  foregroundColor: const Color(0xFFE4E3E6),
  backgroundColor: const Color(0xFF252438),
  //padding: EdgeInsets.symmetric(horizontal: 16),
  shape: const RoundedRectangleBorder(
    borderRadius: BorderRadius.all(Radius.circular(30)),
  ),
);

final ButtonStyle emptyButtonStyle = ElevatedButton.styleFrom(
  padding: const EdgeInsets.only(left: 34, top: 10, right: 34, bottom: 10),
  minimumSize: const Size(150, 55),
  foregroundColor: const Color(0xFFE4E3E6),
  //padding: EdgeInsets.symmetric(horizontal: 16),
  shape: const RoundedRectangleBorder(
      borderRadius: BorderRadius.all(Radius.circular(30)),
      side: BorderSide(color: Color(0xFF5A5968), width: 2)),
);

final ButtonStyle readMoreButton = ElevatedButton.styleFrom(
  padding: const EdgeInsets.only(left: 10, top: 10, right: 10, bottom: 10),
  foregroundColor: const Color(0xFF8E8D98),
  //padding: EdgeInsets.symmetric(horizontal: 16),
  shape: const RoundedRectangleBorder(
      borderRadius: BorderRadius.all(Radius.circular(30)),
      side: BorderSide(color: Color(0xFF5A5968), width: 1)),
);

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
    return TextButton(
        style: minSize != 0
            ? ElevatedButton.styleFrom(
                padding: const EdgeInsets.only(
                    left: 34, top: 10, right: 34, bottom: 10),
                minimumSize: Size(minSize - 30, 55),
                foregroundColor: const Color(0xFFE4E3E6),
                backgroundColor: const Color(0xFF252438),
                //padding: EdgeInsets.symmetric(horizontal: 16),
                shape: const RoundedRectangleBorder(
                  borderRadius: BorderRadius.all(Radius.circular(30)),
                ),
              )
            : empty
                ? emptyButtonStyle
                : raisedButtonStyle,
        onPressed: !loading ? onPressed : null,
        child: Text(text,
            style:
                const TextStyle(fontSize: 21, fontWeight: FontWeight.normal)));
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
    return TextButton(
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
                ? emptyButtonStyle
                : raisedButtonStyle,
        onPressed: !loading ? onPressed : null,
        child: Text(text,
            style: const TextStyle(
                letterSpacing: 1, fontSize: 13, fontWeight: FontWeight.w500)));
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
    return TextButton(
        style: readMoreButton,
        onPressed: !loading ? onPressed : null,
        child:
            Text(text, style: const TextStyle(letterSpacing: 1, fontSize: 12)));
  }
}
