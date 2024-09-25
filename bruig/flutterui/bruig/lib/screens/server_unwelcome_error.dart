import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/screens/overview.dart';
import 'package:flutter/material.dart';

class ServerUnwelcomeErrorScreen extends StatelessWidget {
  static String routeName = "/serverUnwelcomeError";
  const ServerUnwelcomeErrorScreen({super.key});

  @override
  Widget build(BuildContext context) {
    var errMsg = ModalRoute.of(context)?.settings.arguments ??
        "Client/server protocol negotiation error";

    return Scaffold(
      body: Container(
        color: Colors.amber,
        child: Center(
            child: Column(children: [
          const Expanded(child: Empty()),
          const Txt.H("Client software needs upgrade"),
          const SizedBox(height: 10),
          Text("Reason: $errMsg"),
          const SizedBox(height: 10),
          const Text(
              "None of the actions that require a server connection will work."),
          const Expanded(child: Empty()),
          const SizedBox(height: 10),
          OutlinedButton(
              onPressed: () {
                Navigator.of(context)
                    .pushReplacementNamed(OverviewScreen.routeName);
              },
              child: const Text("Return to app")),
          const SizedBox(height: 10),
        ])),
      ),
    );
  }
}
