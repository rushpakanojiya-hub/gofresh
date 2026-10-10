import 'package:flutter/material.dart';
import 'dart:async';
import 'package:flutter/services.dart';
import '../services/api_service.dart';
import 'delivery_map_screen.dart';
import 'qr_scan_screen.dart';

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
  String _staffName = '';
  String _staffPhone = '';
  bool _popupShown = false;
  bool _verified = false;

  @override
  void initState() {
    super.initState();
    final s = widget.order['status']?.toString();
    _storeReady = widget.ready || s == 'handed_over' || s == 'shipped';
    _staffName = _pick(widget.order, ['store_staff_name']);
    _staffPhone = _pick(widget.order, ['store_staff_phone']);
    WidgetsBinding.instance.addPostFrameCallback((_) => _maybeShowStaffPopup());
    if (!_storeReady) {
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
          final ok = s == 'handed_over' || s == 'shipped';
          final nm = (o['store_staff_name'] ?? '').toString();
          final ph = (o['store_staff_phone'] ?? '').toString();
          if (nm.isNotEmpty && (nm != _staffName || ph != _staffPhone) && mounted) {
            setState(() {
              _staffName = nm;
              _staffPhone = ph;
            });
            _maybeShowStaffPopup();
          }
          if (ok != _storeReady && mounted) setState(() => _storeReady = ok);
          if (ok) _poll?.cancel();
          break;
        }
      }
    } catch (_) {}
  }

  void _maybeShowStaffPopup() {
    if (_popupShown || !mounted || _staffName.isEmpty) return;
    _popupShown = true;
    _showStaffPopup();
  }

  Future<void> _showStaffPopup() {
    final initial = _staffName.trim().isEmpty ? '?' : _staffName.trim()[0].toUpperCase();
    bool copied = false;
    return showDialog<void>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setLocal) => Dialog(
          backgroundColor: Colors.white,
          insetPadding: const EdgeInsets.symmetric(horizontal: 28),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(24)),
          child: Padding(
            padding: const EdgeInsets.fromLTRB(24, 28, 24, 20),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Container(
                  width: 64,
                  height: 64,
                  alignment: Alignment.center,
                  decoration: const BoxDecoration(color: Color(0xFFE8F9EE), shape: BoxShape.circle),
                  child: Text(
                    initial,
                    style: const TextStyle(fontSize: 28, fontWeight: FontWeight.bold, color: Color(0xFF0C831F)),
                  ),
                ),
                const SizedBox(height: 16),
                const Text(
                  'COLLECT ORDER FROM',
                  style: TextStyle(fontSize: 11, letterSpacing: 1.2, fontWeight: FontWeight.w600, color: Colors.black45),
                ),
                const SizedBox(height: 6),
                Text(
                  _staffName,
                  textAlign: TextAlign.center,
                  style: const TextStyle(fontSize: 22, fontWeight: FontWeight.bold),
                ),
                const SizedBox(height: 4),
                Text('Order #$_id', style: const TextStyle(fontSize: 13, color: Colors.black54)),
                if (_staffPhone.isNotEmpty) ...[
                  const SizedBox(height: 18),
                  Container(
                    width: double.infinity,
                    padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
                    decoration: BoxDecoration(
                      color: const Color(0xFFF5F5F7),
                      borderRadius: BorderRadius.circular(14),
                    ),
                    child: Row(
                      children: [
                        const Icon(Icons.phone_outlined, size: 20, color: Colors.black87),
                        const SizedBox(width: 10),
                        Expanded(
                          child: Text(
                            _staffPhone,
                            style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600, letterSpacing: 0.5),
                          ),
                        ),
                        TextButton.icon(
                          onPressed: () async {
                            await Clipboard.setData(ClipboardData(text: _staffPhone));
                            setLocal(() => copied = true);
                          },
                          icon: Icon(copied ? Icons.check : Icons.copy_rounded, size: 16),
                          label: Text(copied ? 'Copied' : 'Copy'),
                          style: TextButton.styleFrom(
                            foregroundColor: const Color(0xFF0C831F),
                            visualDensity: VisualDensity.compact,
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
                const SizedBox(height: 22),
                SizedBox(
                  width: double.infinity,
                  height: 50,
                  child: ElevatedButton(
                    onPressed: () => Navigator.pop(ctx),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: _green,
                      foregroundColor: Colors.black,
                      elevation: 0,
                      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
                    ),
                    child: const Text('Got it', style: TextStyle(fontSize: 15, fontWeight: FontWeight.bold)),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
  int get _id => ((widget.order['order_id'] ?? widget.order['id']) as num).toInt();

  Future<void> _scanPicker() async {
    final token = await Navigator.of(context).push<String>(
      MaterialPageRoute(builder: (_) => const QrScanScreen()),
    );
    if (token == null || !mounted) return;
    try {
      final r = await ApiService.verifyPickerQr(_id, token);
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        barrierDismissible: false,
        builder: (ctx) => AlertDialog(
          title: const Text('Pick order now!'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('Order ID: ${r['order_id']}'),
              const SizedBox(height: 6),
              Text('Customer: ${r['customer_name'] ?? ''}'),
            ],
          ),
          actions: [
            ElevatedButton(
              onPressed: () => Navigator.pop(ctx),
              style: ElevatedButton.styleFrom(backgroundColor: Colors.black, foregroundColor: Colors.white),
              child: const Text("Okay, I'm ready"),
            ),
          ],
        ),
      );
      if (!mounted) return;
      setState(() {
        _verified = true;
        _error = null;
      });
    } catch (e) {
      if (mounted) setState(() => _error = e.toString().replaceFirst('Exception: ', ''));
    }
  }

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
                  Container(
                    margin: const EdgeInsets.only(bottom: 8),
                    padding: const EdgeInsets.all(12),
                    color: Colors.white,
                    child: Row(
                      children: [
                        Container(
                          width: 48,
                          height: 48,
                          alignment: Alignment.center,
                          decoration: BoxDecoration(color: Colors.red, borderRadius: BorderRadius.circular(6)),
                          child: Text(
                            _staffName.isEmpty ? 'P' : _staffName[0].toUpperCase(),
                            style: const TextStyle(color: Colors.white, fontSize: 22, fontWeight: FontWeight.bold),
                          ),
                        ),
                        const SizedBox(width: 12),
                        Expanded(
                          child: Text(_staffName.isEmpty ? 'Store picker' : _staffName,
                              style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 14)),
                        ),
                        _verified
                            ? const Icon(Icons.check_circle, color: _green)
                            : ElevatedButton(
                                onPressed: _scanPicker,
                                style: ElevatedButton.styleFrom(
                                  backgroundColor: const Color(0xFF2B2F36),
                                  foregroundColor: Colors.white,
                                ),
                                child: const Text('Scan Picker QR'),
                              ),
                      ],
                    ),
                  ),                  _section(
                    icon: Icons.shopping_bag_outlined,
                    title: 'Order details',
                    subtitle: '${items.length} item${items.length == 1 ? '' : 's'}',
                    open: true,
                    children: [
                      for (final it in items)
                        Padding(
                          padding: const EdgeInsets.only(top: 6),
                          child: Row(children: [
                            Builder(builder: (_) {
                              String img = (it is Map ? (it['image_url'] ?? '') : '').toString();
                              if (img.isNotEmpty && !img.startsWith('http')) {
                                img = 'https://gofresh-evl7.onrender.com${img.startsWith('/') ? '' : '/'}$img';
                              }
                              return ClipRRect(
                                borderRadius: BorderRadius.circular(6),
                                child: img.isEmpty
                                    ? Container(width: 44, height: 44, color: const Color(0xFFF2F4F7), child: const Icon(Icons.image_not_supported_outlined, size: 20, color: Colors.grey))
                                    : Image.network(img, width: 44, height: 44, fit: BoxFit.cover,
                                        errorBuilder: (c, e, s) => Container(width: 44, height: 44, color: const Color(0xFFF2F4F7), child: const Icon(Icons.broken_image_outlined, size: 20, color: Colors.grey))),
                              );
                            }),
                            const SizedBox(width: 10),
                            Expanded(
                              child: Text(
                                '${it is Map ? (it['quantity'] ?? it['qty'] ?? 1) : 1}x  '
                                '${it is Map ? (it['product_name'] ?? it['name'] ?? '') : ''}',
                                style: const TextStyle(fontSize: 13),
                              ),
                            ),
                          ]),
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
                  if (_staffName.isNotEmpty)
                    _section(
                      icon: Icons.badge_outlined,
                      title: 'Store contact',
                      open: true,
                      children: [
                        Text(_staffName, style: const TextStyle(fontWeight: FontWeight.w600)),
                        if (_staffPhone.isNotEmpty)
                          Padding(
                            padding: const EdgeInsets.only(top: 4),
                            child: Text(_staffPhone, style: const TextStyle(fontSize: 13)),
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
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  _SwipeToConfirm(
                    label: 'Order picked',
                    busy: _busy,
                    enabled: _storeReady && _verified,
                    onConfirm: _picked,
                  ),
                  if (!_storeReady)
                    const Padding(
                      padding: EdgeInsets.only(top: 8),
                      child: Text(
                        'Waiting for store to hand over the order',
                        style: TextStyle(fontSize: 12, color: Colors.black54),
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
class _SwipeToConfirm extends StatefulWidget {
  final String label;
  final bool busy;
  final bool enabled;
  final VoidCallback onConfirm;
  const _SwipeToConfirm({
    required this.label,
    required this.busy,
    required this.onConfirm,
    this.enabled = true,
  });

  @override
  State<_SwipeToConfirm> createState() => _SwipeToConfirmState();
}

class _SwipeToConfirmState extends State<_SwipeToConfirm> {
  static const double _knob = 52;
  double _dx = 0;

  @override
  void didUpdateWidget(_SwipeToConfirm old) {
    super.didUpdateWidget(old);
    if ((old.busy && !widget.busy) || (old.enabled && !widget.enabled)) {
      setState(() => _dx = 0);
    }
  }

  @override
  Widget build(BuildContext context) {
    final locked = widget.busy || !widget.enabled;
    return LayoutBuilder(builder: (context, c) {
      final maxDx = c.maxWidth - _knob - 8;
      return Container(
        height: _knob + 8,
        decoration: BoxDecoration(
          color: widget.enabled ? _green : const Color(0xFFD9D9D9),
          borderRadius: BorderRadius.circular(40),
        ),
        child: Stack(
          alignment: Alignment.centerLeft,
          children: [
            Center(
              child: Text(widget.label,
                  style: TextStyle(
                    fontWeight: FontWeight.bold,
                    fontSize: 15,
                    color: widget.enabled ? Colors.black : Colors.black45,
                  )),
            ),
            Positioned(
              left: 4 + _dx,
              child: GestureDetector(
                onHorizontalDragUpdate: locked
                    ? null
                    : (d) => setState(() => _dx = (_dx + d.delta.dx).clamp(0.0, maxDx).toDouble()),
                onHorizontalDragEnd: locked
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
                  decoration: BoxDecoration(
                    color: widget.enabled ? Colors.black : const Color(0xFF9E9E9E),
                    shape: BoxShape.circle,
                  ),
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