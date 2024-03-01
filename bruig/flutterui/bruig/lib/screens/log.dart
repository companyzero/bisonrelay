import 'dart:io';
import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/confirmation_dialog.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/recent_log.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/config.dart';
import 'package:bruig/models/log.dart';
import 'package:dropdown_button2/dropdown_button2.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:bruig/theme_manager.dart';
import 'package:flutter/services.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:path/path.dart' as path;
import 'package:path_provider/path_provider.dart';
import 'package:provider/provider.dart';
import 'package:restart_app/restart_app.dart';

class LogScreenTitle extends StatelessWidget {
  const LogScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Text("Logs",
            style: TextStyle(
                fontSize: theme.getLargeFont(context),
                color: Theme.of(context).focusColor)));
  }
}

class LogScreen extends StatelessWidget {
  static const routeName = '/log';
  final LogModel log;
  const LogScreen(this.log, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var backgroundColor = theme.backgroundColor;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Container(
            margin: const EdgeInsets.all(1),
            decoration: BoxDecoration(
              color: backgroundColor,
              borderRadius: BorderRadius.circular(3),
            ),
            padding: const EdgeInsets.all(16),
            child: Column(children: [
              const SizedBox(height: 20),
              Text("Recent Log",
                  style: TextStyle(
                      color: textColor, fontSize: theme.getLargeFont(context))),
              const SizedBox(height: 20),
              Expanded(child: LogLines(log)),
              const SizedBox(height: 20),
              Row(mainAxisAlignment: MainAxisAlignment.spaceBetween, children: [
                ElevatedButton(
                    onPressed: () {
                      Navigator.of(context, rootNavigator: true)
                          .pushNamed(ExportLogScreen.routeName);
                    },
                    child: const Text("Export Logs")),
                ElevatedButton(
                    onPressed: () {
                      Navigator.of(context, rootNavigator: true)
                          .pushNamed(LogSettingsScreen.routeName);
                    },
                    child: const Text("Settings")),
              ]),
              const SizedBox(height: 20),
            ])));
  }
}

class ExportLogScreen extends StatefulWidget {
  static String routeName = "/exportLogs";
  const ExportLogScreen({super.key});

  @override
  State<ExportLogScreen> createState() => _ExportLogScreenState();
}

class _ExportLogScreenState extends State<ExportLogScreen> {
  final bool isMobile = Platform.isIOS || Platform.isAndroid;
  String destPath = "";
  bool allFiles = false;
  bool golibLogs = true;
  bool lnLogs = true;
  bool exporting = false;

  @override
  void initState() {
    super.initState();

    // Set default dir. The app should be allowed to write to this on mobile.
    () async {
      try {
        var dir = (await getExternalStorageDirectory())?.path;
        setState(() {
          destPath = "$dir/${zipFilename()}";
        });
      } catch (exception) {
        print("Unable to determine downloads dir: $exception");
      }
    }();
  }

  String zipFilename() {
    var nowStr = DateTime.now().toIso8601String().replaceAll(":", "_");
    return "bruig-logs-$nowStr.zip";
  }

  void chooseDestPath() async {
    var dir = await FilePicker.platform.getDirectoryPath(
      dialogTitle: "Select log export dir",
    );
    if (dir == null) return;
    setState(() {
      destPath = "$dir/${zipFilename()}";
    });
  }

  void doExport() async {
    setState(() {
      exporting = true;
    });
    var args = ZipLogsArgs(golibLogs, lnLogs, !allFiles, destPath);
    try {
      await Golib.zipLogs(args);
      if (mounted) {
        showSuccessSnackbar(context, "Exported logs!");
      }
      setState(() {
        // Replace filename to ensure generating again won't overwrite.
        var dir = path.dirname(destPath);
        destPath = "$dir/${zipFilename()}";
      });
    } catch (exception) {
      if (mounted) {
        showErrorSnackbar(context, "Unable to export logs: $exception");
      } else {
        print("Unable to export logs: $exception");
      }
    } finally {
      setState(() {
        exporting = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var backgroundColor = theme.backgroundColor;
    return Scaffold(
        body: Consumer<ThemeNotifier>(
            builder: (context, theme, child) => Container(
                margin: const EdgeInsets.only(
                    left: 1, top: 20, right: 1, bottom: 1),
                decoration: BoxDecoration(
                  color: backgroundColor,
                  borderRadius: BorderRadius.circular(3),
                ),
                padding: const EdgeInsets.all(16),
                child: Column(children: [
                  const SizedBox(height: 20),
                  Text("Export Logs",
                      style: TextStyle(
                          color: textColor,
                          fontSize: theme.getLargeFont(context))),
                  const SizedBox(height: 20),
                  TextButton(
                      onPressed: chooseDestPath,
                      child: destPath != ""
                          ? Text("Export to: $destPath")
                          : const Text("Select Destination")),
                  const SizedBox(height: 20),
                  ToggleButtons(
                      borderRadius: const BorderRadius.all(Radius.circular(8)),
                      constraints:
                          const BoxConstraints(minHeight: 40, minWidth: 100),
                      isSelected: [!allFiles, allFiles],
                      onPressed: (int index) {
                        setState(() {
                          allFiles = index == 1;
                        });
                      },
                      children: const [
                        Text("Latest file"),
                        Text("All files"),
                      ]),
                  ToggleButtons(
                      borderRadius: const BorderRadius.all(Radius.circular(8)),
                      constraints:
                          const BoxConstraints(minHeight: 40, minWidth: 100),
                      isSelected: [!golibLogs, golibLogs],
                      onPressed: (int index) {
                        setState(() {
                          golibLogs = index == 1;
                        });
                      },
                      children: const [
                        Text("No app logs"),
                        Text("App logs"),
                      ]),
                  ToggleButtons(
                      borderRadius: const BorderRadius.all(Radius.circular(8)),
                      constraints:
                          const BoxConstraints(minHeight: 40, minWidth: 100),
                      isSelected: [!lnLogs, lnLogs],
                      onPressed: (int index) {
                        setState(() {
                          lnLogs = index == 1;
                        });
                      },
                      children: const [
                        Text("No LN logs"),
                        Text("LN logs"),
                      ]),
                  const Expanded(child: Empty()),
                  Text(
                      "Note: logs may contain identifying information. Send them only to trusted parties",
                      style: TextStyle(
                          color: textColor,
                          fontSize: 15,
                          decoration: TextDecoration.none)),
                  const SizedBox(height: 20),
                  Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        ElevatedButton(
                            onPressed:
                                destPath != "" && !exporting ? doExport : null,
                            child: const Text("Export")),
                        CancelButton(onPressed: () {
                          Navigator.of(context).pop();
                        }),
                      ]),
                  const SizedBox(height: 20),
                ]))));
  }
}

class LogSettingsScreen extends StatefulWidget {
  static String routeName = "/logSettings";
  const LogSettingsScreen({super.key});

  @override
  State<LogSettingsScreen> createState() => _LogSettingsScreenState();
}

class _LogSettingsScreenState extends State<LogSettingsScreen> {
  List<String> logLevels = ["trace", "debug", "info", "warn"];
  late List<Widget> logLevelWidgets;
  String appLogLevel = "info";
  String lnLogLevel = "info";
  bool logPings = false;

  void readConfig() async {
    var cfg = await loadConfig(mainConfigFilename);
    setState(() {
      if (logLevels.contains(cfg.debugLevel)) {
        appLogLevel = cfg.debugLevel;
      }
      if (logLevels.contains(cfg.lnDebugLevel)) {
        lnLogLevel = cfg.lnDebugLevel;
      }
      logPings = cfg.logPings;
    });
  }

  void doRestart() {
    if (Platform.isAndroid || Platform.isIOS) {
      Restart.restartApp();
    } else {
      SystemNavigator.pop();
    }
  }

  void doChangeConfig() async {
    try {
      await replaceConfig(mainConfigFilename,
          debugLevel: appLogLevel,
          lnDebugLevel: lnLogLevel,
          logPings: logPings);

      if (mounted) {
        confirmationDialog(
            context,
            doRestart,
            "Restart app?",
            "The changes will be applied only after restarting the app.",
            "Restart App",
            "Cancel");
      }
    } catch (exception) {
      showErrorSnackbar(context, "Unable to update config: $exception");
    }
  }

  void confirmApply() {
    confirmationDialog(
        context,
        doChangeConfig,
        "Apply Settings Change",
        "Really apply the changes? After applying the changes, the app will require a restart.",
        "Apply",
        "Cancel");
  }

  @override
  void initState() {
    super.initState();
    logLevelWidgets = logLevels.map((e) => Text(e)).toList();
    readConfig();
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var backgroundColor = theme.backgroundColor;
    return Scaffold(
        body: Consumer<ThemeNotifier>(
            builder: (context, theme, child) => Container(
                margin: const EdgeInsets.only(
                    left: 1, top: 20, right: 1, bottom: 1),
                decoration: BoxDecoration(
                  color: backgroundColor,
                  borderRadius: BorderRadius.circular(3),
                ),
                padding: const EdgeInsets.all(16),
                child: Column(children: [
                  const SizedBox(height: 20),
                  Text("Log Settings",
                      style: TextStyle(
                          color: textColor,
                          fontSize: theme.getLargeFont(context))),
                  const SizedBox(height: 20),
                  Row(children: [
                    SizedBox(
                        width: 100,
                        child: Text("App log level",
                            style: TextStyle(color: textColor))),
                    const SizedBox(width: 20),
                    DropdownButtonHideUnderline(
                      child: DropdownButton2<String>(
                        value: appLogLevel,
                        onChanged: (value) => setState(() {
                          appLogLevel = value ?? "info";
                        }),
                        items: logLevels
                            .map((e) => DropdownMenuItem<String>(
                                  value: e,
                                  child: Text(e),
                                ))
                            .toList(),
                      ),
                    ),
                  ]),
                  Row(children: [
                    SizedBox(
                        width: 100,
                        child: Text("LN log level",
                            style: TextStyle(color: textColor))),
                    const SizedBox(width: 20),
                    DropdownButtonHideUnderline(
                      child: DropdownButton2<String>(
                        value: lnLogLevel,
                        onChanged: (value) => setState(() {
                          lnLogLevel = value ?? "info";
                        }),
                        items: logLevels
                            .map((e) => DropdownMenuItem<String>(
                                  value: e,
                                  child: Text(e),
                                ))
                            .toList(),
                      ),
                    ),
                  ]),
                  /*
                  Row(children: [
                    Checkbox(
                        value: logPings,
                        onChanged: (bool? value) {
                          setState(() {
                            logPings = value ?? false;
                          });
                        }),
                    // const SizedBox(width: 20),
                    Text("Log Pings", style: TextStyle(color: textColor)),
                  ]),
                  */
                  InkWell(
                    onTap: () => setState(() => logPings = !logPings),
                    child: Row(children: [
                      Checkbox(
                        value: logPings,
                        onChanged: (bool? value) =>
                            setState(() => logPings = value ?? false),
                      ),
                      // const SizedBox(width: 20),
                      Text("Log Pings", style: TextStyle(color: textColor)),
                    ]),
                  ),
                  const Expanded(child: Empty()),
                  const SizedBox(height: 20),
                  Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        ElevatedButton(
                            onPressed: confirmApply,
                            child: const Text("Apply")),
                        CancelButton(onPressed: () {
                          Navigator.of(context).pop();
                        }),
                      ]),
                  const SizedBox(height: 20),
                ]))));
  }
}
