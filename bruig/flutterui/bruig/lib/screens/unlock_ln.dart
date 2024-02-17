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

  @override
  void dispose() {
    passCtrl.dispose();
    super.dispose();
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
      var rpcHost = await Golib.lnRunDcrlnd(cfg.internalWalletDir, cfg.network,
          passCtrl.text, cfg.proxyaddr, cfg.torIsolation, cfg.syncFreeList);
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

  void goToAbout() {
    Navigator.of(context).pushNamed("/about");
  }

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    return StartupScreen(Consumer<ThemeNotifier>(
      builder: (context, theme, _) {
        return isScreenSmall
            ? Column(mainAxisAlignment: MainAxisAlignment.end, children: [
                SizedBox(height: MediaQuery.of(context).size.height / 9),
                SizedBox(
                    width: 250,
                    child: Text("Connect to Bison Relay",
                        textAlign: TextAlign.center,
                        style: TextStyle(
                            color: theme.getTheme().indicatorColor,
                            fontSize: theme.getHugeFont(context),
                            fontWeight: FontWeight.w300))),
                const SizedBox(height: 50),
                loading
                    ? SizedBox(
                        width: 50,
                        height: 50,
                        child: CircularProgressIndicator(
                            value: null,
                            backgroundColor: theme.getTheme().backgroundColor,
                            color: theme.getTheme().indicatorColor,
                            strokeWidth: 2),
                      )
                    : const SizedBox(height: 50),
                const SizedBox(height: 20),
                Expanded(
                    child: TextField(
                        enabled: !loading,
                        autofocus: true,
                        cursorColor: theme.getTheme().indicatorColor,
                        decoration: InputDecoration(
                            enabled: !loading,
                            labelText: "Password",
                            labelStyle: TextStyle(
                                letterSpacing: 0,
                                color: theme.getTheme().indicatorColor),
                            errorText: _validate != "" ? _validate : null,
                            errorBorder: const OutlineInputBorder(
                              borderRadius:
                                  BorderRadius.all(Radius.circular(10.0)),
                              borderSide:
                                  BorderSide(color: Colors.red, width: 2.0),
                            ),
                            focusedBorder: OutlineInputBorder(
                              borderRadius:
                                  const BorderRadius.all(Radius.circular(10.0)),
                              borderSide: BorderSide(
                                  color: theme.getTheme().indicatorColor,
                                  width: 2.0),
                            ),
                            border: OutlineInputBorder(
                              borderRadius:
                                  const BorderRadius.all(Radius.circular(10.0)),
                              borderSide: BorderSide(
                                  color: theme.getTheme().cardColor,
                                  width: 2.0),
                            ),
                            hintText: "Password",
                            hintStyle: TextStyle(
                                letterSpacing: 0,
                                fontWeight: FontWeight.w100,
                                color: theme.getTheme().indicatorColor),
                            filled: true,
                            fillColor: theme.getTheme().cardColor),
                        style: TextStyle(
                            letterSpacing: 5,
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
                        })),
                _validate == "" ? const SizedBox(height: 22) : const Empty(),
                const SizedBox(height: 34),
                LoadingScreenButton(
                  minSize: MediaQuery.of(context).size.width,
                  onPressed: !loading ? unlock : null,
                  text: "Unlock Wallet",
                ),
              ])
            : Column(children: [
                Row(children: [
                  IconButton(
                      alignment: Alignment.topLeft,
                      tooltip: "About Bison Relay",
                      iconSize: 50,
                      onPressed: goToAbout,
                      icon: Image.asset(
                        "assets/images/icon.png",
                      )),
                ]),
                const SizedBox(height: 208),
                Text("Connect to Bison Relay",
                    style: TextStyle(
                        color: theme.getTheme().dividerColor,
                        fontSize: theme.getHugeFont(context),
                        fontWeight: FontWeight.w200)),
                const SizedBox(height: 34),
                Column(children: [
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
                                  _validate = value.isEmpty
                                      ? "Password cannot be empty"
                                      : "";
                                });
                              }))),
                  const SizedBox(height: 34),
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
                                        backgroundColor:
                                            theme.getTheme().backgroundColor,
                                        color: theme.getTheme().dividerColor,
                                        strokeWidth: 2),
                                  )
                                : const SizedBox(width: 25),
                          ])))
                ]),
              ]);
      },
    ));
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
    void goToAbout() {
      Navigator.of(context).pushNamed("/about");
    }

    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Container(
            padding: const EdgeInsets.all(10),
            child: Column(children: [
              Row(children: [
                IconButton(
                    alignment: Alignment.topLeft,
                    tooltip: "About Bison Relay",
                    iconSize: 50,
                    onPressed: goToAbout,
                    icon: Image.asset(
                      "assets/images/icon.png",
                    )),
              ]),
              const SizedBox(height: 39),
              Text("Setting up Bison Relay",
                  style: TextStyle(
                      color: theme.getTheme().dividerColor,
                      fontSize: theme.getHugeFont(context),
                      fontWeight: FontWeight.w200)),
              const SizedBox(height: 89),
              Text("Network Sync",
                  style: TextStyle(
                      color: theme.getTheme().focusColor,
                      fontSize: theme.getLargeFont(context),
                      fontWeight: FontWeight.w300)),
              const SizedBox(height: 50),
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
            ])));
    ;
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
