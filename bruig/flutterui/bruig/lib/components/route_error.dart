import 'package:flutter/material.dart';

class RouteErrorPage extends StatelessWidget {
  final String routeName;
  final String replacement;
  const RouteErrorPage(this.routeName, this.replacement, {super.key});

  @override
  Widget build(BuildContext context) {
    return Column(children: [
      Text("Route $routeName not found"),
      OutlinedButton(
          onPressed: () {
            Navigator.of(context, rootNavigator: true)
                .pushReplacementNamed(replacement);
          },
          child: const Text("Back"))
    ]);
  }
}
