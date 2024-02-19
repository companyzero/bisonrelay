import 'dart:async';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/recent_log.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/log.dart';
import 'package:bruig/models/notifications.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

Map<OnboardStage, String> _stageTitle = {
  OnboardStage.stageFetchingInvite: "Fetching invite from server",
  OnboardStage.stageInviteNoFunds: "Invite does not have funds",
  OnboardStage.stageRedeemingFunds: "Redeeming invite funds on-chain",
  OnboardStage.stageWaitingFundsConfirm:
      "Waiting for on-chain funds to confirm",
  OnboardStage.stageOpeningOutbound:
      "Waiting for outbound LN channel to confirm", // Remove step heading
  OnboardStage.stageWaitingOutConfirm:
      "Waiting for outbound LN channel to confirm",
  OnboardStage.stageOpeningInbound: "Opening inbound LN channel",
  OnboardStage.stageInitialKX: "Performing initial KX with inviter",
  OnboardStage.stageOnboardDone: "Onboarding completed",
};

Map<OnboardStage, String> _stageTips = {
  OnboardStage.stageFetchingInvite:
      "The local client is attempting to connect to the server to fetch the invite file specified in the invite key.",
  OnboardStage.stageInviteNoFunds:
      "The invite does not include on-chain funds that can be redeemed to setup the local wallet.",
  OnboardStage.stageRedeemingFunds:
      "The local client is attempting to find and create a transaction to redeem the funds included in the invite file.",
  OnboardStage.stageWaitingFundsConfirm:
      "An on-chain transaction to redeem the on-chain funds was created and broadcast and now needs to be included in the blockchain.",
  OnboardStage.stageOpeningOutbound:
      "The local client is attempting to create an LN channel with its on-chain funds to have outbound capacity to make payments through the Lightning Network.",
  OnboardStage.stageWaitingOutConfirm:
      "An LN channel with outbound capacity has been created and now needs to be included in the blockchain with enough confirmation blocks to be usable (typically 3).",
  OnboardStage.stageOpeningInbound:
      "The local client is attempting to request that an LN node open a channel back to the cliet so that is can receive payments through the Lightning Network.",
  OnboardStage.stageInitialKX:
      "The local client is attempting to perform the Key Exchange (KX) procedure so that it can chat with the original inviter.",
  OnboardStage.stageOnboardDone:
      "Onboarding procedure is completed and the client can be used now.",
};

class OnboardingScreen extends StatefulWidget {
  const OnboardingScreen({super.key});

  @override
  State<OnboardingScreen> createState() => _OnboardingScreenState();
}

class _OnboardingScreenState extends State<OnboardingScreen> {
  OnboardState? ostate;
  String oerror = "";
  TextEditingController keyCtrl = TextEditingController();
  LNBalances balances = LNBalances.empty();
  bool readingState = true;
  bool starting = false;
  StreamSubscription<OnboardState>? listenSub;

  void readState() async {
    try {
      var newState = await Golib.readOnboard();
      var bal = await Golib.lnGetBalances();
      setState(() {
        ostate = newState;
        readingState = false;
        balances = bal;
      });
    } catch (exception) {
      // No onboarding state.
      setState(() {
        readingState = false;
      });
    }
  }

  void skipOnboarding() async {
    listenSub?.cancel();

    // Re-check notifications.
    var ntfns = Provider.of<AppNotifications>(context, listen: false);
    try {
      var balances = await Golib.lnGetBalances();
      if (balances.wallet.totalBalance == 0) {
        ntfns.addNtfn(AppNtfn(AppNtfnType.walletNeedsFunds));
      }
      if (balances.channel.maxOutboundAmount == 0) {
        ntfns.addNtfn(AppNtfn(AppNtfnType.walletNeedsChannels));
      }
      if (balances.channel.maxInboundAmount == 0) {
        ntfns.addNtfn(AppNtfn(AppNtfnType.walletNeedsInChannels));
      }
    } catch (exception) {
      // Ignore if the notifications are not added.
    }
    Navigator.pop(context);
  }

  void startOnboard() async {
    setState(() => starting = true);
    try {
      await Golib.startOnboard(keyCtrl.text);
    } catch (exception) {
      showErrorSnackbar(context, "Unable to start onboarding: $exception");
    } finally {
      setState(() => starting = false);
    }
  }

  void updateBalances() async {
    var bal = await Golib.lnGetBalances();
    setState(() => balances = bal);
  }

  void listenOnboardChanges() async {
    var stream = Golib.onboardStateChanged();
    stream = stream.handleError((err) => setState(() => oerror = err));
    listenSub = stream.listen((msg) {
      var changedStage = ostate?.stage != msg.stage;
      setState(() {
        ostate = msg;
        if (changedStage) {
          oerror = "";
        }
      });
      if (changedStage) {
        updateBalances();
      }
    });
  }

  void cancelOnboarding() async {
    try {
      await Golib.cancelOnboard();
      skipOnboarding();
    } catch (exception) {
      showErrorSnackbar(context, "Unable to cancel onboarding: $exception");
    }
  }

  void retryOnboarding() async {
    try {
      await Golib.retryOnboard();
    } catch (exception) {
      showErrorSnackbar(context, "Unable to retry onboarding: $exception");
    }
  }

  void skipOnboardingStage() async {
    try {
      await Golib.skipOnboardStage();
    } catch (exception) {
      showErrorSnackbar(context, "Unable to skip onboarding stage: $exception");
    }
  }

  @override
  void initState() {
    super.initState();
    listenOnboardChanges();
    readState();
  }

  @override
  Widget build(BuildContext context) {
    void goToAbout() {
      Navigator.of(context).pushNamed("/about");
    }

    List<Widget> children;
    if (readingState) {
      children = [
        Consumer<ThemeNotifier>(
            builder: (context, theme, child) => Text(
                "Reading onboarding state...",
                style: TextStyle(color: theme.getTheme().dividerColor)))
      ];
    } else if (ostate == null) {
      children = [
        SizedBox(
            width: 600,
            child: TextField(
              controller: keyCtrl,
              decoration: const InputDecoration(
                  hintText: "Input invite key (brpik1...)"),
            )),
        const SizedBox(height: 10),
        ElevatedButton(
            onPressed: startOnboard, child: const Text("Start Onboard")),
        const Expanded(child: Empty()),
        CancelButton(onPressed: skipOnboarding, label: "Skip Onboarding")
      ];
    } else if (starting) {
      children = [
        Consumer<ThemeNotifier>(
            builder: (context, theme, child) => Text(
                "Starting onboarding procedure...",
                style: TextStyle(color: theme.getTheme().dividerColor)))
      ];
    } else {
      var line = ((String s, String tt) => [
            Consumer<ThemeNotifier>(
                builder: (context, theme, child) => Tooltip(
                    message: tt,
                    child: Text(s,
                        style:
                            TextStyle(color: theme.getTheme().dividerColor)))),
            const SizedBox(height: 10),
          ]);
      var copyable = ((String lbl, String txt, String tt) => [
            Tooltip(
                message: tt,
                child: Consumer<ThemeNotifier>(
                    builder: (context, theme, child) => Copyable("$lbl: $txt",
                        TextStyle(color: theme.getTheme().dividerColor),
                        textToCopy: txt))),
            const SizedBox(height: 10),
          ]);

      var ost = ostate!;
      var balWallet = formatDCR(atomsToDCR(balances.wallet.totalBalance));
      var balSend = formatDCR(atomsToDCR(balances.channel.maxOutboundAmount));
      var balRecv = formatDCR(atomsToDCR(balances.channel.maxInboundAmount));
      children = [
        ...line("Current Stage: ${_stageTitle[ost.stage]}",
            "${_stageTips[ost.stage]}"),
        ...line("Balances - Wallet: $balWallet, Send: $balSend, Recv: $balRecv",
            """The wallet balance is the total confirmed and unconfirmed on-chain balance.
The send balance is how much the local client may send through LN payments.
The receive balance is how much the local client may receive through LN payments."""),
        ...line("Original Key: ${ost.key}",
            "The key used to fetch the invite and funds"),
        ...(ost.invite != null
            ? [
                ...copyable(
                    "Initial RV Point",
                    ost.invite?.initialRendezvous ?? "",
                    "The shared server ID where the remote client expects the local client to respond to the invite.")
              ]
            : []),
        ...(ost.invite?.funds != null
            ? [
                ...copyable(
                    "Invite funds UTXO",
                    "${ost.invite?.funds?.txid}:${ost.invite?.funds?.index}",
                    "The on-chain transaction where the invite's funds are stored")
              ]
            : []),
        ...(ost.redeemTx != null
            ? [
                ...copyable("On-Chain redemption tx", ost.redeemTx ?? "",
                    "The on-chain transaction where the invite's funds were redeemed to the local wallet")
              ]
            : []),
        ...(ost.redeemTx != null
            ? [
                ...copyable("On-Chain redemption amount", "${ost.redeemAmount}",
                    "The amount redeemed from the invite on-chain")
              ]
            : []),
        ...(ost.outChannelID != ""
            ? [
                ...copyable("Outbound channel ID", ost.outChannelID,
                    "The channelpoint (or ID) of the LN channel opened to a hub with outbound funds")
              ]
            : []),
        ...(ost.inChannelID != ""
            ? [
                ...copyable("Inbound channel ID", ost.inChannelID,
                    "The channelpoint (or ID) of the LN channel opened to a hub with inbound funds")
              ]
            : []),
        const SizedBox(height: 10),
        ...(oerror != ""
            ? [
                Consumer<ThemeNotifier>(
                    builder: (context, theme, child) => Copyable(
                        "Error: ${oerror}",
                        TextStyle(
                            color: theme.getTheme().errorColor,
                            fontWeight: FontWeight.bold))),
                const SizedBox(height: 20)
              ]
            : []),
        Row(mainAxisAlignment: MainAxisAlignment.center, children: [
          oerror != ""
              ? ElevatedButton(
                  onPressed: retryOnboarding,
                  child: const Text("Retry"),
                )
              : const Empty(),
          const SizedBox(width: 20),
          oerror != "" && ost.stage == OnboardStage.stageOpeningInbound
              ? Tooltip(
                  message:
                      "Inbound channels are optional for sending messages. They can be opened later, when the local client needs inbound capacity to receive LN payments.",
                  child: ElevatedButton(
                    onPressed: skipOnboardingStage,
                    child: const Text("Skip Inbound Channel"),
                  ))
              : const Empty(),
          const SizedBox(width: 20),
          ost.stage != OnboardStage.stageOnboardDone
              ? Tooltip(
                  message:
                      """Onboarding proceeds automatically unless there's an error, and once completed this window will close.
Cancelling onboarding means the wallet setup, including obtaining on-chain funds and opening LN channels will need to be done manually.""",
                  child: CancelButton(
                    onPressed: cancelOnboarding,
                    label: "Cancel Onboarding",
                  ))
              : ElevatedButton(
                  onPressed: skipOnboarding,
                  child: const Text("Onboarding Done")),
          const SizedBox(width: 20),
        ]),
        const SizedBox(height: 20),
        const Expanded(child: Empty()),
        Consumer<ThemeNotifier>(
            builder: (context, theme, child) => Text("Recent Log",
                style: TextStyle(
                    color: theme.getTheme().dividerColor,
                    fontSize: theme.getMediumFont(context)))),
        Expanded(
            child: Consumer<LogModel>(
                builder: (context, logModel, child) => LogLines(logModel))),
      ];
    }

    return StartupScreen(Column(children: [
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
      Consumer<ThemeNotifier>(
          builder: (context, theme, child) => Text("Setting up Bison Relay",
              style: TextStyle(
                  color: theme.getTheme().dividerColor,
                  fontSize: theme.getHugeFont(context),
                  fontWeight: FontWeight.w200))),
      const SizedBox(height: 20),
      ...children,
    ]));
  }
}
