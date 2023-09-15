import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';
import 'package:bruig/models/client.dart';

class SettingsScreenTitle extends StatelessWidget {
  const SettingsScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return Text("Bison Relay / Settings",
        style: TextStyle(fontSize: 15, color: Theme.of(context).focusColor));
  }
}

class SettingsScreen extends StatefulWidget {
  final ClientModel client;
  const SettingsScreen(this.client, {Key? key}) : super(key: key);
  static String routeName = "/settings";

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  ClientModel get client => widget.client;
  ServerSessionState connState = ServerSessionState.empty();

  void clientUpdated() async {
    setState(() {
      connState = client.connState;
    });
  }

  @override
  void initState() {
    super.initState();
    clientUpdated();
    client.addListener(clientUpdated);
  }

  @override
  void didUpdateWidget(SettingsScreen oldWidget) {
    oldWidget.client.removeListener(clientUpdated);
    super.didUpdateWidget(oldWidget);
    client.addListener(clientUpdated);
  }

  @override
  void dispose() {
    client.removeListener(clientUpdated);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var backgroundColor = theme.backgroundColor;
    var textColor = theme.focusColor;

    return Consumer<ThemeNotifier>(
      builder: (context, theme, _) => Container(
        margin: const EdgeInsets.all(1),
        decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(3), color: backgroundColor),
        padding: const EdgeInsets.all(10),
        child: Column(children: [
          /*
          // XXX HIDING THEME BUTTON UNTIL LIGHT THEME PROVIDED
          ElevatedButton(
            onPressed: () => {
              theme.getTheme().brightness == Brightness.dark
                  ? theme.setLightMode()
                  : theme.setDarkMode(),
            },
            child: theme.getTheme().brightness == Brightness.dark
                ? Text('Set Light Theme')
                : Text('Set Dark Theme'),
          ),
          */
          ElevatedButton(
            onPressed: () => client.requestResetKXAllOld(),
            child: const Text("Reset all Old KX"),
          ),
          Column(mainAxisAlignment: MainAxisAlignment.spaceEvenly, children: [
            Text('Font Size',
                style: TextStyle(
                    fontSize: 8 + theme.getFontSize() * 7, color: textColor)),
            Row(mainAxisAlignment: MainAxisAlignment.spaceEvenly, children: [
              const SizedBox(width: 50),
              Expanded(
                  child: Slider(
                divisions: 3,
                min: 1,
                max: 4,
                value: theme.getFontSize(),
                onChanged: (double value) {
                  setState(() {
                    if (value < theme.defaultFontSize) {
                      theme.setSmallFontMode();
                    } else if (value >= theme.defaultFontSize &&
                        value < theme.largeFontSize) {
                      theme.setDefaultFontMode();
                    } else if (value >= theme.largeFontSize &&
                        value < theme.hugeFontSize) {
                      theme.setLargeFontMode();
                    } else if (value >= theme.largeFontSize) {
                      theme.setHugeFontMode();
                    }
                  });
                },
              )),
              const SizedBox(width: 50),
            ]),
            Row(mainAxisAlignment: MainAxisAlignment.spaceEvenly, children: [
              Text('Small',
                  style: TextStyle(
                      fontSize: 8 + theme.smallFontSize * 7, color: textColor)),
              Text('Normal',
                  style: TextStyle(
                      fontSize: 8 + theme.defaultFontSize * 7,
                      color: textColor)),
              Text('Large',
                  style: TextStyle(
                      fontSize: 8 + theme.largeFontSize * 7, color: textColor)),
              Text('Huge',
                  style: TextStyle(
                      fontSize: 8 + theme.hugeFontSize * 7, color: textColor)),
            ]),
          ]),
          const SizedBox(height: 20),
        ]),
      ),
    );
  }
}
