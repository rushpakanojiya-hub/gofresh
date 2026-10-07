import 'package:flutter/material.dart';
import 'dart:async';
import '../services/api_service.dart';
import 'delivery_map_screen.dart';

const Color _green = Color(0xFF1ED760);
const Color _pageBg = Color(0xFFF7F1FB);

String _pick(Map<String, dynamic> o, List<String> keys) {
  for (final k in keys) {
    final v = o[k];
    if (v != null && v.toString().trim().isNotEmpty) return v.toString();
  }
  return '';
}

String _money(String raw) {
  final v = double.tryParse(raw);
  if (v == null) return raw.isEmpty ? '-' : raw;
  return v.toStringAsFixed(2);
}

/// Shown right after the partner accepts an order: what to collect and from
/// where, then swipe "Order picked" once the items are in hand.
class OrderPickupScreen extends StatefulWidget {
  final Map<String, dynamic> order;
  final bool ready;
  const OrderPickupScreen({super.key, required this.order, this.ready = false});

  @override
  State<OrderPickupScreen> createState() => _OrderPickupScreenState();
}

class _OrderPickupScreenState extends State<OrderPickupScreen> {
  bool _busy = false;
  String? _error;
  bool _storeReady = false;
  Timer? _poll;

  @override
  void initState() {
    super.initState();
    final s = widget.order['status']?.toString();
    _storeReady = widget.ready || s == 'ready_for_dispatch' || s == 'handed_over';
    if (!widget.ready) {
      _checkReady();
      _poll = Timer.periodic(const Duration(seconds: 4), (_) => _checkReady());
    }
  }

  @override
  void dispose() {
    _poll?.cancel();
    super.dispose();
  }

  Future<void> _checkReady() async {
    try {
      final list = await ApiService.getMyDeliveries();
      for (final o in list) {
        if (o is Map && '${o['order_id'] ?? o['id']}' == '$_id') {
          final s = o['status']?.toString();
          final ok = s == 'ready_for_dispatch' || s == 'handed_over' || s == 'shipped';
          if (ok != _storeReady && mounted) setState(() => _storeReady = ok);
          break;
        }
      }
    } catch (_) {}
  }

  void _iAmReady() {
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(builder: (_) => OrderPickupScreen(order: widget.order, ready: true)),
    );
  }

  int get _id => ((widget.order['order_id'] ?? widget.order['id']) as num).toInt();

  Future<void> _picked() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await ApiService.pickupOrder(_id);
      if (!mounted) return;
      final updated = Map<String, dynamic>.from(widget.order)
        ..['status'] = 'shipped'
        ..['delivery_status'] = 'picked_up';
      Navigator.of(context).pushReplacement(
        MaterialPageRoute(builder: (_) => DeliveryMapScreen(order: updated)),
      );
    } catch (e) {
      if (mounted) {
        setState(() {
          _busy = false;
          _error = e.toString().replaceFirst('Exception: ', '');
        });
      }
    }
  }

  Widget _section({
    required IconData icon,
    required String title,
    String? subtitle,
    bool open = false,
    required List<Widget> children,
  }) {
    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      color: Colors.white,
      child: Theme(
        data: Theme.of(context).copyWith(dividerColor: Colors.transparent),
        child: ExpansionTile(
          initiallyExpanded: open,
          leading: Icon(icon, size: 20, color: Colors.black87),
          title: Text(title, style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 14)),
          subtitle: subtitle == null
              ? null
              : Text(subtitle, style: const TextStyle(fontSize: 12, color: Colors.black54)),
          childrenPadding: const EdgeInsets.fromLTRB(16, 0, 16, 14),
          expandedAlignment: Alignment.centerLeft,
          expandedCrossAxisAlignment: CrossAxisAlignment.start,
          children: children,
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final o = widget.order;
    final store = _pick(o, ['pickup_name', 'warehouse_name', 'store_name']);
    final addr = _pick(o, ['pickup_address', 'warehouse_address', 'store_address']);
    final items = (o['items'] is List) ? o['items'] as List : const [];
    final custName = _pick(o, ['customer_name']);
    final custPhone = _pick(o, ['customer_phone']);
    final custAddr = _pick(o, ['delivery_address']);
    final pay = _pick(o, ['payment_method']).toUpperCase();
    final total = _pick(o, ['total_amount']);

    return Scaffold(
      backgroundColor: _pageBg,
      appBar: AppBar(
        backgroundColor: Colors.white,
        elevation: 0,
        iconTheme: const IconThemeData(color: Colors.black87),
      ),
      body: SafeArea(
        child: Column(
          children: [
            Expanded(
              child: ListView(
                children: [
                  Container(
                    color: Colors.white,
                    padding: const EdgeInsets.only(bottom: 18),
                    margin: const EdgeInsets.only(bottom: 8),
                    child: Column(
                      children: [
                        const Text('ORDER ID',
                            style: TextStyle(fontSize: 11, color: Colors.black54)),
                        const SizedBox(height: 6),
                        Text('#$_id',
                            style: const TextStyle(fontSize: 28, fontWeight: FontWeight.bold)),
                      ],
                    ),
                  ),
                  Container(
                    margin: const EdgeInsets.only(bottom: 8),
                    padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
                    color: const Color(0xFFDCCFE8),
                    child: Row(
                      children: [
                        const Icon(Icons.shopping_cart_outlined, size: 18),
                        const SizedBox(width: 10),
                        Expanded(
                          child: Text(
                            'Collect order from ${store.isEmpty ? 'the store' : store}',
                            style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 13),
                          ),
                        ),
                      ],
                    ),
                  ),
                  _section(
                    icon: Icons.shopping_bag_outlined,
                    title: 'Order details',
                    subtitle: '${items.length} item${items.length == 1 ? '' : 's'}',
                    open: true,
                    children: [
                      for (final it in items)
                        Padding(
                          padding: const EdgeInsets.only(top: 6),
                          child: Text(
                            '${it is Map ? (it['quantity'] ?? it['qty'] ?? 1) : 1}x  '
                            '${it is Map ? (it['product_name'] ?? it['name'] ?? '') : ''}',
                            style: const TextStyle(fontSize: 13),
                          ),
                        ),
                      if (pay.isNotEmpty)
                        Padding(
                          padding: const EdgeInsets.only(top: 12),
                          child: Text(
                            pay == 'COD'
                                ? 'COD: \u20B9${_money(total)}'
                                : 'Paid online',
                            style: const TextStyle(
                                fontSize: 12, fontWeight: FontWeight.w600, color: Colors.black54),
                          ),
                        ),
                    ],
                  ),
                  _section(
                    icon: Icons.person_outline,
                    title: 'Customer details',
                    children: [
                      if (custName.isNotEmpty)
                        Text(custName, style: const TextStyle(fontWeight: FontWeight.w600)),
                      if (custPhone.isNotEmpty)
                        Padding(
                          padding: const EdgeInsets.only(top: 4),
                          child: Text(custPhone, style: const TextStyle(fontSize: 13)),
                        ),
                      if (custAddr.isNotEmpty)
                        Padding(
                          padding: const EdgeInsets.only(top: 4),
                          child: Text(custAddr,
                              style: const TextStyle(fontSize: 12, color: Colors.black54)),
                        ),
                    ],
                  ),
                  _section(
                    icon: Icons.storefront_outlined,
                    title: 'Store details',
                    open: true,
                    children: [
                      Text(store.isEmpty ? 'Store' : store,
                          style: const TextStyle(fontWeight: FontWeight.w600)),
                      if (addr.isNotEmpty)
                        Padding(
                          padding: const EdgeInsets.only(top: 4),
                          child: Text(addr,
                              style: const TextStyle(fontSize: 12, color: Colors.black54)),
                        ),
                    ],
                  ),
                ],
              ),
            ),
            if (_error != null)
              Padding(
                padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
                child: Text(_error!, style: const TextStyle(color: Colors.red)),
              ),
            Padding(
              padding: const EdgeInsets.all(16),
              child: widget.ready
          ? _SwipeToConfirm(label: 'Order picked', busy: _busy, onConfirm: _picked)
          : SizedBox(
              width: double.infinity,
              height: 60,
              child: ElevatedButton(
                onPressed: _storeReady ? _iAmReady : null,
                style: ElevatedButton.styleFrom(
                  backgroundColor: _green,
                  foregroundColor: Colors.black,
                  disabledBackgroundColor: const Color(0xFFD9D9D9),
                  shape: const StadiumBorder(),
                ),
                child: Text(
                  _storeReady ? 'I am ready' : 'Waiting for store to mark ready',
                  style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 15),
                ),
              ),
            ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Green pill with a black knob the partner drags to the right to confirm.
class _SwipeToConfirm extends StatefulWidget {
  final String label;
  final bool busy;
  final VoidCallback onConfirm;
  const _SwipeToConfirm({required this.label, required this.busy, required this.onConfirm});

  @override
  State<_SwipeToConfirm> createState() => _SwipeToConfirmState();
}

class _SwipeToConfirmState extends State<_SwipeToConfirm> {
  static const double _knob = 52;
  double _dx = 0;

  @override
  void didUpdateWidget(_SwipeToConfirm old) {
    super.didUpdateWidget(old);
    if (old.busy && !widget.busy) setState(() => _dx = 0);
  }

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(builder: (context, c) {
      final maxDx = c.maxWidth - _knob - 8;
      return Container(
        height: _knob + 8,
        decoration: BoxDecoration(
          color: _green,
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
                onHorizontalDragUpdate: widget.busy
                    ? null
                    : (d) => setState(() => _dx = (_dx + d.delta.dx).clamp(0.0, maxDx).toDouble()),
                onHorizontalDragEnd: widget.busy
                    ? null
                    : (_) {
                        if (_dx >= maxDx * 0.85) {
                          setState(() => _dx = maxDx);
                          widget.onConfirm();
                        } else {
                          setState(() => _dx = 0);
                        }
                      },
                child: Container(
                  width: _knob,
                  height: _knob,
                  decoration: const BoxDecoration(color: Colors.black, shape: BoxShape.circle),
                  child: widget.busy
                      ? const Padding(
                          padding: EdgeInsets.all(15),
                          child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                        )
                      : const Icon(Icons.arrow_forward, color: Colors.white),
                ),
              ),
            ),
          ],
        ),
      );
    });
  }
}