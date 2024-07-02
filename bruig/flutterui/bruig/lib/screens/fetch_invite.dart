import 'dart:async';
import 'dart:io';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:loading_animation_widget/loading_animation_widget.dart';
import 'package:mobile_scanner/mobile_scanner.dart';
import 'package:path_provider/path_provider.dart';
import 'package:path/path.dart' as path;

typedef _BuildCB = Widget Function(BuildContext);

class _InviteMode {
  final String label;
  final String mode;
  final _BuildCB builder;
  bool selected;

  _InviteMode(this.label, this.mode, this.builder, {this.selected = false});
}

typedef OnInviteChanged = Function(String? invitePath, String? key);

class InvitePanel extends StatefulWidget {
  final OnInviteChanged onInviteChanged;
  final bool allowFile;
  const InvitePanel(this.onInviteChanged, {this.allowFile = false, super.key});

  @override
  State<InvitePanel> createState() => _InvitePanelState();
}

class _InvitePanelState extends State<InvitePanel> {
  final bool isMobile = Platform.isAndroid || Platform.isIOS;
  int selInviteMode = 0;
  List<_InviteMode> inviteModes = [];

  MobileScannerController? scannerCtrl;
  StreamSubscription<Object?>? scannerStream;

  String path = "";
  TextEditingController keyCtrl = TextEditingController();

  void handleScanned(BarcodeCapture event) {
    if (event.barcodes.isEmpty) {
      return;
    }
    if (event.barcodes[0].rawValue == null) {
      return;
    }
    String key = event.barcodes[0].rawValue!;
    if (!key.startsWith("brpik1")) {
      return;
    }
    setState(() {
      keyCtrl.text = key;
      selInviteMode = 0;
      for (var im in inviteModes) {
        im.selected = false;
      }
      inviteModes[selInviteMode].selected = true;
    });
    widget.onInviteChanged(null, key);
  }

  @override
  void initState() {
    super.initState();

    inviteModes = [
      _InviteMode("Key", "key", buildPanelKey, selected: true),
      ...(isMobile ? [_InviteMode("Camera", "camera", buildPanelCamera)] : []),
      ...(widget.allowFile
          ? [_InviteMode("File", "file", buildPanelFile)]
          : []),
    ];

    if (isMobile) {
      scannerCtrl = MobileScannerController();
      scannerStream = scannerCtrl?.barcodes.listen(handleScanned);
      scannerCtrl?.start();
    }
  }

  @override
  void dispose() async {
    scannerStream?.cancel();
    scannerCtrl?.dispose();
    super.dispose();
  }

  void selectPath() async {
    var res = await FilePicker.platform.pickFiles(
      dialogTitle: "Select invitation file location",
      allowMultiple: false,
    );
    if (res == null) return;
    if (res.count == 0) return;
    setState(() {
      path = res.files[0].path!;
    });
    widget.onInviteChanged(path, null);
  }

  Widget buildPanelKey(BuildContext context) {
    return TextField(
      onChanged: (v) => widget.onInviteChanged(null, v),
      controller: keyCtrl,
      decoration: const InputDecoration(
        hintText: "Input key (bpik1...)",
      ),
    );
  }

  Widget buildPanelCamera(BuildContext context) {
    return MobileScanner(
      // fit: BoxFit.contain,
      controller: scannerCtrl!,
      errorBuilder: (context, error, child) {
        return Text("Scanning error: $error");
      },
    );
  }

  Widget buildPanelFile(BuildContext context) {
    return Column(children: [
      ElevatedButton(
        onPressed: selectPath,
        child: Text(path != "" ? path : "Select Path"),
      )
    ]);
  }

  @override
  Widget build(BuildContext context) {
    return Column(children: [
      ToggleButtons(
          onPressed: (index) {
            setState(() {
              for (var im in inviteModes) {
                im.selected = false;
              }
              inviteModes[index].selected = true;
              selInviteMode = index;
            });
          },
          borderRadius: const BorderRadius.all(Radius.circular(8)),
          constraints: const BoxConstraints(minHeight: 40, minWidth: 80),
          // color: Colors.blue,
          isSelected: inviteModes.map((e) => e.selected).toList(),
          children: inviteModes.map((e) => Text(e.label)).toList()),
      const SizedBox(height: 20),
      SizedBox(
        width: 300,
        height: 300,
        child: inviteModes[selInviteMode].builder(context),
      ),
    ]);
  }
}

Future<String> _tempInviteDownloadPath() async {
  bool isMobile = Platform.isIOS || Platform.isAndroid;
  String base = isMobile
      ? (await getApplicationCacheDirectory()).path
      : (await getDownloadsDirectory())?.path ?? "";
  var dir = path.join(base, "invites");
  if (!Directory(dir).existsSync()) Directory(dir).createSync();
  var nowStr = DateTime.now().toIso8601String().replaceAll(":", "_");
  return path.join(dir, "$nowStr.brinvite");
}

class FetchInviteScreen extends StatefulWidget {
  const FetchInviteScreen({super.key});

  @override
  State<FetchInviteScreen> createState() => _FetchInviteScreenState();
}

class _FetchInviteScreenState extends State<FetchInviteScreen> {
  bool loading = false;
  String? invitePath;
  String? inviteKey;

  void onInviteChanged(String? newInvitePath, String? newInviteKey) {
    setState(() {
      invitePath = newInvitePath;
      inviteKey = newInviteKey;
    });
  }

  void loadInvite() async {
    var snackbar = SnackBarModel.of(context);
    setState(() => loading = true);
    try {
      var key = inviteKey ?? "";
      var path = invitePath ?? await _tempInviteDownloadPath();
      var invite = await Future.any([
        Golib.fetchInvite(key, path),
        Future.delayed(const Duration(seconds: 30), () => null)
      ]);
      if (invite == null) {
        throw "No reply after 30 seconds - invite not sent or already fetched";
      }
      if (mounted) {
        Navigator.of(context, rootNavigator: true)
            .pushReplacementNamed("/verifyInvite", arguments: invite);
      }
    } catch (exception) {
      if (mounted) {
        snackbar.error("Unable to fetch invite: $exception");
        setState(() => loading = false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return StartupScreen([
      const Txt.H("Fetch Invite"),
      const SizedBox(height: 20),
      ...(!loading
          ? [
              InvitePanel(onInviteChanged, allowFile: true),
              const SizedBox(height: 20),
              OutlinedButton(
                  onPressed: !loading && ((invitePath ?? inviteKey ?? "") != "")
                      ? loadInvite
                      : null,
                  child: const Text("Fetch invite")),
              const SizedBox(height: 10),
            ]
          : [
              LoadingAnimationWidget.threeArchedCircle(
                //color: theme.getTheme().dividerColor,
                color: Colors.amber,
                size: 50,
              ),
              const SizedBox(height: 20),
            ]),
      CancelButton(onPressed: () => Navigator.pop(context)),
    ]);
  }
}
