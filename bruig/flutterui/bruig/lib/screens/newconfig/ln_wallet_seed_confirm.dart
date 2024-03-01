import 'dart:async';

import 'package:bruig/models/newconfig.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:bruig/components/buttons.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

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

    var confirmSeedWords = widget.newconf.confirmSeedWords;

    return StartupScreen(Consumer<ThemeNotifier>(
      builder: (context, theme, _) => Column(children: [
        const SetupScreenAbountButton(),
        const SizedBox(height: 39),
        Text("Setting up Bison Relay",
            style: TextStyle(
                color: theme.getTheme().dividerColor,
                fontSize: theme.getHugeFont(context),
                fontWeight: FontWeight.w200)),
        const SizedBox(height: 20),
        Text("Confirm New Wallet Seed",
            style: TextStyle(
                color: theme.getTheme().focusColor,
                fontSize: theme.getLargeFont(context),
                fontWeight: FontWeight.w300)),
        const SizedBox(height: 34),
        AnimatedOpacity(
          opacity: _visible ? 1.0 : 0.0,
          duration: const Duration(milliseconds: 500),
          child: currentQuestion < confirmSeedWords.length
              ? !answerWrong
                  ? QuestionArea(confirmSeedWords[currentQuestion], checkAnswer)
                  : IncorrectArea(goBack)
              : Column(children: [
                  Text("Seed Confirmed",
                      style: TextStyle(
                          color: theme.getTheme().dividerColor,
                          fontSize: theme.getLargeFont(context),
                          fontWeight: FontWeight.w200)),
                  const SizedBox(height: 20),
                  Center(
                      child: LoadingScreenButton(
                    onPressed: done,
                    text: "Continue",
                  ))
                ]),
        ),
        const SizedBox(height: 34),
      ]),
    ));
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
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Column(children: [
              Center(
                  child: Text("Word #${currentWords.position + 1}",
                      style: TextStyle(
                          color: theme.getTheme().dividerColor,
                          fontSize: theme.getHugeFont(context),
                          fontWeight: FontWeight.w200))),
              const SizedBox(height: 20),
              SizedBox(
                  width: 600,
                  child: Flex(
                      direction:
                          isScreenSmall ? Axis.vertical : Axis.horizontal,
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        for (var i in currentWords.seedWordChoices)
                          Container(
                              margin: const EdgeInsets.all(5),
                              child: LoadingScreenButton(
                                onPressed: () => checkAnswersCB(
                                    currentWords.correctSeedWord == i),
                                text: i,
                              )),
                      ]))
            ]));
  }
}

class IncorrectArea extends StatelessWidget {
  final Function() goBackCB;
  const IncorrectArea(this.goBackCB, {Key? key}) : super(key: key);
  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Column(children: [
              Center(
                  child: Text(
                      "Incorrect, please go back and copy the seed again.",
                      textAlign: TextAlign.center,
                      style: TextStyle(
                          color: theme.getTheme().dividerColor,
                          fontSize: theme.getLargeFont(context),
                          fontWeight: FontWeight.w200))),
              const SizedBox(height: 20),
              Center(
                  child: LoadingScreenButton(
                onPressed: goBackCB,
                text: "Go back",
              )),
            ]));
  }
}
