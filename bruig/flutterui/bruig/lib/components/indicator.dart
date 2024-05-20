import 'package:flutter/material.dart';

class IndeterminateIndicator extends StatefulWidget {
  final double strokeWidth;
  const IndeterminateIndicator({super.key, this.strokeWidth = 4.0});

  @override
  State<IndeterminateIndicator> createState() => _IndeterminateIndicatorState();
}

class _IndeterminateIndicatorState extends State<IndeterminateIndicator>
    with SingleTickerProviderStateMixin {
  late AnimationController controller;

  @override
  void initState() {
    controller = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1000),
    )..addListener(() {
        setState(() {});
      });
    controller.repeat(reverse: true);
    super.initState();
  }

  @override
  void dispose() {
    controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return CircularProgressIndicator(
      value: controller.value,
      strokeWidth: widget.strokeWidth,
    );
  }
}

class RedDotIndicator extends StatelessWidget {
  const RedDotIndicator({super.key});

  @override
  Widget build(BuildContext context) {
    return const CircleAvatar(backgroundColor: Colors.red, radius: 4);
  }
}
