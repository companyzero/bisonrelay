import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/snackbars.dart';
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
    var backgroundColor = const Color(0xFF19172C);
    var cardColor = const Color(0xFF05031A);
    var textColor = const Color(0xFF8E8D98);
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Scaffold(
            body: Container(
                color: backgroundColor,
                child: Stack(children: [
                  Container(
                      decoration: const BoxDecoration(
                          image: DecorationImage(
                              fit: BoxFit.fill,
                              image:
                                  AssetImage("assets/images/loading-bg.png")))),
                  Center(
                      child: Container(
                          decoration: BoxDecoration(
                              gradient: LinearGradient(
                                  begin: Alignment.bottomLeft,
                                  end: Alignment.topRight,
                                  colors: [
                                cardColor,
                                const Color(0xFF07051C),
                                backgroundColor.withOpacity(0.34),
                              ],
                                  stops: const [
                                0,
                                0.17,
                                1
                              ])),
                          padding: const EdgeInsets.all(10),
                          child: Column(
                              mainAxisAlignment: MainAxisAlignment.center,
                              children: [
                                const Expanded(child: Empty()),
                                Text("Fetch Invite",
                                    style: TextStyle(
                                        color: textColor,
                                        fontSize: theme.getHugeFont(),
                                        fontWeight: FontWeight.w200)),
                                const SizedBox(height: 20),
                                SizedBox(
                                    width: 400,
                                    child: path != ""
                                        ? Center(
                                            child: Text(
                                            "Save to: $path",
                                            style: TextStyle(color: textColor),
                                          ))
                                        : ElevatedButton(
                                            onPressed: selectPath,
                                            child: const Text("Select Path"))),
                                const SizedBox(height: 20),
                                SizedBox(
                                  width: 400,
                                  child: TextField(
                                    controller: keyCtrl,
                                    decoration: const InputDecoration(
                                        hintText: "Input key (bpik1...)"),
                                  ),
                                ),
                                const SizedBox(height: 10),
                                ElevatedButton(
                                    onPressed: !loading && path != "" && hasKey
                                        ? loadInvite
                                        : null,
                                    child: const Text("Fetch invite")),
                                const Expanded(child: Empty()),
                                ElevatedButton(
                                    onPressed: () => Navigator.pop(context),
                                    child: const Text("Cancel"))
                              ])))
                ]))));
  }
}
