import 'dart:io';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/containers.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/md_elements.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/audio.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/notification_service.dart';
import 'package:bruig/screens/config_network.dart';
import 'package:bruig/screens/list_kxs.dart';
import 'package:bruig/screens/config_rpc.dart';
import 'package:bruig/screens/ln_management.dart';
import 'package:bruig/screens/log.dart';
import 'package:bruig/screens/manage_content/manage_content.dart';
import 'package:bruig/screens/paystats.dart';
import 'package:bruig/screens/about.dart';
import 'package:bruig/screens/shutdown.dart';
import 'package:bruig/util.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:bruig/theme_manager.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/util.dart';
import 'package:permission_handler/permission_handler.dart';
import 'package:provider/provider.dart';
import 'package:bruig/models/client.dart';
import 'package:golib_plugin/golib_plugin.dart';
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
    return Consumer<SettingsTitleModel>(
        builder: (context, settingsTitle, child) => Text(settingsTitle.title));
  }
}

class SettingsScreen extends StatefulWidget {
  final ClientModel client;
  const SettingsScreen(this.client, {super.key});
  static String routeName = "/settings";

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  ClientModel get client => widget.client;
  bool loading = false;
  String settingsPage = "main";
  bool showRPCWarning = true;

  void loadSettings() async {
    var showWarning = await StorageManager.readBool(
        StorageManager.showRPCWarningKey,
        defaultVal: true);
    setState(() {
      showRPCWarning = showWarning;
    });
  }

  void connStateChanged() async {
    setState(() {});
  }

  void resetAllOldKX(BuildContext context) async {
    var snackbar = SnackBarModel.of(context);

    if (loading) return;
    setState(() => loading = true);
    try {
      await Golib.resetAllOldKX(0);
      snackbar
          .success('Requesting KX to all old KX no communicated in 30 days...');
    } catch (exception) {
      snackbar.error('Unable to reset all old KX: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  void resetAllOldKX1s(BuildContext context) async {
    if (loading) return;
    setState(() => loading = true);
    var snackbar = SnackBarModel.of(context);

    try {
      await Golib.resetAllOldKX(1);
      snackbar.success('Requesting KX to all old KX...');
    } catch (exception) {
      snackbar.error('Unable to reset all old KX: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  void pickAvatarFile() async {
    var snackbar = SnackBarModel.of(context);

    var filePickRes = await FilePicker.platform.pickFiles(
      allowMultiple: false,
      dialogTitle: "Pick avatar image file",
      type: FileType.custom,
      allowedExtensions: ["bmp", "gif", "jpeg", "jpg", "png", "webp"],
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
      snackbar.error("Unable to set avatar: $exception");
    }
  }

  void subAllPosts() async {
    try {
      await Golib.subscribeToAllRemotePosts();
      showSuccessSnackbar(
          this, "Attempting to subscribe to all remote user's posts");
    } catch (exception) {
      showErrorSnackbar(
          this, "Unable to subscribe to all remote posts: $exception");
    }
  }

  void listKXs() {
    Navigator.of(context, rootNavigator: true)
        .pushNamed(ListKXsScreen.routeName);
  }

  void changePage(String newPage) {
    setState(() {
      client.ui.settingsTitle.title = newPage;
      settingsPage = newPage;
    });
  }

  void shutdown() {
    ShutdownScreen.startShutdown(context);
  }

  @override
  void initState() {
    super.initState();
    loadSettings();
    client.connState.addListener(connStateChanged);
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

  void showRpcWarningDialog() {
    if (!showRPCWarning) {
      changePage("RPC");
      return;
    }

    bool turnOffAlert = false;
    showDialog(
      context: context,
      builder: (BuildContext context) {
        return StatefulBuilder(
          builder: (context, setDialogState) {
            return AlertDialog(
              title: const Text("Allow JSON RPC Access"),
              content: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Text(
                    "Enabling JSON RPC allows connections from third-party applications. Are you sure you want to proceed?",
                  ),
                  Row(
                    children: [
                      Checkbox(
                        value: turnOffAlert,
                        onChanged: (bool? value) {
                          setDialogState(() => turnOffAlert = value ?? false);
                        },
                      ),
                      const Text("Don’t show this message again"),
                    ],
                  ),
                ],
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.of(context).pop(),
                  child: const Text("Cancel"),
                ),
                TextButton(
                  onPressed: () {
                    Navigator.of(context).pop();
                    // Update the setting if the user chose to disable future alerts
                    if (turnOffAlert) {
                      StorageManager.saveBool(
                          StorageManager.showRPCWarningKey, false);
                      setState(() => showRPCWarning = false);
                    }
                    setState(() => settingsPage = "RPC");
                  },
                  child: const Text("Continue"),
                ),
              ],
            );
          },
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = checkIsScreenSmall(context);
    Widget settingsView = isScreenSmall
        ? MainSettingsScreen(client, pickAvatarFile, changePage, shutdown)
        : const Empty();
    switch (settingsPage) {
      case "Account":
        settingsView = AccountSettingsScreen(client, resetAllOldKX1s,
            resetAllOldKX, pickAvatarFile, subAllPosts, listKXs);
        break;
      case "Appearance":
        settingsView = Consumer<ThemeNotifier>(
            builder: (context, theme, _) =>
                AppearanceSettingsScreen(client, theme));
        break;
      case "Notifications":
        settingsView = const NotificationsSettingsScreen();
        break;
      case "Network":
        settingsView = NetworkSettingsScreen(client);
        break;
      case "RPC":
        settingsView = const RpcConfigScreen();
        break;
      case "About":
        settingsView = const AboutScreen(settings: true);
        break;
      case "Audio":
        settingsView = Consumer<AudioModel>(
            builder: (context, audio, child) =>
                AudioSettingsScreen(audio: audio));
        break;
      default:
        break;
    }
    if (isScreenSmall) {
      return Scaffold(
          body: Container(
              padding: const EdgeInsets.symmetric(horizontal: 3),
              child: settingsView));
    }

    // Desktop-sized version.
    return Row(children: [
      SecondarySideMenuList(width: 120, items: [
        ListTile(
          selected: settingsPage == "Account",
          title: const Txt.S("Account"),
          onTap: () => changePage("Account"),
        ),
        ListTile(
          selected: settingsPage == "Appearance",
          title: const Txt.S("Appearance"),
          onTap: () => changePage("Appearance"),
        ),
        ListTile(
          selected: settingsPage == "Notifications",
          title: const Txt.S("Notifications"),
          onTap: () => changePage("Notifications"),
        ),
        ListTile(
          selected: settingsPage == "Network",
          title: const Txt.S("Network"),
          onTap: () => changePage("Network"),
        ),
        ListTile(
          selected: settingsPage == "Audio",
          title: const Txt.S("Audio"),
          onTap: () => changePage("Audio"),
        ),
        ListTile(
          selected: settingsPage == "RPC",
          title: const Txt.S("RPC"),
          onTap: () => showRpcWarningDialog(),
        ),
      ]),
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
      {super.key});

  @override
  Widget build(BuildContext context) {
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
                        Text(client.nick),
                        SizedBox(
                            width: 150,
                            child: Copyable.txt(
                              Txt(client.publicID,
                                  overflow: TextOverflow.ellipsis),
                            ))
                      ])
                    ])),
                ListTile(
                    onTap: () => changePage("Account"),
                    leading: const Icon(Icons.person_outline),
                    title: const Text("Account")),
                ListTile(
                    onTap: () => changePage("Appearance"),
                    leading: const Icon(Icons.brightness_medium_outlined),
                    title: const Text("Appearance")),
                ListTile(
                    onTap: () => changePage("Notifications"),
                    leading: const Icon(Icons.notifications_outlined),
                    title: const Text("Notifications")),
                ListTile(
                    onTap: () => changePage("Network"),
                    leading: const Icon(Icons.shield),
                    title: const Text("Network")),
                ListTile(
                    onTap: () => changePage("Audio"),
                    leading: const Icon(Icons.perm_camera_mic_outlined),
                    title: const Text("Audio")),
                ListTile(
                    onTap: () {
                      Navigator.of(context)
                          .pushReplacementNamed(LNScreen.routeName);
                    },
                    leading: const SidebarSvgIcon(
                        "assets/icons/icons-menu-lnmng.svg"),
                    title: const Text("LN Management")),
                ListTile(
                    onTap: () {
                      Navigator.of(context)
                          .pushReplacementNamed(ManageContent.routeName);
                    },
                    leading: const SidebarSvgIcon(
                        "assets/icons/icons-menu-files.svg"),
                    title: const Text("Manage Content")),
                ListTile(
                    onTap: () {
                      Navigator.of(context)
                          .pushReplacementNamed(PayStatsScreen.routeName);
                    },
                    leading: const SidebarSvgIcon(
                        "assets/icons/icons-menu-stats.svg"),
                    title: const Text("Payment Stats")),
                ListTile(
                    onTap: () {
                      Navigator.of(context)
                          .pushReplacementNamed(LogScreen.routeName);
                    },
                    leading: const Icon(Icons.list_outlined),
                    title: const Text("Logs")),
                ListTile(
                    onTap: () => changePage("About"),
                    leading: const Icon(Icons.question_mark_outlined),
                    title: const Text("About Bison Relay")),
                ListTile(
                    onTap: shutdown,
                    leading: const Icon(Icons.exit_to_app),
                    title: const Text("Quit Bison Relay")),
              ],
            ));
  }
}

class AccountSettingsScreen extends StatelessWidget {
  final ClientModel client;
  final ResetKXCB resetAllKXCB;
  final ResetKXCB resetKXCB;
  final VoidCallback subAllPostsCB;
  final VoidCallback pickAvatarCB;
  final VoidCallback listKXs;
  const AccountSettingsScreen(this.client, this.resetAllKXCB, this.resetKXCB,
      this.pickAvatarCB, this.subAllPostsCB, this.listKXs,
      {super.key});

  @override
  Widget build(BuildContext context) {
    return Column(children: [
      const SizedBox(height: 10),
      SizedBox(
          width: 100,
          height: 100,
          child: SelfAvatar(client, onTap: pickAvatarCB)),
      const SizedBox(height: 10),
      Text(client.nick),
      const SizedBox(height: 10),
      Copyable(client.publicID),
      const SizedBox(height: 10),
      Expanded(
          child: ListView(children: [
        ListTile(
          title: const Text("Reset all KX"),
          onTap: () => resetAllKXCB(context),
        ),
        ListTile(
          title: const Text("Reset KX from users 30d stale"),
          onTap: () => resetKXCB(context),
        ),
        ListTile(
          title: const Text("Subscribe to all posts"),
          onTap: () => subAllPostsCB(),
        ),
        ListTile(
          title: const Text("List ongoing KX attempts"),
          onTap: () => listKXs(),
        ),
      ]))
    ]);
  }
}

class AppearanceSettingsScreen extends StatefulWidget {
  final ClientModel client;
  final ThemeNotifier theme;
  const AppearanceSettingsScreen(this.client, this.theme, {super.key});
  @override
  State<AppearanceSettingsScreen> createState() =>
      _AppearanceSettingsScreenState();
}

void _showSelectThemeDialog(BuildContext context, ThemeNotifier theme) {
  void switchToTheme(BuildContext context, String v) {
    theme.switchTheme(v);
    Navigator.of(context).pop();
  }

  showDialog(
      context: context,
      builder: (BuildContext context) => SimpleDialog(
          backgroundColor: theme.colors.primaryContainer,
          shape: const RoundedRectangleBorder(
              borderRadius: BorderRadius.all(Radius.circular(16.0))),
          children: appThemes.entries
              .map((e) => ListTile(
                  title:
                      Txt(e.value.descr, color: TextColor.onPrimaryContainer),
                  onTap: () => switchToTheme(context, e.key),
                  leading: Radio<String>(
                    value: e.key,
                    groupValue: theme.getThemeMode(),
                    onChanged: (_) => switchToTheme(context, e.key),
                  )))
              .toList()));
}

void _showSelectTextSizeDialog(BuildContext context, ThemeNotifier theme) {
  void switchFontSize(BuildContext context, String key) {
    theme.setFontSize(appFontSizes[key]?.scale ?? 1);
  }

  showDialog(
      context: context,
      builder: (BuildContext context) => SimpleDialog(
          backgroundColor: theme.colors.primaryContainer,
          shape: const RoundedRectangleBorder(
            borderRadius: BorderRadius.all(Radius.circular(16.0)),
          ),
          children: appFontSizes.entries
              .map((e) => ListTile(
                  title:
                      Txt(e.value.descr, color: TextColor.onPrimaryContainer),
                  onTap: () => switchFontSize(context, e.key),
                  leading: Radio<String>(
                    value: e.key,
                    groupValue: appFontSizeKeyForScale(theme.fontScale),
                    onChanged: (_) => switchFontSize(context, e.key),
                  )))
              .toList()));
}

/// This is the private State class that goes with MyStatefulWidget.
class _AppearanceSettingsScreenState extends State<AppearanceSettingsScreen> {
  ThemeNotifier get theme => widget.theme;

  @override
  Widget build(BuildContext context) {
    return ListView(
      children: [
        ListTile(
            onTap: () => _showSelectThemeDialog(context, theme),
            leading: const Icon(Icons.color_lens_outlined),
            title: const Text("Theme")),
        ListTile(
            onTap: () => _showSelectTextSizeDialog(context, theme),
            leading: const Icon(Icons.text_increase),
            title: const Text("Message font size")),
        kDebugMode
            ? ListTile(
                title: const Text("Widget Test Screen"),
                onTap: () {
                  Navigator.of(context, rootNavigator: true)
                      .pushNamed(ThemeTestScreen.routeName);
                })
            : const Empty(),
      ],
    );
  }
}

class NotificationsSettingsScreen extends StatefulWidget {
  const NotificationsSettingsScreen({super.key});

  @override
  State<NotificationsSettingsScreen> createState() =>
      _NotificationsSettingsScreenState();
}

class _NotificationsSettingsScreenState
    extends State<NotificationsSettingsScreen> {
  bool notificationsEnabled = false;
  bool foregroundService = false;
  bool pms = false;
  bool gcms = false;
  bool gcMentions = false;

  void loadSettings() async {
    var enabled = await StorageManager.readBool(StorageManager.notificationsKey,
        defaultVal: true);
    var fgSvc = await StorageManager.readBool(StorageManager.ntfnFgSvcKey);
    var newPms =
        await StorageManager.readBool(StorageManager.ntfnPMs, defaultVal: true);
    var newGcms = await StorageManager.readBool(StorageManager.ntfnGCMs);
    var mentions = await StorageManager.readBool(StorageManager.ntfnGCMentions,
        defaultVal: true);
    setState(() {
      notificationsEnabled = enabled;
      foregroundService = fgSvc;
      pms = newPms;
      gcms = newGcms;
      gcMentions = mentions;
    });
  }

  void updateEnabled(bool value) {
    StorageManager.saveData(StorageManager.notificationsKey, value);
    if (Platform.isAndroid) {
      Golib.setNtfnsEnabled(value);
    }
    setState(() => notificationsEnabled = value);

    // When disabling notifications, also disable foreground service.
    if (!value) updateForegroundSvc(false);
    NotificationService().updateUIConfig();
  }

  void updateForegroundSvc(bool value) {
    if (!Platform.isAndroid) {
      return;
    }

    StorageManager.saveData(StorageManager.ntfnFgSvcKey, value);
    if (value) {
      Golib.startForegroundSvc();
    } else {
      Golib.stopForegroundSvc();
    }

    setState(() => foregroundService = value);
  }

  void updatePMs(bool value) {
    StorageManager.saveData(StorageManager.ntfnPMs, value);
    setState(() => pms = value);
    NotificationService().updateUIConfig();
  }

  void updateGCMs(bool value) {
    StorageManager.saveData(StorageManager.ntfnGCMs, value);
    setState(() => gcms = value);
    NotificationService().updateUIConfig();
  }

  void updateGCMentions(bool value) {
    StorageManager.saveData(StorageManager.ntfnGCMentions, value);
    setState(() => gcMentions = value);
    NotificationService().updateUIConfig();
  }

  @override
  void initState() {
    super.initState();
    loadSettings();
  }

  @override
  Widget build(BuildContext context) {
    if (Platform.isWindows) {
      // See https://github.com/MaikuB/flutter_local_notifications/issues/746
      return Container(
          padding: const EdgeInsets.all(10),
          alignment: Alignment.topLeft,
          child: const Text("Notifications are not supported on Windows"));
    }

    return ListView(children: [
      ListTile(
          title: const Text("Notifications"),
          trailing: Switch(
            value: notificationsEnabled,
            onChanged: updateEnabled,
          )),
      Platform.isAndroid
          ? ListTile(
              title: const Text("Use Foreground Service"),
              trailing: Switch(
                value: foregroundService,
                onChanged: updateForegroundSvc,
              ))
          : const Empty(),
      ListTile(
          title: const Text("Notify on PMs"),
          trailing: Switch(
            value: pms,
            onChanged: updatePMs,
          )),
      ListTile(
          title: const Text("Notify on GC Messages"),
          trailing: Switch(
            value: gcms,
            onChanged: updateGCMs,
          )),
      ListTile(
          title: const Text("Notify on GC Mentions"),
          trailing: Switch(
            value: gcMentions,
            onChanged: updateGCMentions,
          )),
      if (kDebugMode)
        ListTile(
          title: const Text("Test Notification"),
          onTap: () {
            NotificationService().testNotification();
          },
        )
    ]);
  }
}

class NetworkSettingsScreen extends StatefulWidget {
  final ClientModel client;
  const NetworkSettingsScreen(this.client, {super.key});
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
    Widget actionWidget;
    switch (client.connState.state.state) {
      case connStateOnline:
        actionWidget = ListTile(
            onTap: Golib.remainOffline,
            leading: const Icon(Icons.cloud_off),
            title: const Text("Remain Offline"));
        break;

      case connStateCheckingWallet:
        actionWidget = ListTile(
            onTap: Golib.skipWalletCheck,
            leading: const Icon(Icons.cloud_off),
            title: const Text("Skip Wallet Check"));
        break;

      case connStateOffline:
        actionWidget = ListTile(
            onTap: Golib.goOnline,
            leading: const Icon(Icons.cloud_done),
            title: const Text("Go Online"));
        break;

      default:
        actionWidget = const Empty();
        break;
    }

    var policy = client.connState.state.policy;
    var maxPayloadSize = maxMsgPayloadSize(policy.maxMsgSizeVersion);
    var pushDcrGbRate = policy.calcPushCostMAtoms(1000000000).toDouble() / 1e11;

    return Column(children: [
      client.connState.isCheckingWallet && client.connState.checkWalletErr != ""
          ? Container(
              padding: const EdgeInsets.symmetric(vertical: 1, horizontal: 10),
              child: Copyable(
                  "Offline due to failed wallet check: ${client.connState.checkWalletErr}"))
          : const Empty(),
      Expanded(
          child: ListView(
        children: [
          ListTile(
              onTap: () => Navigator.of(context, rootNavigator: true)
                  .pushNamed(ConfigNetworkScreen.routeName),
              leading: const Icon(Icons.network_ping),
              title: const Text("Proxy Settings")),
          actionWidget,
          const Divider(),
          const Txt.L("Server Policy"),
          Txt("Max Message Payload Size: ${ibSize(maxPayloadSize)} (version ${policy.maxMsgSizeVersion})"),
          Txt("Push Rate: ${pushDcrGbRate.toStringAsFixed(8)} DCR/GB (min ${formatDCR(milliatomsToDCR(policy.pushPayRateMinMAtoms))})"),
          Txt("Subscription Rate: ${formatDCR(milliatomsToDCR(policy.subPayRate))}/RV"),
          Txt("Expiration days: ${policy.expirationDays}"),
        ],
      )),
    ]);
  }
}

class AudioSettingsScreen extends StatefulWidget {
  final AudioModel audio;
  const AudioSettingsScreen({required this.audio, super.key});

  @override
  State<AudioSettingsScreen> createState() => _AudioSettingsScreenState();
}

class _AudioSettingsScreenState extends State<AudioSettingsScreen> {
  AudioModel get audio => widget.audio;
  AudioDevices devices = AudioDevices([], []);

  void listAudioDevices() async {
    if (Platform.isAndroid || Platform.isIOS) {
      var micStatus = await Permission.microphone.request();
      if (!micStatus.isGranted) {
        showErrorSnackbar(this, "Microphone permission not given");
        return;
      }
    }

    try {
      var devs = await Golib.listAudioDevices();

      setState(() {
        devices = devs;
      });
    } catch (exception) {
      showErrorSnackbar(this, "Unable to list audio devices: $exception");
    }
  }

  void stopAudio() async {
    try {
      await audio.stop();
    } catch (exception) {
      showErrorSnackbar(this, "Unable to stop audio: $exception");
    }
  }

  void recordAudio() async {
    try {
      await audio.recordNote();
    } catch (exception) {
      showErrorSnackbar(this, "Unable to start audio recording: $exception");
    }
  }

  void playbackAudio() async {
    try {
      await audio.playbackNote();
    } catch (exception) {
      showErrorSnackbar(this, "Unable to start audio playback: $exception");
    }
  }

  void updated() {
    setState(() {});
  }

  void clearRecord() async {
    // Sleep is needed on slower computers to avoid recursive/locked setState
    // exceptions (because clearRecord is called from init/dispose).
    await sleep(const Duration(milliseconds: 1));
    audio.clearRecorded();
  }

  @override
  void initState() {
    super.initState();
    clearRecord();
    audio.addListener(updated);
    listAudioDevices();
  }

  @override
  void dispose() {
    super.dispose();
    audio.removeListener(updated);
    clearRecord();
  }

  @override
  Widget build(BuildContext context) {
    return Container(
        padding: const EdgeInsets.symmetric(horizontal: 20),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          const Text("Capture Device"),
          SizedBox(
              width: double.infinity,
              child: DropdownButton(
                  value: audio.captureDeviceId == ""
                      ? null
                      : audio.captureDeviceId,
                  items: devices.capture
                      .map<DropdownMenuItem<String?>>((e) =>
                          DropdownMenuItem<String?>(
                              value: e.id, child: Txt.S(e.name)))
                      .toList(),
                  onChanged: (newVal) {
                    if (newVal == null) {
                      return;
                    }
                    if (newVal == audio.captureDeviceId) {
                      return;
                    }
                    audio.captureDeviceId = newVal;
                  })),
          const SizedBox(height: 30),
          const Text("Playback Device"),
          SizedBox(
              width: double.infinity,
              child: DropdownButton(
                  value: audio.playbackDeviceId == ""
                      ? null
                      : audio.playbackDeviceId,
                  items: devices.playback
                      .map<DropdownMenuItem<String?>>((e) =>
                          DropdownMenuItem<String?>(
                              value: e.id, child: Txt.S(e.name)))
                      .toList(),
                  onChanged: (newVal) {
                    if (newVal == null) {
                      return;
                    }
                    if (newVal == audio.playbackDeviceId) {
                      return;
                    }
                    audio.playbackDeviceId = newVal;
                  })),
          const SizedBox(height: 30),
          Wrap(spacing: 10, runSpacing: 10, children: [
            TextButton.icon(
                onPressed:
                    !audio.recording && !audio.playing ? recordAudio : null,
                label: const Text("Record Audio Test"),
                icon: const Icon(Icons.record_voice_over_outlined)),
            if (audio.hasRecord)
              TextButton.icon(
                  onPressed:
                      !audio.recording && !audio.playing ? playbackAudio : null,
                  label: const Text("Play Audio Test"),
                  icon: const Icon(Icons.play_arrow_outlined)),
            if (audio.recording || audio.playing)
              TextButton.icon(
                  onPressed: stopAudio,
                  label: const Text("Stop"),
                  icon: const Icon(Icons.stop)),
          ]),
        ]));
  }
}

const _loremOneLine =
    "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua";

const _testMarkdownText = """This is a sample markdown text.

Following this is a code fence section.

```go
func TestOne(t *testing.T) {
  t.Parallel()
  var i uint32 = 1 << 10
  if i == 42 {
    t.Fatalf("You have made a mistake: %v", i)
  }
}
```

Following is a block quote section.

> This is a quote from
> somewhere that i found
> over the internet. There
> will be a second-level quote
> below:
>
>> Second-level quote now
>> full of some text
>> that explains things.

""";

class ThemeTestScreen extends StatelessWidget {
  static String routeName = "/themeTest";

  final ThemeNotifier theme;
  const ThemeTestScreen(this.theme, {super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      floatingActionButton:
          Column(mainAxisAlignment: MainAxisAlignment.end, children: [
        IconButton(
            onPressed: () => _showSelectThemeDialog(context, theme),
            icon: const Icon(Icons.color_lens_outlined)),
        IconButton(
            onPressed: () => _showSelectTextSizeDialog(context, theme),
            icon: const Icon(Icons.text_increase)),
        IconButton(
            icon: const Icon(Icons.cancel_outlined),
            onPressed: () => Navigator.of(context).pop()),
      ]),
      body: SingleChildScrollView(
          padding: const EdgeInsets.all(40),
          child: Wrap(runSpacing: 20, spacing: 40, children: [
            // These are build "manually" for comparison. In parciular, on
            // material design v3, the "onXXXContainer" for primary and tertiary
            // are different on dark mode.
            const SizedBox(
                width: double.infinity,
                child: Text("The following are manually specified components")),

            Container(
              padding: const EdgeInsets.all(20),
              color: theme.colors.primaryContainer,
              constraints: const BoxConstraints.tightFor(width: 400),
              child: Column(children: [
                const Text("Primary Container"),
                Text("Text with on color",
                    style: TextStyle(color: theme.colors.onPrimaryContainer)),
                const Text(
                  "Custom color text",
                  style: TextStyle(color: Colors.amber),
                )
              ]),
            ),

            Container(
              padding: const EdgeInsets.all(20),
              color: theme.colors.tertiaryContainer,
              constraints: const BoxConstraints.tightFor(width: 400),
              child: Column(children: [
                const Text("Tertiary Container"),
                Text("Text with on color",
                    style: TextStyle(color: theme.colors.onTertiaryContainer)),
                const Text(
                  "Custom color text",
                  style: TextStyle(color: Colors.amber),
                )
              ]),
            ),

            const Divider(),
            const SizedBox(
                width: double.infinity,
                child: Text("Containers specified with app components")),

            // Following are using the custom app components.

            const Box(
              padding: EdgeInsets.all(20),
              color: SurfaceColor.primaryContainer,
              constraints: BoxConstraints.tightFor(width: 400),
              child: Column(children: [
                Text("Primary Container"),
                Txt("Text with on color", color: TextColor.onPrimaryContainer),
                Text("Custom color text", style: TextStyle(color: Colors.amber))
              ]),
            ),

            const Box(
              padding: EdgeInsets.all(20),
              color: SurfaceColor.secondaryContainer,
              constraints: BoxConstraints.tightFor(width: 400),
              child: Column(children: [
                Text("Secondary Container"),
                Txt("Text with on color",
                    color: TextColor.onSecondaryContainer),
                Text("Custom color text", style: TextStyle(color: Colors.amber))
              ]),
            ),

            const Box(
              padding: EdgeInsets.all(20),
              color: SurfaceColor.tertiaryContainer,
              constraints: BoxConstraints.tightFor(width: 400),
              child: Column(children: [
                Text("Tertiary Container"),
                Txt("Text with on color", color: TextColor.onTertiaryContainer),
                Text("Custom color text", style: TextStyle(color: Colors.amber))
              ]),
            ),

            const Box(
              padding: EdgeInsets.all(20),
              color: SurfaceColor.errorContainer,
              constraints: BoxConstraints.tightFor(width: 400),
              child: Column(children: [
                Text("Error Container"),
                Txt("Text with on color", color: TextColor.onErrorContainer),
                Text("Custom color text", style: TextStyle(color: Colors.amber))
              ]),
            ),

            const Box(
              padding: EdgeInsets.all(20),
              color: SurfaceColor.surface,
              constraints: BoxConstraints.tightFor(width: 400),
              child: Column(children: [
                Text("Surface"),
                Txt("Text with on color", color: TextColor.onSurface),
                Txt("Text with on variant color",
                    color: TextColor.onSurfaceVariant),
                Text("Custom color text", style: TextStyle(color: Colors.amber))
              ]),
            ),

            const Box(
              padding: EdgeInsets.all(20),
              color: SurfaceColor.surfaceContainer,
              constraints: BoxConstraints.tightFor(width: 400),
              child: Column(children: [
                Text("Surface Container"),
                Txt("Text with on color", color: TextColor.onSurface),
                Txt("Text with on variant color",
                    color: TextColor.onSurfaceVariant),
                Text("Custom color text", style: TextStyle(color: Colors.amber))
              ]),
            ),

            const Box(
              padding: EdgeInsets.all(20),
              color: SurfaceColor.surfaceBright,
              constraints: BoxConstraints.tightFor(width: 400),
              child: Column(children: [
                Text("Surface bright"),
                Txt("Text with on color", color: TextColor.onSurface),
                Txt("Text with on variant color",
                    color: TextColor.onSurfaceVariant),
                Text("Custom color text", style: TextStyle(color: Colors.amber))
              ]),
            ),

            const Box(
              padding: EdgeInsets.all(20),
              color: SurfaceColor.surfaceDim,
              constraints: BoxConstraints.tightFor(width: 400),
              child: Column(children: [
                Text("Surface dim"),
                Txt("Text with on color", color: TextColor.onSurface),
                Txt("Text with on variant color",
                    color: TextColor.onSurfaceVariant),
                Text("Custom color text", style: TextStyle(color: Colors.amber))
              ]),
            ),

            const Box(
              padding: EdgeInsets.all(20),
              color: SurfaceColor.surfaceContainerLowest,
              constraints: BoxConstraints.tightFor(width: 400),
              child: Column(children: [
                Text("Surface container lowest"),
                Txt("Text with on color", color: TextColor.onSurface),
                Txt("Text with on variant color",
                    color: TextColor.onSurfaceVariant),
                Text("Custom color text", style: TextStyle(color: Colors.amber))
              ]),
            ),

            const Box(
              padding: EdgeInsets.all(20),
              color: SurfaceColor.surfaceContainerLow,
              constraints: BoxConstraints.tightFor(width: 400),
              child: Column(children: [
                Text("Surface container low"),
                Txt("Text with on color", color: TextColor.onSurface),
                Txt("Text with on variant color",
                    color: TextColor.onSurfaceVariant),
                Text("Custom color text", style: TextStyle(color: Colors.amber))
              ]),
            ),

            const Box(
              padding: EdgeInsets.all(20),
              color: SurfaceColor.surfaceContainerHigh,
              constraints: BoxConstraints.tightFor(width: 400),
              child: Column(children: [
                Text("Surface container high"),
                Txt("Text with on color", color: TextColor.onSurface),
                Txt("Text with on variant color",
                    color: TextColor.onSurfaceVariant),
                Text("Custom color text", style: TextStyle(color: Colors.amber))
              ]),
            ),

            const Box(
              padding: EdgeInsets.all(20),
              color: SurfaceColor.surfaceContainerHighest,
              constraints: BoxConstraints.tightFor(width: 400),
              child: Column(children: [
                Text("Surface container highest"),
                Txt("Text with on color", color: TextColor.onSurface),
                Txt("Text with on variant color",
                    color: TextColor.onSurfaceVariant),
                Text("Custom color text", style: TextStyle(color: Colors.amber))
              ]),
            ),

            const Box(
              padding: EdgeInsets.all(20),
              color: SurfaceColor.inverseSurface,
              constraints: BoxConstraints.tightFor(width: 400),
              child: Column(children: [
                Text("Inverse Surface"),
                Txt("Text with on color", color: TextColor.onInverseSurface),
                Txt("Text with on inverse primary",
                    color: TextColor.inversePrimary),
                Text("Custom color text", style: TextStyle(color: Colors.amber))
              ]),
            ),

            const SizedBox(
                width: double.infinity,
                child: Txt("Text on surface (all defaults)")),

            const Divider(),
            const SizedBox(
                width: double.infinity,
                child: Text(
                    "Active colors (not usually used for containers, only for components)")),

            const Box(
              padding: EdgeInsets.all(20),
              color: SurfaceColor.primary,
              constraints: BoxConstraints.tightFor(width: 400),
              child: Column(children: [
                Text("Primary"),
                Txt("Text with on color", color: TextColor.onPrimary),
                Text("Custom color text", style: TextStyle(color: Colors.amber))
              ]),
            ),

            const Box(
              padding: EdgeInsets.all(20),
              color: SurfaceColor.secondary,
              constraints: BoxConstraints.tightFor(width: 400),
              child: Column(children: [
                Text("Secondary"),
                Txt("Text with on color", color: TextColor.onSecondary),
                Text("Custom color text", style: TextStyle(color: Colors.amber))
              ]),
            ),

            const Box(
              padding: EdgeInsets.all(20),
              color: SurfaceColor.tertiary,
              constraints: BoxConstraints.tightFor(width: 400),
              child: Column(children: [
                Text("Tertiary"),
                Txt("Text with on color", color: TextColor.onTertiary),
                Text("Custom color text", style: TextStyle(color: Colors.amber))
              ]),
            ),

            const Box(
              padding: EdgeInsets.all(20),
              color: SurfaceColor.error,
              constraints: BoxConstraints.tightFor(width: 400),
              child: Column(children: [
                Text("Error"),
                Txt("Text with on color", color: TextColor.onError),
                Text("Custom color text", style: TextStyle(color: Colors.amber))
              ]),
            ),

            const Box(
              padding: EdgeInsets.all(20),
              color: SurfaceColor.inversePrimary,
              constraints: BoxConstraints.tightFor(width: 400),
              child: Column(children: [
                Text("Inverse Primary"),
                Text("(used when the background surface is inverseSurface)"),
                Txt("Text with on color", color: TextColor.onSurface),
                Text("Custom color text", style: TextStyle(color: Colors.amber))
              ]),
            ),

            const Divider(),
            const SizedBox(width: double.infinity, child: Text("Typography")),

            const SizedBox(
                width: double.infinity, child: Txt.S("Small $_loremOneLine")),
            const SizedBox(
                width: double.infinity, child: Txt("System $_loremOneLine")),
            const SizedBox(
                width: double.infinity, child: Txt.M("Medium $_loremOneLine")),
            const SizedBox(
                width: double.infinity, child: Txt.L("Large $_loremOneLine")),
            const SizedBox(
                width: double.infinity, child: Txt.H("Huge $_loremOneLine")),

            const Divider(),
            const SizedBox(width: double.infinity, child: Text("Components")),

            Container(
              padding: const EdgeInsets.all(20),
              constraints: const BoxConstraints.tightFor(width: 400),
              child: Wrap(
                spacing: 20,
                runSpacing: 10,
                children: [
                  const SizedBox(
                      width: double.infinity, child: Text("Buttons")),
                  ElevatedButton(
                      onPressed: () {}, child: const Text("Elevated Button")),
                  TextButton(
                      onPressed: () {}, child: const Text("Text Button")),
                  FilledButton(
                      onPressed: () {}, child: const Text("Filled Button")),
                  FilledButton.tonal(
                      onPressed: () {},
                      child: const Text("Filled Tonal Button")),
                  OutlinedButton(
                      onPressed: () {}, child: const Text("Outlined Button")),
                  CancelButton(onPressed: () {}),
                  IconButton(
                      onPressed: () {},
                      icon: const Icon(Icons.baby_changing_station_rounded)),
                  FloatingActionButton.small(
                      heroTag: "test.fab.small",
                      onPressed: () {},
                      child: const Icon(Icons.settings)),
                  FloatingActionButton.large(
                      heroTag: "test.fab.large",
                      key: const Key("test.fab.large"),
                      onPressed: () {},
                      child: const Icon(Icons.checklist_sharp)),
                  FloatingActionButton.extended(
                      heroTag: "test.fab.extended",
                      key: const Key("test.fab.extended"),
                      onPressed: () {},
                      label: const Text("Do the thing"),
                      icon: const Icon(Icons.account_tree)),
                ],
              ),
            ),

            Container(
              padding: const EdgeInsets.all(20),
              constraints: const BoxConstraints.tightFor(width: 400),
              child: const Wrap(
                spacing: 20,
                runSpacing: 10,
                children: [
                  SizedBox(
                      width: double.infinity, child: Text("Disabled Buttons")),
                  ElevatedButton(
                      onPressed: null, child: Text("Elevated Button")),
                  TextButton(onPressed: null, child: Text("Text Button")),
                  FilledButton(onPressed: null, child: Text("Filled Button")),
                  FilledButton.tonal(
                      onPressed: null, child: Text("Filled Tonal Button")),
                  OutlinedButton(
                      onPressed: null, child: Text("Outlined Button")),
                  CancelButton(onPressed: null),
                  IconButton(
                      onPressed: null,
                      icon: Icon(Icons.baby_changing_station_rounded)),
                  FloatingActionButton.small(
                      heroTag: "test.fab.small.disabled",
                      onPressed: null,
                      child: Icon(Icons.settings)),
                  FloatingActionButton.large(
                      heroTag: "test.fab.large.disabled",
                      onPressed: null,
                      child: Icon(Icons.checklist_sharp)),
                  FloatingActionButton.extended(
                      heroTag: "test.fab.extended.disabled",
                      onPressed: null,
                      label: Text("Do the thing"),
                      icon: Icon(Icons.account_tree)),
                ],
              ),
            ),

            Container(
                padding: const EdgeInsets.all(20),
                constraints: const BoxConstraints.tightFor(width: 800),
                child: Wrap(spacing: 20, runSpacing: 10, children: [
                  const SizedBox(width: double.infinity, child: Text("List")),
                  SizedBox(
                      height: 180,
                      child: ListView.builder(
                          itemCount: 10,
                          itemBuilder: (context, index) => ListTile(
                                selected: index == 6,
                                title: Txt.S("Item $index"),
                                onTap: () {},
                              )))
                ])),

            Container(
                padding: const EdgeInsets.all(20),
                constraints: const BoxConstraints.tightFor(width: 400),
                child: MarkdownArea(_testMarkdownText, false)),
          ])),
    );
  }
}
