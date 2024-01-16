import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/screens/overview.dart';
import 'package:flutter/material.dart';

class ServerUnwelcomeErrorScreen extends StatelessWidget {
  static String routeName = "/serverUnwelcomeError";
  const ServerUnwelcomeErrorScreen({Key? key}) : super(key: key);

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
          const Text("Client software needs upgrade",
              style: TextStyle(fontSize: 20)),
          const SizedBox(height: 10),
          Text("Reason: $errMsg"),
          const SizedBox(height: 10),
          const Text(
              "None of the actions that require a server connection will work."),
          const Expanded(child: Empty()),
          const SizedBox(height: 10),
          ElevatedButton(
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
