import 'dart:async';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/collapsable.dart';
import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/info_grid.dart';
import 'package:bruig/components/recent_log.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/log.dart';
import 'package:bruig/models/notifications.dart';
import 'package:bruig/screens/fetch_invite.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:bruig/util.dart';
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
          "The local client is attempting to connect to the server to fetch the "
          "invite file specified in the invite key."),
  OnboardStage.stageInviteUnpaid: _StageInfo(
      step: 1,
      title: "Invite not paid on server",
      tip: "The server replied that the key was not paid for by the inviter. "
          "This can happen if the key is old, was not yet paid for by the sender, "
          "was paid on a different server or was somehow corrupted."),
  OnboardStage.stageInviteFetchTimeout: _StageInfo(
      step: 1,
      title: "Timeout waiting for invite",
      tip: "The server did not send the invite in a timely manner. This can "
          "happen if the inviter did not send the invite key to the server yet, "
          "or if the invite was already fetched."),
  OnboardStage.stageRedeemingFunds: _StageInfo(
    step: 2,
    title: "Redeeming invite funds on-chain",
    tip: "The local client is attempting to find the funds included in the "
        "invite file in the blockchain. If they are not mined yet, it will "
        "check again on every new block.",
  ),
  OnboardStage.stageWaitingFundsConfirm: _StageInfo(
    step: 3,
    title: "Waiting for on-chain funds to confirm",
    tip: "An on-chain transaction to redeem the invite funds was created and "
        "broadcast and now needs to be included in the blockchain.",
  ),
  OnboardStage.stageOpeningOutbound: _StageInfo(
    step: 4,
    title: "Opening outbound channel",
    tip: "The local client is attempting to create an LN channel with its "
        "on-chain funds to have outbound capacity to make payments through the "
        "Lightning Network.",
  ),
  OnboardStage.stageWaitingOutMined: _StageInfo(
    step: 5,
    title: "Waiting for outbound LN channel tx to be mined",
    tip: "An LN channel with outbound capacity has been created and now needs "
        "to be included in the blockchain (mined).",
  ),
  OnboardStage.stageWaitingOutConfirm: _StageInfo(
    step: 6,
    title: "Waiting for outbound LN channel to confirm",
    tip: "An LN channel with outbound capacity has been mined and now "
        "additional blocks need to be mined to make it usable.",
  ),
  OnboardStage.stageOpeningInbound: _StageInfo(
    step: 7,
    title: "Opening inbound LN channel",
    tip: "The local client is attempting to request that an LN node open a "
        "channel back to the client so that is can receive payments through the "
        "Lightning Network.",
  ),
  OnboardStage.stageInitialKX: _StageInfo(
    step: 8,
    title: "Performing initial KX with inviter",
    tip: "The local client is attempting to perform the Key Exchange (KX) "
        "procedure so that it can chat with the original inviter.",
  ),
  OnboardStage.stageOnboardDone: _StageInfo(
    step: 9,
    title: "Onboarding completed",
    tip: "",
  ),
  OnboardStage.stageInviteNoFunds: _StageInfo(
    step: 9, // Needs to be the same as stageOnboardDone
    title: "Invite does not have funds",
    tip: "The invite does not include on-chain funds that can be redeemed to "
        "setup the local wallet. This usually happens when the inviter fails to "
        "include funds in the invite. Request a new invite key.",
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
  String inviteKey = "";
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
    popNavigatorFromState(this);
  }

  void startOnboard() async {
    setState(() => starting = true);
    try {
      await Golib.startOnboard(inviteKey);
    } catch (exception) {
      showErrorSnackbar(this, "Unable to start onboarding: $exception");
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
      showErrorSnackbar(this, "Unable to cancel onboarding: $exception");
    }
  }

  void cancelAndInputInviteKey() async {
    try {
      await Golib.cancelOnboard();
      setState(() {
        inviteKey = "";
        ostate = null;
      });
    } catch (exception) {
      showErrorSnackbar(this, "Unable to cancel onboarding: $exception");
    }
  }

  void retryOnboarding() async {
    try {
      await Golib.retryOnboard();
    } catch (exception) {
      showErrorSnackbar(this, "Unable to retry onboarding: $exception");
    }
  }

  void skipOnboardingStage() async {
    try {
      await Golib.skipOnboardStage();
    } catch (exception) {
      showErrorSnackbar(this, "Unable to skip onboarding stage: $exception");
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
    if (readingState) {
      children = [const Text("Reading onboarding state...")];
    } else if (ostate == null && showInitialText) {
      const inviteURL = "https://bisonrelay.org/invites";
      children = [
        const Text(
            // ignore: prefer_interpolation_to_compose_strings, prefer_adjacent_string_concatenation
            "Bison Relay requires funded Lightning Network (LN) channels to send and receive messages.\n\n" +
                "Also, users can only chat with each other after performing a Key Exchange (KX) process.\n\n" +
                "Both of these can be done manually (by advanced users) or automatically using an Invite Key provided by an existing user.\n\n" +
                "Read further instructions on how to obtain an invite key in the following website:"),
        const SizedBox(height: 20),
        TextButton(
            onPressed: () async {
              try {
                if (!await launchUrl(Uri.parse(inviteURL))) {
                  showErrorSnackbar(
                      this, "Unable to launch browser to $inviteURL");
                }
              } catch (exception) {
                showErrorSnackbar(
                    this, "Error launching browser to $inviteURL: $exception");
              }
            },
            child: const Text(inviteURL)),
        const SizedBox(height: 20),
        Wrap(
          runSpacing: 10,
          children: [
            OutlinedButton(
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
        const Text(
            // ignore: prefer_adjacent_string_concatenation, prefer_interpolation_to_compose_strings
            "You may completely abort the onboarding procedure or just " +
                "skip the current attempt and go to the application's main UI " +
                "(restart the software to restart the onboard process again).\n\n" +
                "Note: completely aborting the onboarding procedure may or may not " +
                "allow you to restart it, depending on how much of the setup the client has already performed."),
        const SizedBox(height: 30),
        Wrap(spacing: 20, runSpacing: 10, children: [
          FilledButton(
              onPressed: () {
                setState(() {
                  confirmingCancel = false;
                });
              },
              child: const Text("Continue onboarding")),
          OutlinedButton(
              onPressed: skipOnboarding, child: const Text("Skip to main app")),
          CancelButton(onPressed: cancelOnboarding, label: "Abort Onboarding"),
        ]),
      ];
    } else if (ostate == null) {
      children = [
        InvitePanel(
          (newInvitePath, newInviteKey, newByKey) {
            setState(() {
              inviteKey = newByKey ? newInviteKey ?? "" : "";
            });
          },
        ),
        const SizedBox(height: 20),
        Wrap(
          runSpacing: 10,
          children: [
            ElevatedButton(
                onPressed: inviteKey != "" ? startOnboard : null,
                child: const Text("Start Onboard")),
            const SizedBox(width: 20),
            CancelButton(onPressed: skipOnboarding, label: "Manual Setup"),
          ],
        ),
      ];
    } else if (starting) {
      children = [const Text("Starting onboarding procedure...")];
    } else {
      var ost = ostate!;
      var balWallet = formatDCR(atomsToDCR(balances.wallet.totalBalance));
      var balSend = formatDCR(atomsToDCR(balances.channel.maxOutboundAmount));
      var balRecv = formatDCR(atomsToDCR(balances.channel.maxInboundAmount));
      var stageInfo = _stageInfos[ost.stage]!;
      var nbSteps = _stageInfos[OnboardStage.stageOnboardDone]!.step;

      // Stages for which the "Try new invite" action button is displayed.
      var newInviteStages = [
        OnboardStage.stageInviteNoFunds,
        OnboardStage.stageInviteUnpaid,
        OnboardStage.stageInviteFetchTimeout,
      ];

      children = [
        Txt.L("Step ${stageInfo.step} / $nbSteps"),
        const SizedBox(height: 5),
        Text(stageInfo.title),
        const SizedBox(height: 3),
        Txt.S(
          ost.stage == OnboardStage.stageWaitingOutConfirm
              ? "${stageInfo.tip} ${ost.outChannelConfsLeft} ${ost.outChannelConfsLeft == 1 ? 'block' : 'blocks'} left to confirm."
              : stageInfo.tip,
          style: const TextStyle(fontStyle: FontStyle.italic),
        ),
        const SizedBox(height: 20),
        Collapsable("Additional Information",
            child:
                SimpleInfoGridAdv(colLabelSize: 130, separatorWidth: 5, items: [
              [
                "Wallet Balance",
                Copyable(balWallet,
                    tooltip: "Total confirmed and unconfirmed on-chain balance")
              ],
              [
                "Send Capacity",
                Copyable(balSend,
                    tooltip:
                        "How much the local client may send through LN payments")
              ],
              [
                "Receive Capacity",
                Copyable(balRecv,
                    tooltip:
                        "How much the local client may receive through LN payments")
              ],
              [
                "Original Key",
                Copyable(ost.key ?? "",
                    tooltip: "The key used to fetch the invite and funds")
              ],
              if (ost.invite != null)
                [
                  "Initial RV",
                  Copyable(ost.invite?.initialRendezvous ?? "",
                      tooltip:
                          "The shared server ID where the remote client expects the local client to respond to the invite.")
                ],
              if (ost.invite?.funds != null)
                [
                  "Funds UTXO",
                  Copyable(
                      "${ost.invite?.funds?.txid}:${ost.invite?.funds?.index}",
                      tooltip:
                          "The on-chain transaction where the invite's funds are stored")
                ],
              if (ost.redeemTx != "")
                [
                  "Redemption TX",
                  Copyable(ost.redeemTx,
                      tooltip:
                          "The on-chain transaction where the invite's funds were redeemed to the local wallet")
                ],
              if (ost.redeemAmount > 0)
                [
                  "Redemption Amount",
                  Copyable(formatDCR(atomsToDCR(ost.redeemAmount)),
                      tooltip: "The amount redeemed from the invite on-chain")
                ],
              if (ost.outChannelID != "")
                [
                  "Outbound Channel ID",
                  Copyable(ost.outChannelID,
                      tooltip:
                          "The channelpoint (or ID) of the LN channel opened to a hub with outbound funds")
                ],
              if (ost.inChannelID != "")
                [
                  "Inbound Channel ID",
                  Copyable(ost.inChannelID,
                      tooltip:
                          "The channelpoint (or ID) of the LN channel opened to a hub with inbound funds")
                ],
              if (ost.stage == OnboardStage.stageWaitingOutConfirm)
                [
                  "Confirmations Left",
                  Copyable("${ost.outChannelConfsLeft}",
                      tooltip:
                          "How many confirmations left to enable the channel for use")
                ],
            ])),
        const SizedBox(height: 10),
        Collapsable("Recent Log",
            child: SizedBox(
                height: 300,
                child: Consumer<LogModel>(
                    builder: (context, logModel, child) =>
                        LogLines(logModel)))),
        const SizedBox(height: 20),
        if (oerror != "") ...[
          Copyable.txt(Txt("Error: $oerror", color: TextColor.error)),
          const SizedBox(height: 20)
        ],

        if (oerror != "" && ost.stage == OnboardStage.stageOpeningInbound) ...[
          const Txt.S(
            "Note: inbound channels are optional for sending messages. They can be opened later, when the local client needs inbound capacity to receive LN payments.",
          ),
          const SizedBox(height: 20),
        ],

        // Action buttons.
        Wrap(runSpacing: 10, spacing: 20, children: [
          if (oerror != "")
            OutlinedButton(
              onPressed: retryOnboarding,
              child: const Text("Retry"),
            ),
          if (newInviteStages.contains(ost.stage))
            OutlinedButton(
              onPressed: cancelAndInputInviteKey,
              child: const Text("Use different invite key"),
            ),
          if (oerror != "" && ost.stage == OnboardStage.stageOpeningInbound)
            Tooltip(
                message:
                    "Opening inbound channels is optional and can be done later",
                child: OutlinedButton(
                  onPressed: skipOnboardingStage,
                  child: const Text("Skip Inbound Channel"),
                )),
          ost.stage != OnboardStage.stageOnboardDone
              ? Tooltip(
                  message:
                      """Onboarding proceeds automatically unless there's an error, and once completed this window will close.
Cancelling onboarding means the wallet setup, including obtaining on-chain funds and opening LN channels will need to be done manually.""",
                  child: CancelButton(
                    onPressed: showConfirmCancel,
                    label: "Cancel Onboarding",
                  ))
              : FilledButton(
                  onPressed: skipOnboarding,
                  child: const Text("Start using Bison Relay")),
        ]),
        const SizedBox(height: 20),
      ];
    }

    return StartupScreen(childrenWidth: 620, [
      const Txt.H("Setting up Bison Relay"),
      const SizedBox(height: 20),
      ...children,
    ]);
  }
}
