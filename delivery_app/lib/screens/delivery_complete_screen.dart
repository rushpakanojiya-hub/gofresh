import 'package:flutter/material.dart';
import '../services/api_service.dart';
import 'home_shell.dart';

const Color _green = Color(0xFF1ED760);
const Color _pageBg = Color(0xFFF7F1FB);

String _money(dynamic v) {
  final n = v is num ? v : num.tryParse('${v ?? ''}');
  if (n == null) return '-';
  return n.toStringAsFixed(2);
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

  @override
  void initState() {
    super.initState();
    _earnings.then((d) {
      if (!mounted || d['per_delivery_rate'] == null) return;
      _showPopup(_money(d['per_delivery_rate']));
    }).catchError((_) {});
  }

  void _showPopup(String amount) {
    final id = (widget.order['order_id'] ?? widget.order['id'] ?? '').toString();
    showDialog<void>(
      context: context,
      barrierDismissible: false,
      builder: (ctx) => Dialog(
        backgroundColor: Colors.white,
        insetPadding: const EdgeInsets.symmetric(horizontal: 24),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 72,
                height: 72,
                decoration: const BoxDecoration(color: _green, shape: BoxShape.circle),
                child: const Icon(Icons.payments_outlined, size: 36, color: Colors.black87),
              ),
              const SizedBox(height: 14),
              const Text('Great job!',
                  style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
              if (id.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.only(top: 4),
                  child: Text('Order : $id',
                      style: const TextStyle(fontSize: 12, color: Colors.black54)),
                ),
              const SizedBox(height: 18),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 28, vertical: 14),
                decoration: BoxDecoration(
                  border: Border.all(color: Colors.black12),
                  borderRadius: BorderRadius.circular(10),
                ),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text('\u20B9$amount',
                        style: const TextStyle(fontSize: 24, fontWeight: FontWeight.bold)),
                    const SizedBox(height: 4),
                    const Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.check, size: 14, color: _green),
                        SizedBox(width: 4),
                        Text('Trip earning',
                            style: TextStyle(fontSize: 11, color: Colors.black54)),
                      ],
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 18),
              SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  onPressed: () => Navigator.of(ctx).pop(),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: _green,
                    foregroundColor: Colors.black,
                    padding: const EdgeInsets.symmetric(vertical: 14),
                  ),
                  child: const Text('Okay', style: TextStyle(fontWeight: FontWeight.bold)),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _home() {
    Navigator.of(context).pushAndRemoveUntil(
      MaterialPageRoute(builder: (_) => const HomeShell()),
      (route) => false,
    );
  }

  @override
  Widget build(BuildContext context) {
    final id = (widget.order['order_id'] ?? widget.order['id'] ?? '').toString();

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) {
        if (!didPop) _home();
      },
      child: Scaffold(
        backgroundColor: _pageBg,
        body: Column(
          children: [
            Container(
              width: double.infinity,
              color: _green,
              child: SafeArea(
                bottom: false,
                child: Column(
                  children: [
                    Align(
                      alignment: Alignment.centerLeft,
                      child: IconButton(
                        icon: const Icon(Icons.close, color: Colors.black87),
                        onPressed: _home,
                      ),
                    ),
                    const Icon(Icons.check_circle, size: 64, color: Colors.black87),
                    const SizedBox(height: 10),
                    const Text('Delivery complete',
                        style: TextStyle(fontSize: 20, fontWeight: FontWeight.bold)),
                    if (id.isNotEmpty)
                      Padding(
                        padding: const EdgeInsets.only(top: 4),
                        child: Text('Order #$id',
                            style: const TextStyle(fontSize: 12, color: Colors.black54)),
                      ),
                    const SizedBox(height: 24),
                  ],
                ),
              ),
            ),
            Expanded(
              child: FutureBuilder<Map<String, dynamic>>(
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
                  return ListView(
                    padding: const EdgeInsets.all(16),
                    children: [
                      const SizedBox(height: 8),
                      const Center(
                        child: Text('Trip earnings',
                            style: TextStyle(fontSize: 13, color: Colors.black54)),
                      ),
                      const SizedBox(height: 6),
                      Center(
                        child: Text('$sign$amount',
                            style: const TextStyle(fontSize: 36, fontWeight: FontWeight.bold)),
                      ),
                      const SizedBox(height: 24),
                      Container(
                        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
                        color: Colors.white,
                        child: Row(
                          children: [
                            const Expanded(
                              child: Text('Trip pay',
                                  style: TextStyle(fontWeight: FontWeight.w600, fontSize: 14)),
                            ),
                            Text('$sign$amount',
                                style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 14)),
                          ],
                        ),
                      ),
                    ],
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }
}