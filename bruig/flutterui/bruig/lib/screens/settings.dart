import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';
import 'package:bruig/models/client.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:bruig/components/snackbars.dart';

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
  bool loading = false;

  void clientUpdated() async {
    setState(() {
      connState = client.connState;
    });
  }

  void resetAllOldKX(BuildContext context) async {
    if (loading) return;
    setState(() => loading = true);
    try {
      await Golib.resetAllOldKX(0);
      showSuccessSnackbar(context, 'Requesting KX to all old KX...');
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to reset all old KX: $exception');
    } finally {
      setState(() => loading = false);
    }
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
          const SizedBox(height: 20),
          ElevatedButton(
            onPressed: () => loading ? null : resetAllOldKX(context),
            child: const Text("Reset all Old KX"),
          ),
          const SizedBox(height: 50),
          Column(mainAxisAlignment: MainAxisAlignment.spaceEvenly, children: [
            Row(mainAxisAlignment: MainAxisAlignment.spaceEvenly, children: [
              Expanded(
                  child: Container(
                margin: EdgeInsets.only(left: 10, right: 10),
                decoration: new BoxDecoration(
                    borderRadius:
                        new BorderRadius.all(new Radius.circular(5.0)),
                    boxShadow: [
                      new BoxShadow(
                          color: Colors.black38,
                          offset: new Offset(0.0, 2.0),
                          blurRadius: 10)
                    ]),
                child: Slider(
                  value: theme.getFontCoef(),
                  activeColor: Colors.white,
                  inactiveColor: Colors.white,
                  onChanged: (double s) => theme.setFontSize(s),
                  divisions: 3,
                  min: 1.0,
                  max: 4.0,
                ),
              )),
            ]),
            const SizedBox(height: 20),
            Row(mainAxisAlignment: MainAxisAlignment.spaceEvenly, children: [
              Text('Small',
                  style: TextStyle(fontSize: 12 * 1, color: textColor)),
              const SizedBox(width: 35),
              Text('Normal',
                  style: TextStyle(fontSize: 12 * 2, color: textColor)),
              const SizedBox(width: 20),
              Text('Large',
                  style: TextStyle(fontSize: 12 * 3, color: textColor)),
              const SizedBox(width: 10),
              Text('Huge',
                  style: TextStyle(fontSize: 12 * 4, color: textColor)),
            ]),
            const SizedBox(height: 20),
            Container(
                margin: const EdgeInsets.only(left: 20),
                height: 50,
                child: Row(children: [
                  Text('Current Font Size: ',
                      style: TextStyle(fontSize: 12, color: textColor)),
                  const SizedBox(width: 20),
                  Text('Bison Relay',
                      style: TextStyle(
                          fontSize: theme.getFontSize(), color: textColor)),
                ])),
          ]),
          const SizedBox(height: 20),
        ]),
      ),
    );
  }
}
