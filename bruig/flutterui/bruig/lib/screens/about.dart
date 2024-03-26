import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:package_info_plus/package_info_plus.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

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
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;

    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => StartupScreen(
              about: true,
              [
                const SizedBox(height: 89),
                Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Flexible(
                        child: Align(
                            child: SizedBox(
                                width: 600,
                                height: 300,
                                child: Container(
                                    padding: const EdgeInsets.all(10),
                                    decoration: BoxDecoration(
                                      borderRadius: const BorderRadius.all(
                                          Radius.circular(30)),
                                      border: Border.all(
                                          color: theme.getTheme().focusColor),
                                    ),
                                    child: Flex(
                                        direction: isScreenSmall
                                            ? Axis.vertical
                                            : Axis.horizontal,
                                        children: [
                                          Image.asset(
                                            "assets/images/icon.png",
                                            width: isScreenSmall ? 100 : 200,
                                            height: isScreenSmall ? 100 : 200,
                                          ),
                                          Column(
                                            mainAxisAlignment:
                                                MainAxisAlignment.center,
                                            children: [
                                              Text(
                                                  textAlign: TextAlign.left,
                                                  "Bison Relay",
                                                  style: TextStyle(
                                                      color: theme
                                                          .getTheme()
                                                          .focusColor,
                                                      fontSize: theme
                                                          .getHugeFont(context),
                                                      fontWeight:
                                                          FontWeight.w200)),
                                              const SizedBox(height: 10),
                                              Text("Version $version",
                                                  style: TextStyle(
                                                      color: theme
                                                          .getTheme()
                                                          .focusColor,
                                                      fontSize:
                                                          theme.getLargeFont(
                                                              context),
                                                      fontWeight:
                                                          FontWeight.w200)),
                                              const SizedBox(height: 10),
                                              RichText(
                                                  text: TextSpan(children: [
                                                WidgetSpan(
                                                    alignment:
                                                        PlaceholderAlignment
                                                            .middle,
                                                    child: Icon(
                                                        color: theme
                                                            .getTheme()
                                                            .focusColor,
                                                        size:
                                                            theme.getMediumFont(
                                                                context),
                                                        Icons.copyright)),
                                                TextSpan(
                                                    text:
                                                        "2022-2024 Company 0, LLC",
                                                    style: TextStyle(
                                                        color: theme
                                                            .getTheme()
                                                            .focusColor,
                                                        fontSize:
                                                            theme.getLargeFont(
                                                                context),
                                                        fontWeight:
                                                            FontWeight.w200))
                                              ]))
                                            ],
                                          )
                                        ]))))),
                  ],
                ),
                const SizedBox(height: 89),
                widget.settings
                    ? const Empty()
                    : Row(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                            LoadingScreenButton(
                              empty: true,
                              onPressed: () => goBack(context),
                              text: "Go Back",
                            ),
                          ]),
              ],
            ));
  }
}
