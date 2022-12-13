import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';

class UserProfile extends StatefulWidget {
  final ClientModel client;
  final ChatModel chat;
  const UserProfile(this.client, this.chat, {Key? key}) : super(key: key);

  @override
  State<UserProfile> createState() => _UserProfileState();
}

class _UserProfileState extends State<UserProfile> {
  ChatModel get chat => widget.chat;
  bool isIgnored = false;
  bool loading = false;
  bool firstLoading = true;

  void readProfile() async {
    try {
      var newIgnored = await Golib.isIgnored(chat.id);

      setState(() {
        isIgnored = newIgnored;
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to load profile: $exception");
    } finally {
      setState(() {
        firstLoading = false;
      });
    }
  }

  void ignore() async {
    try {
      setState(() {
        loading = true;
      });
      await Golib.ignoreUser(chat.id);
      setState(() {
        isIgnored = true;
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to ignore user: $exception");
    } finally {
      setState(() {
        loading = false;
      });
    }
  }

  void unignore() async {
    try {
      setState(() {
        loading = true;
      });
      await Golib.unignoreUser(chat.id);
      setState(() {
        isIgnored = false;
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to un-ignore user: $exception");
    } finally {
      setState(() {
        loading = false;
      });
    }
  }

  void block() async {
    try {
      setState(() {
        loading = true;
      });
      await Golib.blockUser(chat.id);
      //Navigator.pop(context);
      widget.client.removeChat(chat);
    } catch (exception) {
      showErrorSnackbar(context, "Unable to block user: $exception");
    } finally {
      setState(() {
        loading = false;
      });
    }
  }

  @override
  void initState() {
    super.initState();
    readProfile();
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;

    return Column(
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Text("User Profile - ",
                style: TextStyle(color: textColor, fontSize: 20)),
            Text(chat.nick,
                style: TextStyle(
                    color: textColor,
                    fontSize: 20,
                    fontWeight: FontWeight.bold)),
          ],
        ),
        Text(chat.id,
            style: TextStyle(color: textColor, fontWeight: FontWeight.w100)),
        const SizedBox(height: 20),
        isIgnored
            ? ElevatedButton(
                onPressed: !loading ? unignore : null,
                child: const Text("Un-ignore user"))
            : ElevatedButton(
                onPressed: !loading ? ignore : null,
                child: const Text("Ignore user")),
        const SizedBox(height: 20),
        ElevatedButton(
          onPressed: !loading ? block : null,
          style: ElevatedButton.styleFrom(backgroundColor: theme.errorColor),
          child: const Text("Block User"),
        ),
        const Expanded(
          child: Text(""),
        ),
        ElevatedButton(
            onPressed: () => widget.client.profile = null,
            child: const Text("Done"))
      ],
    );
  }
}
