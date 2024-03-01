import 'dart:async';
import 'dart:io';
import 'dart:math';
import 'dart:developer' as developer;

import 'package:bruig/components/attach_file.dart';
import 'package:bruig/components/route_error.dart';
import 'package:bruig/models/menus.dart';
import 'package:bruig/models/payments.dart';
import 'package:bruig/models/resources.dart';
import 'package:bruig/notification_service.dart';
import 'package:bruig/screens/about.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/screens/contacts_msg_times.dart';
import 'package:bruig/screens/fetch_invite.dart';
import 'package:bruig/screens/generate_invite.dart';
import 'package:bruig/screens/onboarding.dart';
import 'package:bruig/screens/server_unwelcome_error.dart';
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
import 'package:optimize_battery/optimize_battery.dart';

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

  // Get user to stop optimizing battery usage on Android.
  if (Platform.isAndroid) OptimizeBattery.stopOptimizingBatteryUsage();

  // The MockGolib was mostly useful during early stages of development.
  //UseMockGolib();

  var defAppDir = await defaultAppDataDir();
  developer.log("Default app dir: $defAppDir");

  try {
    Config cfg = await configFromArgs(args);
    await Golib.createLockFile(cfg.dbRoot);

    var runState = await Golib.getRunState();
    if (cfg.walletType == "internal" && !runState.dcrlndRunning) {
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
      ChangeNotifierProvider(create: (c) => ClientModel(), lazy: false),
      ChangeNotifierProvider(create: (c) => FeedModel()),
      ChangeNotifierProvider.value(value: globalLogModel),
      ChangeNotifierProvider(create: (c) => DownloadsModel()),
      ChangeNotifierProvider(create: (c) => AppNotifications()),
      ChangeNotifierProvider(create: (c) => ThemeNotifier()),
      ChangeNotifierProvider(create: (c) => MainMenuModel()),
      ChangeNotifierProvider(create: (c) => ResourcesModel()),
      ChangeNotifierProvider(create: (c) => SnackBarModel()),
      ChangeNotifierProvider(create: (c) => PaymentsModel()),
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
  final isMobile = Platform.isIOS || Platform.isAndroid;
  bool pushedToShutdown = false;

  @override
  void initState() {
    super.initState();
    !isMobile ? windowManager.addListener(this) : null;
    handleNotifications();
    initClient();
    !isMobile ? windowManager.setPreventClose(true) : null;
    NotificationService().init();
  }

  @override
  void dispose() {
    !isMobile ? windowManager.removeListener(this) : null;
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
        cfg.embedsDir,
        cfg.serverAddr,
        cfg.lnRPCHost,
        cfg.lnTLSCert,
        cfg.lnMacaroonPath,
        cfg.logFile,
        cfg.msgRoot,
        cfg.debugLevel,
        true,
        cfg.resourcesUpstream,
        cfg.simpleStorePayType,
        cfg.simpleStoreAccount,
        cfg.simpleStoreShipCharge,
        cfg.proxyaddr,
        cfg.torIsolation,
        cfg.proxyUsername,
        cfg.proxyPassword,
        cfg.circuitLimit,
        cfg.noLoadChatHistory,
        cfg.autoHandshakeInterval,
        cfg.autoRemoveIdleUsersInterval,
        cfg.autoRemoveIgnoreList,
        cfg.sendRecvReceipts,
        cfg.autoSubPosts,
      );
      await Golib.initClient(initArgs);
    } catch (exception) {
      print("XXXXXX $exception");
      if ("$exception".contains("client already initialized")) {
        // Not a fatal error, just resuming from a prior state. Consider the
        // addressbook loaded and start fetching client data.
        addressBookLoaded(true);
        return;
      }
      navkey.currentState!.pushNamed('/fatalError', arguments: exception);
    }
  }

  Future<void> doWalletChecks(bool wasAlreadyRunning) async {
    var ntfns = Provider.of<AppNotifications>(context, listen: false);
    try {
      var balances = await Golib.lnGetBalances();
      var pushed = false;
      bool hasOnboard = false;
      try {
        await Golib.readOnboard();
        hasOnboard = true;
      } catch (exception) {
        // Ignore because hasOnboard will be false.
      }

      var emptyAddressBook = (await Golib.addressBook()).isEmpty;

      if (emptyAddressBook || hasOnboard) {
        navkey.currentState!.pushNamed("/onboarding");

        // Do not perform other checks because they'll be taken care of during onboarding.
        return;
      }

      // The following checks are only done if this is not resuming from a background
      // transition (e.g. mobile notification received) to avoid showing them
      // multiple times.
      if (wasAlreadyRunning) {
        return;
      }

      if (balances.wallet.totalBalance == 0) {
        ntfns.addNtfn(AppNtfn(AppNtfnType.walletNeedsFunds));
        navkey.currentState!.pushNamed("/needsFunds");
        pushed = true;
      }
      if (balances.channel.maxOutboundAmount == 0) {
        ntfns.addNtfn(AppNtfn(AppNtfnType.walletNeedsChannels));
        if (!pushed) {
          navkey.currentState!.pushNamed("/needsOutChannel");
          pushed = true;
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

  Future<void> addressBookLoaded(bool wasAlreadyRunning) async {
    navkey.currentState!.pushReplacementNamed(OverviewScreen.routeName);
    await doWalletChecks(wasAlreadyRunning);
    var client = Provider.of<ClientModel>(context, listen: false);
    await client.fetchNetworkInfo();
    await client.readAddressBook();
    await client.fetchMyAvatar();
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

        case NTServerUnwelcomeError:
          Golib.remainOffline();
          var ntfns = Provider.of<AppNotifications>(context, listen: false);
          var msg = ntf.payload as String;
          ntfns.addNtfn(AppNtfn(AppNtfnType.serverUnwelcomeError, msg: msg));
          break;

        case NTAddressBookLoaded:
          await addressBookLoaded(false);
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
              debugShowCheckedModeBanner: false,
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
                '/generateInvite': (context) => const GenerateInviteScreen(),
                '/verifyInvite': (context) => const VerifyInviteScreen(),
                '/fetchInvite': (context) => const FetchInviteScreen(),
                '/ln/confirmRecvChannelPay': (context) =>
                    const LNConfirmRecvChanPaymentScreen(),
                '/confirmFileDownload': (context) =>
                    Consumer2<ClientModel, DownloadsModel>(
                        builder: (context, client, downloads, child) =>
                            ConfirmFileDownloadScreen(client, downloads)),
                AttachFileScreen.routeName: (context) =>
                    Consumer2<ClientModel, DownloadsModel>(
                        builder: (context, client, downloads, child) =>
                            const AttachFileScreen()),
                '/needsFunds': (context) => Consumer<AppNotifications>(
                    builder: (context, ntfns, child) =>
                        NeedsFundsScreen(ntfns)),
                '/needsInChannel': (context) =>
                    Consumer2<AppNotifications, ClientModel>(
                        builder: (context, ntfns, client, child) =>
                            NeedsInChannelScreen(ntfns, client)),
                '/onboarding': (context) => const OnboardingScreen(),
                ContactsLastMsgTimesScreen.routeName: (context) =>
                    Consumer<ClientModel>(
                        builder: (context, client, child) =>
                            ContactsLastMsgTimesScreen(client)),
                '/fatalError': (context) => const FatalErrorScreen(),
                ServerUnwelcomeErrorScreen.routeName: (context) =>
                    const ServerUnwelcomeErrorScreen(),
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
                  page = Consumer6<
                          DownloadsModel,
                          ClientModel,
                          AppNotifications,
                          MainMenuModel,
                          FeedModel,
                          SnackBarModel>(
                      builder: (context, down, client, ntfns, mainMenu, feed,
                              snackBar, child) =>
                          OverviewScreen(down, client, ntfns, initialRoute,
                              mainMenu, feed, snackBar));
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
