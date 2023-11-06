import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/buttons.dart';
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
  String nick = "";

  void connectPressed() async {
    if (connecting) return;
    if (!_formKey.currentState!.validate()) return;
    _formKey.currentState!.save();

    setState(() => connecting = true);

    try {
      await Golib.initID(IDInit(name, nick));
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to connect to server: $exception');
    } finally {
      setState(() => connecting = false);
    }

    Navigator.pop(context);
  }

  @override
  Widget build(BuildContext context) {
    var backgroundColor = const Color(0xFF19172C);
    var cardColor = const Color(0xFF05031A);
    var textColor = const Color(0xFF8E8D98);
    var secondaryTextColor = const Color(0xFFE4E3E6);

    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Scaffold(
            body: Center(
                child: Container(
                    color: backgroundColor,
                    child: Stack(children: [
                      Container(
                          decoration: const BoxDecoration(
                              image: DecorationImage(
                                  fit: BoxFit.fill,
                                  image: AssetImage(
                                      "assets/images/loading-bg.png")))),
                      Container(
                        decoration: BoxDecoration(
                            gradient: LinearGradient(
                                begin: Alignment.bottomLeft,
                                end: Alignment.topRight,
                                colors: [
                              cardColor,
                              const Color(0xFF07051C),
                              backgroundColor.withOpacity(0.34),
                            ],
                                stops: const [
                              0,
                              0.17,
                              1
                            ])),
                        padding: const EdgeInsets.all(10),
                        child: Column(children: [
                          const SizedBox(height: 89),
                          Text("Setting up Bison Relay",
                              style: TextStyle(
                                  color: textColor,
                                  fontSize: theme.getHugeFont(context),
                                  fontWeight: FontWeight.w200)),
                          const SizedBox(height: 20),
                          Text("Choose Username/Nick",
                              style: TextStyle(
                                  color: secondaryTextColor,
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
                                            hintText:
                                                'Full name of the user (ex."John Doe")'),
                                        onSaved: (String? v) => name = v!,
                                        validator: (String? value) {
                                          if (value != null &&
                                              value.trim().isEmpty) {
                                            return 'Cannot be blank';
                                          }
                                          return null;
                                        },
                                      ),
                                      TextFormField(
                                        decoration: const InputDecoration(
                                            icon: Icon(Icons.person_outline),
                                            labelText: 'Nick',
                                            hintText: 'Short alias (ex."jd")'),
                                        onSaved: (String? v) => nick = v!,
                                        validator: (String? value) {
                                          if (value != null &&
                                              value.trim().isEmpty) {
                                            return 'Cannot be blank';
                                          }
                                          if (value!
                                              .contains(RegExp(r"[\W]"))) {
                                            return 'Must be a single word without special chars';
                                          }
                                          return null;
                                        },
                                      ),
                                      Container(height: 20),
                                      Center(
                                          child: LoadingScreenButton(
                                        onPressed:
                                            !connecting ? connectPressed : null,
                                        text: "Confirm",
                                      ))
                                    ],
                                  )
                                ]))
                          ]),
                        ]),
                      )
                    ])))));
  }
}
