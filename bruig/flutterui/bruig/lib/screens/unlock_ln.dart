import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/recent_log.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/buttons.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/screens/about.dart';
import 'package:bruig/config.dart';
import 'package:bruig/main.dart';
import 'package:bruig/models/log.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:flutter/widgets.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:path/path.dart' as path;
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

class UnlockLNApp extends StatefulWidget {
  Config cfg;
  final String initialRoute;
  SnackBarModel snackBar;
  UnlockLNApp(this.cfg, this.initialRoute, this.snackBar, {Key? key})
      : super(key: key);

  void setCfg(Config c) {
    cfg = c;
  }

  @override
  State<UnlockLNApp> createState() => _UnlockLNAppState();
}

class _UnlockLNAppState extends State<UnlockLNApp> {
  Config get cfg => widget.cfg;
  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: "Connect to Bison Relay",
      initialRoute: widget.initialRoute,
      routes: {
        "/": (context) => _LNUnlockPage(widget.cfg, widget.setCfg),
        "/sync": (context) => _LNChainSyncPage(widget.cfg),
        '/about': (context) => const AboutScreen(),
      },
      builder: (BuildContext context, Widget? child) => Scaffold(
        body: SnackbarDisplayer(widget.snackBar, child ?? const Empty()),
      ),
    );
  }
}

class _LNUnlockPage extends StatefulWidget {
  final Config cfg;
  final Function(Config) setCfg;
  const _LNUnlockPage(this.cfg, this.setCfg, {Key? key}) : super(key: key);

  @override
  State<_LNUnlockPage> createState() => __LNUnlockPageState();
}

class __LNUnlockPageState extends State<_LNUnlockPage> {
  bool loading = false;
  final TextEditingController passCtrl = TextEditingController();
  String _validate = "";
  bool compactingDb = false;
  bool compactingDbErrored = false;
  bool migratingDb = false;

  void logUpdated() {
    var log = globalLogModel;
    if (compactingDb != log.compactingDb ||
        compactingDbErrored != log.compactingDbErrored ||
        migratingDb != log.migratingDb) {
      setState(() {
        compactingDb = log.compactingDb;
        compactingDbErrored = log.compactingDbErrored;
        migratingDb = log.migratingDb;
      });
    }
  }

  @override
  void initState() {
    super.initState();
    globalLogModel.addListener(logUpdated);
  }

  @override
  void dispose() {
    passCtrl.dispose();
    super.dispose();
    globalLogModel.removeListener(logUpdated);
  }

  Future<void> unlock() async {
    setState(() {
      loading = true;
      _validate = passCtrl.text.isEmpty ? "Password cannot be empty" : "";
    });
    try {
      // Validation failed so don't even attempt
      if (_validate.isNotEmpty) {
        return;
      }
      var cfg = widget.cfg;
      var rpcHost = await Golib.lnRunDcrlnd(
          cfg.internalWalletDir,
          cfg.network,
          passCtrl.text,
          cfg.proxyaddr,
          cfg.torIsolation,
          cfg.syncFreeList,
          cfg.autoCompact,
          cfg.autoCompactMinAge,
          cfg.lnDebugLevel);
      var tlsCert = path.join(cfg.internalWalletDir, "tls.cert");
      var macaroonPath = path.join(cfg.internalWalletDir, "data", "chain",
          "decred", cfg.network, "admin.macaroon");
      widget.setCfg(Config.newWithRPCHost(cfg, rpcHost, tlsCert, macaroonPath));
      Navigator.of(context).pushNamed("/sync");
    } catch (exception) {
      if (exception.toString().contains("invalid passphrase")) {
        _validate = "Incorrect password, please try again.";
      } else {
        showErrorSnackbar(context, "Unable to unlock wallet: $exception");
      }
      // Catch error and show error in errorText?
    } finally {
      setState(() {
        loading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;

    Widget extraInfo = const Empty();
    if (loading && migratingDb) {
      extraInfo = const Text(
        "Upgrading DB. This might take a while.",
        style: TextStyle(color: Colors.amber, fontWeight: FontWeight.w500),
      );
    } else if (loading && compactingDbErrored) {
      extraInfo = const Text(
        "Compacting DB errored. Look at the logs to see the cause.",
        style: TextStyle(color: Colors.red),
      );
    } else if (loading && compactingDb) {
      extraInfo = const Text(
        "Compacting DB. This might take a while.",
        style: TextStyle(color: Colors.amber, fontWeight: FontWeight.w500),
      );
    }

    return Consumer<ThemeNotifier>(
      builder: (context, theme, _) => StartupScreen(<Widget>[
        Text("Connect to Bison Relay",
            style: TextStyle(
                color: theme.getTheme().dividerColor,
                fontSize: theme.getHugeFont(context),
                fontWeight: FontWeight.w200)),
        SizedBox(height: isScreenSmall ? 8 : 34),
        SizedBox(
            width: 377,
            child: Text("Password",
                textAlign: TextAlign.left,
                style: TextStyle(
                    color: theme.getTheme().indicatorColor,
                    fontSize: theme.getMediumFont(context),
                    fontWeight: FontWeight.w300))),
        const SizedBox(height: 5),
        Center(
            child: SizedBox(
                width: 377,
                child: TextField(
                    autofocus: true,
                    cursorColor: theme.getTheme().indicatorColor,
                    decoration: InputDecoration(
                        errorText: _validate,
                        border: InputBorder.none,
                        hintText: "Password",
                        hintStyle: TextStyle(
                            fontSize: theme.getLargeFont(context),
                            color: theme.getTheme().dividerColor),
                        filled: true,
                        fillColor: theme.getTheme().cardColor),
                    style: TextStyle(
                        color: theme.getTheme().indicatorColor,
                        fontSize: theme.getLargeFont(context)),
                    controller: passCtrl,
                    obscureText: true,
                    onSubmitted: (value) {
                      if (!loading) {
                        unlock();
                      }
                    },
                    onChanged: (value) {
                      setState(() {
                        _validate =
                            value.isEmpty ? "Password cannot be empty" : "";
                      });
                    }))),
        SizedBox(height: isScreenSmall ? 8 : 34),
        Center(
            child: SizedBox(
                width: 283,
                child: Row(children: [
                  const SizedBox(width: 35),
                  LoadingScreenButton(
                    onPressed: !loading ? unlock : null,
                    text: "Unlock Wallet",
                  ),
                  const SizedBox(width: 10),
                  loading
                      ? SizedBox(
                          height: 25,
                          width: 25,
                          child: CircularProgressIndicator(
                              value: null,
                              backgroundColor: theme.getTheme().backgroundColor,
                              color: theme.getTheme().dividerColor,
                              strokeWidth: 2),
                        )
                      : const SizedBox(width: 25),
                ]))),
        const SizedBox(height: 10),
        extraInfo,
        const SizedBox(height: 10),
        loading
            ? SizedBox(
                // width: 400,
                // height: 200,
                child: LogLines(globalLogModel,
                    maxLines: 15,
                    optionalTextColor: theme.getTheme().dividerColor))
            : const Empty(),
      ]),
    );
  }
}

class _LNChainSyncPage extends StatefulWidget {
  final Config cfg;
  const _LNChainSyncPage(this.cfg, {Key? key}) : super(key: key);

  @override
  State<_LNChainSyncPage> createState() => _LNChainSyncPageState();
}

class _LNChainSyncPageState extends State<_LNChainSyncPage> {
  int blockHeight = 0;
  String blockHash = "";
  DateTime blockTimestamp = DateTime.fromMicrosecondsSinceEpoch(0);
  double currentTimeStamp = DateTime.now().millisecondsSinceEpoch / 1000;
  bool synced = false;
  static const startBlockTimestamp = 1454907600;
  static const fiveMinBlock = 300;
  double progress = 0;

  void readSyncProgress() async {
    var stream = Golib.lnInitChainSyncProgress();
    try {
      await for (var update in stream) {
        setState(() {
          blockHeight = update.blockHeight;
          blockHash = update.blockHash;
          blockTimestamp =
              DateTime.fromMillisecondsSinceEpoch(update.blockTimestamp * 1000);
          synced = update.synced;
          progress = update.blockHeight /
              ((currentTimeStamp - startBlockTimestamp) / fiveMinBlock);
        });
        if (update.synced) {
          syncCompleted();
        }
      }
    } catch (exception) {
      showErrorSnackbar(
          context, "Unable to read chain sync updates: $exception");
    }
  }

  @override
  void initState() {
    super.initState();
    readSyncProgress();

    // TODO: check if already synced.
  }

  void syncCompleted() async {
    runMainApp(widget.cfg);
  }

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;

    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => StartupScreen([
              Text("Setting up Bison Relay",
                  style: TextStyle(
                      color: theme.getTheme().dividerColor,
                      fontSize: theme.getHugeFont(context),
                      fontWeight: FontWeight.w200)),
              SizedBox(height: isScreenSmall ? 8 : 89),
              Text("Network Sync",
                  style: TextStyle(
                      color: theme.getTheme().focusColor,
                      fontSize: theme.getLargeFont(context),
                      fontWeight: FontWeight.w300)),
              SizedBox(height: isScreenSmall ? 8 : 50),
              Center(
                  child: SizedBox(
                      width: 740,
                      child: Row(children: [
                        const SizedBox(width: 65),
                        Expanded(
                            child: ClipRRect(
                                borderRadius:
                                    const BorderRadius.all(Radius.circular(5)),
                                child: LinearProgressIndicator(
                                    minHeight: 8,
                                    value: progress > 1 ? 1 : progress,
                                    color: theme.getTheme().cardColor,
                                    backgroundColor: theme.getTheme().cardColor,
                                    valueColor: AlwaysStoppedAnimation<Color>(
                                        theme.getTheme().dividerColor)))),
                        const SizedBox(width: 20),
                        Text(
                            "${((progress > 1 ? 1 : progress) * 100).toStringAsFixed(0)}%",
                            style: TextStyle(
                                color: theme.getTheme().dividerColor,
                                fontSize: theme.getMediumFont(context),
                                fontWeight: FontWeight.w300))
                      ]))),
              const SizedBox(height: 21),
              Center(
                child: Container(
                    margin: const EdgeInsets.all(0),
                    width: 610,
                    height: 251,
                    padding: const EdgeInsets.all(10),
                    color: theme.getTheme().cardColor,
                    child: Column(children: [
                      Flex(
                          direction:
                              isScreenSmall ? Axis.vertical : Axis.horizontal,
                          children: [
                            RichText(
                                text: TextSpan(children: [
                              TextSpan(
                                  text: "Block Height: ",
                                  style: TextStyle(
                                      color: theme.getTheme().dividerColor,
                                      fontSize: theme.getSmallFont(context),
                                      fontWeight: FontWeight.w300)),
                              TextSpan(
                                  text: "$blockHeight",
                                  style: TextStyle(
                                      color: theme.getTheme().dividerColor,
                                      fontSize: theme.getSmallFont(context),
                                      fontWeight: FontWeight.w300))
                            ])),
                            isScreenSmall
                                ? const SizedBox(height: 5)
                                : const SizedBox(width: 21),
                            RichText(
                                text: TextSpan(children: [
                              TextSpan(
                                  text: "Block Hash: ",
                                  style: TextStyle(
                                      color: theme.getTheme().dividerColor,
                                      fontSize: theme.getSmallFont(context),
                                      fontWeight: FontWeight.w300)),
                              TextSpan(
                                  text: "$blockHeight",
                                  style: TextStyle(
                                      color: theme.getTheme().dividerColor,
                                      fontSize: theme.getSmallFont(context),
                                      fontWeight: FontWeight.w300))
                            ])),
                            isScreenSmall
                                ? const SizedBox(height: 5)
                                : const SizedBox(width: 21),
                            RichText(
                                text: TextSpan(children: [
                              TextSpan(
                                  text: "Block Time: ",
                                  style: TextStyle(
                                      color: theme.getTheme().dividerColor,
                                      fontSize: theme.getSmallFont(context),
                                      fontWeight: FontWeight.w300)),
                              TextSpan(
                                  text: blockTimestamp.toString(),
                                  style: TextStyle(
                                      color: theme.getTheme().dividerColor,
                                      fontSize: theme.getSmallFont(context),
                                      fontWeight: FontWeight.w300))
                            ]))
                          ]),
                      Expanded(
                          child: LogLines(globalLogModel,
                              maxLines: 15,
                              optionalTextColor: theme.getTheme().dividerColor))
                    ])),
              )
            ]));
  }
}

Future<void> runUnlockDcrlnd(Config cfg) async {
  runApp(MultiProvider(
    providers: [
      ChangeNotifierProvider(create: (c) => SnackBarModel()),
      ChangeNotifierProvider(create: (c) => ThemeNotifier()),
    ],
    child: Consumer<SnackBarModel>(
        builder: (context, snackBar, child) => UnlockLNApp(cfg, "/", snackBar)),
  ));
}

Future<void> runChainSyncDcrlnd(Config cfg) async {
  runApp(MultiProvider(
    providers: [
      ChangeNotifierProvider(create: (c) => SnackBarModel()),
      ChangeNotifierProvider(create: (c) => ThemeNotifier()),
    ],
    child: Consumer<SnackBarModel>(
        builder: (context, snackBar, child) =>
            UnlockLNApp(cfg, "/sync", snackBar)),
  ));
}

Future<void> runMovePastWindowsSetup(Config cfg) async {
  runApp(MultiProvider(
    providers: [
      ChangeNotifierProvider(create: (c) => SnackBarModel()),
      ChangeNotifierProvider(create: (c) => ThemeNotifier()),
    ],
    child: Consumer<SnackBarModel>(
        builder: (context, snackBar, child) =>
            UnlockLNApp(cfg, "/windowsmove", snackBar)),
  ));
}
