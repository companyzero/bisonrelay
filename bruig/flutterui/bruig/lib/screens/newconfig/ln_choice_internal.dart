import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/buttons.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:flutter/material.dart';
import 'package:bruig/models/snackbar.dart';

class LNInternalWalletPage extends StatefulWidget {
  final NewConfigModel newconf;
  final SnackBarModel snackBar;
  const LNInternalWalletPage(this.newconf, this.snackBar, {Key? key})
      : super(key: key);

  @override
  State<LNInternalWalletPage> createState() => _LNInternalWalletPageState();
}

class _LNInternalWalletPageState extends State<LNInternalWalletPage> {
  SnackBarModel get snackBar => widget.snackBar;
  NewConfigModel get newconf => widget.newconf;
  TextEditingController passCtrl = TextEditingController();
  TextEditingController passRepeatCtrl = TextEditingController();
  bool loading = false;

  Future<void> createWallet() async {
    if (passCtrl.text == "") {
      snackBar.error("Password cannot be empty");
      return;
    }
    if (passCtrl.text != passRepeatCtrl.text) {
      snackBar.error("Passwords are different");
      return;
    }
    if (passCtrl.text.length < 8) {
      snackBar.error("Password cannot have less then 8 chars");
      return;
    }
    setState(() {
      loading = true;
    });
    try {
      await widget.newconf
          .createNewWallet(passCtrl.text, newconf.seedToRestore);
      Navigator.of(context).pushNamed("/newconf/seed");
    } catch (exception) {
      snackBar.error("Unable to create new LN wallet: $exception");
      if (newconf.seedToRestore.isNotEmpty) {
        // This assumes if there was a previously existing wallet, the user was
        // already given the choice to delete it.
        newconf.deleteLNWalletDir();
        Navigator.of(context).pushNamed("/newconf/restore");
      }
    } finally {
      setState(() {
        loading = false;
      });
    }
  }

  void startAdvancedSetup() {
    newconf.advancedSetup = true;
    Navigator.of(context).pushNamed("/newconf/networkChoice");
  }

  void startSeedRestore() {
    Navigator.of(context).pushNamed("/newconf/restore");
  }

  @override
  Widget build(BuildContext context) {
    var backgroundColor = const Color(0xFF19172C);
    var cardColor = const Color(0xFF05031A);
    var textColor = const Color(0xFF8E8D98);
    var secondaryTextColor = const Color(0xFFE4E3E6);
    var darkTextColor = const Color(0xFF5A5968);

    void goToAbout() {
      Navigator.of(context).pushNamed("/about");
    }

    return Container(
        color: backgroundColor,
        child: Stack(children: [
          Container(
              decoration: const BoxDecoration(
                  image: DecorationImage(
                      fit: BoxFit.fill,
                      image: AssetImage("assets/images/loading-bg.png")))),
          Container(
              decoration: BoxDecoration(
                  gradient: LinearGradient(
                      begin: Alignment.bottomLeft,
                      end: Alignment.topRight,
                      colors: [
                    cardColor,
                    const Color(0xFF07051C),
                    backgroundColor.withOpacity(0.34),
                  ],
                      stops: const [
                    0,
                    0.17,
                    1
                  ])),
              padding: const EdgeInsets.all(10),
              child: Column(children: [
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
                Text("Setting up Bison Relay",
                    style: TextStyle(
                        color: textColor,
                        fontSize: 34,
                        fontWeight: FontWeight.w200)),
                const SizedBox(height: 20),
                Text(
                    newconf.seedToRestore.isEmpty
                        ? "Creating New Wallet"
                        : "Restoring Wallet",
                    style: TextStyle(
                        color: secondaryTextColor,
                        fontSize: 21,
                        fontWeight: FontWeight.w300)),
                const SizedBox(height: 34),
                Column(children: [
                  SizedBox(
                      width: 377,
                      child: Text("Wallet Password",
                          textAlign: TextAlign.left,
                          style: TextStyle(
                              color: darkTextColor,
                              fontSize: 13,
                              fontWeight: FontWeight.w300))),
                  Center(
                      child: SizedBox(
                          width: 377,
                          child: TextField(
                              cursorColor: secondaryTextColor,
                              decoration: InputDecoration(
                                  border: InputBorder.none,
                                  hintText: "Password",
                                  hintStyle:
                                      TextStyle(fontSize: 21, color: textColor),
                                  filled: true,
                                  fillColor: cardColor),
                              style: TextStyle(
                                  color: secondaryTextColor, fontSize: 21),
                              controller: passCtrl,
                              obscureText: true))),
                  const SizedBox(height: 13),
                  SizedBox(
                      width: 377,
                      child: Text("Repeat Password",
                          textAlign: TextAlign.left,
                          style: TextStyle(
                              color: darkTextColor,
                              fontSize: 13,
                              fontWeight: FontWeight.w300))),
                  Center(
                    child: SizedBox(
                        width: 377,
                        child: TextField(
                            cursorColor: secondaryTextColor,
                            decoration: InputDecoration(
                                border: InputBorder.none,
                                hintText: "Confirm",
                                hintStyle:
                                    TextStyle(fontSize: 21, color: textColor),
                                filled: true,
                                fillColor: cardColor),
                            //decoration: InputDecoration(),
                            style: TextStyle(
                                color: secondaryTextColor, fontSize: 21),
                            controller: passRepeatCtrl,
                            obscureText: true)),
                  ),
                  const SizedBox(height: 34),
                  Center(
                      child: SizedBox(
                          width: 278,
                          child: Row(children: [
                            const SizedBox(width: 35),
                            LoadingScreenButton(
                              onPressed: !loading ? createWallet : null,
                              text: "Create Wallet",
                            ),
                            const SizedBox(width: 10),
                            loading
                                ? SizedBox(
                                    height: 25,
                                    width: 25,
                                    child: CircularProgressIndicator(
                                        value: null,
                                        backgroundColor: backgroundColor,
                                        color: textColor,
                                        strokeWidth: 2),
                                  )
                                : const SizedBox(width: 25),
                          ]))),
                ]),
                const Expanded(child: Empty()),
                Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                  !newconf.advancedSetup
                      ? TextButton(
                          onPressed: startAdvancedSetup,
                          child: Text("Advanced Setup",
                              style: TextStyle(color: textColor)),
                        )
                      : const Empty(),
                  newconf.seedToRestore.isEmpty
                      ? TextButton(
                          onPressed: startSeedRestore,
                          child: Text("Restore from Seed",
                              style: TextStyle(color: textColor)),
                        )
                      : const Empty(),
                ])
              ]))
        ]));
  }
}
