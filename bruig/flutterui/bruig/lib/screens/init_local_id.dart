import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/buttons.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

class InitLocalIDScreen extends StatefulWidget {
  const InitLocalIDScreen({Key? key}) : super(key: key);

  @override
  InitLocalIDScreenState createState() {
    return InitLocalIDScreenState();
  }
}

class InitLocalIDScreenState extends State<InitLocalIDScreen> {
  final _formKey = GlobalKey<FormState>();
  bool connecting = false;

  String name = "";

  void connectPressed() async {
    if (connecting) return;
    if (!_formKey.currentState!.validate()) return;
    _formKey.currentState!.save();

    setState(() => connecting = true);

    try {
      await Golib.initID(IDInit(name, name));
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to connect to server: $exception');
    } finally {
      setState(() => connecting = false);
    }

    Navigator.pop(context);
  }

  @override
  Widget build(BuildContext context) {
    return StartupScreen(Consumer<ThemeNotifier>(
      builder: (context, theme, child) => Column(children: [
        const SizedBox(height: 89),
        Text("Setting up Bison Relay",
            style: TextStyle(
                color: theme.getTheme().dividerColor,
                fontSize: theme.getHugeFont(context),
                fontWeight: FontWeight.w200)),
        const SizedBox(height: 20),
        Text("Choose Username/Nick",
            style: TextStyle(
                color: theme.getTheme().focusColor,
                fontSize: theme.getLargeFont(context),
                fontWeight: FontWeight.w300)),
        const SizedBox(height: 34),
        Column(children: [
          Form(
              key: _formKey,
              child: Column(children: [
                Wrap(
                  runSpacing: 10,
                  children: <Widget>[
                    TextFormField(
                      decoration: const InputDecoration(
                          icon: Icon(Icons.person),
                          labelText: 'User Name',
                          hintText: 'Full name of the user (ex."John Doe")'),
                      onSaved: (String? v) => name = v!,
                      validator: (String? value) {
                        if (value != null && value.trim().isEmpty) {
                          return 'Cannot be blank';
                        }
                        return null;
                      },
                    ),
                    Container(height: 20),
                    Center(
                        child: LoadingScreenButton(
                      onPressed: !connecting ? connectPressed : null,
                      text: "Confirm",
                    ))
                  ],
                )
              ]))
        ]),
      ]),
    ));
  }
}
