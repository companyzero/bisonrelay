import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';

class RestoreWalletPage extends StatefulWidget {
  final NewConfigModel newconf;
  const RestoreWalletPage(this.newconf, {super.key});

  @override
  State<RestoreWalletPage> createState() => _RestoreWalletPageState();
}

class _RestoreWalletPageState extends State<RestoreWalletPage> {
  NewConfigModel get newconf => widget.newconf;
  TextEditingController seedCtrl = TextEditingController();

  @override
  void initState() {
    super.initState();
    seedCtrl.text = newconf.seedToRestore.join(" ");
  }

  void useSeed() {
    var split = seedCtrl.text.split(' ').map((s) => s.trim()).toList();
    split.removeWhere((s) => s.isEmpty);
    if (split.length != 24) {
      showErrorSnackbar(context,
          "Seed contains ${split.length} words instead of the required 24");
      return;
    }

    // TODO: prevalidate seed words?

    newconf.seedToRestore = split;
    Navigator.of(context).pushNamed("/newconf/lnChoice/internal");
  }

  @override
  Widget build(BuildContext context) {
    var backgroundColor = const Color(0xFF19172C);
    var cardColor = const Color(0xFF05031A);
    var textColor = const Color(0xFF8E8D98);
    var secondaryTextColor = const Color(0xFFE4E3E6);
    var darkTextColor = const Color(0xFF5A5968);
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
                const SizedBox(height: 89),
                Text("Restoring wallet",
                    style: TextStyle(
                        color: textColor,
                        fontSize: 34,
                        fontWeight: FontWeight.w200)),
                const SizedBox(height: 20),
                SizedBox(
                    width: 577,
                    child: Text("Seed Words",
                        textAlign: TextAlign.left,
                        style: TextStyle(
                            color: darkTextColor,
                            fontSize: 13,
                            fontWeight: FontWeight.w300))),
                Center(
                    child: SizedBox(
                        width: 577,
                        child: TextField(
                            maxLines: 5,
                            cursorColor: secondaryTextColor,
                            decoration: InputDecoration(
                                border: InputBorder.none,
                                hintText: "Seed words",
                                hintStyle:
                                    TextStyle(fontSize: 21, color: textColor),
                                filled: true,
                                fillColor: cardColor),
                            style: TextStyle(
                                color: secondaryTextColor, fontSize: 21),
                            controller: seedCtrl))),
                SizedBox(height: 34),
                Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                  LoadingScreenButton(
                    onPressed: useSeed,
                    text: "Continue",
                  ),
                ]),
              ]))
        ]));
  }
}
