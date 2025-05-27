import 'dart:math';

import 'package:flutter/material.dart';

class VolumeGainControl extends StatefulWidget {
  final ValueChanged<double>? onChangedDelta;
  final ValueChanged<double>? onChanged;
  final double initialValue;
  const VolumeGainControl(
      {super.key, this.onChangedDelta, this.onChanged, this.initialValue = 0});

  @override
  State<VolumeGainControl> createState() => _VolumeGainControlState();
}

class _VolumeGainControlState extends State<VolumeGainControl> {
  // Default gain value of 0
  double _gainValue = 0.0;

  final double minGain = -40;
  final double maxGain = 20;

  double clamp(double v) => max(min(v, maxGain), minGain);

  @override
  void initState() {
    super.initState();
    _gainValue = clamp(widget.initialValue);
  }

  @override
  void didUpdateWidget(covariant VolumeGainControl oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget != widget) {
      _gainValue = clamp(widget.initialValue);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Slider(
      value: _gainValue,
      min: minGain,
      max: maxGain,
      divisions: (maxGain - minGain).floor(),
      // label: '${_gainValue.toStringAsFixed(1)} dB',
      onChanged: (double value) {
        setState(() {
          var delta = value - _gainValue;
          _gainValue = value;
          if (widget.onChangedDelta != null) {
            widget.onChangedDelta!(delta);
          }
          if (widget.onChanged != null) {
            widget.onChanged!(value);
          }
        });
      },
    );
  }
}
