import 'package:flutter/material.dart';
import '../services/api_service.dart';
import 'home_shell.dart';

const Color _green = Color(0xFF1ED760);
const Color _pageBg = Color(0xFFFBEFEF);

String _money(dynamic v) {
  final n = v is num ? v : num.tryParse('${v ?? ''}');
  if (n == null) return '-';
  return n.toStringAsFixed(2);
}

dynamic _pick(Map<String, dynamic> o, List<String> keys) {
  for (final k in keys) {
    final v = o[k];
    if (v != null && '$v'.isNotEmpty) return v;
  }
  return null;
}

class _HeaderClipper extends CustomClipper<Path> {
  @override
  Path getClip(Size size) {
    final path = Path()
      ..lineTo(0, size.height - 36)
      ..quadraticBezierTo(size.width / 2, size.height + 16, size.width, size.height - 36)
      ..lineTo(size.width, 0)
      ..close();
    return path;
  }

  @override
  bool shouldReclip(covariant CustomClipper<Path> oldClipper) => false;
}

/// Shown after the delivery is confirmed: trip earnings, then X goes Home.
class DeliveryCompleteScreen extends StatefulWidget {
  final Map<String, dynamic> order;
  const DeliveryCompleteScreen({super.key, required this.order});

  @override
  State<DeliveryCompleteScreen> createState() => _DeliveryCompleteScreenState();
}

class _DeliveryCompleteScreenState extends State<DeliveryCompleteScreen> {
  late final Future<Map<String, dynamic>> _earnings = ApiService.getEarnings();

  void _home() {
    Navigator.of(context).pushAndRemoveUntil(
      MaterialPageRoute(builder: (_) => const HomeShell()),
      (route) => false,
    );
  }

  Widget _row(IconData icon, String label, String value, {bool plain = false}) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 9),
      child: Row(
        children: [
          if (!plain) ...[
            Icon(icon, size: 18, color: Colors.black87),
            const SizedBox(width: 10),
          ],
          Expanded(
            child: Text(label,
                style: TextStyle(
                    fontSize: 14,
                    color: plain ? Colors.black54 : Colors.black87)),
          ),
          Text(value, style: const TextStyle(fontSize: 14, color: Colors.black54)),
        ],
      ),
    );
  }

  Widget _card(List<Widget> children) {
    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 6),
      decoration: BoxDecoration(
        color: Colors.white.withValues(alpha: 0.6),
        border: Border.all(color: Colors.black12),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(children: children),
    );
  }

  @override
  Widget build(BuildContext context) {
    final o = widget.order;
    final dist = _pick(o, ['distance_km', 'delivery_distance_km', 'distance']);
    final mins = _pick(o, ['duration_mins', 'delivery_time_mins', 'duration']);
    final distText = dist == null ? '-' : '${_money(dist).replaceAll(RegExp(r'\.?0+$'), '')} kms';
    final timeText = mins == null ? '-' : '${num.tryParse('$mins')?.round() ?? mins} mins';
    final top = MediaQuery.of(context).padding.top;

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) {
        if (!didPop) _home();
      },
      child: Scaffold(
        backgroundColor: _pageBg,
        body: FutureBuilder<Map<String, dynamic>>(
          future: _earnings,
          builder: (context, snap) {
            final String amount;
            if (snap.hasData) {
              amount = _money(snap.data!['per_delivery_rate']);
            } else if (snap.hasError) {
              amount = '-';
            } else {
              amount = '...';
            }
            final sign = amount == '-' || amount == '...' ? '' : '\u20B9';
            final amt = '$sign$amount';
            return Column(
              children: [
                SizedBox(
                  height: top + 270,
                  child: Stack(
                    alignment: Alignment.topCenter,
                    children: [
                      ClipPath(
                        clipper: _HeaderClipper(),
                        child: Container(height: top + 220, width: double.infinity, color: _green),
                      ),
                      Positioned(
                        top: top + 4,
                        left: 4,
                        child: IconButton(
                          icon: const Icon(Icons.close, color: Colors.white, size: 26),
                          onPressed: _home,
                        ),
                      ),
                      Positioned(
                        top: top + 150,
                        child: Container(
                          width: 104,
                          height: 104,
                          padding: const EdgeInsets.all(7),
                          decoration: const BoxDecoration(color: Colors.white, shape: BoxShape.circle),
                          child: Container(
                            decoration: const BoxDecoration(color: _green, shape: BoxShape.circle),
                            child: const Icon(Icons.check, size: 56, color: Colors.white),
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
                Expanded(
                  child: ListView(
                    padding: const EdgeInsets.fromLTRB(16, 0, 16, 24),
                    children: [
                      const Center(
                        child: Text('Great job! Delivery complete \u{1F44D}',
                            style: TextStyle(fontSize: 16, fontWeight: FontWeight.w500)),
                      ),
                      const SizedBox(height: 8),
                      const Center(
                        child: Text('Trip earnings',
                            style: TextStyle(fontSize: 15, color: Colors.black54)),
                      ),
                      const SizedBox(height: 6),
                      Center(
                        child: Text(amt,
                            style: const TextStyle(fontSize: 34, fontWeight: FontWeight.bold)),
                      ),
                      const SizedBox(height: 20),
                      _card([
                        _row(Icons.two_wheeler, 'Trip pay', amt),
                        _row(Icons.currency_rupee, 'Trip earnings', amt, plain: true),
                      ]),
                      _card([
                        _row(Icons.alt_route, 'Trip distance', distText),
                        _row(Icons.access_time_filled, 'Trip time', timeText),
                      ]),
                      Container(
                        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 14),
                        decoration: BoxDecoration(
                          color: const Color(0xFFE6DAF3),
                          borderRadius: BorderRadius.circular(10),
                        ),
                        child: const Row(
                          children: [
                            Icon(Icons.error, size: 20, color: Colors.black87),
                            SizedBox(width: 10),
                            Expanded(
                              child: Text('Review customer address',
                                  style: TextStyle(fontSize: 14, fontWeight: FontWeight.w500)),
                            ),
                            Icon(Icons.chevron_right, color: Colors.black54),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            );
          },
        ),
      ),
    );
  }
}