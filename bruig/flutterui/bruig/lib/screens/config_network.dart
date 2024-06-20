import 'dart:io';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/confirmation_dialog.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/config.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import 'package:restart_app/restart_app.dart';

final _nonDigitsRegexp = RegExp(r'[^0-9]');

class ConfigNetworkScreen extends StatefulWidget {
  static const String routeName = "/configNetwork";
  final NewConfigModel? newConf;
  const ConfigNetworkScreen({this.newConf, super.key});

  @override
  State<ConfigNetworkScreen> createState() => _ConfigNetworkScreenState();
}

class _ConfigNetworkScreenState extends State<ConfigNetworkScreen> {
  NewConfigModel? get newConfigModel => widget.newConf;
  TextEditingController proxyAddrCtrl = TextEditingController();
  TextEditingController proxyUserCtrl = TextEditingController();
  TextEditingController proxyPwdCtrl = TextEditingController();
  TextEditingController torCirtuitLimitCtrl = TextEditingController();
  bool torCircuitIsolation = false;

  void doRestart() {
    if (Platform.isAndroid || Platform.isIOS) {
      Restart.restartApp();
    } else {
      SystemNavigator.pop();
    }
  }

  void changeConfig() async {
    await replaceConfig(
      mainConfigFilename,
      proxyAddr: proxyAddrCtrl.text,
      proxyUsername: proxyUserCtrl.text,
      proxyPassword: proxyPwdCtrl.text,
      torCircuitLimit: torCirtuitLimitCtrl.text == ""
          ? 32
          : int.parse(torCirtuitLimitCtrl.text),
      torIsolation: torCircuitIsolation,
    );
    if (!mounted) return;
    confirmationDialog(
      context,
      doRestart,
      "Restart App?",
      "App restart is required to apply network settings changes.",
      "Restart",
      "Cancel",
      onCancel: () {
        Navigator.of(context).pop();
      },
    );
  }

  void confirmAcceptChanges() {
    // When setting up a new wallet, just accept the config.
    if (newConfigModel != null) {
      var newConfigModel = Provider.of<NewConfigModel>(context, listen: false);
      newConfigModel.proxyAddr = proxyAddrCtrl.text;
      newConfigModel.proxyUser = proxyUserCtrl.text;
      newConfigModel.proxyPassword = proxyPwdCtrl.text;
      newConfigModel.torCircuitLimit = int.parse(torCirtuitLimitCtrl.text);
      newConfigModel.torIsolation = torCircuitIsolation;
      Navigator.of(context).pop();
      return;
    }

    confirmationDialog(
        context,
        changeConfig,
        onCancel: () => Navigator.of(context).pop(),
        "Change Config?",
        "Change network config? To apply the changes, the app will require a restart.",
        "Accept",
        "Cancel");
  }

  void readConfig() async {
    if (newConfigModel != null) {
      setState(() {
        proxyAddrCtrl.text = newConfigModel!.proxyAddr;
        proxyUserCtrl.text = newConfigModel!.proxyUser;
        proxyPwdCtrl.text = newConfigModel!.proxyPassword;
        torCirtuitLimitCtrl.text = newConfigModel!.torCircuitLimit.toString();
        torCircuitIsolation = newConfigModel!.torIsolation;
      });
      return;
    }

    var cfg = await loadConfig(mainConfigFilename);
    setState(() {
      proxyAddrCtrl.text = cfg.proxyaddr;
      proxyUserCtrl.text = cfg.proxyUsername;
      proxyPwdCtrl.text = cfg.proxyPassword;
      torCirtuitLimitCtrl.text = cfg.circuitLimit.toString();
      torCircuitIsolation = cfg.torIsolation;
    });
  }

  @override
  void initState() {
    super.initState();
    readConfig();
  }

  @override
  Widget build(BuildContext context) {
    return StartupScreen(childrenWidth: 400, [
      const Txt.H("Configure Network Options"),
      const SizedBox(height: 20),
      TextField(
          controller: proxyAddrCtrl,
          decoration: const InputDecoration(
              labelText: "Proxy Address", hintText: "127.0.0.1:9050")),
      const SizedBox(height: 10),
      TextField(
          controller: proxyUserCtrl,
          decoration: const InputDecoration(
              labelText: "Proxy Username", hintText: "proxyuser")),
      const SizedBox(height: 10),
      TextField(
          controller: proxyPwdCtrl,
          decoration: const InputDecoration(
              labelText: "Proxy Password", hintText: "proxypass")),
      const SizedBox(height: 10),
      TextField(
          keyboardType: TextInputType.number,
          controller: torCirtuitLimitCtrl,
          decoration: const InputDecoration(
              labelText: "Tor Circuit Limit", hintText: "32"),
          onChanged: (value) {
            if (value.contains(_nonDigitsRegexp)) {
              torCirtuitLimitCtrl.text = value.replaceAll(_nonDigitsRegexp, "");
            }
          }),
      const SizedBox(height: 20),
      SizedBox(
          width: 230,
          child: InkWell(
            onTap: () =>
                setState(() => torCircuitIsolation = !torCircuitIsolation),
            child: Row(children: [
              Checkbox(
                value: torCircuitIsolation,
                onChanged: (bool? value) =>
                    setState(() => torCircuitIsolation = value ?? false),
              ),
              const Text("Tor Circuit Isolation"),
            ]),
          )),
      const SizedBox(height: 30),
      Wrap(runSpacing: 10, children: [
        OutlinedButton(
            onPressed: confirmAcceptChanges, child: const Text("Accept")),
        const SizedBox(width: 50),
        CancelButton(onPressed: () => Navigator.pop(context)),
      ]),
    ]);
  }
}
