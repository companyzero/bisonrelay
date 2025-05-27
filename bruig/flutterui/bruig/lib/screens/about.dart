import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:package_info_plus/package_info_plus.dart';
import 'package:bruig/theme_manager.dart';

class AboutScreen extends StatefulWidget {
  final bool settings;
  const AboutScreen({this.settings = false, super.key});

  @override
  State<AboutScreen> createState() => _AboutScreenState();
}

class _AboutScreenState extends State<AboutScreen> {
  String version = "";

  Future<void> getPlatform() async {
    PackageInfo packageInfo = await PackageInfo.fromPlatform();
    setState(() {
      version = packageInfo.version;
    });
  }

  @override
  void initState() {
    super.initState();
    getPlatform();
  }

  void goBack(BuildContext context) {
    Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    var theme = ThemeNotifier.of(context);

    String newVersion = "";
    try {
      var connState = ConnStateModel.of(context, listen: false);
      newVersion = connState.suggestedVersion;
    } catch (exception) {
      // Ignore because about screen may be called before there's a ConnStateModel created.
    }

    return StartupScreen(hideAboutButton: true, [
      if (newVersion != "") ...[
        Txt.H("New Software Version Available: $newVersion"),
        Txt.S("Please update to the new version"),
        const SizedBox(height: 20),
      ],
      Container(
        width: 500,
        padding: const EdgeInsets.symmetric(vertical: 60, horizontal: 15),
        decoration: BoxDecoration(
            border: Border.all(color: theme.colors.outlineVariant),
            borderRadius: const BorderRadius.all(Radius.circular(30))),
        child: Wrap(
            crossAxisAlignment: WrapCrossAlignment.center,
            alignment: WrapAlignment.spaceEvenly,
            runSpacing: 30,
            children: [
              Image.asset("assets/images/icon-cropped.png"),
              Column(children: [
                const Txt.H("Bison Relay", textAlign: TextAlign.center),
                Txt.L("Version $version"),
                const Txt.M("© 2022-2024 Company 0, LLC",
                    textAlign: TextAlign.center),
              ]),
            ]),
      ),
      if (!widget.settings) ...[
        const SizedBox(height: 30),
        LoadingScreenButton(
            empty: true, onPressed: () => goBack(context), text: "Go Back")
      ],
    ]);
  }
}
