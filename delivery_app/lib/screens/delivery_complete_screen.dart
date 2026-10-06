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

  String _addressText(Map<String, dynamic> o) {
    final a = o['address'] ?? o['delivery_address'];
    if (a is Map) {
      final parts = [a['line1'], a['line2'], a['city'], a['state'], a['pincode']]
          .where((v) => v != null && '$v'.trim().isNotEmpty)
          .map((v) => '$v');
      final s = parts.join(', ');
      return s.isEmpty ? '-' : s;
    }
    return (a == null || '$a'.isEmpty) ? '-' : '$a';
  }

  String _customerField(Map<String, dynamic> o, String flat, String key) {
    final direct = o[flat];
    if (direct != null && '$direct'.isNotEmpty) return '$direct';
    for (final holder in ['user', 'customer', 'address', 'delivery_address']) {
      final h = o[holder];
      if (h is Map) {
        final v = h[key] ?? (key == 'name' ? h['full_name'] : null);
        if (v != null && '$v'.isNotEmpty) return '$v';
      }
    }
    return '-';
  }

  List<String> _itemLines(Map<String, dynamic> o) {
    final raw = o['items'] ?? o['order_items'];
    if (raw is! List) return [];
    return raw.map((it) {
      if (it is! Map) return '$it';
      final p = it['product'];
      final name = it['name'] ?? it['product_name'] ?? (p is Map ? p['name'] : null) ?? 'Item';
      return '$name x ${it['quantity'] ?? it['qty'] ?? 1}';
    }).toList();
  }

  String _paymentText(Map<String, dynamic> o) {
    final m = '${_pick(o, ['payment_method']) ?? '-'}'.toLowerCase();
    final via = '${o['collected_via'] ?? ''}'.toLowerCase();
    if (m == 'cod') return via == 'upi' ? 'COD - paid by UPI' : 'COD - cash';
    return m == '-' ? '-' : m.toUpperCase();
  }

  Widget _detailRow(String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 96,
            child: Text(label, style: const TextStyle(fontSize: 13, color: Colors.black54)),
          ),
          Expanded(
            child: Text(value, style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w500)),
          ),
        ],
      ),
    );
  }

  void _showOrderDetails() {
    final o = widget.order;
    final items = _itemLines(o);
    final orderId = _pick(o, ['id', 'order_id']);
    final isReturn = o['type'] == 'return_pickup' || o['return_request_id'] != null;
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.white,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(18)),
      ),
      builder: (ctx) => SafeArea(
        child: ConstrainedBox(
          constraints: BoxConstraints(maxHeight: MediaQuery.of(ctx).size.height * 0.8),
          child: SingleChildScrollView(
            padding: const EdgeInsets.fromLTRB(20, 16, 20, 24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Center(
                  child: Container(
                    width: 40,
                    height: 4,
                    decoration: BoxDecoration(
                      color: Colors.black12,
                      borderRadius: BorderRadius.circular(2),
                    ),
                  ),
                ),
                const SizedBox(height: 14),
                Text(isReturn ? 'Return order details' : 'Order details',
                    style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
                const SizedBox(height: 10),
                _detailRow('Order', orderId == null ? '-' : '#$orderId'),
                if (isReturn) ...[
                  _detailRow('Type', 'Return pickup'),
                  _detailRow('Refund', o['total_amount'] == null ? '-' : '\u20B9${_money(o['total_amount'])}'),
                  _detailRow('Reason', '${o['reason'] ?? '-'}'),
                ] else ...[
                  _detailRow('Total', o['total_amount'] == null ? '-' : '\u20B9${_money(o['total_amount'])}'),
                  _detailRow('Payment', _paymentText(o)),
                ],
                const Divider(height: 24),
                const Text('Items', style: TextStyle(fontSize: 14, fontWeight: FontWeight.w600)),
                const SizedBox(height: 4),
                if (items.isEmpty)
                  const Padding(
                    padding: EdgeInsets.symmetric(vertical: 6),
                    child: Text('-', style: TextStyle(fontSize: 14)),
                  )
                else
                  for (final line in items)
                    Padding(
                      padding: const EdgeInsets.symmetric(vertical: 3),
                      child: Text(line, style: const TextStyle(fontSize: 14)),
                    ),
                const Divider(height: 24),
                const Text('Customer', style: TextStyle(fontSize: 14, fontWeight: FontWeight.w600)),
                const SizedBox(height: 4),
                _detailRow('Name', _customerField(o, 'customer_name', 'name')),
                _detailRow('Phone', _customerField(o, 'customer_phone', 'phone')),
                _detailRow('Address', _addressText(o)),
              ],
            ),
          ),
        ),
      ),
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
                      GestureDetector(
                        onTap: _showOrderDetails,
                        child: Container(
                          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 14),
                          decoration: BoxDecoration(
                            color: const Color(0xFFE6DAF3),
                            borderRadius: BorderRadius.circular(10),
                          ),
                          child: const Row(
                            children: [
                              Icon(Icons.receipt_long, size: 20, color: Colors.black87),
                              SizedBox(width: 10),
                              Expanded(
                                child: Text('Review order details',
                                    style: TextStyle(fontSize: 14, fontWeight: FontWeight.w500)),
                              ),
                              Icon(Icons.chevron_right, color: Colors.black54),
                            ],
                          ),
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