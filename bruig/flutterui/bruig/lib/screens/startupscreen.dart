import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

class StartupScreen extends StatelessWidget {
  final Widget? child;
  final bool? about;
  const StartupScreen(this.child, {this.about = false, Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    var mobileBG = const DecorationImage(
        fit: BoxFit.fill, image: AssetImage("assets/images/testBG.gif"));
    return Scaffold(
        body: Consumer<ThemeNotifier>(
            builder: (context, theme, child) => Container(
                color: theme.getTheme().backgroundColor,
                child: Stack(children: [
                  Container(
                      decoration: BoxDecoration(
                          image: isScreenSmall
                              ? theme.getTheme().brightness == Brightness.light
                                  ? null
                                  : mobileBG
                              : DecorationImage(
                                  fit: BoxFit.fill,
                                  image: const AssetImage(
                                      "assets/images/loading-bg.png"),
                                  opacity: theme.getTheme().brightness ==
                                          Brightness.light
                                      ? 0.25
                                      : 1))),
                  Container(
                    decoration: isScreenSmall
                        ? null
                        : BoxDecoration(
                            gradient: LinearGradient(
                                begin: Alignment.bottomLeft,
                                end: Alignment.topRight,
                                colors: [
                                theme.getTheme().canvasColor,
                                theme.getTheme().backgroundColor,
                                theme.getTheme().canvasColor.withOpacity(0.34),
                              ],
                                stops: const [
                                0,
                                0.17,
                                1
                              ])),
                    padding: const EdgeInsets.all(30),
                  ),
                  Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [Expanded(child: Center(child: this.child))]),
                  about == null || about == false
                      ? const Positioned(
                          top: 10, left: 10, child: AboutButton())
                      : const Empty(),
                ]))));
  }
}
