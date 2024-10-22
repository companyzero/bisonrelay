import 'dart:io';
import 'package:args/args.dart';
import 'package:bruig/util.dart';
import "package:ini/ini.dart" as ini;
import 'package:path_provider/path_provider.dart';
import 'package:path/path.dart' as path;

// ignore: constant_identifier_names
const APPNAME = "bruig";

const defaultAutoRemoveIgnoreList = [
  "86abd31f2141b274196d481edd061a00ab7a56b61a31656775c8a590d612b966", // Oprah
  "ad716557157c1f191d8b5f8c6757ea41af49de27dc619fc87f337ca85be325ee", // GC bot
];

String mainConfigFilename = ""; // Set at the begining of main().

String homeDir() {
  var env = Platform.environment;
  if (Platform.isWindows) {
    return env['UserProfile'] ?? "";
  } else {
    return env['HOME'] ?? "";
  }
}

String cleanAndExpandPath(String p) {
  if (p == "") {
    return p;
  }

  if (p.startsWith("~")) {
    p = homeDir() + p.substring(1);
  }

  return path.canonicalize(p);
}

Future<String> defaultAppDataDir() async {
  if (Platform.isLinux) {
    final home = Platform.environment["HOME"];
    if (home != null && home != "") {
      return path.join(home, ".$APPNAME");
    }
  }

  if (Platform.isWindows && Platform.environment.containsKey("LOCALAPPDATA")) {
    return path.join(Platform.environment["LOCALAPPDATA"]!, APPNAME);
  }

  if (Platform.isMacOS) {
    // getApplicationSupportDirectory adds "com.foo.bar" to application support,
    // so go to parent and append default APPNAME.
    final baseDir = (await getApplicationSupportDirectory()).parent.path;
    return path.join(baseDir, APPNAME);
  }

  // Default behavior: use app support dir.
  final dir = await getApplicationSupportDirectory();
  return dir.path;
}

String defaultLndDir() {
  return path.join(homeDir(), ".dcrlnd");
}

class Config {
  late final String appDataDir;
  late final String dbRoot;
  late final String downloadsDir;
  late final String embedsDir;
  late final String serverAddr;
  late final String lnRPCHost;
  late final String lnTLSCert;
  late final String lnMacaroonPath;
  late final String lnDebugLevel;
  late final String logFile;
  late final String msgRoot;
  late final String debugLevel;
  late final String walletType;
  late final String network;
  late final String internalWalletDir;
  late final String resourcesUpstream;
  late final String simpleStorePayType;
  late final String simpleStoreAccount;
  late final double simpleStoreShipCharge;
  late final String proxyaddr;
  late final bool torIsolation;
  late final String proxyUsername;
  late final String proxyPassword;
  late final int circuitLimit;
  late final bool noLoadChatHistory;
  late final bool syncFreeList;
  late final bool autoCompact;
  late final int autoCompactMinAge;
  late final int autoHandshakeInterval;
  late final int autoRemoveIdleUsersInterval;
  late final List<String> autoRemoveIgnoreList;
  late final bool sendRecvReceipts;
  late final bool autoSubPosts;
  late final bool logPings;
  late final List<String> jsonRPCListen;
  late final String rpcCertPath;
  late final String rpcKeyPath;
  late final bool rpcIssueClientCert;
  late final String rpcClientCApath;
  late final String rpcUser;
  late final String rpcPass;
  late final String rpcAuthMode;
  late final bool rpcAllowRemoteSendTip;
  late final double rpcMaxRemoteSendTipAmt;

  Config();
  Config.filled(
      {this.appDataDir = "",
      this.dbRoot = "",
      this.downloadsDir = "",
      this.embedsDir = "",
      this.serverAddr = "",
      this.lnRPCHost = "",
      this.lnTLSCert = "",
      this.lnMacaroonPath = "",
      this.logFile = "",
      this.msgRoot = "",
      this.debugLevel = "",
      this.lnDebugLevel = "info",
      this.walletType = "",
      this.network = "",
      this.internalWalletDir = "",
      this.resourcesUpstream = "",
      this.simpleStorePayType = "",
      this.simpleStoreAccount = "",
      this.simpleStoreShipCharge = 0,
      this.proxyaddr = "",
      this.torIsolation = false,
      this.proxyUsername = "",
      this.proxyPassword = "",
      this.circuitLimit = 32,
      this.noLoadChatHistory = true,
      this.syncFreeList = true,
      this.autoCompact = true,
      this.autoCompactMinAge = 14 * 24 * 60 * 60,
      this.autoHandshakeInterval = 21 * 24 * 60 * 60,
      this.autoRemoveIdleUsersInterval = 60 * 24 * 60 * 60,
      this.autoRemoveIgnoreList = defaultAutoRemoveIgnoreList,
      this.sendRecvReceipts = true,
      this.autoSubPosts = true,
      this.logPings = false,
      this.jsonRPCListen = const [],
      this.rpcCertPath = "",
      this.rpcKeyPath = "",
      this.rpcIssueClientCert = false,
      this.rpcClientCApath = "",
      this.rpcUser = "",
      this.rpcPass = "",
      this.rpcAuthMode = "",
      this.rpcAllowRemoteSendTip = false,
      this.rpcMaxRemoteSendTipAmt = 0});
  factory Config.newWithRPCHost(
          Config cfg, String rpcHost, String tlsCert, String macaroonPath) =>
      Config.filled(
        appDataDir: cfg.appDataDir,
        dbRoot: cfg.dbRoot,
        downloadsDir: cfg.downloadsDir,
        embedsDir: cfg.embedsDir,
        serverAddr: cfg.serverAddr,
        lnRPCHost: rpcHost,
        lnTLSCert: tlsCert,
        lnMacaroonPath: macaroonPath,
        logFile: cfg.logFile,
        msgRoot: cfg.msgRoot,
        debugLevel: cfg.debugLevel,
        lnDebugLevel: cfg.lnDebugLevel,
        walletType: cfg.walletType,
        network: cfg.network,
        internalWalletDir: cfg.internalWalletDir,
        resourcesUpstream: cfg.resourcesUpstream,
        simpleStorePayType: cfg.simpleStorePayType,
        simpleStoreAccount: cfg.simpleStoreAccount,
        simpleStoreShipCharge: cfg.simpleStoreShipCharge,
        proxyaddr: cfg.proxyaddr,
        torIsolation: cfg.torIsolation,
        proxyUsername: cfg.proxyUsername,
        proxyPassword: cfg.proxyPassword,
        circuitLimit: cfg.circuitLimit,
        noLoadChatHistory: cfg.noLoadChatHistory,
        syncFreeList: cfg.syncFreeList,
        autoCompact: cfg.autoCompact,
        autoCompactMinAge: cfg.autoCompactMinAge,
        autoHandshakeInterval: cfg.autoHandshakeInterval,
        autoRemoveIdleUsersInterval: cfg.autoRemoveIdleUsersInterval,
        autoRemoveIgnoreList: cfg.autoRemoveIgnoreList,
        sendRecvReceipts: cfg.sendRecvReceipts,
        autoSubPosts: cfg.autoSubPosts,
        logPings: cfg.logPings,
        jsonRPCListen: cfg.jsonRPCListen,
        rpcCertPath: cfg.rpcCertPath,
        rpcKeyPath: cfg.rpcKeyPath,
        rpcIssueClientCert: cfg.rpcIssueClientCert,
        rpcClientCApath: cfg.rpcClientCApath,
        rpcUser: cfg.rpcUser,
        rpcPass: cfg.rpcPass,
        rpcAuthMode: cfg.rpcAuthMode,
        rpcAllowRemoteSendTip: cfg.rpcAllowRemoteSendTip,
        rpcMaxRemoteSendTipAmt: cfg.rpcMaxRemoteSendTipAmt,
      );

  // Save a new config from scratch.
  Future<void> saveNewConfig(String filepath) async {
    var f = ini.Config.fromString("\n[payment]\n");
    set(String section, String opt, String val) =>
        val != "" ? f.set(section, opt, val) : null;

    // Do not save the root app data path in ios, but rely on defaultAppDataDir()
    // to return the correct path on every execution, because the root path changes
    // on every recompilation.
    if (!Platform.isIOS) {
      set("default", "root", appDataDir);
    }
    set("default", "server", serverAddr);
    set("payment", "wallettype", walletType);
    set("payment", "network", network);
    if (walletType == "external") {
      set("payment", "lnrpchost", lnRPCHost);
      set("payment", "lntlscert", lnTLSCert);
      set("payment", "lnmacaroonpath", lnMacaroonPath);
    }

    set("default", "proxyaddr", proxyaddr);
    set("default", "proxyuser", proxyUsername);
    set("default", "proxypass", proxyPassword);
    set("default", "circuitlimit", "$circuitLimit");
    set("default", "torisolation", torIsolation ? "1" : "0");

    // Create the dir and write the config file.
    await File(filepath).parent.create(recursive: true);
    await File(filepath).writeAsString(f.toString());
  }
}

// replaceConfig replaces the settings that can be modified by the GUI, while
// preserving manual chages made to the config file.
Future<void> replaceConfig(
  String filepath, {
  String? debugLevel,
  String? lnDebugLevel,
  bool? logPings,
  String? proxyAddr,
  String? proxyUsername,
  String? proxyPassword,
  int? torCircuitLimit,
  bool? torIsolation,
}) async {
  var f = ini.Config.fromStrings(File(filepath).readAsLinesSync());

  void set(String section, String opt, String? val) {
    if (val == null) return;
    if (section != "default" && !f.hasSection(section)) {
      f.addSection(section);
    }
    f.set(section, opt, val);
  }

  void setBool(String section, String opt, bool? val) {
    if (val == null) return;
    set(section, opt, val ? "1" : "0");
  }

  void setInt(String section, String opt, int? val) {
    if (val == null) return;
    set(section, opt, "$val");
  }

  set("log", "debuglevel", debugLevel);
  setBool("log", "pings", logPings);
  set("payment", "lndebuglevel", lnDebugLevel);

  set("default", "proxyaddr", proxyAddr);
  set("default", "proxyuser", proxyUsername);
  set("default", "proxypass", proxyPassword);
  setInt("default", "circuitlimit", torCircuitLimit);
  setBool("default", "torisolation", torIsolation);

  await File(filepath).writeAsString(f.toString());
}

Future<Config> loadConfig(String filepath) async {
  var f = ini.Config.fromStrings(File(filepath).readAsLinesSync());
  var appDataDir = await defaultAppDataDir();
  var iniAppData = f.get("default", "root");
  if (iniAppData != null && iniAppData != "") {
    appDataDir = cleanAndExpandPath(iniAppData);
  }

  String getPath(String section, String option, String def) {
    var iniVal = f.get(section, option);
    if (iniVal == null || iniVal == "") {
      return def;
    }
    return cleanAndExpandPath(iniVal);
  }

  getBool(String section, String opt) {
    var v = f.get(section, opt);
    return v == "yes" || v == "true" || v == "1" ? true : false;
  }

  getBoolDefaultTrue(String section, String opt) {
    var v = f.get(section, opt);
    return v == "no" || v == "false" || v == "0" ? false : true;
  }

  getInt(String section, String opt) {
    var v = f.get(section, opt);
    return v != null && v != "" ? int.tryParse(v) : null;
  }

  getCommaList(String section, String opt) {
    var v = f.get(section, opt);
    return v != null && v != ""
        ? v.split(",").map((e) => e.trim()).toList()
        : null;
  }

  var iniLogFile = f.get("log", "logfile");
  String logfile = path.join(appDataDir, "applogs", "$APPNAME.log");
  if (iniLogFile != null) {
    iniLogFile = iniLogFile.trim();

    if (iniLogFile == "") {
      logfile = "";
    } else if (!iniLogFile.contains("/") && !iniLogFile.contains("\\")) {
      // logfile does not contain path separator. Use default dir with the
      // specified file name.
      logfile = path.join(appDataDir, "logs", iniLogFile);
    } else {
      logfile = cleanAndExpandPath(iniLogFile);
    }
  }

  String msgRoot = path.join(appDataDir, "logs");
  var iniMsgsRoot = f.get("log", "msglog");
  if (iniMsgsRoot != null) {
    iniMsgsRoot = iniMsgsRoot.trim();
    if (iniMsgsRoot == "") {
      msgRoot = "";
    } else if (!iniMsgsRoot.contains("/") && !iniMsgsRoot.contains("\\")) {
      // msgsroot does not contain path separator. Use default dir with the
      // specified subdir name.
      msgRoot = path.join(appDataDir, iniMsgsRoot);
    } else {
      msgRoot = cleanAndExpandPath(iniMsgsRoot);
    }
  }

  var c = Config();
  c.appDataDir = appDataDir;
  c.dbRoot = path.join(appDataDir, "db");
  c.downloadsDir = path.join(appDataDir, "downloads");
  c.embedsDir = path.join(appDataDir, "embeds");
  c.serverAddr = f.get("default", "server") ?? "localhost:443";
  c.logFile = logfile;
  c.msgRoot = msgRoot;
  c.debugLevel = f.get("log", "debuglevel") ?? "info";
  c.logPings = getBool("log", "pings");
  c.walletType = f.get("payment", "wallettype") ?? "disabled";
  c.network = f.get("payment", "network") ?? "mainnet";
  c.internalWalletDir = path.join(appDataDir, "ln-wallet");

  c.proxyaddr = f.get("default", "proxyaddr") ?? "";
  c.proxyUsername = f.get("default", "proxyuser") ?? "";
  c.proxyPassword = f.get("default", "proxypass") ?? "";
  c.torIsolation = getBool("default", "torisolation");
  c.circuitLimit = getInt("default", "circuitlimit") ?? 32;
  c.noLoadChatHistory = getBool("default", "noloadchathistory");
  c.syncFreeList = getBoolDefaultTrue("default", "syncfreelist");
  c.autoCompact = getBoolDefaultTrue("default", "autocompact");
  c.autoCompactMinAge =
      parseDurationSeconds(f.get("default", "autocompact_min_age") ?? "14d");
  c.autoHandshakeInterval =
      parseDurationSeconds(f.get("default", "autohandshakeinterval") ?? "21d");
  c.autoRemoveIdleUsersInterval = parseDurationSeconds(
      f.get("default", "autoremoveidleusersinterval") ?? "60d");
  c.autoRemoveIgnoreList = getCommaList("default", "autoremoveignorelist") ??
      defaultAutoRemoveIgnoreList;
  c.autoSubPosts = getBoolDefaultTrue("default", "autosubposts");

  if (c.autoRemoveIdleUsersInterval <= c.autoHandshakeInterval) {
    throw "invalid values: 'autoremoveinterval' is not greater than 'autohandshakeinterval'";
  }

  if (c.walletType != "disabled") {
    c.lnRPCHost = f.get("payment", "lnrpchost") ?? "localhost:10009";
    c.lnTLSCert =
        getPath("payment", "lntlscert", path.join(defaultLndDir(), "tls.cert"));
    c.lnMacaroonPath = getPath(
        "payment",
        "lnmacaroonpath",
        path.join(defaultLndDir(), "data", "chain", "decred", "mainnet",
            "admin.macaroon"));
  } else {
    c.lnRPCHost = "";
    c.lnTLSCert = "";
    c.lnMacaroonPath = "";
  }
  c.lnDebugLevel = f.get("payment", "lndebuglevel") ?? "info";

  var resUpstream = f.get("resources", "upstream") ?? "";
  if (resUpstream.startsWith("pages:")) {
    var path = resUpstream.substring("pages:".length);
    path = cleanAndExpandPath(path);
    resUpstream = "pages:$path";
  } else if (resUpstream.startsWith("simplestore:")) {
    var path = resUpstream.substring("simplestore:".length);
    path = cleanAndExpandPath(path);
    resUpstream = "simplestore:$path";
  }

  c.sendRecvReceipts = getBoolDefaultTrue("default", "sendrecvreceipts");

  c.resourcesUpstream = resUpstream;
  c.simpleStorePayType = f.get("simplestore", "paytype") ?? "";
  c.simpleStoreAccount = f.get("resources", "account") ?? "";
  c.simpleStoreShipCharge =
      double.tryParse(f.get("resources", "shipcharge") ?? "0") ?? 0;

  c.jsonRPCListen = getCommaList("clientrpc", "jsonrpclisten") ?? [];
  c.rpcCertPath = f.get("clientrpc", "rpccertpath") ?? "";
  c.rpcKeyPath = f.get("clientrpc", "rpckeypath") ?? "";
  c.rpcIssueClientCert = f.get("clientrpc", "rpcissueclientcert") == "true";
  c.rpcClientCApath = f.get("clientrpc", "rpcclientcapath") ?? "";
  c.rpcUser = f.get("clientrpc", "rpcuser") ?? "";
  c.rpcPass = f.get("clientrpc", "rpcpass") ?? "";
  c.rpcAuthMode = f.get("clientrpc", "rpcauthmode") ?? "";
  c.rpcAllowRemoteSendTip = getBool("clientrpc", "rpcallowremotesendtip");
  c.rpcMaxRemoteSendTipAmt =
      double.tryParse(f.get("clientrpc", "rpcmaxremotesendtipamt") ?? "0") ?? 0;

  return c;
}

final usageException = Exception("Usage Displayed");
final newConfigNeededException = Exception("Config needed");
final unableToMoveOldWallet = Exception("Existing wallet in new location");

Future<ArgParser> appArgParser() async {
  var defaultCfgFile = path.join(await defaultAppDataDir(), "$APPNAME.conf");
  var p = ArgParser();
  p.addFlag("help", abbr: "h", help: "Display usage info", negatable: false);
  p.addOption("configfile",
      abbr: "c", defaultsTo: defaultCfgFile, help: "Path to config file");
  return p;
}

Future<String> configFileName(List<String> args) async {
  var p = await appArgParser();
  var res = p.parse(args);
  return res["configfile"];
}

Future<Config> configFromArgs(List<String> args) async {
  var p = await appArgParser();
  var res = p.parse(args);

  if (res["help"]) {
    // ignore: avoid_print
    print(p.usage);
    throw usageException;
  }

  var cfgFilePath = res["configfile"];
  if (!File(cfgFilePath).existsSync()) {
    throw newConfigNeededException;
  }

  return loadConfig(cfgFilePath);
}
