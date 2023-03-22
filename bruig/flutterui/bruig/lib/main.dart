import 'dart:async';
import 'dart:io';
import 'dart:math';
import 'dart:developer' as developer;

import 'package:bruig/components/attach_file.dart';
import 'package:bruig/components/route_error.dart';
import 'package:bruig/models/menus.dart';
import 'package:bruig/screens/about.dart';
import 'package:bruig/screens/contacts_msg_times.dart';
import 'package:bruig/theme_manager.dart';
import 'package:bruig/config.dart';
import 'package:bruig/models/downloads.dart';
import 'package:bruig/screens/overview.dart';
import 'package:bruig/models/log.dart';
import 'package:bruig/models/notifications.dart';
import 'package:bruig/screens/confirm_file_download.dart';
import 'package:bruig/screens/fatal_error.dart';
// import 'package:dart_vlc/dart_vlc.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/feed.dart';
import 'package:bruig/screens/init_local_id.dart';
import 'package:bruig/screens/ln_management.dart';
import 'package:bruig/screens/needs_funds.dart';
import 'package:bruig/screens/needs_in_channel.dart';
import 'package:bruig/screens/needs_out_channel.dart';
import 'package:bruig/screens/new_config.dart';
import 'package:bruig/screens/new_gc.dart';
import 'package:bruig/screens/shutdown.dart';
import 'package:bruig/screens/unlock_ln.dart';
import 'package:bruig/screens/verify_invite.dart';
import 'package:bruig/screens/verify_server.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';
import 'package:window_manager/window_manager.dart';
import './screens/app_start.dart';

final Random random = Random();

void main(List<String> args) async {
  // Ensure the platform bindings are initialized.
  WidgetsFlutterBinding.ensureInitialized();

  // This debugs both the dart platform adapter and the native bindings.
  developer.log("Platform: ${Golib.majorPlatform}/${Golib.minorPlatform}");
  Golib.platformVersion
      .then((value) => developer.log("Platform Version: $value"));
  Golib.captureDcrlndLog();
  // DartVLC.initialize();

  // The MockGolib was mostly useful during early stages of development.
  //UseMockGolib();

  var defAppDir = await defaultAppDataDir();
  developer.log("Default app dir: $defAppDir");

  try {
    Config cfg = await configFromArgs(args);
    await Golib.createLockFile(cfg.dbRoot);
    if (cfg.walletType == "internal") {
      await runUnlockDcrlnd(cfg);
      return;
    }
    await runMainApp(cfg);
    await Golib.closeLockFile(cfg.dbRoot);
  } catch (exception) {
    if (exception == usageException) {
      exit(0);
    }
    if (exception == newConfigNeededException) {
      // Go to new config wizard.
      runNewConfigApp(args);
      return;
    }
    runFatalErrorApp(exception);
  }
}

Future<void> runMainApp(Config cfg) async {
  runApp(MultiProvider(
    providers: [
      ChangeNotifierProvider(create: (c) => ClientModel()),
      ChangeNotifierProvider(create: (c) => FeedModel()),
      ChangeNotifierProvider.value(value: globalLogModel),
      ChangeNotifierProvider(create: (c) => DownloadsModel()),
      ChangeNotifierProvider(create: (c) => AppNotifications()),
      ChangeNotifierProvider(create: (c) => ThemeNotifier()),
      ChangeNotifierProvider(create: (c) => MainMenuModel())
    ],
    child: App(cfg),
  ));
}

class App extends StatefulWidget {
  final Config cfg;
  const App(this.cfg, {Key? key}) : super(key: key);

  @override
  State<App> createState() => _AppState();
}

class _AppState extends State<App> with WindowListener {
  final navkey = GlobalKey<NavigatorState>(debugLabel: "main-navigator");
  final StreamController<ConfNotification> shutdownNtfs =
      StreamController<ConfNotification>();
  bool pushedToShutdown = false;

  @override
  void initState() {
    super.initState();
    windowManager.addListener(this);
    handleNotifications();
    initClient();
    windowManager.setPreventClose(true);
  }

  @override
  void dispose() {
    windowManager.removeListener(this);
    super.dispose();
  }

  @override
  void onWindowClose() async {
    var isPreventClose = await windowManager.isPreventClose();
    if (!isPreventClose) return;
    if (!pushedToShutdown) {
      navkey.currentState!
          .pushNamedAndRemoveUntil('/shutdown', (Route r) => false);
      pushedToShutdown = true;
    }
  }

  void initClient() async {
    try {
      var cfg = widget.cfg;
      InitClient initArgs = InitClient(
          cfg.dbRoot,
          cfg.downloadsDir,
          cfg.serverAddr,
          cfg.lnRPCHost,
          cfg.lnTLSCert,
          cfg.lnMacaroonPath,
          cfg.logFile,
          cfg.msgRoot,
          cfg.debugLevel,
          true);
      await Golib.initClient(initArgs);

      navkey.currentState!.pushReplacementNamed(OverviewScreen.routeName);

      doWalletChecks();
    } catch (exception) {
      navkey.currentState!.pushNamed('/fatalError', arguments: exception);
    }
  }

  void doWalletChecks() async {
    var ntfns = Provider.of<AppNotifications>(context, listen: false);
    try {
      var balances = await Golib.lnGetBalances();
      var pushed = false;
      if (balances.wallet.totalBalance == 0) {
        ntfns.addNtfn(AppNtfn(AppNtfnType.walletNeedsFunds));
        navkey.currentState!.pushNamed("/needsFunds");
        pushed = true;
      }
      if (balances.channel.maxOutboundAmount == 0) {
        ntfns.addNtfn(AppNtfn(AppNtfnType.walletNeedsChannels));
        if (!pushed) {
          navkey.currentState!.pushNamed("/needsOutChannel");
        }
      }
      if (balances.channel.maxInboundAmount == 0) {
        ntfns.addNtfn(AppNtfn(AppNtfnType.walletNeedsInChannels));
        if (!pushed) {
          navkey.currentState!.pushNamed("/needsInChannel");
        }
      }
    } catch (exception) {
      ntfns.addNtfn(AppNtfn(AppNtfnType.error,
          msg: "Unable to perform initial wallet checks: $exception"));
    }
  }

  void handleNotifications() async {
    final confStream = Golib.confirmations();
    await for (var ntf in confStream) {
      switch (ntf.type) {
        case NTLocalIDNeeded:
          navkey.currentState!.pushNamed('/initLocalID');
          break;

        case NTFConfServerCert:
          var cert = ntf.payload as ServerCert;
          navkey.currentState!
              .pushNamed('/startup/verifyServer', arguments: cert);
          break;

        case NTLNConfPayReqRecvChan:
          var est = ntf.payload as LNReqChannelEstValue;
          navkey.currentState!
              .pushNamed("/ln/confirmRecvChannelPay", arguments: est);
          break;

        case NTConfFileDownload:
          var data = ntf.payload as ConfirmFileDownload;
          navkey.currentState!
              .pushNamed("/confirmFileDownload", arguments: data);
          break;

        case NTLNDcrlndStopped:
          shutdownNtfs.add(ntf);
          break;

        case NTClientStopped:
          String? currentPath;
          navkey.currentState?.popUntil((route) {
            currentPath = route.settings.name;
            return true;
          });
          if (currentPath != "/shutdown") {
            // Not a clean shutdown.
            navkey.currentState!.pushNamedAndRemoveUntil(
                "/fatalError", (route) => false,
                arguments: ntf.payload);
          }
          shutdownNtfs.add(ntf);
          break;

        case NTInvoiceGenFailed:
          var fail = ntf.payload as InvoiceGenFailed;
          var ntfns = Provider.of<AppNotifications>(context, listen: false);
          var msg =
              "Failed to generate invoice to user ${fail.nick} for ${fail.dcrAmount} DCR: ${fail.err}";
          ntfns.addNtfn(AppNtfn(AppNtfnType.invoiceGenFailed, msg: msg));
          break;

        default:
          developer.log("Unknown conf ntf received ${ntf.type}");
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => MaterialApp(
              title: 'Bison Relay',
              theme: theme.getTheme(),
              navigatorKey: navkey,
              initialRoute: '/',
              routes: {
                '/': (context) => const AppStartingLoadScreen(),
                '/about': (context) => const AboutScreen(),
                '/initLocalID': (context) => const InitLocalIDScreen(),
                '/startup/verifyServer': (context) =>
                    const VerifyServerScreen(),
                '/verifyInvite': (context) => const VerifyInviteScreen(),
                '/newGC': (context) => const NewGCScreen(),
                '/ln/confirmRecvChannelPay': (context) =>
                    const LNConfirmRecvChanPaymentScreen(),
                '/confirmFileDownload': (context) =>
                    Consumer2<ClientModel, DownloadsModel>(
                        builder: (context, client, downloads, child) =>
                            ConfirmFileDownloadScreen(client, downloads)),
                AttachFileScreen.routeName: (context) =>
                    Consumer2<ClientModel, DownloadsModel>(
                        builder: (context, client, downloads, child) =>
                            AttachFileScreen()),
                '/needsFunds': (context) => Consumer<AppNotifications>(
                    builder: (context, ntfns, child) =>
                        NeedsFundsScreen(ntfns)),
                '/needsInChannel': (context) =>
                    Consumer2<AppNotifications, ClientModel>(
                        builder: (context, ntfns, client, child) =>
                            NeedsInChannelScreen(ntfns, client)),
                ContactsLastMsgTimesScreen.routeName: (context) =>
                    Consumer<ClientModel>(
                        builder: (context, client, child) =>
                            ContactsLastMsgTimesScreen(client)),
                '/fatalError': (context) => const FatalErrorScreen(),
                '/shutdown': (context) => Consumer<LogModel>(
                    builder: (context, log, child) => ShutdownScreen(
                        widget.cfg.walletType == "internal",
                        shutdownNtfs.stream,
                        log)),
              },
              onGenerateRoute: (settings) {
                late Widget page;
                if (settings.name!.startsWith(OverviewScreen.routeName)) {
                  var initialRoute =
                      settings.name!.substring(OverviewScreen.routeName.length);
                  page = Consumer4<DownloadsModel, ClientModel,
                          AppNotifications, MainMenuModel>(
                      builder:
                          (context, down, client, ntfns, mainMenu, child) =>
                              OverviewScreen(
                                  down, client, ntfns, initialRoute, mainMenu));
                } else if (settings.name!
                    .startsWith(NeedsOutChannelScreen.routeName)) {
                  page = Consumer2<AppNotifications, ClientModel>(
                      builder: (context, ntfns, client, child) =>
                          NeedsOutChannelScreen(ntfns, client));
                } else {
                  page = RouteErrorPage(
                      settings.name ?? "", OverviewScreen.routeName);
                }

                return MaterialPageRoute<dynamic>(
                  builder: (context) => page,
                  settings: settings,
                );
              },
              builder: (context, child) {
                return child ?? const Text("no child");
              },
            ));
  }
}

// You can pass any object to the arguments parameter.
// In this example, create a class that contains both
// a customizable title and message.
class ScreenArguments {
  final Widget? title;
  final String? screen;

  ScreenArguments(this.title, this.screen);
}
