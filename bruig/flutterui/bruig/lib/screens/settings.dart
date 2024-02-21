import 'dart:io';

import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/screens/ln_management.dart';
import 'package:bruig/screens/log.dart';
import 'package:bruig/screens/manage_content/manage_content.dart';
import 'package:bruig/screens/paystats.dart';
import 'package:bruig/screens/about.dart';
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

typedef ChangePageCB = void Function(String);
typedef NotficationsCB = void Function(bool);
typedef ResetKXCB = void Function(BuildContext);

class SettingsScreenTitle extends StatelessWidget {
  const SettingsScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer2<ClientModel, ThemeNotifier>(
        builder: (context, client, theme, child) => Text(
            client.settingsPageTitle,
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
  String settingsPage = "main";

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

  void changePage(String newPage) {
    setState(() {
      client.settingsPageTitle = newPage;
      settingsPage = newPage;
    });
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
    WidgetsBinding.instance
        .addPostFrameCallback((_) => client.settingsPageTitle = "Settings");
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
    Widget settingsView =
        MainSettingsScreen(client, pickAvatarFile, changePage);
    switch (settingsPage) {
      case "Account":
        settingsView =
            AccountSettingsScreen(client, resetAllOldKX1s, resetAllOldKX);
        break;
      case "Appearance":
        settingsView = Consumer<ThemeNotifier>(
            builder: (context, theme, _) =>
                AppearanceSettingsScreen(client, theme));
        break;
      case "Notifications":
        settingsView = NotificationsSettingsScreen(
            client, updateNotificationSettings, notificationsEnabled);
        break;
      case "About":
        settingsView = const AboutScreen(settings: true);
        break;
      default:
        break;
    }
    if (isScreenSmall) {
      return Consumer<ThemeNotifier>(
          builder: (context, theme, _) => Scaffold(
                backgroundColor: canvasColor,
                body: settingsView,
              ));
    }
    return Consumer<ThemeNotifier>(
      builder: (context, theme, _) => Container(
        margin: const EdgeInsets.all(1),
        decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(3), color: backgroundColor),
        padding: const EdgeInsets.all(10),
        child: Column(children: [
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

class MainSettingsScreen extends StatelessWidget {
  final ClientModel client;
  final VoidCallback pickAvatarFile;
  final ChangePageCB changePage;
  const MainSettingsScreen(this.client, this.pickAvatarFile, this.changePage,
      {Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var backgroundColor = theme.backgroundColor;
    var textColor = theme.focusColor;

    var avatarColor = colorFromNick(client.nick);
    var darkTextColor = theme.indicatorColor;
    var hightLightTextColor = theme.dividerColor; // NAME TEXT COLOR
    var avatarTextColor =
        ThemeData.estimateBrightnessForColor(avatarColor) == Brightness.dark
            ? hightLightTextColor
            : darkTextColor;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => ListView(
              children: [
                ListTile(
                    title: Row(
                        mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                        children: [
                      Container(
                          margin: const EdgeInsets.all(10),
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
                    onTap: () => changePage("Account"),
                    hoverColor: backgroundColor,
                    leading: const Icon(Icons.person_outline),
                    title: Text("Account",
                        style: TextStyle(
                            fontSize: theme.getMediumFont(context),
                            color: textColor))),
                ListTile(
                    onTap: () => changePage("Appearance"),
                    hoverColor: backgroundColor,
                    leading: const Icon(Icons.brightness_medium_outlined),
                    title: Text("Appearance",
                        style: TextStyle(
                            fontSize: theme.getMediumFont(context),
                            color: textColor))),
                ListTile(
                    onTap: () => changePage("Notifications"),
                    hoverColor: backgroundColor,
                    leading: const Icon(Icons.notifications_outlined),
                    title: Text("Notifications",
                        style: TextStyle(
                            fontSize: theme.getMediumFont(context),
                            color: textColor))),
                ListTile(
                    onTap: () {
                      Navigator.of(context)
                          .pushReplacementNamed(LNScreen.routeName);
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
                      Navigator.of(context)
                          .pushReplacementNamed(ManageContent.routeName);
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
                      Navigator.of(context)
                          .pushReplacementNamed(PayStatsScreen.routeName);
                    },
                    hoverColor: backgroundColor,
                    leading: const SidebarSvgIcon(
                        "assets/icons/icons-menu-stats.svg"),
                    title: Text("Payment Stats",
                        style: TextStyle(
                            fontSize: theme.getMediumFont(context),
                            color: textColor))),
                ListTile(
                    onTap: () {
                      Navigator.of(context)
                          .pushReplacementNamed(LogScreen.routeName);
                    },
                    hoverColor: backgroundColor,
                    leading: const Icon(Icons.list_outlined),
                    title: Text("Logs",
                        style: TextStyle(
                            fontSize: theme.getMediumFont(context),
                            color: textColor))),
                ListTile(
                    onTap: () => changePage("About"),
                    hoverColor: backgroundColor,
                    leading: const Icon(Icons.question_mark_outlined),
                    title: Text("About Bison Relay",
                        style: TextStyle(
                            fontSize: theme.getMediumFont(context),
                            color: textColor))),
              ],
            ));
  }
}

class AccountSettingsScreen extends StatelessWidget {
  final ClientModel client;
  final ResetKXCB resetAllKXCB;
  final ResetKXCB resetKXCB;
  const AccountSettingsScreen(this.client, this.resetAllKXCB, this.resetKXCB,
      {Key? key})
      : super(key: key);

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
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => ListView(children: [
              ListTile(
                title: Text("Reset all KX",
                    style: TextStyle(
                        fontSize: theme.getLargeFont(context),
                        color: textColor)),
                onTap: () => resetAllKXCB(context),
              ),
              ListTile(
                title: Text("Reset KX from users 30d stale",
                    style: TextStyle(
                        fontSize: theme.getLargeFont(context),
                        color: textColor)),
                onTap: () => resetKXCB(context),
              )
            ]));
  }
}

enum FontChoices { small, medium, large, xlarge }

enum ThemeChoices { light, dark, system }

class AppearanceSettingsScreen extends StatefulWidget {
  final ClientModel client;
  final ThemeNotifier theme;
  const AppearanceSettingsScreen(this.client, this.theme, {Key? key})
      : super(key: key);
  @override
  State<AppearanceSettingsScreen> createState() =>
      _AppearanceSettingsScreenState();
}

/// This is the private State class that goes with MyStatefulWidget.
class _AppearanceSettingsScreenState extends State<AppearanceSettingsScreen> {
  ClientModel get client => widget.client;
  ThemeNotifier get theme => widget.theme;
  FontChoices _fontChoices = FontChoices.medium;
  ThemeChoices _themeChoices = ThemeChoices.dark;

  @override
  void initState() {
    setState(() {
      if (theme.getThemeMode() == "dark") {
        _themeChoices = ThemeChoices.dark;
      } else if (theme.getThemeMode() == "system") {
        _themeChoices = ThemeChoices.system;
      } else if (theme.getThemeMode() == "light") {
        _themeChoices = ThemeChoices.system;
      }
    });
    super.initState();
  }

  @override
  void didUpdateWidget(AppearanceSettingsScreen oldWidget) {
    setState(() {
      if (theme.getThemeMode() == "dark") {
        _themeChoices = ThemeChoices.dark;
      } else if (theme.getThemeMode() == "system") {
        _themeChoices = ThemeChoices.system;
      } else if (theme.getThemeMode() == "light") {
        _themeChoices = ThemeChoices.light;
      }
    });
    super.didUpdateWidget(oldWidget);
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var backgroundColor = theme.backgroundColor;
    var textColor = theme.focusColor;

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => ListView(
              children: [
                ListTile(
                    onTap: () => showDialog(
                          context: context,
                          builder: (BuildContext context) {
                            return Dialog(
                                shadowColor: backgroundColor,
                                insetPadding: const EdgeInsets.symmetric(
                                    horizontal: 40.0, vertical: 275.0),
                                backgroundColor: backgroundColor,
                                shape: const RoundedRectangleBorder(
                                    borderRadius: BorderRadius.all(
                                        Radius.circular(16.0))),
                                child: Container(
                                    margin: const EdgeInsets.all(20),
                                    child: Column(children: [
                                      Row(children: [
                                        Text("Theme",
                                            style: TextStyle(
                                                fontSize:
                                                    theme.getLargeFont(context),
                                                color: textColor)),
                                      ]),
                                      ListTile(
                                          title: const Text('Light'),
                                          leading: Radio<ThemeChoices>(
                                            value: ThemeChoices.light,
                                            groupValue: _themeChoices,
                                            onChanged: (ThemeChoices? value) {
                                              setState(() {
                                                _themeChoices = value!;
                                                theme.setLightMode();
                                                Navigator.of(context).pop();
                                              });
                                            },
                                          )),
                                      ListTile(
                                          title: const Text('Dark'),
                                          leading: Radio<ThemeChoices>(
                                            value: ThemeChoices.dark,
                                            groupValue: _themeChoices,
                                            onChanged: (ThemeChoices? value) {
                                              setState(() {
                                                _themeChoices = value!;
                                                theme.setDarkMode();
                                                Navigator.of(context).pop();
                                              });
                                            },
                                          )),
                                      ListTile(
                                          title:
                                              const Text('Use System Default'),
                                          leading: Radio<ThemeChoices>(
                                            value: ThemeChoices.system,
                                            groupValue: _themeChoices,
                                            onChanged: (ThemeChoices? value) {
                                              setState(() {
                                                _themeChoices = value!;
                                                theme.setSystemMode();
                                                Navigator.of(context).pop();
                                              });
                                            },
                                          )),
                                    ])));
                          },
                        ),
                    hoverColor: backgroundColor,
                    leading: const Icon(Icons.person_outline),
                    title: Text("Theme",
                        style: TextStyle(
                            fontSize: theme.getMediumFont(context),
                            color: textColor))),
                ListTile(
                    onTap: () => showDialog(
                          context: context,
                          builder: (BuildContext context) {
                            return Dialog(
                                shadowColor: backgroundColor,
                                insetPadding: const EdgeInsets.symmetric(
                                    horizontal: 40.0, vertical: 255.0),
                                backgroundColor: backgroundColor,
                                shape: const RoundedRectangleBorder(
                                    borderRadius: BorderRadius.all(
                                        Radius.circular(16.0))),
                                child: Container(
                                    margin: const EdgeInsets.all(20),
                                    child: Column(children: [
                                      Row(children: [
                                        Text("Message font size",
                                            style: TextStyle(
                                                fontSize:
                                                    theme.getLargeFont(context),
                                                color: textColor)),
                                      ]),
                                      ListTile(
                                          title: const Text('Small'),
                                          leading: Radio<FontChoices>(
                                            value: FontChoices.small,
                                            groupValue: _fontChoices,
                                            onChanged: (FontChoices? value) {
                                              setState(() {
                                                _fontChoices = value!;
                                                theme.setFontSize(1);
                                                Navigator.of(context).pop();
                                              });
                                            },
                                          )),
                                      ListTile(
                                          title: const Text('Medium'),
                                          leading: Radio<FontChoices>(
                                            value: FontChoices.medium,
                                            groupValue: _fontChoices,
                                            onChanged: (FontChoices? value) {
                                              setState(() {
                                                _fontChoices = value!;
                                                theme.setFontSize(2);
                                                Navigator.of(context).pop();
                                              });
                                            },
                                          )),
                                      ListTile(
                                          title: const Text('Large'),
                                          leading: Radio<FontChoices>(
                                            value: FontChoices.large,
                                            groupValue: _fontChoices,
                                            onChanged: (FontChoices? value) {
                                              setState(() {
                                                _fontChoices = value!;
                                                theme.setFontSize(3);
                                                Navigator.of(context).pop();
                                              });
                                            },
                                          )),
                                      ListTile(
                                          title: const Text('Extra Large'),
                                          leading: Radio<FontChoices>(
                                            value: FontChoices.xlarge,
                                            groupValue: _fontChoices,
                                            onChanged: (FontChoices? value) {
                                              setState(() {
                                                _fontChoices = value!;
                                                theme.setFontSize(4);
                                                Navigator.of(context).pop();
                                              });
                                            },
                                          )),
                                    ])));
                          },
                        ),
                    hoverColor: backgroundColor,
                    leading: const Icon(Icons.person_outline),
                    title: Text("Message font size",
                        style: TextStyle(
                            fontSize: theme.getMediumFont(context),
                            color: textColor))),
              ],
            ));
  }
}

class NotificationsSettingsScreen extends StatelessWidget {
  final ClientModel client;
  final NotficationsCB notficationsCB;
  final bool notificationsEnabled;
  const NotificationsSettingsScreen(
      this.client, this.notficationsCB, this.notificationsEnabled,
      {Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => ListView(children: [
              ListTile(
                  leading: Text("Notifications",
                      style: TextStyle(
                          fontSize: theme.getLargeFont(context),
                          color: Theme.of(context).focusColor)),
                  trailing: Switch(
                    // thumb color (round icon)
                    activeColor: Theme.of(context).focusColor,
                    activeTrackColor: Theme.of(context).highlightColor,
                    inactiveThumbColor: Theme.of(context).indicatorColor,
                    inactiveTrackColor: Theme.of(context).dividerColor,
                    //splashRadius: 20.0,
                    // boolean variable value
                    value: notificationsEnabled,
                    // changes the state of the switch
                    onChanged: (value) => notficationsCB(value),
                  ))
            ]));
  }
}
