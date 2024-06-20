import 'dart:io';
import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/confirmation_dialog.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/recent_log.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/config.dart';
import 'package:bruig/models/log.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/storage_manager.dart';
import 'package:dropdown_button2/dropdown_button2.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:bruig/theme_manager.dart';
import 'package:flutter/services.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:path/path.dart' as path;
import 'package:path_provider/path_provider.dart';
import 'package:restart_app/restart_app.dart';
import 'package:share_plus/share_plus.dart';

Future<String> timedProfilingDir() async {
  bool isMobile = Platform.isIOS || Platform.isAndroid;
  String base = isMobile
      ? (await getApplicationCacheDirectory()).path
      : (await getDownloadsDirectory())?.path ?? "";
  return path.join(base, "perfprofiles");
}

class LogScreenTitle extends StatelessWidget {
  const LogScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return const Text("Logs");
  }
}

class LogScreen extends StatelessWidget {
  static const routeName = '/log';
  final LogModel log;
  const LogScreen(this.log, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    return Container(
        margin: const EdgeInsets.all(1),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(3),
        ),
        padding: const EdgeInsets.all(16),
        child: Column(children: [
          const SizedBox(height: 20),
          const Txt.L("Recent Log"),
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
        ]));
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
  String destProfilingPath = "";
  bool allFiles = false;
  bool golibLogs = true;
  bool lnLogs = true;
  bool exporting = false;
  int debugModeGotoCfgCounter = 0;
  bool timedProfilingEnabled = false;

  @override
  void initState() {
    super.initState();

    // Set default dir. The app should be allowed to write to this on mobile.
    () async {
      try {
        var dir = isMobile
            ? (await getApplicationCacheDirectory()).path
            : (await getDownloadsDirectory())?.path ?? "";
        bool goTimedProfilingEnabled =
            await StorageManager.readBool(StorageManager.goTimedProfilingKey);
        setState(() {
          destPath = path.join(dir, zipFilename());
          destProfilingPath = path.join(dir, profilingZipFilename());
          timedProfilingEnabled = goTimedProfilingEnabled;
        });
      } catch (exception) {
        showErrorSnackbar(
            this, "Unable to determine downloads dir: $exception");
      }
    }();
  }

  String zipFilename() {
    var nowStr = DateTime.now().toIso8601String().replaceAll(":", "_");
    return "bruig-logs-$nowStr.zip";
  }

  String profilingZipFilename() {
    var nowStr = DateTime.now().toIso8601String().replaceAll(":", "_");
    return "bruig-profile-$nowStr.zip";
  }

  void chooseDestPath() async {
    var dir = await FilePicker.platform.getDirectoryPath(
      dialogTitle: "Select log export dir",
    );
    if (dir == null) return;
    setState(() {
      destPath = path.join(dir, zipFilename());
      destProfilingPath = path.join(dir, profilingZipFilename());
    });
  }

  void doExport() async {
    var snackbar = SnackBarModel.of(context);
    setState(() {
      exporting = true;
    });
    var args = ZipLogsArgs(golibLogs, lnLogs, !allFiles, destPath);
    try {
      await Golib.zipLogs(args);
      if (!isMobile) {
        if (mounted) {
          snackbar.success("Exported logs!");
        }
      } else {
        Share.shareXFiles([XFile(destPath)], text: "bruig logs");
      }
      setState(() {
        // Replace filename to ensure generating again won't overwrite.
        var dir = path.dirname(destPath);
        destPath = path.join(dir, zipFilename());
      });
    } catch (exception) {
      snackbar.error("Unable to export logs: $exception");
    } finally {
      setState(() {
        exporting = false;
      });
    }
  }

  void doExportProfilings() async {
    var snackbar = SnackBarModel.of(context);
    setState(() {
      exporting = true;
    });
    try {
      await Golib.zipProfilingLogs(destProfilingPath);
      if (!isMobile) {
        if (mounted) {
          snackbar.success("Exported profiles to $destProfilingPath");
        }
      } else {
        Share.shareXFiles([XFile(destProfilingPath)], text: "bruig profile");
      }
      setState(() {
        // Replace filename to ensure generating again won't overwrite.
        var dir = path.dirname(destProfilingPath);
        destProfilingPath = path.join(dir, profilingZipFilename());
      });
    } catch (exception) {
      snackbar.error("Unable to export profiles: $exception");
    } finally {
      setState(() {
        exporting = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
        body: Container(
            alignment: Alignment.center,
            margin: const EdgeInsets.only(left: 1, top: 5, right: 1, bottom: 1),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(3),
            ),
            padding: const EdgeInsets.all(16),
            child: Column(children: [
              const Txt.L("Export Logs"),
              kReleaseMode
                  ? const SizedBox(height: 20)
                  : InkWell(
                      onTap: () {
                        debugModeGotoCfgCounter += 1;
                        if (debugModeGotoCfgCounter == 3) {
                          showSuccessSnackbar(this,
                              "Going to manual config file with 3 more taps");
                        }
                        if (debugModeGotoCfgCounter == 6) {
                          Navigator.of(context, rootNavigator: true)
                              .pushNamed(ManualCfgModifyScreen.routeName);
                        }
                      },
                      child: const SizedBox(height: 20, width: 40),
                    ),
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
              const Txt.S(
                  "Note: logs may contain identifying information. Send them only to trusted parties",
                  color: TextColor.onSurfaceVariant),
              const SizedBox(height: 20),
              SizedBox(
                  width: 600,
                  child: Wrap(
                      alignment: WrapAlignment.spaceBetween,
                      runSpacing: 5,
                      children: [
                        ElevatedButton(
                            onPressed:
                                destPath != "" && !exporting ? doExport : null,
                            child: const Text("Export")),
                        timedProfilingEnabled
                            ? ElevatedButton(
                                onPressed: destProfilingPath != "" && !exporting
                                    ? doExportProfilings
                                    : null,
                                child:
                                    const Text("Export Performance Profiles"))
                            : const Empty(),
                        CancelButton(onPressed: () {
                          Navigator.of(context).pop();
                        }),
                      ])),
            ])));
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
  bool enableGoProfiler = false;
  bool enableTimedProfiling = false;

  void readConfig() async {
    var cfg = await loadConfig(mainConfigFilename);
    var goProfilerEnabled =
        await StorageManager.readBool(StorageManager.goProfilerEnabledKey);
    var goTimedProfilingEnabled =
        await StorageManager.readBool(StorageManager.goTimedProfilingKey);
    setState(() {
      if (logLevels.contains(cfg.debugLevel)) {
        appLogLevel = cfg.debugLevel;
      }
      if (logLevels.contains(cfg.lnDebugLevel)) {
        lnLogLevel = cfg.lnDebugLevel;
      }
      logPings = cfg.logPings;
      enableGoProfiler = goProfilerEnabled;
      enableTimedProfiling = goTimedProfilingEnabled;
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
    var snackbar = SnackBarModel.of(context);
    try {
      await replaceConfig(mainConfigFilename,
          debugLevel: appLogLevel,
          lnDebugLevel: lnLogLevel,
          logPings: logPings);
      await StorageManager.saveData(
          StorageManager.goProfilerEnabledKey, enableGoProfiler);
      await StorageManager.saveData(
          StorageManager.goTimedProfilingKey, enableTimedProfiling);

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
      snackbar.error("Unable to update config: $exception");
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
    return Scaffold(
        body: Center(
            child: Container(
                width: 600,
                margin:
                    const EdgeInsets.only(left: 1, top: 5, right: 1, bottom: 1),
                padding: const EdgeInsets.all(16),
                child: Column(children: [
                  const Txt.L("Log Settings"),
                  const SizedBox(height: 20),
                  Row(children: [
                    const SizedBox(width: 100, child: Text("App log level")),
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
                    const SizedBox(width: 100, child: Text("LN log level")),
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
                  InkWell(
                    onTap: () => setState(() => logPings = !logPings),
                    child: Row(children: [
                      Checkbox(
                        value: logPings,
                        onChanged: (bool? value) =>
                            setState(() => logPings = value ?? false),
                      ),
                      // const SizedBox(width: 20),
                      const Text("Log Pings"),
                    ]),
                  ),
                  InkWell(
                    onTap: () =>
                        setState(() => enableGoProfiler = !enableGoProfiler),
                    child: Row(children: [
                      Checkbox(
                        value: enableGoProfiler,
                        onChanged: (bool? value) =>
                            setState(() => enableGoProfiler = value ?? false),
                      ),
                      // const SizedBox(width: 20),
                      const Text("Enable Go Profiler"),
                    ]),
                  ),
                  InkWell(
                    onTap: () => setState(
                        () => enableTimedProfiling = !enableTimedProfiling),
                    child: Row(children: [
                      Checkbox(
                        value: enableTimedProfiling,
                        onChanged: (bool? value) => setState(
                            () => enableTimedProfiling = value ?? false),
                      ),
                      // const SizedBox(width: 20),
                      const Text("Enable Continous Hourly Profiling"),
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
                ]))));
  }
}

class ManualCfgModifyScreen extends StatefulWidget {
  static String routeName = "/manualScreenModify";
  const ManualCfgModifyScreen({super.key});

  @override
  State<ManualCfgModifyScreen> createState() => _ManualCfgModifyScreenState();
}

class _ManualCfgModifyScreenState extends State<ManualCfgModifyScreen> {
  TextEditingController txtCtrl = TextEditingController();

  void loadConfig() async {
    var content = await File(mainConfigFilename).readAsString();
    setState(() {
      txtCtrl.text = content;
    });
  }

  void doOverwriteConfig() async {
    var snackbar = SnackBarModel.of(context);
    try {
      await File(mainConfigFilename).writeAsString(txtCtrl.text);
      if (mounted) {
        confirmationDialog(context, () {
          if (Platform.isAndroid || Platform.isIOS) {
            Restart.restartApp();
          } else {
            SystemNavigator.pop();
          }
        },
            "Restart app?",
            "The changes will be applied only after restarting the app.",
            "Restart App",
            "Cancel");
      }
    } catch (exception) {
      snackbar.error("Unable to modify config file: $exception");
    }
  }

  void confirmAply() async {
    confirmationDialog(
        context,
        doOverwriteConfig,
        "Overwrite config file",
        "Really modify the config file?\n\nTHIS MAY MAKE THE APP UNABLE TO RUN",
        "Overwite",
        "Cancel");
  }

  @override
  void initState() {
    super.initState();
    loadConfig();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
        body: Container(
            margin: const EdgeInsets.only(left: 1, top: 5, right: 1, bottom: 1),
            padding: const EdgeInsets.all(16),
            child: Column(children: [
              const Txt.L("Manual Config Modification"),
              const SizedBox(height: 20),
              Expanded(
                  child: TextField(
                controller: txtCtrl,
                maxLines: null,
              )),
              const SizedBox(height: 20),
              Row(mainAxisAlignment: MainAxisAlignment.spaceBetween, children: [
                ElevatedButton(
                    onPressed: confirmAply, child: const Text("Apply")),
                CancelButton(onPressed: () {
                  Navigator.of(context).pop();
                }),
              ]),
            ])));
  }
}
