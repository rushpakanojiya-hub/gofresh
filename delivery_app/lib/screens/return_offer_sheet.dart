import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../widgets/swipe_confirm.dart';
import 'return_map_screen.dart';

const Color _ringGreen = Color(0xFF1B8A4B);
const int _seconds = 30;

String _pick(Map<String, dynamic> o, List<String> keys) {
  for (final k in keys) {
    final v = o[k];
    if (v != null && v.toString().trim().isNotEmpty) return v.toString();
  }
  return '';
}

/// Half-screen popup shown when a return pickup is assigned.
class ReturnOfferSheet extends StatefulWidget {
  final Map<String, dynamic> pickup;
  const ReturnOfferSheet({super.key, required this.pickup});

  static Future<void> show(BuildContext context, Map<String, dynamic> pickup) {
    return showModalBottomSheet<void>(
      context: context,
      isDismissible: false,
      enableDrag: false,
      backgroundColor: Colors.transparent,
      barrierColor: const Color(0xFFE9E4F0),
      builder: (_) => ReturnOfferSheet(pickup: pickup),
    );
  }

  @override
  State<ReturnOfferSheet> createState() => _ReturnOfferSheetState();
}

class _ReturnOfferSheetState extends State<ReturnOfferSheet> with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl;
  late final int _startLeft;
  bool _busy = false;
  String? _error;

  int get _id => (widget.pickup['return_request_id'] as num).toInt();

  int _initialLeft() {
    final raw = widget.pickup['pickup_assignment_expires_at'];
    final dt = raw == null ? null : DateTime.tryParse(raw.toString());
    if (dt == null) return _seconds;
    final secs = dt.difference(DateTime.now()).inSeconds;
    if (secs < 1) return 1;
    return secs > _seconds ? _seconds : secs;
  }

  @override
  void initState() {
    super.initState();
    _startLeft = _initialLeft();
    _ctrl = AnimationController(vsync: this, duration: Duration(seconds: _startLeft))
      ..addStatusListener((s) {
        if (s == AnimationStatus.completed && mounted && !_busy) Navigator.of(context).pop();
      })
      ..forward();
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  Future<void> _accept() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await ApiService.acceptReturnPickup(_id);
      if (!mounted) return;
      final nav = Navigator.of(context);
      final updated = {...widget.pickup, 'pickup_status': 'accepted'};
      nav.pop();
      nav.push(MaterialPageRoute(builder: (_) => ReturnMapScreen(pickup: updated)));
    } catch (e) {
      if (mounted) {
        setState(() {
          _busy = false;
          _error = e.toString().replaceFirst('Exception: ', '');
        });
      }
    }
  }

  Future<void> _deny() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await ApiService.rejectReturnPickup(_id);
      if (mounted) Navigator.of(context).pop();
    } catch (e) {
      if (mounted) {
        setState(() {
          _busy = false;
          _error = e.toString().replaceFirst('Exception: ', '');
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final o = widget.pickup;
    final name = _pick(o, ['customer_name']);
    final addr = _pick(o, ['delivery_address']);
    final orderId = _pick(o, ['order_id']);
    final refund = num.tryParse('${o['refund_amount'] ?? ''}');
    return Container(
      decoration: const BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      child: SafeArea(
        top: false,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: double.infinity,
              padding: const EdgeInsets.fromLTRB(20, 12, 12, 16),
              decoration: const BoxDecoration(
                color: Color(0xFFFFC72C),
                borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
              ),
              child: Column(
                children: [
                  Row(
                    children: [
                      const Spacer(),
                      TextButton(
                        onPressed: _busy ? null : _deny,
                        child: const Text('Deny', style: TextStyle(color: Colors.black87, fontWeight: FontWeight.w600)),
                      ),
                    ],
                  ),
                  Center(
                    child: AnimatedBuilder(
                      animation: _ctrl,
                      builder: (context, child) => CustomPaint(
                        painter: _PillRingPainter((_startLeft / _seconds) * (1 - _ctrl.value)),
                        child: child,
                      ),
                      child: const Padding(
                        padding: EdgeInsets.symmetric(horizontal: 28, vertical: 12),
                        child: Text('New return', style: TextStyle(fontWeight: FontWeight.w600)),
                      ),
                    ),
                  ),
                  const SizedBox(height: 14),
                  Padding(
                    padding: const EdgeInsets.only(right: 8),
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            const Text('ORDER ID', style: TextStyle(fontSize: 11, color: Colors.black54)),
                            Text('#$orderId', style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
                          ],
                        ),
                        if (refund != null)
                          Text('Refund \u20B9${refund.toStringAsFixed(2)}',
                              style: const TextStyle(fontWeight: FontWeight.w600)),
                      ],
                    ),
                  ),
                ],
              ),
            ),
            Padding(
              padding: const EdgeInsets.all(20),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                    decoration: BoxDecoration(color: const Color(0xFF3B2A8C), borderRadius: BorderRadius.circular(4)),
                    child: const Text('PICKUP FROM CUSTOMER',
                        style: TextStyle(color: Colors.white, fontSize: 12, fontWeight: FontWeight.bold)),
                  ),
                  const SizedBox(height: 8),
                  Text(name.isEmpty ? 'Customer' : name,
                      style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600)),
                  if (addr.isNotEmpty)
                    Padding(
                      padding: const EdgeInsets.only(top: 4),
                      child: Text(addr, style: const TextStyle(fontSize: 12, color: Colors.black54)),
                    ),
                  if (_error != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 10),
                      child: Text(_error!, style: const TextStyle(color: Colors.red)),
                    ),
                  const SizedBox(height: 16),
                  SwipeConfirm(
                    label: 'Accept return',
                    enabled: !_busy,
                    onConfirm: () => _accept(),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// White pill whose border is a green arc that shrinks as time runs out.
class _PillRingPainter extends CustomPainter {
  final double fraction;
  _PillRingPainter(this.fraction);

  @override
  void paint(Canvas canvas, Size size) {
    const sw = 3.5;
    final w = size.width - sw;
    final h = size.height - sw;
    final r = h / 2;
    final path = Path()
      ..moveTo(w / 2, 0)
      ..lineTo(w - r, 0)
      ..arcToPoint(Offset(w - r, h), radius: Radius.circular(r))
      ..lineTo(r, h)
      ..arcToPoint(Offset(r, 0), radius: Radius.circular(r))
      ..close();
    canvas.save();
    canvas.translate(sw / 2, sw / 2);
    canvas.drawPath(path, Paint()..color = Colors.white);
    canvas.drawPath(path, Paint()
      ..style = PaintingStyle.stroke
      ..strokeWidth = sw
      ..color = Colors.black12);
    final metric = path.computeMetrics().first;
    final f = fraction < 0 ? 0.0 : (fraction > 1 ? 1.0 : fraction);
    canvas.drawPath(
      metric.extractPath(0, metric.length * f),
      Paint()
        ..style = PaintingStyle.stroke
        ..strokeWidth = sw
        ..strokeCap = StrokeCap.round
        ..color = _ringGreen,
    );
    canvas.restore();
  }

  @override
  bool shouldRepaint(_PillRingPainter old) => old.fraction != fraction;
}