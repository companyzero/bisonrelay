import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

class VerifyServerScreen extends StatelessWidget {
  const VerifyServerScreen({Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    final cert = ModalRoute.of(context)!.settings.arguments as ServerCert;

    return _VerifyServerScreen(cert);
  }
}

class _VerifyServerScreen extends StatelessWidget {
  final ServerCert cert;
  const _VerifyServerScreen(this.cert);

  void onAcceptServerCreds(context) {
    Golib.replyConfServerCert(true);
    Navigator.pop(context);
  }

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => StartupScreen([
              Text("Accept Server Fingerprint",
                  style: TextStyle(
                      color: theme.getTheme().dividerColor,
                      fontSize: theme.getHugeFont(context),
                      fontWeight: FontWeight.w200)),
              const SizedBox(height: 34),
              Text("Inner Fingerprint: ${cert.innerFingerprint}",
                  style: TextStyle(
                      color: theme.getTheme().dividerColor,
                      fontSize: theme.getMediumFont(context),
                      fontWeight: FontWeight.w300)),
              Text("Outer Fingerprint: ${cert.outerFingerprint}",
                  style: TextStyle(
                      color: theme.getTheme().dividerColor,
                      fontSize: theme.getMediumFont(context),
                      fontWeight: FontWeight.w300)),
              const SizedBox(height: 34),
              ElevatedButton(
                  onPressed: () => onAcceptServerCreds(context),
                  child: const Text("Accept")),
              Container(height: 10)
            ]));
  }
}
