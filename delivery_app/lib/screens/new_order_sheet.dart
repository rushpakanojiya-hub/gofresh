import 'package:flutter/material.dart';
import '../services/api_service.dart';
import 'order_pickup_screen.dart';

const Color _purple = Color(0xFF5B2A9E);
const Color _ringGreen = Color(0xFF1B8A4B);
const int _seconds = 30;

String _pick(Map<String, dynamic> o, List<String> keys) {
  for (final k in keys) {
    final v = o[k];
    if (v != null && v.toString().trim().isNotEmpty) return v.toString();
  }
  return '';
}

/// Half-screen popup shown when a new order is assigned.
class NewOrderSheet extends StatefulWidget {
  final Map<String, dynamic> order;
  const NewOrderSheet({super.key, required this.order});

  static Future<void> show(BuildContext context, Map<String, dynamic> order) {
    return showModalBottomSheet<void>(
      context: context,
      isDismissible: false,
      enableDrag: false,
      backgroundColor: Colors.transparent,
      barrierColor: const Color(0xFFE9E4F0),
      builder: (_) => NewOrderSheet(order: order),
    );
  }

  @override
  State<NewOrderSheet> createState() => _NewOrderSheetState();
}

class _NewOrderSheetState extends State<NewOrderSheet>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl;
  late final int _startLeft;
  bool _busy = false;
  String? _error;

  int get _id => ((widget.order['order_id'] ?? widget.order['id']) as num).toInt();

  int _initialLeft() {
    final raw = widget.order['assignment_expires_at'];
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
        if (s == AnimationStatus.completed && mounted && !_busy) {
          Navigator.of(context).pop();
        }
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
      await ApiService.acceptAssignment(_id);
      if (!mounted) return;
      final nav = Navigator.of(context);
      nav.pop();
      nav.push(MaterialPageRoute(builder: (_) => OrderPickupScreen(order: widget.order)));
    } catch (e) {
      if (mounted) {
        setState(() {
          _busy = false;
          _error = e.toString().replaceFirst('Exception: ', '');
        });
      }
    }
  }

  Future<void> _reject() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await ApiService.rejectAssignment(_id);
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
    final o = widget.order;
    final store = _pick(o, ['pickup_name', 'warehouse_name', 'store_name']);
    final addr = _pick(o, ['pickup_address', 'warehouse_address', 'store_address']);
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
              padding: const EdgeInsets.fromLTRB(20, 16, 20, 16),
              decoration: const BoxDecoration(
                color: Color(0xFFFFC72C),
                borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
              ),
              child: Column(
                children: [
                  Center(
                    child: AnimatedBuilder(
                      animation: _ctrl,
                      builder: (context, child) => CustomPaint(
                        painter: _PillRingPainter((_startLeft / _seconds) * (1 - _ctrl.value)),
                        child: child,
                      ),
                      child: const Padding(
                        padding: EdgeInsets.symmetric(horizontal: 28, vertical: 12),
                        child: Text('New order', style: TextStyle(fontWeight: FontWeight.w600)),
                      ),
                    ),
                  ),
                  const SizedBox(height: 14),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const Text('ORDER ID',
                              style: TextStyle(fontSize: 11, color: Colors.black54)),
                          Text('#$_id',
                              style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
                        ],
                      ),
                      const Icon(Icons.delivery_dining, size: 36, color: _purple),
                    ],
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
                    decoration: BoxDecoration(
                      color: Colors.black87,
                      borderRadius: BorderRadius.circular(4),
                    ),
                    child: const Text('Pick up',
                        style: TextStyle(color: Colors.white, fontSize: 12, fontWeight: FontWeight.bold)),
                  ),
                  const SizedBox(height: 8),
                  Text(store.isEmpty ? 'Pick up from the store' : store,
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
                  SizedBox(
                    width: double.infinity,
                    child: ElevatedButton(
                      onPressed: _busy ? null : _accept,
                      style: ElevatedButton.styleFrom(
                        backgroundColor: const Color(0xFF1ED760),
                        foregroundColor: Colors.black,
                        padding: const EdgeInsets.all(16),
                      ),
                      child: _busy
                          ? const SizedBox(
                              height: 20,
                              width: 20,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Text('Accept order',
                              style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
                    ),
                  ),
                  Center(
                    child: TextButton(
                      onPressed: _busy ? null : _reject,
                      child: const Text('Reject', style: TextStyle(color: Colors.red)),
                    ),
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
    canvas.drawPath(
      path,
      Paint()
        ..style = PaintingStyle.stroke
        ..strokeWidth = sw
        ..color = Colors.black12,
    );
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