import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

class FetchInviteScreen extends StatefulWidget {
  const FetchInviteScreen({super.key});

  @override
  State<FetchInviteScreen> createState() => _FetchInviteScreenState();
}

class _FetchInviteScreenState extends State<FetchInviteScreen> {
  String path = "";
  TextEditingController keyCtrl = TextEditingController();
  bool loading = false;
  bool hasKey = false;

  @override
  void initState() {
    super.initState();
    keyCtrl.addListener(() {
      if (hasKey != keyCtrl.text.isNotEmpty) {
        setState(() {
          hasKey = keyCtrl.text.isNotEmpty;
        });
      }
    });
  }

  void selectPath() async {
    var filePath = await FilePicker.platform.saveFile(
      dialogTitle: "Select invitation file location",
      fileName: "invite.bin",
    );
    if (filePath == null) return;
    setState(() {
      path = filePath;
    });
  }

  void loadInvite() async {
    setState(() => loading = true);
    try {
      var key = keyCtrl.text;
      var res = await Future.any([
        Golib.fetchInvite(key, path),
        Future.delayed(const Duration(seconds: 30), () => null)
      ]);
      if (res == null) {
        throw "No reply after 30 seconds - invite not sent or already fetched";
      }
      var invite = res as Invitation;
      if (mounted) {
        Navigator.of(context, rootNavigator: true)
            .pushReplacementNamed("/verifyInvite", arguments: invite);
      }
    } catch (exception) {
      if (mounted) {
        showErrorSnackbar(context, "Unable to fetch invite: $exception");
        setState(() => loading = false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => StartupScreen([
              Text("Fetch Invite",
                  style: TextStyle(
                      color: theme.getTheme().dividerColor,
                      fontSize: theme.getHugeFont(context),
                      fontWeight: FontWeight.w200)),
              const SizedBox(height: 20),
              SizedBox(
                  width: 400,
                  child: path != ""
                      ? Center(
                          child: Text(
                          "Save to: $path",
                          style:
                              TextStyle(color: theme.getTheme().dividerColor),
                        ))
                      : ElevatedButton(
                          onPressed: selectPath,
                          child: const Text("Select Path"))),
              const SizedBox(height: 20),
              SizedBox(
                width: 400,
                child: TextField(
                  controller: keyCtrl,
                  decoration:
                      const InputDecoration(hintText: "Input key (bpik1...)"),
                ),
              ),
              const SizedBox(height: 10),
              ElevatedButton(
                  onPressed:
                      !loading && path != "" && hasKey ? loadInvite : null,
                  child: const Text("Fetch invite")),
              const SizedBox(height: 10),
              ElevatedButton(
                  onPressed: () => Navigator.pop(context),
                  child: const Text("Cancel"))
            ]));
  }
}
