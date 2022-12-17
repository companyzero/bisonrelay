import 'dart:async';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/chats_list.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/notifications.dart';
import 'package:bruig/screens/needs_out_channel.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';
import '../components/active_chat.dart';

class ChatsScreenTitle extends StatelessWidget {
  const ChatsScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer<ClientModel>(builder: (context, client, child) {
      var activeHeading = client.active;
      if (activeHeading == null) {
        return Text("Bison Relay / Chat",
            style:
                TextStyle(fontSize: 15, color: Theme.of(context).focusColor));
      }
      var chat = client.getExistingChat(activeHeading.id);
      var profile = client.profile;
      var suffix = chat?.nick != "" ? " / ${chat?.nick}" : "";
      var profileSuffix = profile != null
          ? chat!.isGC
              ? " / Manage Group Chat"
              : " / Profile"
          : "";
      return Text("Bison Relay / Chat$suffix$profileSuffix",
          style: TextStyle(fontSize: 15, color: Theme.of(context).focusColor));
    });
  }
}

class ChatsScreen extends StatefulWidget {
  static const routeName = '/chat';
  final ClientModel client;
  final AppNotifications ntfns;
  const ChatsScreen(this.client, this.ntfns, {Key? key}) : super(key: key);

  @override
  State<ChatsScreen> createState() => _ChatsScreenState();
}

class _FundsNeededPage extends StatelessWidget {
  const _FundsNeededPage({super.key});

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var backgroundColor = theme.backgroundColor;
    var textColor = const Color(0xFF8E8D98);
    var secondaryTextColor = const Color(0xFFE4E3E6);

    return Container(
        padding: const EdgeInsets.all(20),
        decoration: BoxDecoration(color: backgroundColor),
        child: Center(
            child: Column(
          children: [
            const SizedBox(height: 34),
            Text("Fund Wallet and Channels",
                style: TextStyle(
                    color: textColor,
                    fontSize: 34,
                    fontWeight: FontWeight.w200)),
            const SizedBox(height: 34),
            Text('''
Bison relay requires active LN channels with outbound capacity to pay to send
messages to the server.
''',
                style: TextStyle(
                    color: secondaryTextColor,
                    fontSize: 13,
                    fontWeight: FontWeight.w300)),
            const SizedBox(height: 34),
            Center(
              child:
                  Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                LoadingScreenButton(
                  onPressed: () => Navigator.of(context, rootNavigator: true)
                      .pushNamed("/needsFunds"),
                  text: "Add wallet funds",
                ),
                const SizedBox(width: 34),
                LoadingScreenButton(
                  onPressed: () => Navigator.of(context, rootNavigator: true)
                      .pushNamed(NeedsOutChannelScreen.routeName),
                  text: "Create outbound channels",
                )
              ]),
            ),
          ],
        )));
  }
}

class _InviteNeededPage extends StatelessWidget {
  const _InviteNeededPage({super.key});

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var backgroundColor = theme.backgroundColor;
    var textColor = const Color(0xFF8E8D98);
    var secondaryTextColor = const Color(0xFFE4E3E6);

    return Container(
        padding: const EdgeInsets.all(20),
        decoration: BoxDecoration(color: backgroundColor),
        child: Center(
            child: Column(
          children: [
            const SizedBox(height: 34),
            Text("Initial Invitation",
                style: TextStyle(
                    color: textColor,
                    fontSize: 34,
                    fontWeight: FontWeight.w200)),
            const SizedBox(height: 34),
            Text('''
Bison Relay does not rely on a central server for user accounts, so to chat
with someone else you need to exchange an invitation with them. This is 
just a file that should be sent via some other secure transfer method.

After the invitation is accepted, you'll be able to chat with them, and if they
know other people, they'll be able to connect you with them.
''',
                style: TextStyle(
                    color: secondaryTextColor,
                    fontSize: 13,
                    fontWeight: FontWeight.w300)),
            const SizedBox(height: 34),
            Center(
              child:
                  Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                LoadingScreenButton(
                  onPressed: () => loadInvite(context),
                  text: "Load Invitation",
                ),
                const SizedBox(width: 34),
                LoadingScreenButton(
                  onPressed: () => generateInvite(context),
                  text: "Create Invitation",
                )
              ]),
            ),
          ],
        )));
  }
}

class _ChatsScreenState extends State<ChatsScreen> {
  ClientModel get client => widget.client;
  AppNotifications get ntfns => widget.ntfns;
  ServerSessionState connState = ServerSessionState.empty();
  FocusNode editLineFocusNode = FocusNode();
  bool hasLNBalance = false;
  Timer? checkLNTimer;

  // check if ln wallet has balance. busywait, needs to be changed into a ntf.
  void keepCheckingLNHasBalance() async {
    if (client.chats.isNotEmpty) {
      // Doesn't matter, we already have contacts, so won't show onboard pages.
      return;
    }

    check() async {
      var balances = await Golib.lnGetBalances();
      var newHasBalance = balances.channel.maxOutboundAmount > 0;
      if (!newHasBalance) return false;
      if (mounted) {
        setState(() {
          hasLNBalance = newHasBalance;
        });
      }
      return true;
    }

    if (await check()) return;

    checkLNTimer = Timer.periodic(const Duration(seconds: 1), (timer) async {
      if (await check()) timer.cancel();
    });
  }

  void clientChanged() {
    var newConnState = client.connState;
    if (newConnState.state != connState.state ||
        newConnState.checkWalletErr != connState.checkWalletErr) {
      setState(() {
        connState = newConnState;
      });
      ntfns.delType(AppNtfnType.walletCheckFailed);
      if (newConnState.state == connStateCheckingWallet &&
          newConnState.checkWalletErr != null) {
        var msg = "LN wallet check failed: ${newConnState.checkWalletErr}";
        ntfns.addNtfn(AppNtfn(AppNtfnType.walletCheckFailed, msg: msg));
      }
    }
  }

  @override
  void initState() {
    super.initState();
    connState = widget.client.connState;
    widget.client.addListener(clientChanged);
    keepCheckingLNHasBalance();
  }

  @override
  void didUpdateWidget(ChatsScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.client.removeListener(clientChanged);
    widget.client.addListener(clientChanged);
  }

  @override
  void dispose() {
    widget.client.removeListener(clientChanged);
    checkLNTimer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var backgroundColor = theme.backgroundColor;

    if (client.chats.isEmpty) {
      if (!hasLNBalance) {
        // Only show f user never had any contacts.
        return const _FundsNeededPage();
      }
      return const _InviteNeededPage();
    }

    return Row(children: [
      Container(width: 163, child: ChatDrawerMenu(editLineFocusNode)),
      Expanded(
          child: Container(
        margin: const EdgeInsets.all(1),
        decoration: BoxDecoration(
          color: backgroundColor,
          borderRadius: BorderRadius.circular(3),
        ),
        child: ActiveChat(client, editLineFocusNode),
      )),
    ]);
  }
}
