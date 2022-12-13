import 'package:flutter/material.dart';

class RouteErrorPage extends StatelessWidget {
  final String routeName;
  final String replacement;
  const RouteErrorPage(this.routeName, this.replacement, {super.key});

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var backgroundColor = theme.backgroundColor;
    var textColor = theme.focusColor;
    return Container(
      color: backgroundColor,
      child: Column(
        children: [
          Text("Route $routeName not found",
              style: TextStyle(color: textColor, fontSize: 20)),
          ElevatedButton(
              onPressed: () {
                Navigator.of(context, rootNavigator: true)
                    .pushReplacementNamed(replacement);
              },
              child: const Text("Back"))
        ],
      ),
    );
  }
}
