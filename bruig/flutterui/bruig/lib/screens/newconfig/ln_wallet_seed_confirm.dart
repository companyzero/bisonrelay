import 'dart:async';

import 'package:bruig/components/text.dart';
import 'package:bruig/models/newconfig.dart';
import 'package:bruig/screens/startupscreen.dart';
import 'package:flutter/material.dart';
import 'package:bruig/components/buttons.dart';
import 'package:provider/provider.dart';
import 'package:bruig/theme_manager.dart';

class ConfirmLNWalletSeedPage extends StatefulWidget {
  final NewConfigModel newconf;
  const ConfirmLNWalletSeedPage(this.newconf, {super.key});

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

    return StartupScreen(childrenWidth: 600, [
      const Txt.H("Setting up Bison Relay"),
      const SizedBox(height: 20),
      const Txt.L("Confirm New Wallet Seed"),
      const SizedBox(height: 34),
      AnimatedOpacity(
        opacity: _visible ? 1.0 : 0.0,
        duration: const Duration(milliseconds: 500),
        child: currentQuestion < confirmSeedWords.length
            ? !answerWrong
                ? QuestionArea(confirmSeedWords[currentQuestion], checkAnswer)
                : IncorrectArea(goBack)
            : Column(children: [
                const Txt.L("Seed Confirmed"),
                const SizedBox(height: 20),
                LoadingScreenButton(onPressed: done, text: "Continue")
              ]),
      )
    ]);
  }
}

class QuestionArea extends StatelessWidget {
  final ConfirmSeedWords currentWords;
  final Function(bool) checkAnswersCB;
  const QuestionArea(this.currentWords, this.checkAnswersCB, {super.key});

  @override
  Widget build(BuildContext context) {
    return Column(children: [
      Txt.H("Word #${currentWords.position + 1}"),
      const SizedBox(height: 20),
      SizedBox(
          width: double.infinity,
          child: Wrap(
              alignment: WrapAlignment.spaceEvenly,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                for (var i in currentWords.seedWordChoices)
                  Container(
                      margin: const EdgeInsets.all(5),
                      child: LoadingScreenButton(
                          onPressed: () =>
                              checkAnswersCB(currentWords.correctSeedWord == i),
                          text: i)),
              ]))
    ]);
  }
}

class IncorrectArea extends StatelessWidget {
  final Function() goBackCB;
  const IncorrectArea(this.goBackCB, {super.key});
  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Column(children: [
              const Text("Incorrect. Please go back and copy the seed again."),
              const SizedBox(height: 30),
              LoadingScreenButton(onPressed: goBackCB, text: "Go back"),
            ]));
  }
}
