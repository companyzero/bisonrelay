import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:flutter/material.dart';

const _warnMsg = "There is an existing, incomplete install of"
    "the wallet. If this really was an unusued wallet, delete "
    "the wallet below and restart the setup process.\n\n"
    "THIS ACTION CANNOT BE REVERSED";

class DeleteOldWalletPage extends StatefulWidget {
  final NewConfigModel newconf;
  const DeleteOldWalletPage(this.newconf, {super.key});

  @override
  State<DeleteOldWalletPage> createState() => _DeleteOldWalletPageState();
}

class _DeleteOldWalletPageState extends State<DeleteOldWalletPage> {
  NewConfigModel get newconf => widget.newconf;
  bool deleteAccepted = false;
  bool deleting = false;

  void deleteWalletDir(BuildContext context) async {
    setState(() {
      deleting = true;
    });
    try {
      await newconf.deleteLNWalletDir();
    } catch (exception) {
      showErrorSnackbar(context, "Unable to delete wallet dir: $exception");
      return;
    }
    Navigator.of(context).pushReplacementNamed("/newconf/lnChoice/internal");
  }

  @override
  Widget build(BuildContext context) {
    var backgroundColor = const Color(0xFF19172C);
    var cardColor = const Color(0xFF05031A);
    var textColor = const Color(0xFF8E8D98);

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
                Text("Remove old wallet",
                    style: TextStyle(
                        color: textColor,
                        fontSize: 34,
                        fontWeight: FontWeight.w200)),
                const SizedBox(height: 20),
                Column(children: [
                  SizedBox(
                      width: 377,
                      child: Text(_warnMsg,
                          textAlign: TextAlign.left,
                          style: TextStyle(
                              color: textColor,
                              fontSize: 13,
                              fontWeight: FontWeight.w300))),
                  Center(
                    child: SizedBox(
                        width: 377,
                        child: CheckboxListTile(
                          title: Text("Wallet does not have any funds",
                              style: TextStyle(color: textColor)),
                          activeColor: textColor,
                          value: deleteAccepted,
                          side: BorderSide(color: textColor),
                          onChanged: (val) {
                            setState(() {
                              deleteAccepted = val ?? false;
                            });
                          },
                        )),
                  ),
                  const SizedBox(height: 34),
                  Center(
                      child: SizedBox(
                          width: 278,
                          child: Row(children: [
                            const SizedBox(width: 35),
                            LoadingScreenButton(
                              onPressed: deleteAccepted && !deleting
                                  ? () => deleteWalletDir(context)
                                  : null,
                              text: "Delete Wallet",
                            ),
                            const SizedBox(width: 10),
                          ])))
                ]),
              ]))
        ]));
  }
}
