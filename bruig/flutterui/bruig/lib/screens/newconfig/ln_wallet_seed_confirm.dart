import 'dart:async';

import 'package:bruig/models/newconfig.dart';
import 'package:flutter/material.dart';
import 'package:bruig/components/buttons.dart';

class ConfirmLNWalletSeedPage extends StatefulWidget {
  final NewConfigModel newconf;
  const ConfirmLNWalletSeedPage(this.newconf, {Key? key}) : super(key: key);

  @override
  State<ConfirmLNWalletSeedPage> createState() =>
      _ConfirmLNWalletSeedPageState();
}

class _ConfirmLNWalletSeedPageState extends State<ConfirmLNWalletSeedPage> {
  bool _visible = true;
  bool answerWrong = false;
  int currentQuestion = 0;

  void checkAnswer(bool answer) {
    setState(() {
      // Only update seed if it hasn't been already st
      _visible = false;
      Timer(const Duration(milliseconds: 500), () {
        setState(() {
          if (answer) {
            currentQuestion++;
          } else {
            answerWrong = true;
          }
          _visible = true;
        });
      });
    });
  }

  @override
  Widget build(BuildContext context) {
    void done() {
      Navigator.of(context).pushNamed("/newconf/server");
    }

    void goBack() {
      // Go back to copy seed page
      Navigator.of(context).pop();
      // Generate new seed questions
      widget.newconf.confirmSeedWords =
          widget.newconf.createConfirmSeedWords(widget.newconf.newWalletSeed);
    }

    void goToAbout() {
      Navigator.of(context).pushNamed("/about");
    }

    var backgroundColor = const Color(0xFF19172C);
    var cardColor = const Color(0xFF05031A);
    var textColor = const Color(0xFF8E8D98);
    var secondaryTextColor = const Color(0xFFE4E3E6);
    var confirmSeedWords = widget.newconf.confirmSeedWords;

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
              Text("Confirm New Wallet Seed",
                  style: TextStyle(
                      color: secondaryTextColor,
                      fontSize: 21,
                      fontWeight: FontWeight.w300)),
              const SizedBox(height: 34),
              AnimatedOpacity(
                opacity: _visible ? 1.0 : 0.0,
                duration: const Duration(milliseconds: 500),
                child: currentQuestion < confirmSeedWords.length
                    ? !answerWrong
                        ? QuestionArea(
                            confirmSeedWords[currentQuestion], checkAnswer)
                        : IncorrectArea(goBack)
                    : Column(children: [
                        Text("Seed Confirmed",
                            style: TextStyle(
                                color: textColor,
                                fontSize: 20,
                                fontWeight: FontWeight.w200)),
                        const SizedBox(height: 20),
                        Center(
                            child: LoadingScreenButton(
                          onPressed: done,
                          text: "Continue",
                        ))
                      ]),
              ),
              const SizedBox(height: 10),
              const SizedBox(height: 34),
            ]),
          )
        ]));
  }
}

class QuestionArea extends StatelessWidget {
  final ConfirmSeedWords currentWords;
  final Function(bool) checkAnswersCB;
  const QuestionArea(this.currentWords, this.checkAnswersCB, {Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    var textColor = const Color(0xFF8E8D98);
    return Column(children: [
      Center(
          child: Text("Word #${currentWords.position + 1}",
              style: TextStyle(
                  color: textColor,
                  fontSize: 34,
                  fontWeight: FontWeight.w200))),
      const SizedBox(height: 20),
      SizedBox(
          width: 600,
          child: Flex(
              direction: isScreenSmall ? Axis.vertical : Axis.horizontal,
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                for (var i in currentWords.seedWordChoices)
                  Container(
                      margin: const EdgeInsets.all(5),
                      child: LoadingScreenButton(
                        onPressed: () =>
                            checkAnswersCB(currentWords.correctSeedWord == i),
                        text: i,
                      )),
              ]))
    ]);
  }
}

class IncorrectArea extends StatelessWidget {
  final Function() goBackCB;
  const IncorrectArea(this.goBackCB, {Key? key}) : super(key: key);
  @override
  Widget build(BuildContext context) {
    var textColor = const Color(0xFF8E8D98);
    return Column(children: [
      Center(
          child: Text("Incorrect, please go back and copy the seed again.",
              textAlign: TextAlign.center,
              style: TextStyle(
                  color: textColor,
                  fontSize: 20,
                  fontWeight: FontWeight.w200))),
      const SizedBox(height: 20),
      Center(
          child: LoadingScreenButton(
        onPressed: goBackCB,
        text: "Go back",
      )),
    ]);
  }
}
