import 'package:bruig/theme_manager.dart';
import 'package:flutter/material.dart';

class EqualizerIcon extends StatefulWidget {
  final bool isActive;
  final double width;
  final double height;

  const EqualizerIcon({
    super.key,
    this.isActive = false,
    this.width = 24.0,
    this.height = 24.0,
  });

  @override
  State<EqualizerIcon> createState() => _EqualizerIconState();
}

class _EqualizerIconState extends State<EqualizerIcon>
    with TickerProviderStateMixin {
  late List<AnimationController> _controllers;
  late List<Animation<double>> _animations;

  // Define different heights for each bar's animation
  final List<double> _maxHeights = [0.7, 0.9, 0.6, 1.0, 0.8];
  final List<int> _animationDurations = [900, 700, 800, 600, 750];
  final double _minHeight = 0.3;

  @override
  void initState() {
    super.initState();
    _setupAnimations();
  }

  void _setupAnimations() {
    // Initialize controllers and animations
    _controllers = List.generate(
      5,
      (index) => AnimationController(
        duration: Duration(milliseconds: _animationDurations[index]),
        vsync: this,
      ),
    );

    _animations = List.generate(
      5,
      (index) => Tween<double>(
        begin: _minHeight,
        end: _maxHeights[index],
      ).animate(
        CurvedAnimation(
          parent: _controllers[index],
          curve: Curves.easeInOut,
        ),
      ),
    );

    // Start animations if widget is active
    if (widget.isActive) {
      _startAnimations();
    }
  }

  void _startAnimations() {
    for (var controller in _controllers) {
      controller.repeat(reverse: true);
    }
  }

  void _stopAnimations() {
    for (var controller in _controllers) {
      // Instead of stop and reset, animate to the beginning
      if (controller.status == AnimationStatus.forward) {
        controller.reverse();
      }
      controller.animateTo(0.0, duration: const Duration(milliseconds: 300));
    }
  }

  @override
  void didUpdateWidget(EqualizerIcon oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.isActive != oldWidget.isActive) {
      if (widget.isActive) {
        _startAnimations();
      } else {
        _stopAnimations();
      }
    }
  }

  @override
  void dispose() {
    for (var controller in _controllers) {
      controller.dispose();
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var theme = ThemeNotifier.of(context, listen: false);
    return SizedBox(
      width: widget.width,
      height: widget.height,
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceEvenly,
        children: List.generate(
          5,
          (index) => AnimatedBuilder(
            animation: _animations[index],
            builder: (context, child) {
              return Container(
                width: widget.width / 7,
                height: widget.height * _animations[index].value,
                decoration: BoxDecoration(
                  color: theme.colors.primary,
                  borderRadius: BorderRadius.circular(widget.width / 14),
                ),
              );
            },
          ),
        ),
      ),
    );
  }
}
