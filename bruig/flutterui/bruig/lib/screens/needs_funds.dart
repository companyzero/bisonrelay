import 'dart:async';

import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/info_grid.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/notifications.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:golib_plugin/util.dart';
import 'package:qr_flutter/qr_flutter.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

class NeedsFundsScreen extends StatefulWidget {
  final AppNotifications ntfns;
  const NeedsFundsScreen(this.ntfns, {Key? key}) : super(key: key);

  @override
  State<NeedsFundsScreen> createState() => _NeedsFundsScreenState();
}

class _NeedsFundsScreenState extends State<NeedsFundsScreen> {
  String addr = "";
  int confirmedBalance = 0;
  int unconfirmedBalance = 0;
  Timer? updateTimer;
  bool createdNeedsChanNtf = false;
  bool redeeming = false;
  RedeemedInviteFunds? redeemed;
  bool? forwardIfBalance;

  void getNewAddress() async {
    var snackbar = SnackBarModel.of(context);
    try {
      var res = await Golib.lnGetDepositAddr("");
      setState(() {
        addr = res;
      });
    } catch (exception) {
      snackbar.error("Unable to load deposit address: $exception");
    }
  }

  void updateBalance() async {
    var done = false;
    try {
      var res = await Golib.lnGetBalances();
      setState(() {
        confirmedBalance = res.wallet.confirmedBalance;
        unconfirmedBalance = res.wallet.unconfirmedBalance;
      });

      if (res.wallet.confirmedBalance > 0) {
        widget.ntfns.delType(AppNtfnType.walletNeedsFunds);

        if (res.channel.maxOutboundAmount == 0 && !createdNeedsChanNtf) {
          widget.ntfns.addNtfn(AppNtfn(AppNtfnType.walletNeedsChannels));
          createdNeedsChanNtf = true;

          if (forwardIfBalance ?? false) {
            Navigator.of(context, rootNavigator: true).pop();
            Navigator.of(context, rootNavigator: true)
                .pushNamed("/needsOutChannel");
            done = true;
          }
          forwardIfBalance = false;
        }
      } else {
        forwardIfBalance = true;
      }
    } catch (exception) {
      showErrorSnackbar(this, "Unable to update wallet balance: $exception");
    } finally {
      if (!done) {
        updateTimer = Timer(const Duration(seconds: 5), updateBalance);
      }
    }
  }

  void redeemFunds() async {
    var snackbar = SnackBarModel.of(context);

    try {
      // Decode the invite and send to the user verification screen.
      var filePickRes = await FilePicker.platform.pickFiles();
      if (filePickRes == null) return;
      var filePath = filePickRes.files.first.path;
      if (filePath == null) return;
      filePath = filePath.trim();
      if (filePath == "") return;
      var invite = await Golib.decodeInvite(filePath);
      if (invite.invite.funds == null) {
        throw "Invite does not include funds";
      }
      setState(() => redeeming = true);
      var res = await Golib.redeemInviteFunds(invite.invite.funds!);
      setState(() => redeemed = res);
    } catch (exception) {
      setState(() => redeeming = false);
      snackbar.error("Unable to redeem invite funds: $exception");
    }
  }

  @override
  void initState() {
    super.initState();
    getNewAddress();
    updateBalance();
  }

  @override
  void dispose() {
    updateTimer?.cancel();
    super.dispose();
  }

  List<Widget> buildFundsWidgets(BuildContext context) {
    if (redeemed != null) {
      var total = formatDCR(atomsToDCR(redeemed!.total));
      return [
        Text("Redeemed $total on the following tx:"),
        Copyable(redeemed!.txid),
        const Text("The funds will be available after the tx is mined."),
      ];
    }

    if (redeeming) {
      return [
        const Text("Attempting to redeem funds from invite file.\n"),
      ];
    }

    return [
      const Text('''
If someone sent you an invite file with funds, you may also attempt to redeem it by clicking the button.
'''),
      OutlinedButton(onPressed: redeemFunds, child: const Text("Redeem Funds")),
    ];
  }

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
      builder: (context, theme, child) => StartupScreen(
        childrenWidth: 650,
        [
          const Txt.H("Setting up Bison Relay"),
          const SizedBox(height: 20),
          const Txt.L("Receive Wallet Funds"),
          const SizedBox(height: 20),
          const Text('''
The wallet requires on-chain DCR funds to be able to open Lightning Network (LN) channels and perform payments to the server and other users of the Bison Relay network.

Send DCR funds to the following address to receive funds in your wallet. Note that the wallet seed will be needed to recover these funds if the wallet data in this computer is corrupted or lost.
'''),
          ...buildFundsWidgets(context),
          const SizedBox(height: 13),
          Container(
              margin: const EdgeInsets.all(10),
              color: Colors.white,
              child: QrImageView(
                data: addr,
                version: QrVersions.auto,
                size: 200.0,
              )),
          const SizedBox(height: 13),
          Copyable(addr),
          const SizedBox(height: 15),
          SimpleInfoGridAdv(
              rowAlignment: MainAxisAlignment.center,
              colLabelSize: 180,
              items: [
                [
                  "Unconfirmed wallet balance",
                  formatDCR(atomsToDCR(unconfirmedBalance))
                ],
                [
                  "Confirmed wallet balance",
                  formatDCR(atomsToDCR(confirmedBalance))
                ],
              ]),
          const SizedBox(height: 20),
          LoadingScreenButton(
            onPressed: () => Navigator.of(context).pop(),
            text: "Finish",
          ),
        ],
      ),
    );
  }
}
