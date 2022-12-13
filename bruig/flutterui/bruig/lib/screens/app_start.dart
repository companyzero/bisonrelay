import 'package:flutter/material.dart';

class AppStartingLoadScreen extends StatelessWidget {
  const AppStartingLoadScreen({Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var appStartBackground = theme.backgroundColor;
    return Scaffold(
      body: Container(
        color: appStartBackground,
        child: const Center(
          child: Text("Loading App..."),
        ),
      ),
    );
  }
}
