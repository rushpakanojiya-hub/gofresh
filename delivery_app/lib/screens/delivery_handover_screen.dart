import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import '../services/api_service.dart';
import 'delivery_complete_screen.dart';

const Color _green = Color(0xFF1ED760);
const Color _pageBg = Color(0xFFF7F1FB);

String _pick(Map<String, dynamic> o, List<String> keys) {
  for (final k in keys) {
    final v = o[k];
    if (v != null && v.toString().trim().isNotEmpty) return v.toString();
  }
  return '';
}

String _money(dynamic v) {
  final n = v is num ? v : num.tryParse('${v ?? ''}');
  if (n == null) return '-';
  return n.toStringAsFixed(2);
}

/// Shown after "Reached drop": payment status, items and customer details,
/// then a swipe to move on to the delivery confirmation screen.
class DeliveryHandoverScreen extends StatefulWidget {
  final Map<String, dynamic> order;
  const DeliveryHandoverScreen({super.key, required this.order});

  @override
  State<DeliveryHandoverScreen> createState() => _DeliveryHandoverScreenState();
}

class _DeliveryHandoverScreenState extends State<DeliveryHandoverScreen> {
  bool _busy = false;
  String? _error;
  String? _step;
  bool _cashCollected = false;

  Future<void> _next() async {
    if (_busy) return;
    if (_pick(widget.order, ['payment_method']).toLowerCase() == 'cod' && !_cashCollected) {
      setState(() => _error = 'Please collect the cash and tick the box first');
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
      _step = 'started';
    });
    try {
      final id = ((widget.order['order_id'] ?? widget.order['id']) as num).toInt();
      const chain = [
        'accepted',
        'going_to_store',
        'arrived_at_store',
        'picked_up',
        'out_for_delivery',
        'arrived_at_customer',
        'delivered',
      ];
      final list = await ApiService.getMyDeliveries();
      dynamic cur;
      for (final o in list) {
        if (o is Map && '${o['order_id'] ?? o['id']}' == '$id') {
          cur = o;
          break;
        }
      }
      var status =
          (cur is Map ? cur['delivery_status'] : widget.order['delivery_status'])?.toString();
      if (status == null || status.isEmpty || status == 'assigned') {
        await ApiService.acceptAssignment(id);
        status = 'accepted';
      }
      if (status == 'arrived') status = 'arrived_at_customer';
      final from = chain.indexOf(status);
      if (from < 0) throw Exception('Unexpected delivery status: $status');
      for (var i = from + 1; i < chain.length; i++) {
        if (mounted) setState(() => _step = '${chain[i]}');
        debugPrint('handover step: ${chain[i]}');
        await ApiService.updateDeliveryStatus(id, chain[i]);
      }
      if (mounted) setState(() => _step = 'confirmDelivery');
      debugPrint('handover step: confirmDelivery');
      final shot = await ImagePicker().pickImage(
        source: ImageSource.camera,
        imageQuality: 70,
        maxWidth: 1280,
      );
      if (shot == null) throw Exception('Delivery proof photo is required');
      if (mounted) setState(() => _step = 'uploading photo');
      await ApiService.uploadDeliveryProof(id, shot.path);
      if (mounted) setState(() => _step = 'confirmDelivery');
      final confirm = await ApiService.confirmDelivery(id);
      if (!mounted) return;
      final updated = Map<String, dynamic>.from(widget.order);
      final o = confirm['order'];
      if (o is Map) updated.addAll(Map<String, dynamic>.from(o));
      Navigator.of(context).pushReplacement(
        MaterialPageRoute(builder: (_) => DeliveryCompleteScreen(order: updated)),
      );
    } catch (e) {
      debugPrint('handover failed: $e');
      if (mounted) {
        setState(() {
          _busy = false;
          _error = e.toString().replaceFirst('Exception: ', '');
        });
      }
    }
  }

  Widget _section(
    BuildContext context, {
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
    final id = _pick(o, ['order_id', 'id']);
    final items = (o['items'] is List) ? o['items'] as List : const [];
    final isCod = _pick(o, ['payment_method']).toLowerCase() == 'cod';
    final name = _pick(o, ['customer_name']);
    final phone = _pick(o, ['customer_phone']);
    final addr = _pick(o, ['delivery_address']);

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
                        const Text('ORDER ID', style: TextStyle(fontSize: 11, color: Colors.black54)),
                        const SizedBox(height: 6),
                        Text('#$id',
                            style: const TextStyle(fontSize: 28, fontWeight: FontWeight.bold)),
                      ],
                    ),
                  ),
                  Container(
                    margin: const EdgeInsets.only(bottom: 8),
                    padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
                    color: Colors.white,
                    child: Row(
                      children: [
                        Icon(
                          isCod ? Icons.payments_outlined : Icons.check_circle,
                          color: isCod ? Colors.orange[800] : Colors.black87,
                        ),
                        const SizedBox(width: 14),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(isCod ? 'Collect cash' : 'Paid online',
                                  style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 14)),
                              const SizedBox(height: 2),
                              Text(
                                isCod
                                    ? '\u20B9${_money(o['total_amount'])} from customer'
                                    : 'Order: #$id',
                                style: const TextStyle(fontSize: 12, color: Colors.black54),
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),
                  if (isCod)
                    Container(
                      margin: const EdgeInsets.only(bottom: 8),
                      color: Colors.white,
                      child: CheckboxListTile(
                        value: _cashCollected,
                        onChanged: _busy ? null : (v) => setState(() {
                          _cashCollected = v ?? false;
                          _error = null;
                        }),
                        controlAffinity: ListTileControlAffinity.trailing,
                        activeColor: Colors.green,
                        title: Text(
                          'Collected \u20B9${_money(o['total_amount'])} cash',
                          style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 14),
                        ),
                        subtitle: const Text(
                          'Tick after collecting cash from the customer',
                          style: TextStyle(fontSize: 12, color: Colors.black54),
                        ),
                      ),
                    ),
                  _section(
                    context,
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
                    ],
                  ),
                  _section(
                    context,
                    icon: Icons.person_outline,
                    title: 'Customer details',
                    children: [
                      if (name.isNotEmpty)
                        Text(name, style: const TextStyle(fontWeight: FontWeight.w600)),
                      if (phone.isNotEmpty)
                        Padding(
                          padding: const EdgeInsets.only(top: 4),
                          child: Text(phone, style: const TextStyle(fontSize: 13)),
                        ),
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
            Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (_busy && _step != null)
                    Padding(
                      padding: const EdgeInsets.only(bottom: 8),
                      child: Text('Step: $_step'),
                    ),
                  if (_error != null)
                    Padding(
                      padding: const EdgeInsets.only(bottom: 8),
                      child: Text(_error!, style: const TextStyle(color: Colors.red)),
                    ),
                  Opacity(
                    opacity: (isCod && !_cashCollected) ? 0.4 : 1,
                    child: IgnorePointer(
                      ignoring: isCod && !_cashCollected,
                      child: _SwipeButton(label: 'Order delivered', busy: _busy, onConfirm: _next),
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

/// Green pill with a black knob the partner drags to the right to confirm.
class _SwipeButton extends StatefulWidget {
  final String label;
  final bool busy;
  final VoidCallback onConfirm;
  const _SwipeButton({required this.label, required this.busy, required this.onConfirm});

  @override
  State<_SwipeButton> createState() => _SwipeButtonState();
}

class _SwipeButtonState extends State<_SwipeButton> {
  static const double _knob = 52;
  double _dx = 0;

  @override
  void didUpdateWidget(_SwipeButton old) {
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
                onHorizontalDragUpdate: (d) =>
                    setState(() => _dx = (_dx + d.delta.dx).clamp(0.0, maxDx).toDouble()),
                onHorizontalDragEnd: (_) {
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