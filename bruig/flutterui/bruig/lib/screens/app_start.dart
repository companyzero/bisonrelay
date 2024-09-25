import 'package:flutter/material.dart';

class AppStartingLoadScreen extends StatelessWidget {
  const AppStartingLoadScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Container(
          alignment: Alignment.center, child: const Text("Loading App...")),
    );
  }
}
