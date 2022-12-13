import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';

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
    return Scaffold(
        body: Center(
            child: Container(
                padding: const EdgeInsets.all(40),
                constraints: const BoxConstraints(maxWidth: 800),
                child: Column(children: [
                  Text('''Inner Fingerprint: ${cert.innerFingerprint}

Outer Fingerprint: ${cert.outerFingerprint}'''),
                  Container(height: 50),
                  ElevatedButton(
                      onPressed: () => onAcceptServerCreds(context),
                      child: const Text("Accept"))
                ]))));
  }
}
