import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/models/notifications.dart';
import 'package:flutter/material.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

class _NotificationW extends StatelessWidget {
  final AppNotifications ntfns;
  final AppNtfn ntf;
  const _NotificationW(this.ntf, this.ntfns, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    String content = "(unknown ntf type)";
    String tooltip = "";
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;

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
    }

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => ListTile(
              title: Tooltip(
                  message: tooltip,
                  child: Text(
                    content,
                    style: TextStyle(
                        fontSize: theme.getSmallFont(), color: textColor),
                  )),
              leading: Icon(size: 10, Icons.arrow_forward, color: textColor),
              onTap: onTap,
            ));
  }
}

class NotificationsDrawerHeader extends StatefulWidget {
  final AppNotifications ntfns;
  const NotificationsDrawerHeader(this.ntfns, {Key? key}) : super(key: key);

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
    var theme = Theme.of(context);
    var appWarning = theme.bottomAppBarColor;
    return DrawerHeader(
        child: Column(children: [
      Align(
          alignment: Alignment.centerLeft,
          child: Icon(Icons.warning_amber_outlined, color: appWarning)),
      Expanded(
          child: ListView.builder(
        itemCount: list.length,
        itemBuilder: (context, index) => _NotificationW(list[index], ntfns),
      )),
    ]));
  }
}
