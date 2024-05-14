import 'dart:io';
import 'dart:math';

import 'package:bruig/config.dart';
import 'package:bruig/storage_manager.dart';
import 'package:bruig/wordlist.dart';
import 'package:bruig/screens/unlock_ln.dart';
import 'package:bruig/main.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter/services.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:io/io.dart';
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

class ConfirmSeedWords {
  final int position;
  final String correctSeedWord;
  final List<String> seedWordChoices;

  ConfirmSeedWords(this.position, this.correctSeedWord, this.seedWordChoices);
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
  List<String> seedToRestore = [];
  Uint8List? multichanBackupRestore;
  List<ConfirmSeedWords> confirmSeedWords = [];

  String proxyAddr = "";
  String proxyUser = "";
  String proxyPassword = "";
  int torCircuitLimit = 32;
  bool torIsolation = false;

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
      proxyaddr: proxyAddr,
      proxyUsername: proxyUser,
      proxyPassword: proxyPassword,
      circuitLimit: torCircuitLimit,
      torIsolation: torIsolation,
    );
    await cfg.saveNewConfig(await configFileName(appArgs));
    cfg = await configFromArgs(appArgs); // Reload to fill defaults.
    cfg = Config.newWithRPCHost(cfg, rpcHost, tlsCertPath, macaroonPath);

    // Flutter App settings.
    var isMobile = Platform.isAndroid || Platform.isIOS;

    // Set notifications as enabled by default on mobile because app goes to
    // background and is hidden. Enable foreground service on android by default
    // because it continues to work even after flutter is detached.
    StorageManager.saveData(StorageManager.notificationsKey, isMobile);
    StorageManager.saveData(StorageManager.ntfnFgSvcKey, Platform.isAndroid);

    return cfg;
  }

  List<ConfirmSeedWords> createConfirmSeedWords(String seed) {
    List<ConfirmSeedWords> confirmSeedWords = [];
    var seedWords = seed.trim().split(' ');
    var numWords = 5;
    var numChoices = 3;
    for (int i = 0; i < numWords; i++) {
      int position;
      bool positionUsed;
      // Keep generating new positions until we have one that isn't used yet.
      do {
        positionUsed = false;
        position = Random().nextInt(seedWords.length);
        for (int k = 0; k < confirmSeedWords.length; k++) {
          if (position == confirmSeedWords[k].position) {
            positionUsed = true;
          }
        }
      } while (positionUsed);
      List<String> seedWordChoices = [seedWords[position]];
      // Keep generating new words in the seedWordChoice list until its length
      // is equal to the number of choices set above.
      do {
        var randomSeedWord = Random().nextInt(defaultWordList.length);
        var found = false;
        for (int j = 0; j < seedWordChoices.length; j++) {
          if (defaultWordList[randomSeedWord] == seedWordChoices[j]) {
            // Skip word if it's already in the list.
            found = true;
          }
        }
        if (!found) {
          seedWordChoices.add(defaultWordList[randomSeedWord]);
        }
      } while (seedWordChoices.length < numChoices);
      // Sort the word choices alphabetically each
      seedWordChoices.sort((a, b) {
        return a.toLowerCase().compareTo(b.toLowerCase());
      });
      confirmSeedWords.add(
          ConfirmSeedWords(position, seedWords[position], seedWordChoices));
    }
    // Sort the questions by position
    confirmSeedWords.sort((a, b) {
      return a.position.compareTo(b.position);
    });
    return confirmSeedWords;
  }

  Future<void> createNewWallet(
      String password, List<String> existingSeed) async {
    var rootPath = await lnWalletDir();
    await Directory(rootPath).create(recursive: true);
    var res = await Golib.lnInitDcrlnd(
        rootPath,
        NetworkTypeStr(netType),
        password,
        existingSeed,
        multichanBackupRestore,
        proxyAddr,
        torIsolation,
        proxyUser,
        proxyPassword,
        torCircuitLimit,
        true, // syncfreelist
        true, // autocompact
        60 * 60 * 24 * 14, // autocompact_min_age (14 days)
        "info");
    tlsCertPath = path.join(rootPath, "tls.cert");
    macaroonPath = path.join(rootPath, "data", "chain", "decred",
        NetworkTypeStr(netType), "admin.macaroon");
    rpcHost = res.rpcHost;
    newWalletSeed = res.seed;
    confirmSeedWords = createConfirmSeedWords(newWalletSeed);
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

  Future<bool> hasOldVersionWindowsWalletDB() async {
    if (Platform.isWindows &&
        Platform.environment.containsKey("LOCALAPPDATA")) {
      var oldVersionConfigFile = path.join(
          Platform.environment["LOCALAPPDATA"]!,
          "Packages",
          "com.flutter.bruig_ywj3797wkq8tj",
          "LocalCache",
          "Local",
          "${APPNAME}",
          "${APPNAME}.conf");
      if (File(oldVersionConfigFile).existsSync()) {
        return true;
      }
    }
    return false;
  }

  Future<void> moveOldWalletVersion() async {
    //var cfgFile = path.join(Platform.environment["LOCALAPPDATA"]!, APPNAME);
    if (await hasLNWalletDB()) {
      print("Can't move old windows wallet to better location");
      throw unableToMoveOldWallet;
    }
    var oldPath = path.join(Platform.environment["LOCALAPPDATA"]!, "Packages",
        "com.flutter.bruig_ywj3797wkq8tj", "LocalCache", "Local", APPNAME);
    var oldPathCopied = path.join(
        Platform.environment["LOCALAPPDATA"]!,
        "Packages",
        "com.flutter.bruig_ywj3797wkq8tj",
        "LocalCache",
        "Local",
        "${APPNAME}_copied");
    var newPath = path.join(Platform.environment["LOCALAPPDATA"]!, APPNAME);

    print("Moving old windows wallet to better location");
    // Copy data to new location.
    await copyPath(oldPath, newPath);

    // Rename old data directory.
    var oldPathDir = Directory(oldPath);
    await oldPathDir.rename(oldPathCopied);

    Config cfg = await configFromArgs([]);
    await Golib.createLockFile(cfg.dbRoot);
    if (cfg.walletType == "internal") {
      await runUnlockDcrlnd(cfg);
      return;
    }
    await runMainApp(cfg);
  }

  Future<void> deleteLNWalletDir() async {
    var dir = await lnWalletDir();
    await File(dir).delete(recursive: true);
  }
}
