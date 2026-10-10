// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'dart:async';
import 'package:flutter/material.dart';
import '../services/api_service.dart';
import 'package:qr_flutter/qr_flutter.dart';

class PickingScreen extends StatefulWidget {
  final int orderId;
  final int allottedSeconds;
  const PickingScreen({super.key, required this.orderId, required this.allottedSeconds});

  @override
  State<PickingScreen> createState() => _PickingScreenState();
}

class _PickingScreenState extends State<PickingScreen> {
  static const Color _green = Color(0xFF16A34A);
  static const Color _ink = Color(0xFF1B2A1F);

  bool _loading = true;
  bool _busy = false;
  bool _completed = false;
  String? _error;
  String? _msg;
  int _idx = 0;
  final Map<int, int> _sel = {};
  DateTime _startedAt = DateTime.now();
  List<Map<String, dynamic>> _items = [];
  Timer? _ticker;
  Timer? _retry;

  @override
  void initState() {
    super.initState();
    _ticker = Timer.periodic(const Duration(seconds: 1), (_) {
      if (mounted) setState(() {});
    });
    _boot();
  }

  @override
  void dispose() {
    _ticker?.cancel();
    _retry?.cancel();
    super.dispose();
  }

  String _status(Map<String, dynamic> it) => (it['status'] ?? 'pending').toString();
  int _need(Map<String, dynamic> it) => (it['quantity_needed'] as num?)?.toInt() ?? 1;
  int _qtyOf(Map<String, dynamic> it) {
    final need = _need(it);
    final v = _sel[(it['id'] as num).toInt()] ?? need;
    return v.clamp(1, need).toInt();
  }
  String _weightOf(Map<String, dynamic> it) => (_prod(it)['weight'] ?? '').toString().trim();
  bool _isPiece(Map<String, dynamic> it) => _weightOf(it).isEmpty;

  Map<String, dynamic> _prod(Map<String, dynamic> it) {
    final p = it['product'];
    return p is Map ? Map<String, dynamic>.from(p) : <String, dynamic>{};
  }

  bool get _allMarked => _items.isNotEmpty && _items.every((e) => _status(e) != 'pending');
  int get _remaining => widget.allottedSeconds - DateTime.now().difference(_startedAt).inSeconds;

  int _nextPending(int from) {
    if (_items.isEmpty) return 0;
    for (var k = 1; k <= _items.length; k++) {
      final i = (from + k) % _items.length;
      if (_status(_items[i]) == 'pending') return i;
    }
    return _idx < _items.length ? _idx : 0;
  }

  Future<void> _boot() async {
    try {
      final s = await ApiService.startPicking(widget.orderId);
      final err = s['error']?.toString();
      if (err != null) {
        if (err.toLowerCase().contains('already completed')) {
          _completed = true;
        } else {
          if (!mounted) return;
          setState(() {
            _error = err;
            _loading = false;
          });
          return;
        }
      }
      final t = await ApiService.getPickingTask(widget.orderId);
      if (!mounted) return;
      if (t['error'] != null) {
        setState(() {
          _error = t['error'].toString();
          _loading = false;
        });
        return;
      }
      final st = DateTime.tryParse((t['started_at'] ?? '').toString());
      if (st != null) _startedAt = st.toLocal();
      final raw = t['items'];
      final items = raw is List
          ? raw.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList()
          : <Map<String, dynamic>>[];
      setState(() {
        _items = items;
        _idx = _nextPending(-1);
        _loading = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _error = 'Network error. Please try again.';
        _loading = false;
      });
    }
  }

  String _loc(Map<String, dynamic> it) {
    final loc = it['location'];
    if (loc is! Map) return 'Location not set';
    final bin = loc['bin'];
    if (bin is! Map) return 'Location not set';
    final parts = <String>[];
    final rack = bin['rack'];
    if (rack is Map) {
      final zone = rack['zone'];
      if (zone is Map && (zone['name'] ?? '').toString().isNotEmpty) parts.add(zone['name'].toString());
      if ((rack['name'] ?? '').toString().isNotEmpty) parts.add(rack['name'].toString());
    }
    if ((bin['name'] ?? '').toString().isNotEmpty) parts.add(bin['name'].toString());
    return parts.isEmpty ? 'Location not set' : parts.join(' - ');
  }

  String _money(num v) => v == v.roundToDouble() ? v.toInt().toString() : v.toStringAsFixed(2);

  Widget _img(String? url, double size) {
    if (url == null || url.isEmpty) {
      return Icon(Icons.shopping_basket_outlined, size: size * 0.5, color: Colors.grey);
    }
    return Image.network(
      url,
      width: size,
      height: size,
      fit: BoxFit.contain,
      errorBuilder: (context, error, stackTrace) =>
          Icon(Icons.image_not_supported_outlined, size: size * 0.5, color: Colors.grey),
    );
  }

  Widget _timeBox(String v, String l, Color c) {
    return Container(
      width: 46,
      padding: const EdgeInsets.symmetric(vertical: 5),
      decoration: BoxDecoration(color: c, borderRadius: BorderRadius.circular(8)),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(v, style: const TextStyle(color: Colors.white, fontSize: 17, fontWeight: FontWeight.w800)),
          Text(l, style: const TextStyle(color: Colors.white, fontSize: 9, fontWeight: FontWeight.w700)),
        ],
      ),
    );
  }

  Widget _topBar() {
    final rem = _remaining;
    final over = rem < 0;
    final show = over ? 0 : rem;
    final c = over ? Colors.red : _green;
    return Padding(
      padding: const EdgeInsets.fromLTRB(4, 8, 16, 8),
      child: Row(
        children: [
          IconButton(onPressed: () => Navigator.pop(context, false), icon: const Icon(Icons.arrow_back)),
          Expanded(
            child: Text(
              'Order ID : ${widget.orderId}',
              style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w800),
            ),
          ),
          _timeBox((show ~/ 60).toString().padLeft(2, '0'), 'MIN', c),
          const SizedBox(width: 6),
          _timeBox((show % 60).toString().padLeft(2, '0'), 'SEC', c),
        ],
      ),
    );
  }

  Widget _thumbs() {
    return SizedBox(
      height: 70,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: 12),
        itemCount: _items.length,
        separatorBuilder: (context, i) => const SizedBox(width: 8),
        itemBuilder: (context, i) {
          final it = _items[i];
          final sel = i == _idx;
          final st = _status(it);
          return GestureDetector(
            onTap: () => setState(() => _idx = i),
            child: Stack(
              children: [
                Container(
                  width: 62,
                  height: 62,
                  padding: const EdgeInsets.all(4),
                  decoration: BoxDecoration(
                    color: Colors.white,
                    borderRadius: BorderRadius.circular(10),
                    border: Border.all(color: sel ? _ink : const Color(0xFFD0D5DD), width: sel ? 2 : 1),
                  ),
                  child: _img(_prod(it)['image_url']?.toString(), 50),
                ),
                Positioned(
                  left: 2,
                  bottom: 2,
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 1),
                    decoration: BoxDecoration(color: _ink, borderRadius: BorderRadius.circular(8)),
                    child: Text('${_need(it)}', style: const TextStyle(color: Colors.white, fontSize: 10)),
                  ),
                ),
                if (st != 'pending')
                  Positioned(
                    right: 2,
                    top: 2,
                    child: Icon(
                      st == 'picked' ? Icons.check_circle : Icons.error,
                      size: 16,
                      color: st == 'picked' ? _green : Colors.orange,
                    ),
                  ),
              ],
            ),
          );
        },
      ),
    );
  }

  Widget _qtyControl(Map<String, dynamic> it) {
    final need = _need(it);
    final pending = _status(it) == 'pending';
    final q = _qtyOf(it);
    final id = (it['id'] as num).toInt();
    const big = TextStyle(fontSize: 34, fontWeight: FontWeight.w800);
    if (!pending) {
      return Text('${it['quantity_picked'] ?? 0}', style: big);
    }
    if (need <= 1) {
      return Text('$need', style: big);
    }
    final canDec = q > 1;
    final canInc = q < need;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            InkWell(
              onTap: canDec ? () => setState(() => _sel[id] = q - 1) : null,
              child: Icon(Icons.remove_circle_outline, size: 32, color: canDec ? Colors.black : Colors.black26),
            ),
            Text('$q', style: const TextStyle(fontSize: 30, fontWeight: FontWeight.w800)),
            InkWell(
              onTap: canInc ? () => setState(() => _sel[id] = q + 1) : null,
              child: Icon(Icons.add_circle_outline, size: 32, color: canInc ? Colors.black : Colors.black26),
            ),
          ],
        ),
        Text('of $need ${_isPiece(it) ? 'pcs' : 'ordered'}', style: const TextStyle(fontSize: 11, color: Colors.black54)),
      ],
    );
  }

  Widget _itemView(Map<String, dynamic> it) {
    final p = _prod(it);
    final price = p['price'] is num ? p['price'] as num : 0;
    final st = _status(it);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Container(
          margin: const EdgeInsets.symmetric(horizontal: 16),
          padding: const EdgeInsets.all(14),
          decoration: BoxDecoration(color: const Color(0xFFFFF3D0), borderRadius: BorderRadius.circular(12)),
          child: Row(
            children: [
              const Icon(Icons.location_on_outlined),
              const SizedBox(width: 8),
              Expanded(
                child: Text(_loc(it), style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w800)),
              ),
            ],
          ),
        ),
        const SizedBox(height: 14),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                flex: 5,
                child: Container(
                  height: 190,
                  alignment: Alignment.center,
                  child: _img(p['image_url']?.toString(), 170),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                flex: 4,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Container(
                      padding: const EdgeInsets.all(12),
                      decoration: BoxDecoration(
                        color: const Color(0xFFE3F2FD),
                        borderRadius: BorderRadius.circular(12),
                      ),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const Text('Quantity', style: TextStyle(fontSize: 12, color: Colors.black54)),
                          _qtyControl(it),
                        ],
                      ),
                    ),
                    const SizedBox(height: 10),
                    Text(_isPiece(it) ? 'Unit' : 'Weight', style: const TextStyle(fontSize: 12, color: Colors.black54)),
                    Text(
                      _isPiece(it) ? 'Piece' : _weightOf(it),
                      style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w800),
                    ),
                    const SizedBox(height: 10),
                    const Text('Price', style: TextStyle(fontSize: 12, color: Colors.black54)),
                    Text('\u20B9${_money(price)}', style: const TextStyle(fontSize: 20, fontWeight: FontWeight.w800)),
                  ],
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 12),
        Container(
          margin: const EdgeInsets.symmetric(horizontal: 16),
          padding: const EdgeInsets.all(14),
          decoration: BoxDecoration(color: const Color(0xFFFFF8E1), borderRadius: BorderRadius.circular(12)),
          child: Text(
            (p['name'] ?? 'Product').toString(),
            style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w700),
          ),
        ),
        if (st != 'pending')
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 10, 16, 0),
            child: Text(
              st == 'picked'
                  ? 'Picked'
                  : (st == 'short' ? 'Short: ${it['quantity_picked'] ?? 0} of ${_need(it)} picked' : 'Marked unavailable'),
              style: TextStyle(
                fontWeight: FontWeight.w700,
                color: st == 'picked' ? _green : Colors.orange,
              ),
            ),
          ),
      ],
    );
  }

  Future<int?> _askQty(int need) {
    var q = need - 1;
    return showDialog<int>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setS) => AlertDialog(
          title: const Text('How many did you find?'),
          content: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              IconButton(
                onPressed: q > 1 ? () => setS(() => q--) : null,
                icon: const Icon(Icons.remove_circle_outline),
              ),
              Text('$q', style: const TextStyle(fontSize: 26, fontWeight: FontWeight.w800)),
              IconButton(
                onPressed: q < need - 1 ? () => setS(() => q++) : null,
                icon: const Icon(Icons.add_circle_outline),
              ),
            ],
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Cancel')),
            TextButton(onPressed: () => Navigator.pop(ctx, q), child: const Text('Confirm')),
          ],
        ),
      ),
    );
  }

  Future<void> _cantFind() async {
    final need = _need(_items[_idx]);
    final choice = await showModalBottomSheet<String>(
      context: context,
      backgroundColor: Colors.white,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (ctx) {
        Widget opt(IconData icon, String title, String sub, String value) => ListTile(
              leading: Icon(icon, color: Colors.black87),
              title: Text(title, style: const TextStyle(fontWeight: FontWeight.w700)),
              subtitle: Text(sub, style: const TextStyle(fontSize: 12)),
              onTap: () => Navigator.pop(ctx, value),
            );
        return SafeArea(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Padding(
                padding: EdgeInsets.fromLTRB(20, 18, 20, 6),
                child: Align(
                  alignment: Alignment.centerLeft,
                  child: Text("Why can't you pick this item?",
                      style: TextStyle(fontSize: 16, fontWeight: FontWeight.w800)),
                ),
              ),
              opt(Icons.search_off, 'Not on shelf', 'Item is missing from its bin', 'notfound'),
              opt(Icons.broken_image_outlined, 'Damaged', 'Packaging or product is damaged', 'damaged'),
              opt(Icons.event_busy_outlined, 'Expired', 'Past its expiry date', 'expired'),
              if (need > 1)
                opt(Icons.remove_circle_outline, 'Short quantity', 'Fewer units than needed', 'short'),
              const SizedBox(height: 8),
            ],
          ),
        );
      },
    );
    if (choice == null || !mounted) return;
    if (choice == 'short') {
      final q = await _askQty(need);
      if (q == null || !mounted) return;
      await _mark('short', qty: q, reason: 'Short quantity');
    } else {
      const reasons = {'notfound': 'Item not found', 'damaged': 'Damaged', 'expired': 'Expired'};
      await _mark('unavailable', reason: reasons[choice] ?? 'Item not found');
    }
  }
  Future<void> _mark(String status, {int? qty, String? reason}) async {
    if (_items.isEmpty || _busy) return;
    final it = _items[_idx];
    final id = (it['id'] as num).toInt();
    setState(() {
      _busy = true;
      _msg = null;
    });
    try {
      final r = await ApiService.markPickItem(id, status, quantityPicked: qty, reason: reason);
      if (!mounted) return;
      if (r['error'] != null) {
        setState(() {
          _busy = false;
          _msg = r['error'].toString();
        });
        return;
      }
      final need = _need(it);
      setState(() {
        it['status'] = status;
        it['quantity_picked'] = status == 'picked' ? need : (status == 'short' ? (qty ?? 0) : 0);
        _busy = false;
        _idx = _nextPending(_idx);
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _msg = 'Network error. Please try again.';
      });
    }
  }

  Future<void> _pickPressed() async {
    if (_items.isEmpty) return;
    final it = _items[_idx];
    final need = _need(it);
    final q = _qtyOf(it);
    if (q >= need) {
      await _mark('picked');
    } else {
      await _mark('short', qty: q, reason: 'Short quantity');
    }
  }

  Future<void> _showHandoverQr() async {
    try {
      final r = await ApiService.getPickerHandoverQR(widget.orderId);
      final token = (r['token'] ?? '').toString();
      if (token.isEmpty || !mounted) return;
      await showDialog<void>(
        context: context,
        barrierDismissible: false,
        builder: (ctx) => AlertDialog(
          title: Text('Order #${widget.orderId}'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text('Show this QR to the delivery partner'),
              SizedBox(height: 12),
              SizedBox(width: 220, height: 220, child: QrImageView(data: token)),
            ],
          ),
          actions: [TextButton(onPressed: () => Navigator.pop(ctx), child: Text('Done'))],
        ),
      );
    } catch (_) {}
  }
  Future<void> _handover() async {
    if (_busy) return;
    _retry?.cancel();
    setState(() {
      _busy = true;
      _msg = null;
    });
    try {
      if (!_completed) {
        final c = await ApiService.completePicking(widget.orderId);
        final err = c['error']?.toString();
        if (err != null && !err.toLowerCase().contains('already completed')) {
          if (!mounted) return;
          setState(() {
            _busy = false;
            _msg = err;
          });
          return;
        }
        _completed = true;
      }
      final h = await ApiService.pickerHandover(widget.orderId);
      if (!mounted) return;
      final st = (h['status'] ?? '').toString();
      if (h['success'] == true && (st == 'handed_over' || st == 'shipped' || st == 'delivered')) {
        await _showHandoverQr();
        if (!mounted) return;
        Navigator.pop(context, true);
        return;
      }
      final waiting = h['waiting_for_partner'] == true;
      setState(() {
        _busy = false;
        _msg = waiting
            ? 'Waiting for a delivery partner. Retrying automatically...'
            : (h['error'] ?? 'Handover failed').toString();
      });
      if (waiting) {
        _retry = Timer(const Duration(seconds: 5), () {
          if (mounted) _handover();
        });
      }
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _msg = 'Network error. Please try again.';
      });
    }
  }

  Widget _bottom() {
    final msg = _msg;
    final Widget actions;
    if (_allMarked) {
      actions = SizedBox(
        width: double.infinity,
        child: ElevatedButton(
          onPressed: _busy ? null : _handover,
          style: ElevatedButton.styleFrom(
            backgroundColor: _green,
            foregroundColor: Colors.white,
            padding: const EdgeInsets.symmetric(vertical: 16),
            shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
          ),
          child: _busy
              ? const SizedBox(
                  height: 20,
                  width: 20,
                  child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                )
              : const Text('Handover', style: TextStyle(fontSize: 17, fontWeight: FontWeight.w800)),
        ),
      );
    } else {
      actions = Row(
        children: [
          Expanded(
            child: OutlinedButton(
              onPressed: _busy ? null : _cantFind,
              style: OutlinedButton.styleFrom(
                padding: const EdgeInsets.symmetric(vertical: 16),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
              ),
              child: const Text("Can't find", style: TextStyle(fontWeight: FontWeight.w700)),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            flex: 2,
            child: ElevatedButton(
              onPressed: _busy ? null : _pickPressed,
              style: ElevatedButton.styleFrom(
                backgroundColor: Colors.black,
                foregroundColor: Colors.white,
                padding: const EdgeInsets.symmetric(vertical: 16),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
              ),
              child: const Text('Picked', style: TextStyle(fontSize: 16, fontWeight: FontWeight.w800)),
            ),
          ),
        ],
      );
    }
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (msg != null)
          Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: Text(
              msg,
              textAlign: TextAlign.center,
              style: TextStyle(color: msg.startsWith('Waiting') ? Colors.orange.shade800 : Colors.red),
            ),
          ),
        actions,
      ],
    );
  }

  Widget _body() {
    if (_items.isEmpty) {
      return Column(
        children: [
          _topBar(),
          const Expanded(child: Center(child: Text('No items in this order'))),
        ],
      );
    }
    return Column(
      children: [
        _topBar(),
        _thumbs(),
        const SizedBox(height: 8),
        Expanded(child: SingleChildScrollView(child: _itemView(_items[_idx]))),
        Padding(padding: const EdgeInsets.fromLTRB(16, 8, 16, 12), child: _bottom()),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    Widget content;
    if (_loading) {
      content = const Center(child: CircularProgressIndicator());
    } else if (_error != null) {
      content = Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(_error!, textAlign: TextAlign.center, style: const TextStyle(color: Colors.red)),
              const SizedBox(height: 12),
              TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Back')),
            ],
          ),
        ),
      );
    } else {
      content = _body();
    }
    return Scaffold(backgroundColor: Colors.white, body: SafeArea(child: content));
  }
}