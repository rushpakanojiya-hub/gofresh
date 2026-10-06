import 'package:flutter/material.dart';

/// Pill with a black knob the partner drags to the right to confirm.
class SwipeConfirm extends StatefulWidget {
  final String label;
  final VoidCallback onConfirm;
  final bool enabled;
  final Color color;
  const SwipeConfirm({
    super.key,
    required this.label,
    required this.onConfirm,
    this.enabled = true,
    this.color = const Color(0xFF1ED760),
  });

  @override
  State<SwipeConfirm> createState() => _SwipeConfirmState();
}

class _SwipeConfirmState extends State<SwipeConfirm> {
  static const double _knob = 52;
  double _dx = 0;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(builder: (context, c) {
      final maxDx = c.maxWidth - _knob - 8;
      return Container(
        height: _knob + 8,
        decoration: BoxDecoration(
          color: widget.enabled ? widget.color : Colors.grey.shade400,
          borderRadius: BorderRadius.circular(40),
        ),
        child: Stack(
          alignment: Alignment.centerLeft,
          children: [
            Center(
              child: Text(widget.label,
                  style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 15)),
            ),
            Positioned(
              left: 4 + _dx,
              child: GestureDetector(
                onHorizontalDragUpdate: widget.enabled
                    ? (d) => setState(() => _dx = (_dx + d.delta.dx).clamp(0.0, maxDx).toDouble())
                    : null,
                onHorizontalDragEnd: widget.enabled
                    ? (_) {
                        final ok = _dx >= maxDx * 0.85;
                        setState(() => _dx = 0);
                        if (ok) widget.onConfirm();
                      }
                    : null,
                child: Container(
                  width: _knob,
                  height: _knob,
                  decoration: const BoxDecoration(color: Colors.black, shape: BoxShape.circle),
                  child: const Icon(Icons.arrow_forward, color: Colors.white),
                ),
              ),
            ),
          ],
        ),
      );
    });
  }
}