import 'dart:async';
import 'dart:io';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/collapsable.dart';
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
import 'package:url_launcher/url_launcher.dart';

class _StageInfo {
  final String title;
  final String tip;
  final int step;

  _StageInfo({required this.title, required this.tip, required this.step});
}

Map<OnboardStage, _StageInfo> _stageInfos = {
  OnboardStage.stageFetchingInvite: _StageInfo(
      step: 1,
      title: "Fetching invite from server",
      tip:
          "The local client is attempting to connect to the server to fetch the invite file specified in the invite key."),
  OnboardStage.stageRedeemingFunds: _StageInfo(
    step: 2,
    title: "Redeeming invite funds on-chain",
    tip:
        "The local client is attempting to find the funds included in the invite file in the blockchain. If they are not mined yet, it will check again on every new block.",
  ),
  OnboardStage.stageWaitingFundsConfirm: _StageInfo(
    step: 3,
    title: "Waiting for on-chain funds to confirm",
    tip:
        "An on-chain transaction to redeem the invite funds was created and broadcast and now needs to be included in the blockchain.",
  ),
  OnboardStage.stageOpeningOutbound: _StageInfo(
    step: 4,
    title: "Opening outbound channel",
    tip:
        "The local client is attempting to create an LN channel with its on-chain funds to have outbound capacity to make payments through the Lightning Network.",
  ),
  OnboardStage.stageWaitingOutMined: _StageInfo(
    step: 5,
    title: "Waiting for outbound LN channel tx to be mined",
    tip:
        "An LN channel with outbound capacity has been created and now needs to be included in the blockchain (mined).",
  ),
  OnboardStage.stageWaitingOutConfirm: _StageInfo(
    step: 6,
    title: "Waiting for outbound LN channel to confirm",
    tip:
        "An LN channel with outbound capacity has been mined and now additional blocks need to be mined to make it usable.",
  ),
  OnboardStage.stageOpeningInbound: _StageInfo(
    step: 7,
    title: "Opening inbound LN channel",
    tip:
        "The local client is attempting to request that an LN node open a channel back to the client so that is can receive payments through the Lightning Network.",
  ),
  OnboardStage.stageInitialKX: _StageInfo(
    step: 8,
    title: "Performing initial KX with inviter",
    tip:
        "The local client is attempting to perform the Key Exchange (KX) procedure so that it can chat with the original inviter.",
  ),
  OnboardStage.stageOnboardDone: _StageInfo(
    step: 9,
    title: "Onboarding completed",
    tip: "",
  ),
  OnboardStage.stageInviteNoFunds: _StageInfo(
    step: 9, // Needs to be the same as stageOnboardDone
    title: "Invite does not have funds",
    tip:
        "The invite does not include on-chain funds that can be redeemed to setup the local wallet.",
  ),
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
  bool showInitialText = true;
  bool showLog = false;
  bool confirmingCancel = false;
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

  void cancelAndInputInviteKey() async {
    try {
      await Golib.cancelOnboard();
      setState(() {
        keyCtrl.clear();
        ostate = null;
      });
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

  void showConfirmCancel() {
    setState(() {
      confirmingCancel = true;
    });
  }

  @override
  void initState() {
    super.initState();
    listenOnboardChanges();
    readState();
  }

  @override
  Widget build(BuildContext context) {
    List<Widget> children;
    var themeNtf = Provider.of<ThemeNotifier>(context);
    var theme = themeNtf.getTheme();
    if (readingState) {
      children = [
        Consumer<ThemeNotifier>(
            builder: (context, theme, child) => Text(
                "Reading onboarding state...",
                style: TextStyle(color: theme.getTheme().dividerColor)))
      ];
    } else if (ostate == null && showInitialText) {
      const inviteURL = "https://bisonrelay.org/invites";
      children = [
        SizedBox(
            width: 600,
            child: Text(
                // ignore: prefer_interpolation_to_compose_strings, prefer_adjacent_string_concatenation
                "Bison Relay requires funded Lightning Network (LN) channels to send and receive messages.\n\n" +
                    "Also, users can only chat with each other after performing a Key Exchange (KX) process.\n\n" +
                    "Both of these can be done manually (by advanced users) or automatically using an Invite Key provided by an existing user.\n\n" +
                    "Read further instructions on how to obtain an invite key in the following website:",
                style: TextStyle(
                    color: theme.dividerColor, fontStyle: FontStyle.italic))),
        const SizedBox(height: 20),
        TextButton(
            onPressed: () async {
              try {
                if (!await launchUrl(Uri.parse(inviteURL))) {
                  showErrorSnackbar(
                      context, "Unable to launch browser to $inviteURL");
                }
              } catch (exception) {
                print("Unable to launch url: $exception");
              }
            },
            child: const Text(inviteURL)),
        const SizedBox(height: 20),
        Wrap(
          runSpacing: 10,
          children: [
            ElevatedButton(
                onPressed: () {
                  setState(() {
                    showInitialText = false;
                  });
                },
                child: const Text("I have an Invite Key")),
            const SizedBox(width: 20),
            CancelButton(onPressed: skipOnboarding, label: "Manual Setup"),
          ],
        ),
      ];
    } else if (confirmingCancel) {
      children = [
        SizedBox(
            width: 600,
            child: Text(
                // ignore: prefer_adjacent_string_concatenation, prefer_interpolation_to_compose_strings
                "You may completely abort the onboarding procedure or just " +
                    "skip the current attempt and go to the application's main UI " +
                    "(restart the software to restart the onboard process again).\n\n" +
                    "Note: completely aborting the onboarding procedure may or may not " +
                    "allow you to restart it, depending on how much of the setup the client has already performed.",
                style: TextStyle(
                    color: theme.dividerColor, fontStyle: FontStyle.italic))),
        const SizedBox(height: 20),
        Wrap(spacing: 20, runSpacing: 10, children: [
          ElevatedButton(
              onPressed: () {
                setState(() {
                  confirmingCancel = false;
                });
              },
              child: const Text("Continue onboarding")),
          ElevatedButton(
              onPressed: skipOnboarding, child: const Text("Skip to main app")),
          CancelButton(onPressed: cancelOnboarding, label: "Abort Onboarding"),
        ]),
      ];
    } else if (ostate == null) {
      children = [
        SizedBox(
            width: 600,
            child: TextField(
              controller: keyCtrl,
              decoration:
                  const InputDecoration(hintText: "Invite key (brpik1...)"),
            )),
        const SizedBox(height: 20),
        Wrap(
          runSpacing: 10,
          children: [
            ElevatedButton(
                onPressed: startOnboard, child: const Text("Start Onboard")),
            const SizedBox(width: 20),
            CancelButton(onPressed: skipOnboarding, label: "Manual Setup"),
          ],
        ),
      ];
    } else if (starting) {
      children = [
        Consumer<ThemeNotifier>(
            builder: (context, theme, child) => Text(
                "Starting onboarding procedure...",
                style: TextStyle(color: theme.getTheme().dividerColor)))
      ];
    } else if (ostate != null &&
        ostate!.stage == OnboardStage.stageInviteNoFunds) {
      var stageInfo = _stageInfos[ostate!.stage]!;
      children = [
        Text(
          stageInfo.title,
          style: TextStyle(
              color: theme.dividerColor,
              fontSize: themeNtf.getMediumFont(context)),
        ),
        const SizedBox(height: 3),
        SizedBox(
            width: 500,
            child: Text(
              stageInfo.tip,
              style: TextStyle(
                  color: theme.dividerColor,
                  fontStyle: FontStyle.italic,
                  fontSize: themeNtf.getSmallFont(context)),
            )),
        Wrap(runSpacing: 10, spacing: 10, children: [
          ElevatedButton(
            onPressed: cancelAndInputInviteKey,
            child: const Text("Use different invite key"),
          ),
          CancelButton(
            onPressed: showConfirmCancel,
            label: "Cancel Onboarding",
          )
        ]),
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
      var stageInfo = _stageInfos[ost.stage]!;
      var nbSteps = _stageInfos[OnboardStage.stageOnboardDone]!.step;
      children = [
        Text(
          "Step ${stageInfo.step} / $nbSteps",
          style: TextStyle(
              color: theme.dividerColor,
              fontSize: themeNtf.getLargeFont(context)),
        ),
        const SizedBox(height: 5),
        Text(
          stageInfo.title,
          style: TextStyle(
              color: theme.dividerColor,
              fontSize: themeNtf.getMediumFont(context)),
        ),
        const SizedBox(height: 3),
        SizedBox(
            width: 500,
            child: Text(
              ost.stage == OnboardStage.stageWaitingOutConfirm
                  ? "${stageInfo.tip} ${ost.outChannelConfsLeft} ${ost.outChannelConfsLeft == 1 ? 'block' : 'blocks'} left to confirm."
                  : stageInfo.tip,
              style: TextStyle(
                  color: theme.dividerColor,
                  fontStyle: FontStyle.italic,
                  fontSize: themeNtf.getSmallFont(context)),
            )),
        const SizedBox(height: 20),
        Collapsable("Additional Information",
            child: Column(children: [
              const SizedBox(height: 10),
              ...line(
                  "Balances - Wallet: $balWallet, Send: $balSend, Recv: $balRecv",
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
                      ...copyable(
                          "On-Chain redemption amount",
                          "${ost.redeemAmount}",
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
              ...(ost.stage == OnboardStage.stageWaitingOutConfirm
                  ? [
                      ...copyable(
                          "Confirmations left",
                          "${ost.outChannelConfsLeft}",
                          "How many confirmations left to enable the channel for use")
                    ]
                  : []),
            ])),
        const SizedBox(height: 10),
        Collapsable("Recent Log",
            child: SizedBox(
                height: 300,
                child: Consumer<LogModel>(
                    builder: (context, logModel, child) =>
                        LogLines(logModel)))),
        const SizedBox(height: 20),
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

        ...(oerror != "" && ost.stage == OnboardStage.stageOpeningInbound
            ? [
                SizedBox(
                    width: 600,
                    child: Text(
                        "Note: inbound channels are optional for sending messages. They can be opened later, when the local client needs inbound capacity to receive LN payments.",
                        style: TextStyle(
                            color: theme.dividerColor,
                            fontSize: themeNtf.getSmallFont(context)))),
                const SizedBox(height: 20),
              ]
            : []),

        // Action buttons.
        Wrap(runSpacing: 10, children: [
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
                      "Opening inbound channels is optional and can be done later",
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
                    onPressed: showConfirmCancel,
                    label: "Cancel Onboarding",
                  ))
              : ElevatedButton(
                  onPressed: skipOnboarding,
                  child: const Text("Start using Bison Relay")),
          const SizedBox(width: 20),
        ]),
        const SizedBox(height: 20),
      ];
    }

    return StartupScreen([
      Consumer<ThemeNotifier>(
          builder: (context, theme, child) => Text("Setting up Bison Relay",
              style: TextStyle(
                  color: theme.getTheme().dividerColor,
                  fontSize: theme.getHugeFont(context),
                  fontWeight: FontWeight.w200))),
      const SizedBox(height: 20),
      ...children,
    ]);
  }
}
