import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

class VerifyServerScreen extends StatelessWidget {
  const VerifyServerScreen({super.key});

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
              const Txt.H("Accept Server Fingerprint"),
              const SizedBox(height: 34),
              Copyable(cert.innerFingerprint,
                  child: Text("Inner Fingerprint: ${cert.innerFingerprint}")),
              Copyable(cert.outerFingerprint,
                  child: Text("Outer Fingerprint: ${cert.outerFingerprint}")),
              const SizedBox(height: 34),
              OutlinedButton(
                  onPressed: () => onAcceptServerCreds(context),
                  child: const Text("Accept")),
            ]));
  }
}
