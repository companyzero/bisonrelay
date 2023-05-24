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
    var backgroundColor = const Color(0xFF19172C);
    var cardColor = const Color(0xFF05031A);
    var textColor = const Color(0xFF8E8D98);
    return Scaffold(
        body: Container(
            color: backgroundColor,
            child: Stack(children: [
              Container(
                  decoration: const BoxDecoration(
                      image: DecorationImage(
                          fit: BoxFit.fill,
                          image: AssetImage("assets/images/loading-bg.png")))),
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
                    const SizedBox(height: 258),
                    Text("Accept Server Fingerprint",
                        style: TextStyle(
                            color: textColor,
                            fontSize: 34,
                            fontWeight: FontWeight.w200)),
                    const SizedBox(height: 34),
                    Text("Inner Fingerprint: ${cert.innerFingerprint}",
                        style: TextStyle(
                            color: textColor,
                            fontSize: 15,
                            fontWeight: FontWeight.w300)),
                    Text("Outer Fingerprint: ${cert.outerFingerprint}",
                        style: TextStyle(
                            color: textColor,
                            fontSize: 15,
                            fontWeight: FontWeight.w300)),
                    const SizedBox(height: 34),
                    ElevatedButton(
                        onPressed: () => onAcceptServerCreds(context),
                        child: const Text("Accept")),
                    Container(height: 10)
                  ]))
            ])));
  }
}
