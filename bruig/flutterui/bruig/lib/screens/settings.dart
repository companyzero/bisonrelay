import 'dart:io';

import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/config_network.dart';
import 'package:bruig/screens/ln_management.dart';
import 'package:bruig/screens/log.dart';
import 'package:bruig/screens/manage_content/manage_content.dart';
import 'package:bruig/screens/paystats.dart';
import 'package:bruig/screens/about.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:bruig/theme_manager.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:provider/provider.dart';
import 'package:bruig/models/client.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/storage_manager.dart';
import 'package:bruig/models/menus.dart';
import 'package:bruig/components/copyable.dart';

typedef ChangePageCB = void Function(String);
typedef NotficationsCB = void Function(bool?, bool?);
typedef ResetKXCB = void Function(BuildContext);
typedef ShutdownCB = void Function();

class SettingsScreenTitle extends StatelessWidget {
  const SettingsScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer2<SettingsTitleModel, ThemeNotifier>(
        builder: (context, settingsTitle, themeNtf, child) => Text(
            settingsTitle.title,
            style: TextStyle(
                fontSize: themeNtf.getLargeFont(context),
                color: themeNtf.getTheme().focusColor)));
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
  bool loading = false;
  bool notificationsEnabled = false;
  bool foregroundService = false;
  String settingsPage = "main";

  void connStateChanged() async {
    setState(() {});
  }

  void updateNotificationSettings(bool? value, bool? foregroundSvc) {
    if (value != null) {
      StorageManager.saveData(StorageManager.notificationsKey, value);
      if (Platform.isAndroid) {
        Golib.setNtfnsEnabled(value);
      }
    }
    if (foregroundSvc != null && Platform.isAndroid) {
      StorageManager.saveData(StorageManager.ntfnFgSvcKey, foregroundSvc);
      if (foregroundSvc) {
        Golib.startForegroundSvc();
      } else {
        Golib.stopForegroundSvc();
      }
    }
    setState(() {
      notificationsEnabled = value ?? notificationsEnabled;
      foregroundService = foregroundSvc ?? foregroundService;
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
      client.myAvatar.loadAvatar(fileData);
    } catch (exception) {
      showErrorSnackbar(context, "Unable to set avatar: $exception");
    }
  }

  void changePage(String newPage) {
    setState(() {
      client.ui.settingsTitle.title = newPage;
      settingsPage = newPage;
    });
  }

  void shutdown() {
    Navigator.of(context, rootNavigator: true)
        .pushReplacementNamed("/shutdown");
  }

  @override
  void initState() {
    super.initState();
    client.connState.addListener(connStateChanged);
    StorageManager.readData(StorageManager.notificationsKey).then((value) {
      if (value != null) {
        setState(() {
          notificationsEnabled = value;
        });
      }
    });
    StorageManager.readData(StorageManager.ntfnFgSvcKey).then((value) {
      if (value != null) {
        setState(() {
          foregroundService = value;
        });
      }
    });
  }

  @override
  void didUpdateWidget(SettingsScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.client != widget.client) {
      oldWidget.client.connState.removeListener(connStateChanged);
      client.connState.addListener(connStateChanged);
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.addPostFrameCallback(
        (_) => client.ui.settingsTitle.title = "Settings");
    client.connState.removeListener(connStateChanged);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var themeNtf = Provider.of<ThemeNotifier>(context);
    var theme = themeNtf.getTheme();
    var canvasColor = theme.canvasColor;
    var unselectedTextColor = theme.dividerColor;
    var selectedTextColor = theme.focusColor; // MESSAGE TEXT COLOR
    var sidebarBackground = theme.backgroundColor;
    var hoverColor = theme.hoverColor;

    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    Widget settingsView = isScreenSmall
        ? MainSettingsScreen(client, pickAvatarFile, changePage, shutdown)
        : const Empty();
    switch (settingsPage) {
      case "Account":
        settingsView = AccountSettingsScreen(
            client, resetAllOldKX1s, resetAllOldKX, pickAvatarFile);
        break;
      case "Appearance":
        settingsView = Consumer<ThemeNotifier>(
            builder: (context, theme, _) =>
                AppearanceSettingsScreen(client, theme));
        break;
      case "Notifications":
        settingsView = NotificationsSettingsScreen(
            client,
            updateNotificationSettings,
            notificationsEnabled,
            foregroundService);
        break;
      case "Network":
        settingsView = NetworkSettingsScreen(client);
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
                body: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 3),
                    child: settingsView),
              ));
    }

    var itemTs = TextStyle(
        color: unselectedTextColor, fontSize: themeNtf.getSmallFont(context));
    var selItemTs = TextStyle(
        color: selectedTextColor, fontSize: themeNtf.getSmallFont(context));

    // Desktop-sized version.
    return Row(children: [
      Container(
          margin: const EdgeInsets.all(1),
          width: 120,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(5),
            gradient: LinearGradient(
                begin: Alignment.centerRight,
                end: Alignment.centerLeft,
                colors: [
                  hoverColor,
                  sidebarBackground,
                  sidebarBackground,
                ],
                stops: const [
                  0,
                  0.51,
                  1
                ]),
          ),
          //color: theme.colorScheme.secondary,
          child: ListView(children: [
            ListTile(
              title: Text("Account",
                  style: settingsPage == "Account" ? selItemTs : itemTs),
              onTap: () => changePage("Account"),
            ),
            ListTile(
              title: Text("Appearance",
                  style: settingsPage == "Appearance" ? selItemTs : itemTs),
              onTap: () => changePage("Appearance"),
            ),
            ListTile(
              title: Text("Notifications",
                  style: settingsPage == "Notifications" ? selItemTs : itemTs),
              onTap: () => changePage("Notifications"),
            ),
            ListTile(
              title: Text("Network",
                  style: settingsPage == "Network" ? selItemTs : itemTs),
              onTap: () => changePage("Network"),
            ),
          ])),
      Expanded(child: settingsView),
    ]);
  }
}

class MainSettingsScreen extends StatelessWidget {
  final ClientModel client;
  final VoidCallback pickAvatarFile;
  final ChangePageCB changePage;
  final ShutdownCB shutdown;
  const MainSettingsScreen(
      this.client, this.pickAvatarFile, this.changePage, this.shutdown,
      {Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var backgroundColor = theme.backgroundColor;
    var textColor = theme.focusColor;

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => ListView(
              children: [
                ListTile(
                    title: Row(
                        mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                        children: [
                      Container(
                        margin: const EdgeInsets.all(10),
                        child: SelfAvatar(client, onTap: pickAvatarFile),
                      ),
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
                    onTap: () => changePage("Network"),
                    hoverColor: backgroundColor,
                    leading: const Icon(Icons.shield),
                    title: Text("Network",
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
                ListTile(
                    onTap: shutdown,
                    hoverColor: backgroundColor,
                    leading: const Icon(Icons.exit_to_app),
                    title: Text("Quit Bison Relay",
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
  final VoidCallback pickAvatarCB;
  const AccountSettingsScreen(
      this.client, this.resetAllKXCB, this.resetKXCB, this.pickAvatarCB,
      {Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Column(children: [
              const SizedBox(height: 10),
              SizedBox(
                  width: 100,
                  height: 100,
                  child: SelfAvatar(client, onTap: pickAvatarCB)),
              const SizedBox(height: 10),
              Text(client.nick, style: TextStyle(color: textColor)),
              const SizedBox(height: 10),
              Copyable(client.publicID, TextStyle(color: textColor)),
              const SizedBox(height: 10),
              Expanded(
                  child: ListView(children: [
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
              ]))
            ]));
  }
}

enum FontChoices { small, medium, large, xlarge }

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

  @override
  void initState() {
    super.initState();
  }

  @override
  void didUpdateWidget(AppearanceSettingsScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
  }

  void switchToTheme(BuildContext context, String v) {
    setState(() {
      theme.switchTheme(v);
      Navigator.of(context).pop();
    });
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
                            return SimpleDialog(
                                shadowColor: backgroundColor,
                                insetPadding: const EdgeInsets.symmetric(
                                    horizontal: 40.0, vertical: 275.0),
                                backgroundColor: backgroundColor,
                                shape: const RoundedRectangleBorder(
                                    borderRadius: BorderRadius.all(
                                        Radius.circular(16.0))),
                                children: appThemes.entries
                                    .map((e) => ListTile(
                                        title: Text(e.value.descr),
                                        onTap: () =>
                                            switchToTheme(context, e.key),
                                        leading: Radio<String>(
                                          value: e.key,
                                          groupValue: theme.getThemeMode(),
                                          onChanged: (_) =>
                                              switchToTheme(context, e.key),
                                        )))
                                    .toList());
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
  final bool foregroundService;
  const NotificationsSettingsScreen(this.client, this.notficationsCB,
      this.notificationsEnabled, this.foregroundService,
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
                    onChanged: (value) =>
                        // When disabling notifications, also disable foreground service.
                        notficationsCB(value, !value ? false : null),
                  )),
              Platform.isAndroid
                  ? ListTile(
                      leading: Text("Use Foreground Service",
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
                        value: foregroundService,
                        // changes the state of the switch
                        onChanged: (value) => notficationsCB(null, value),
                      ))
                  : const Empty(),
            ]));
  }
}

class NetworkSettingsScreen extends StatefulWidget {
  final ClientModel client;
  const NetworkSettingsScreen(this.client, {Key? key}) : super(key: key);
  @override
  State<NetworkSettingsScreen> createState() => _NetworkSettingsScreenState();
}

/// This is the private State class that goes with MyStatefulWidget.
class _NetworkSettingsScreenState extends State<NetworkSettingsScreen> {
  ClientModel get client => widget.client;

  void connStateChanged() {
    setState(() {});
  }

  @override
  void initState() {
    super.initState();
    client.connState.addListener(connStateChanged);
  }

  @override
  void didUpdateWidget(NetworkSettingsScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.client != client) {
      oldWidget.client.connState.removeListener(connStateChanged);
      client.connState.addListener(connStateChanged);
    }
  }

  @override
  void dispose() {
    client.connState.removeListener(connStateChanged);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var themeNtf = Provider.of<ThemeNotifier>(context);
    var theme = themeNtf.getTheme();
    var backgroundColor = theme.backgroundColor;
    var textColor = theme.focusColor;

    Widget actionWidget;
    switch (client.connState.state.state) {
      case connStateOnline:
        actionWidget = ListTile(
            onTap: Golib.remainOffline,
            hoverColor: backgroundColor,
            leading: const Icon(Icons.cloud_off),
            title: Text("Remain Offline",
                style: TextStyle(
                    fontSize: themeNtf.getMediumFont(context),
                    color: textColor)));
        break;

      case connStateCheckingWallet:
        actionWidget = ListTile(
            onTap: Golib.skipWalletCheck,
            hoverColor: backgroundColor,
            leading: const Icon(Icons.cloud_off),
            title: Text("Skip Wallet Check",
                style: TextStyle(
                    fontSize: themeNtf.getMediumFont(context),
                    color: textColor)));
        break;

      case connStateOffline:
        actionWidget = ListTile(
            onTap: Golib.goOnline,
            hoverColor: backgroundColor,
            leading: const Icon(Icons.cloud_done),
            title: Text("Go Online",
                style: TextStyle(
                    fontSize: themeNtf.getMediumFont(context),
                    color: textColor)));
        break;

      default:
        actionWidget = const Empty();
        break;
    }

    return Column(children: [
      client.connState.isCheckingWallet && client.connState.checkWalletErr != ""
          ? Copyable(
              "Offline due to failed wallet check: ${client.connState.checkWalletErr}",
              TextStyle(color: textColor))
          : const Empty(),
      Expanded(
          child: ListView(
        children: [
          ListTile(
              onTap: () => Navigator.of(context, rootNavigator: true)
                  .pushNamed(ConfigNetworkScreen.routeName),
              hoverColor: backgroundColor,
              leading: const Icon(Icons.network_ping),
              title: Text("Proxy Settings",
                  style: TextStyle(
                      fontSize: themeNtf.getMediumFont(context),
                      color: textColor))),
          actionWidget,
        ],
      ))
    ]);
  }
}
