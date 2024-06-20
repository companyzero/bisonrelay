import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

class StartupScreen extends StatelessWidget {
  final List<Widget> widgetList;
  final bool hideAboutButton;
  final Widget? fab;
  final double? childrenWidth;
  const StartupScreen(this.widgetList,
      {this.hideAboutButton = false, this.fab, this.childrenWidth, Key? key})
      : super(key: key);

  Widget _buildChildren() {
    return Column(
        mainAxisAlignment: MainAxisAlignment.center, children: widgetList);
  }

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    return Scaffold(
        body: Consumer<ThemeNotifier>(
            builder: (context, theme, child) => Container(
                decoration: const BoxDecoration(
                    image: DecorationImage(
                  alignment: Alignment.topRight,
                  fit: BoxFit.fitHeight,
                  image: AssetImage("assets/images/loading-bg.png"),
                )),
                child: Stack(children: [
                  Container(
                      alignment: Alignment.center,
                      decoration: theme.fullTheme.startupScreenBoxDecoration,
                      padding: const EdgeInsets.all(30),
                      child: SingleChildScrollView(
                          child: childrenWidth != null
                              ? SizedBox(
                                  width: childrenWidth, child: _buildChildren())
                              : _buildChildren())),
                  !hideAboutButton
                      ? Positioned(
                          top: 5,
                          left: 5,
                          child: SizedBox(
                              height: isScreenSmall ? 70 : 100,
                              width: isScreenSmall ? 70 : 100,
                              child: const Center(child: AboutButton())))
                      : const Empty(),
                  if (fab != null)
                    Positioned(right: 10, bottom: 10, child: fab!),
                ]))));
  }
}
