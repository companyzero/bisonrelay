import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/confirmation_dialog.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/config.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/screens/shutdown.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

class RpcConfigScreen extends StatefulWidget {
  static const String routeName = "/rpcConfig";
  final NewConfigModel? newConf;
  const RpcConfigScreen({this.newConf, super.key});

  @override
  State<RpcConfigScreen> createState() => _RpcConfigScreenState();
}

class _RpcConfigScreenState extends State<RpcConfigScreen> {
  NewConfigModel? get newConfigModel => widget.newConf;
  TextEditingController rpcListenCtrl = TextEditingController();
  TextEditingController rpcCertPathCtrl = TextEditingController();
  TextEditingController rpcKeyPathCtrl = TextEditingController();
  TextEditingController rpcClientCACtrl = TextEditingController();
  TextEditingController rpcUserCtrl = TextEditingController();
  TextEditingController rpcPassCtrl = TextEditingController();
  TextEditingController rpcAuthModeCtrl = TextEditingController();
  TextEditingController rpcMaxRemoteSendTipAmtCtrl = TextEditingController();
  bool rpcIssueClientCert = false;
  bool rpcAllowRemoteSendTip = false;

  void doRestart() {
    ShutdownScreen.startShutdown(context, restart: true);
    Navigator.pop(context);
  }

  void changeConfig() async {
    await replaceConfig(
      mainConfigFilename,
      jsonRPCListen: rpcListenCtrl.text,
      rpcCertPath: rpcCertPathCtrl.text,
      rpcKeyPath: rpcKeyPathCtrl.text,
      rpcClientCApath: rpcClientCACtrl.text,
      rpcUser: rpcUserCtrl.text,
      rpcPass: rpcPassCtrl.text,
      rpcAuthMode: rpcAuthModeCtrl.text,
      rpcIssueClientCert: rpcIssueClientCert,
      rpcAllowRemoteSendTip: rpcAllowRemoteSendTip,
      rpcMaxRemoteSendTipAmt:
          double.tryParse(rpcMaxRemoteSendTipAmtCtrl.text) ?? 0,
    );
    if (!mounted) return;
    confirmationDialog(
      context,
      doRestart,
      "Restart App?",
      "App restart is required to apply RPC settings changes.",
      "Restart",
      "Cancel",
      onCancel: () {
        Navigator.of(context).pop();
      },
    );
  }

  void confirmAcceptChanges() {
    if (newConfigModel != null) {
      var newConfigModel = Provider.of<NewConfigModel>(context, listen: false);
      newConfigModel.jsonRPCListen =
          rpcListenCtrl.text.split(',').map((addr) => addr.trim()).toList();
      newConfigModel.rpcCertPath = rpcCertPathCtrl.text;
      newConfigModel.rpcKeyPath = rpcKeyPathCtrl.text;
      newConfigModel.rpcClientCApath = rpcClientCACtrl.text;
      newConfigModel.rpcUser = rpcUserCtrl.text;
      newConfigModel.rpcPass = rpcPassCtrl.text;
      newConfigModel.rpcAuthMode = rpcAuthModeCtrl.text;
      newConfigModel.rpcIssueClientCert = rpcIssueClientCert;
      newConfigModel.rpcAllowRemoteSendTip = rpcAllowRemoteSendTip;
      newConfigModel.rpcMaxRemoteSendTipAmt =
          double.tryParse(rpcMaxRemoteSendTipAmtCtrl.text) ?? 0;
      Navigator.of(context).pop();
      return;
    }

    confirmationDialog(
        context,
        changeConfig,
        onCancel: () => Navigator.of(context).pop(),
        "Change Config?",
        "Change RPC config? To apply the changes, the app will require a restart.",
        "Accept",
        "Cancel");
  }

  void readConfig() async {
    if (newConfigModel != null) {
      setState(() {
        rpcListenCtrl.text = newConfigModel!.jsonRPCListen.join(", ");
        rpcCertPathCtrl.text = newConfigModel!.rpcCertPath;
        rpcKeyPathCtrl.text = newConfigModel!.rpcKeyPath;
        rpcClientCACtrl.text = newConfigModel!.rpcClientCApath;
        rpcUserCtrl.text = newConfigModel!.rpcUser;
        rpcPassCtrl.text = newConfigModel!.rpcPass;
        rpcAuthModeCtrl.text = newConfigModel!.rpcAuthMode;
        rpcIssueClientCert = newConfigModel!.rpcIssueClientCert;
        rpcAllowRemoteSendTip = newConfigModel!.rpcAllowRemoteSendTip;
        rpcMaxRemoteSendTipAmtCtrl.text =
            newConfigModel!.rpcMaxRemoteSendTipAmt.toString();
      });
      return;
    }

    var cfg = await loadConfig(mainConfigFilename);
    setState(() {
      rpcListenCtrl.text = cfg.jsonRPCListen.join(", ");
      rpcCertPathCtrl.text = cfg.rpcCertPath;
      rpcKeyPathCtrl.text = cfg.rpcKeyPath;
      rpcClientCACtrl.text = cfg.rpcClientCApath;
      rpcUserCtrl.text = cfg.rpcUser;
      rpcPassCtrl.text = cfg.rpcPass;
      rpcAuthModeCtrl.text = cfg.rpcAuthMode;
      rpcIssueClientCert = cfg.rpcIssueClientCert;
      rpcAllowRemoteSendTip = cfg.rpcAllowRemoteSendTip;
      rpcMaxRemoteSendTipAmtCtrl.text = cfg.rpcMaxRemoteSendTipAmt.toString();
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
      const Txt.H("Configure RPC Options"),
      const SizedBox(height: 20),
      TextField(
          controller: rpcListenCtrl,
          decoration: const InputDecoration(
              labelText: "JSON-RPC Listen Address",
              hintText: "127.0.0.1:7676")),
      const SizedBox(height: 10),
      TextField(
          controller: rpcCertPathCtrl,
          decoration: const InputDecoration(
              labelText: "RPC Certificate Path", hintText: "/path/to/cert")),
      const SizedBox(height: 10),
      TextField(
          controller: rpcKeyPathCtrl,
          decoration: const InputDecoration(
              labelText: "RPC Key Path", hintText: "/path/to/key")),
      const SizedBox(height: 10),
      TextField(
          controller: rpcClientCACtrl,
          decoration: const InputDecoration(
              labelText: "RPC Client CA Path", hintText: "/path/to/ca")),
      const SizedBox(height: 10),
      TextField(
          controller: rpcUserCtrl,
          decoration: const InputDecoration(
              labelText: "RPC Username", hintText: "rpcuser")),
      const SizedBox(height: 10),
      TextField(
          controller: rpcPassCtrl,
          decoration: const InputDecoration(
              labelText: "RPC Password", hintText: "rpcpass")),
      const SizedBox(height: 10),
      TextField(
          controller: rpcAuthModeCtrl,
          decoration: const InputDecoration(
              labelText: "RPC Auth Mode", hintText: "authmode")),
      const SizedBox(height: 10),
      TextField(
          keyboardType: TextInputType.number,
          controller: rpcMaxRemoteSendTipAmtCtrl,
          decoration: const InputDecoration(
              labelText: "Max Remote Send Tip Amount", hintText: "0.0")),
      const SizedBox(height: 20),
      SwitchListTile(
        title: const Text("Issue Client Certificate"),
        value: rpcIssueClientCert,
        onChanged: (value) => setState(() => rpcIssueClientCert = value),
      ),
      const SizedBox(height: 10),
      SwitchListTile(
        title: const Text("Allow Remote Send Tip"),
        value: rpcAllowRemoteSendTip,
        onChanged: (value) => setState(() => rpcAllowRemoteSendTip = value),
      ),
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
