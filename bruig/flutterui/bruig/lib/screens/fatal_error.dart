import 'package:flutter/material.dart';

class FatalErrorScreen extends StatelessWidget {
  final Object? exception;
  const FatalErrorScreen({this.exception, Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var exc = exception ??
        ModalRoute.of(context)?.settings.arguments ??
        Exception("unknown exception");

    return Scaffold(
      body: Container(
        color: Colors.red,
        child: Center(
          child: Text("Fatal error: $exc"),
        ),
      ),
    );
  }
}

void runFatalErrorApp(Object exception) {
  runApp(MaterialApp(
    title: "Fatal Error",
    theme: ThemeData(
      primarySwatch: Colors.green,
    ),
    initialRoute: "/",
    routes: {
      "/": (context) => FatalErrorScreen(exception: exception),
    },
  ));
}
