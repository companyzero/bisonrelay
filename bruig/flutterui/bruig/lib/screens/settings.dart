import 'dart:io';

import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/util.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';
import 'package:bruig/models/client.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/storage_manager.dart';
import 'package:bruig/models/menus.dart';
import 'package:bruig/components/copyable.dart';

class SettingsScreenTitle extends StatelessWidget {
  const SettingsScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Text("Settings",
            style: TextStyle(
                fontSize: theme.getLargeFont(context),
                color: Theme.of(context).focusColor)));
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
  bool notificationsEnabled = false;

  void clientUpdated() async {
    setState(() {
      connState = client.connState;
    });
  }

  void updateNotificationSettings(bool value) {
    StorageManager.saveData('notifications', value);
    setState(() {
      notificationsEnabled = value;
    });
  }

  void resetAllOldKX(BuildContext context) async {
    if (loading) return;
    setState(() => loading = true);
    try {
      await Golib.resetAllOldKX(0);
      showSuccessSnackbar(
          context, 'Requesting KX to all old KX no communicated in 30 days...');
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to reset all old KX: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  void resetAllOldKX1s(BuildContext context) async {
    if (loading) return;
    setState(() => loading = true);
    try {
      await Golib.resetAllOldKX(1);
      showSuccessSnackbar(context, 'Requesting KX to all old KX...');
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to reset all old KX: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  void pickAvatarFile() async {
    var filePickRes = await FilePicker.platform.pickFiles(
      allowMultiple: false,
      dialogTitle: "Pick avatar image file",
      type: FileType.image,
    );
    if (filePickRes == null) return;
    var fPath = filePickRes.files.first.path;
    if (fPath == null) return;
    var filePath = fPath.trim();
    var fileData = await File(filePath).readAsBytes();
    try {
      await Golib.setMyAvatar(fileData);
      client.myAvatar = MemoryImage(fileData);
    } catch (exception) {
      showErrorSnackbar(context, "Unable to set avatar: $exception");
    }
  }

  @override
  void initState() {
    super.initState();
    clientUpdated();
    client.addListener(clientUpdated);

    StorageManager.readData('notifications').then((value) {
      if (value != null) {
        setState(() {
          notificationsEnabled = value;
        });
      }
    });
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
    var canvasColor = theme.canvasColor;

    var avatarColor = colorFromNick(client.nick);
    var darkTextColor = theme.indicatorColor;
    var hightLightTextColor = theme.dividerColor; // NAME TEXT COLOR
    var avatarTextColor =
        ThemeData.estimateBrightnessForColor(avatarColor) == Brightness.dark
            ? hightLightTextColor
            : darkTextColor;

    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    if (isScreenSmall) {
      return Consumer<ThemeNotifier>(
          builder: (context, theme, _) => Scaffold(
              backgroundColor: canvasColor,
              body: ListView(
                children: [
                  ListTile(
                      title: Row(
                          mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                          children: [
                        Container(
                            margin: EdgeInsets.all(10),
                            child: InkWell(
                                onTap: pickAvatarFile,
                                child: CircleAvatar(
                                    radius: 50,
                                    backgroundColor: colorFromNick(client.nick),
                                    backgroundImage: client.myAvatar,
                                    child: client.myAvatar != null
                                        ? const Empty()
                                        : Text(client.nick[0].toUpperCase(),
                                            style: TextStyle(
                                                color: avatarTextColor,
                                                fontSize: theme
                                                    .getLargeFont(context)))))),
                        Column(children: [
                          Text(client.nick,
                              style: TextStyle(
                                  fontSize: theme.getMediumFont(context),
                                  color: textColor)),
                          SizedBox(
                              width: 150,
                              child: Copyable(
                                client.publicID,
                                TextStyle(
                                    fontSize: theme.getSmallFont(context),
                                    color: textColor),
                                textOverflow: TextOverflow.ellipsis,
                              ))
                        ])
                      ])),
                  ListTile(
                      onTap: () {
                        print("account");
                      },
                      hoverColor: backgroundColor,
                      leading: const Icon(Icons.person_outline),
                      title: Text("Account",
                          style: TextStyle(
                              fontSize: theme.getMediumFont(context),
                              color: textColor))),
                  ListTile(
                      onTap: () {
                        print("account");
                      },
                      hoverColor: backgroundColor,
                      leading: const Icon(Icons.brightness_medium_outlined),
                      title: Text("Appearance",
                          style: TextStyle(
                              fontSize: theme.getMediumFont(context),
                              color: textColor))),
                  ListTile(
                      onTap: () {
                        print("account");
                      },
                      hoverColor: backgroundColor,
                      leading: const Icon(Icons.notifications_outlined),
                      title: Text("Notifications",
                          style: TextStyle(
                              fontSize: theme.getMediumFont(context),
                              color: textColor))),
                  ListTile(
                      onTap: () {
                        print("account");
                      },
                      hoverColor: backgroundColor,
                      leading: const SidebarSvgIcon(
                          "assets/icons/icons-menu-lnmng.svg"),
                      title: Text("LN Management",
                          style: TextStyle(
                              fontSize: theme.getMediumFont(context),
                              color: textColor))),
                  ListTile(
                      onTap: () {
                        print("account");
                      },
                      hoverColor: backgroundColor,
                      leading: const SidebarSvgIcon(
                          "assets/icons/icons-menu-files.svg"),
                      title: Text("Manage Content",
                          style: TextStyle(
                              fontSize: theme.getMediumFont(context),
                              color: textColor))),
                  ListTile(
                      onTap: () {
                        print("account");
                      },
                      hoverColor: backgroundColor,
                      leading: const SidebarSvgIcon(
                          "assets/icons/icons-menu-stats.svg"),
                      title: Text("Stats",
                          style: TextStyle(
                              fontSize: theme.getMediumFont(context),
                              color: textColor))),
                  ListTile(
                      onTap: () {
                        print("account");
                      },
                      hoverColor: backgroundColor,
                      leading: const Icon(Icons.list_outlined),
                      title: Text("Logs",
                          style: TextStyle(
                              fontSize: theme.getMediumFont(context),
                              color: textColor))),
                  ListTile(
                      onTap: () {
                        print("about");
                      },
                      hoverColor: backgroundColor,
                      leading: Icon(Icons.question_mark_outlined),
                      title: Text("About Bison Relay",
                          style: TextStyle(
                              fontSize: theme.getMediumFont(context),
                              color: textColor))),
                ],
              )));
    }
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
          Tooltip(
              message: "User avatar. Click to select a new image.",
              child: InkWell(
                  onTap: pickAvatarFile,
                  child: CircleAvatar(
                    radius: 50,
                    backgroundColor: colorFromNick(client.nick),
                    backgroundImage: client.myAvatar,
                    child: client.myAvatar != null
                        ? const Empty()
                        : Text(client.nick[0].toUpperCase(),
                            style: TextStyle(
                                color: avatarTextColor,
                                fontSize: theme.getLargeFont(context))),
                  ))),
          Row(mainAxisAlignment: MainAxisAlignment.end, children: [
            Text("Notifications",
                style: TextStyle(
                    fontSize: theme.getLargeFont(context),
                    color: Theme.of(context).focusColor)),
            const SizedBox(width: 50),
            Switch(
                // thumb color (round icon)
                activeColor: Theme.of(context).focusColor,
                activeTrackColor: Theme.of(context).highlightColor,
                inactiveThumbColor: Theme.of(context).indicatorColor,
                inactiveTrackColor: Theme.of(context).dividerColor,
                //splashRadius: 20.0,
                // boolean variable value
                value: notificationsEnabled,
                // changes the state of the switch
                onChanged: (value) =>
                    setState(() => updateNotificationSettings(value))),
          ]),
          const SizedBox(height: 20),
          ElevatedButton(
            onPressed: () => loading ? null : resetAllOldKX(context),
            child: const Text(
              "Reset all Older than 30d KX",
            ),
          ),
          const SizedBox(height: 20),
          ElevatedButton(
            onPressed: () => loading ? null : resetAllOldKX1s(context),
            child: const Text(
              "Reset ALL KX",
            ),
          ),
          const SizedBox(height: 50),
          Column(mainAxisAlignment: MainAxisAlignment.spaceEvenly, children: [
            Row(mainAxisAlignment: MainAxisAlignment.spaceEvenly, children: [
              Expanded(
                  child: Container(
                margin: const EdgeInsets.only(left: 10, right: 10),
                decoration: const BoxDecoration(
                    borderRadius: BorderRadius.all(Radius.circular(5.0)),
                    boxShadow: [
                      BoxShadow(
                          color: Colors.black38,
                          offset: Offset(0.0, 2.0),
                          blurRadius: 10)
                    ]),
                child: Slider(
                  value: theme.getFontCoef(),
                  activeColor: Colors.white,
                  inactiveColor: Colors.white,
                  onChanged: (double s) => theme.setFontSize(s),
                  divisions: 3,
                  min: 0.0,
                  max: 3.0,
                ),
              )),
            ]),
            const SizedBox(height: 20),
            Row(mainAxisAlignment: MainAxisAlignment.spaceEvenly, children: [
              Text('Small', style: TextStyle(fontSize: 13, color: textColor)),
              const SizedBox(width: 35),
              Text('Normal', style: TextStyle(fontSize: 15, color: textColor)),
              const SizedBox(width: 20),
              Text('Large', style: TextStyle(fontSize: 17, color: textColor)),
              const SizedBox(width: 10),
              Text('Extra Large',
                  style: TextStyle(fontSize: 19, color: textColor)),
            ]),
            const SizedBox(height: 20),
            Container(
                margin: const EdgeInsets.only(left: 20),
                child: Row(children: [
                  Text('Current Font Size: ',
                      style: TextStyle(fontSize: 12, color: textColor)),
                  const SizedBox(width: 30),
                  Column(children: [
                    Text('Small Text',
                        style: TextStyle(
                            fontSize: theme.getSmallFont(context),
                            color: textColor)),
                    const SizedBox(height: 20),
                    Text('Medium Text',
                        style: TextStyle(
                            fontSize: theme.getMediumFont(context),
                            color: textColor)),
                    const SizedBox(height: 20),
                    Text('Large Text',
                        style: TextStyle(
                            fontSize: theme.getLargeFont(context),
                            color: textColor)),
                    const SizedBox(height: 20),
                    Text('Huge Text',
                        style: TextStyle(
                            fontSize: theme.getHugeFont(context),
                            color: textColor)),
                    const SizedBox(height: 20),
                  ])
                ])),
          ]),
          const SizedBox(height: 20),
        ]),
      ),
    );
  }
}
