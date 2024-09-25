import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/icons.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/notifications.dart';
import 'package:bruig/screens/server_unwelcome_error.dart';
import 'package:bruig/theme_manager.dart';
import 'package:flutter/material.dart';

class _NotificationW extends StatelessWidget {
  final AppNotifications ntfns;
  final AppNtfn ntf;
  const _NotificationW(this.ntf, this.ntfns);

  @override
  Widget build(BuildContext context) {
    String content = "(unknown ntf type)";
    String tooltip = "";

    Function() onTap = () {};

    switch (ntf.type) {
      case AppNtfnType.walletNeedsFunds:
        content = "Wallet Needs Funds";
        tooltip =
            "The wallet needs on-chain funds in order to open LN channels "
            "and perform payments to the server.";
        onTap = () {
          ntfns.delNtfn(ntf);
          Navigator.of(context, rootNavigator: true).pushNamed("/needsFunds");
        };
        break;

      case AppNtfnType.walletNeedsChannels:
        content = "Wallet Needs Outbound Channels";
        tooltip = "The wallet needs an LN channel with outbound capacity "
            "to perform payments to the server and other users.";
        onTap = () {
          ntfns.delNtfn(ntf);
          Navigator.of(context, rootNavigator: true)
              .pushNamed("/needsOutChannel");
        };
        break;

      case AppNtfnType.walletNeedsInChannels:
        content = "Wallet Needs Inbound Channels";
        tooltip = "The wallet needs an LN channel with inbound capacity "
            "to receive payments from other users.";
        onTap = () {
          ntfns.delNtfn(ntf);
          Navigator.of(context, rootNavigator: true)
              .pushNamed("/needsInChannel");
        };
        break;

      case AppNtfnType.error:
        content = "Error - ${ntf.msg}";
        tooltip = "${ntf.msg}";
        onTap = () {
          ntfns.delNtfn(ntf);
        };
        break;

      case AppNtfnType.walletCheckFailed:
        content = "Wallet check failed after server connection";
        tooltip =
            "${ntf.msg}\n\nAnother check will be performed in a few seconds";
        onTap = () {
          ntfns.delNtfn(ntf);
        };
        break;

      case AppNtfnType.invoiceGenFailed:
        content = "Failed to generate invoice to receive funds";
        tooltip =
            "${ntf.msg}\n\nOpen inbound channels to add receive capacity to your wallet.";
        onTap = () {
          ntfns.delNtfn(ntf);
          Navigator.of(context, rootNavigator: true)
              .pushNamed("/needsInChannel");
        };
        break;

      case AppNtfnType.serverUnwelcomeError:
        content = "Client software needs upgrade";
        tooltip = ntf.msg ?? "Client/server protocol negotiation error.";
        onTap = () {
          ntfns.delNtfn(ntf);
          Navigator.of(context, rootNavigator: true).pushReplacementNamed(
              ServerUnwelcomeErrorScreen.routeName,
              arguments: ntf.msg);
        };
        break;
    }

    return ListTile(
      title: Tooltip(
          message: tooltip,
          child: Txt.S(content, color: TextColor.onSurfaceVariant)),
      leading: const Txt.S("→", color: TextColor.onSurfaceVariant),
      onTap: onTap,
    );
  }
}

class NotificationsDrawerHeader extends StatefulWidget {
  final AppNotifications ntfns;
  const NotificationsDrawerHeader(this.ntfns, {super.key});

  @override
  State<NotificationsDrawerHeader> createState() =>
      _NotificationsDrawerHeaderState();
}

class _NotificationsDrawerHeaderState extends State<NotificationsDrawerHeader> {
  AppNotifications get ntfns => widget.ntfns;
  List<AppNtfn> list = [];

  void notificationsUpdated() {
    setState(() {
      list = ntfns.ntfns.toList();
    });
  }

  @override
  void initState() {
    super.initState();
    ntfns.addListener(notificationsUpdated);
    notificationsUpdated();
  }

  @override
  void didUpdateWidget(NotificationsDrawerHeader oldWidget) {
    oldWidget.ntfns.removeListener(notificationsUpdated);
    super.didUpdateWidget(oldWidget);
    ntfns.addListener(notificationsUpdated);
  }

  @override
  void dispose() {
    super.dispose();
    ntfns.removeListener(notificationsUpdated);
  }

  @override
  Widget build(BuildContext context) {
    if (ntfns.count == 0) {
      return const Empty();
    }
    return DrawerHeader(
        child: Column(children: [
      const Align(
          alignment: Alignment.centerLeft,
          child: ColoredIcon(Icons.warning_amber_outlined,
              color: TextColor.onSurfaceVariant)),
      Expanded(
          child: ListView.builder(
        itemCount: list.length,
        itemBuilder: (context, index) => _NotificationW(list[index], ntfns),
      )),
    ]));
  }
}
