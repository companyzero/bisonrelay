import 'dart:io';

import 'package:bruig/config.dart';
import 'package:flutter/cupertino.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:path/path.dart' as path;

enum LNNodeType { internal, external }

enum NetworkType { mainnet, testnet, simnet }

String NetworkTypeStr(NetworkType net) {
  switch (net) {
    case NetworkType.mainnet:
      return "mainnet";
    case NetworkType.testnet:
      return "testnet";
    case NetworkType.simnet:
      return "simnet";
  }
}

class NewConfigModel extends ChangeNotifier {
  final List<String> appArgs;
  NewConfigModel(this.appArgs);

  Future<String> appDataDir() async =>
      path.dirname(await configFileName(appArgs));
  Future<String> lnWalletDir() async =>
      path.join(await appDataDir(), "ln-wallet");

  LNNodeType nodeType = LNNodeType.internal;
  NetworkType netType = NetworkType.mainnet;

  String rpcHost = "";
  String tlsCertPath = "";
  String macaroonPath = "";
  String serverAddr = "";
  String newWalletSeed = "";
  bool advancedSetup = false;

  Future<LNInfo> tryExternalDcrlnd(
      String host, String tlsPath, String macaroonPath) async {
    var res = await Golib.lnTryExternalDcrlnd(host, tlsPath, macaroonPath);
    this.rpcHost = host;
    this.tlsCertPath = tlsPath;
    this.macaroonPath = macaroonPath;
    return res;
  }

  Future<Config> generateConfig() async {
    var dataDir = await appDataDir();
    var cfg = Config.filled(
      appDataDir: dataDir,
      serverAddr: serverAddr,
      lnRPCHost: rpcHost,
      lnTLSCert: tlsCertPath,
      lnMacaroonPath: macaroonPath,
      walletType: newWalletSeed != "" ? "internal" : "external",
      network: NetworkTypeStr(netType),
    );
    await cfg.saveConfig(await configFileName(appArgs));
    cfg = await configFromArgs(appArgs); // Reload to fill defaults.
    cfg = Config.newWithRPCHost(cfg, rpcHost, tlsCertPath, macaroonPath);
    return cfg;
  }

  Future<void> createNewWallet(String password) async {
    var rootPath = await lnWalletDir();
    await Directory(rootPath).create(recursive: true);
    var res =
        await Golib.lnInitDcrlnd(rootPath, NetworkTypeStr(netType), password);
    tlsCertPath = path.join(rootPath, "tls.cert");
    macaroonPath = path.join(rootPath, "data", "chain", "decred",
        NetworkTypeStr(netType), "admin.macaroon");
    rpcHost = res.rpcHost;
    newWalletSeed = res.seed;
  }

  Future<bool> hasLNWalletDB() async {
    // Check for any of the networks.
    for (var net in NetworkType.values) {
      var fname = path.join(await lnWalletDir(), "data", "chain", "decred",
          NetworkTypeStr(net), "wallet.db");
      if (File(fname).existsSync()) {
        netType = net;
        return true;
      }
    }

    return false;
  }

  Future<void> deleteLNWalletDir() async {
    var dir = await lnWalletDir();
    await File(dir).delete(recursive: true);
  }
}
