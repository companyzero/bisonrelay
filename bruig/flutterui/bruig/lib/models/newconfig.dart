import 'dart:io';
import 'dart:math';

import 'package:bruig/config.dart';
import 'package:bruig/wordlist.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter/services.dart';
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
    var res = await Golib.lnInitDcrlnd(rootPath, NetworkTypeStr(netType),
        password, existingSeed, multichanBackupRestore);
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

  Future<void> deleteLNWalletDir() async {
    var dir = await lnWalletDir();
    await File(dir).delete(recursive: true);
  }
}
