import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/text.dart';
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
    return Consumer<ThemeNotifier>(
      builder: (context, theme, child) => StartupScreen(childrenWidth: 500, [
        const SizedBox(height: 89),
        const Txt.H("Setting up Bison Relay"),
        const SizedBox(height: 20),
        const Txt.L("Choose Username/Nick"),
        const SizedBox(height: 34),
        Form(
            key: _formKey,
            child: TextFormField(
              decoration: const InputDecoration(
                  icon: Icon(Icons.person),
                  labelText: 'User Name',
                  hintText: 'Nick or alias of user (ex."john10")'),
              onSaved: (String? v) => name = v!,
              validator: (String? value) {
                if (value != null && value.trim().isEmpty) {
                  return 'Cannot be blank';
                }
                return null;
              },
            )),
        const SizedBox(height: 30),
        LoadingScreenButton(
          onPressed: !connecting ? connectPressed : null,
          text: "Confirm",
        ),
      ]),
    );
  }
}
