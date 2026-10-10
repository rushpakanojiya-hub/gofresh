// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../utils/fmt.dart';
import 'package:qr_flutter/qr_flutter.dart';

class SummaryScreen extends StatefulWidget {
  const SummaryScreen({super.key});

  @override
  State<SummaryScreen> createState() => _SummaryScreenState();
}

class _SummaryScreenState extends State<SummaryScreen> {
  static const Color _green = Color(0xFF16A34A);
  static const Color _ink = Color(0xFF1B2A1F);

  bool _loading = true;
  String? _error;
  Map<String, dynamic> _s = {};
  Map<String, dynamic>? _booking;
  int _active = 0;
  List<Map<String, dynamic>> _history = [];

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final r = await ApiService.getPickerSummary();
      final b = await ApiService.getMyBookings();
      final p = await ApiService.getPresence();
      List<Map<String, dynamic>> hist = [];
      try {
        final h = await ApiService.getPickerHistory();
        final ho = h['orders'];
        if (ho is List) {
          hist = ho.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
        }
      } catch (_) {}
      if (!mounted) return;
      if (r['error'] != null) {
        setState(() {
          _error = r['error'].toString();
          _loading = false;
        });
        return;
      }
      final list = b['bookings'];
      Map<String, dynamic>? today;
      if (list is List) {
        final t = fmtDate(DateTime.now());
        for (final e in list) {
          if (e is Map && e['date'] == t) {
            today = Map<String, dynamic>.from(e);
            break;
          }
        }
      }
      setState(() {
        _s = r;
        _history = hist;
        _booking = today;
        _active = (p['active_seconds'] as num?)?.toInt() ?? 0;
        _error = null;
        _loading = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _error = 'Network error. Pull down to retry.';
        _loading = false;
      });
    }
  }

  int _int(String k) => (_s[k] as num?)?.toInt() ?? 0;
  String _money(num v) => v == v.roundToDouble() ? v.toInt().toString() : v.toStringAsFixed(2);

  Widget _stat(String value, String label, Color c) {
    return Expanded(
      child: Column(
        children: [
          Text(value, style: TextStyle(fontSize: 22, fontWeight: FontWeight.w800, color: c)),
          const SizedBox(height: 2),
          Text(label, style: TextStyle(fontSize: 12, color: Colors.grey.shade600)),
        ],
      ),
    );
  }

  int _nextLeft(List<Map<String, dynamic>> tiers, int done) {
    for (final t in tiers) {
      final n = (t['orders'] as num).toInt();
      if (done < n) return n - done;
    }
    return 0;
  }

  Widget _bonusMeter() {
    final done = _int('on_time_orders');
    final raw = _s['tiers'];
    final tiers = raw is List
        ? raw.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList()
        : <Map<String, dynamic>>[];
    if (tiers.isEmpty) return SizedBox.shrink();
    final maxOrders = (tiers.last['orders'] as num).toInt();
    final progress = maxOrders == 0 ? 0.0 : (done / maxOrders).clamp(0.0, 1.0).toDouble();
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        border: Border.all(color: const Color(0xFFD0D5DD)),
        borderRadius: BorderRadius.circular(14),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text('Bonus Meter (orders picked on time)',
              style: TextStyle(fontSize: 15, fontWeight: FontWeight.w800)),
          const SizedBox(height: 14),
          ClipRRect(
            borderRadius: BorderRadius.circular(6),
            child: LinearProgressIndicator(
              value: progress,
              minHeight: 10,
              backgroundColor: const Color(0xFFE4E7EC),
              color: _green,
            ),
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              for (final t in tiers)
                Expanded(
                  child: Column(
                    children: [
                      Icon(
                        t['unlocked'] == true ? Icons.check_circle : Icons.lock_outline,
                        color: t['unlocked'] == true ? _green : Colors.grey,
                      ),
                      const SizedBox(height: 4),
                      Text('\u20B9${_money((t['amount'] as num))}',
                          style: const TextStyle(fontWeight: FontWeight.w800)),
                      Text('${t['orders']} fast orders',
                          style: TextStyle(fontSize: 11, color: Colors.grey.shade600)),
                    ],
                  ),
                ),
            ],
          ),
          const SizedBox(height: 10),
          Text(
            done >= maxOrders
                ? 'Max bonus unlocked'
                : 'Pick ${_nextLeft(tiers, done)} more fast orders for the next bonus',
            style: TextStyle(fontSize: 12, color: Colors.grey.shade700),
          ),
        ],
      ),
    );
  }

  String _when(String raw) {
    final d = DateTime.tryParse(raw)?.toLocal();
    if (d == null) return '';
    const m = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
    final h = d.hour % 12 == 0 ? 12 : d.hour % 12;
    final mm = d.minute.toString().padLeft(2, '0');
    return '${d.day} ${m[d.month - 1]}, $h:$mm ${d.hour >= 12 ? 'PM' : 'AM'}';
  }

  Widget _itemTile(Map it) {
    String img = (it['image_url'] ?? '').toString();
    if (img.isNotEmpty && !img.startsWith('http')) {
      img = 'https://gofresh-evl7.onrender.com${img.startsWith('/') ? '' : '/'}$img';
    }
    Widget box(IconData ic) => Container(
          width: 44,
          height: 44,
          color: const Color(0xFFF2F4F7),
          child: Icon(ic, size: 20, color: Colors.grey),
        );
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(6),
          child: img.isEmpty
              ? box(Icons.image_not_supported_outlined)
              : Image.network(img, width: 44, height: 44, fit: BoxFit.cover,
                  errorBuilder: (c, e, st) => box(Icons.broken_image_outlined)),
        ),
        const SizedBox(width: 10),
        Expanded(child: Text('${it['name'] ?? ''}', style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600))),
        Text('${it['picked'] ?? 0}/${it['quantity'] ?? 1}', style: TextStyle(fontSize: 12, color: Colors.grey.shade600)),
      ]),
    );
  }

  void _showOrderDetail(Map<String, dynamic> o) {
    final mins = (((o['duration_seconds'] as num?) ?? 0) / 60).ceil();
    final onTime = o['on_time'] == true;
    Widget row(String k, String v) => Padding(
          padding: const EdgeInsets.symmetric(vertical: 6),
          child: Row(children: [
            Expanded(child: Text(k, style: TextStyle(color: Colors.grey.shade600))),
            Text(v, style: const TextStyle(fontWeight: FontWeight.w700)),
          ]),
        );
    showModalBottomSheet<void>(
      isScrollControlled: true,
      context: context,
      builder: (ctx) => Padding(
        padding: const EdgeInsets.fromLTRB(20, 20, 20, 28),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Order #${o['order_id']}', style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w800)),
            const SizedBox(height: 8),
            row('Completed', _when((o['completed_at'] ?? '').toString())),
            row('Items picked', '${o['items_picked']}/${o['items_needed']}'),
            row('Pick time', '$mins min'),
            row('Status', onTime ? 'On time' : 'Late'),
            const SizedBox(height: 8),
            Text('Items', style: const TextStyle(fontWeight: FontWeight.w800)),
            ConstrainedBox(
              constraints: const BoxConstraints(maxHeight: 280),
              child: ListView(
                shrinkWrap: true,
                children: [
                  for (final it in (o['items'] is List ? o['items'] as List : const []))
                    if (it is Map) _itemTile(it),
                ],
              ),
            ),
            const SizedBox(height: 12),
            SizedBox(
              width: double.infinity,
              child: OutlinedButton.icon(
                onPressed: () {
                  Navigator.pop(ctx);
                  _showQr(o['order_id']);
                },
                icon: Icon(Icons.qr_code_2),
                label: Text('Show handover QR'),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _showQr(dynamic orderId) async {
    final id = (orderId as num?)?.toInt();
    if (id == null) return;
    try {
      final r = await ApiService.getPickerHandoverQR(id);
      final token = (r['token'] ?? '').toString();
      if (!mounted) return;
      if (token.isEmpty) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text((r['error'] ?? 'QR not available').toString())),
        );
        return;
      }
      await showDialog<void>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: Text('Order #$id'),
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
    } catch (_) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Network error. Please try again.')));
    }
  }

  Widget _historyList() {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        border: Border.all(color: const Color(0xFFD0D5DD)),
        borderRadius: BorderRadius.circular(14),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Expanded(
                child: Text('Order history (last 7 days)',
                    style: TextStyle(fontSize: 15, fontWeight: FontWeight.w800)),
              ),
              Text('${_history.length} orders', style: TextStyle(fontSize: 12, color: Colors.grey.shade600)),
            ],
          ),
          const SizedBox(height: 10),
          if (_history.isEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 18),
              child: Center(
                child: Text('No completed orders yet', style: TextStyle(color: Colors.grey.shade600)),
              ),
            ),
          for (final o in _history)
            InkWell(
              onTap: () => _showOrderDetail(o),
              child: Container(
              padding: const EdgeInsets.symmetric(vertical: 12),
              decoration: const BoxDecoration(
                border: Border(top: BorderSide(color: Color(0xFFEAECF0))),
              ),
              child: Row(
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text('Order #${o['order_id']}',
                            style: const TextStyle(fontWeight: FontWeight.w800, fontSize: 14)),
                        const SizedBox(height: 3),
                        Text(
                          '${_when((o['completed_at'] ?? '').toString())}  |  '
                          '${o['items_picked']}/${o['items_needed']} items  |  '
                          '${(((o['duration_seconds'] as num?) ?? 0) / 60).ceil()} min',
                          style: TextStyle(fontSize: 12, color: Colors.grey.shade600),
                        ),
                      ],
                    ),
                  ),
                  IconButton(
                    tooltip: 'Show handover QR',
                    icon: Icon(Icons.qr_code_2),
                    onPressed: () => _showQr(o['order_id']),
                  ),
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                    decoration: BoxDecoration(
                      color: o['on_time'] == true ? const Color(0xFFE8F9EE) : const Color(0xFFFFF4E5),
                      borderRadius: BorderRadius.circular(20),
                    ),
                    child: Text(
                      o['on_time'] == true ? 'On time' : 'Late',
                      style: TextStyle(
                        fontSize: 11,
                        fontWeight: FontWeight.w700,
                        color: o['on_time'] == true ? _green : const Color(0xFFB45309),
                      ),
                    ),
                  ),
                ],
              ),
            ),
            ),
        ],
      ),
    );
  }
  Widget _body() {
    final b = _booking;
    final slot = b == null
        ? 'No slot booked today'
        : 'Slot details: ${fmtTime((b['start_time'] ?? '').toString())} - ${fmtTime((b['end_time'] ?? '').toString())}';
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.all(16),
        children: [
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              border: Border.all(color: const Color(0xFFD0D5DD)),
              borderRadius: BorderRadius.circular(14),
            ),
            child: Column(
              children: [
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
                  decoration: BoxDecoration(color: _green, borderRadius: BorderRadius.circular(20)),
                  child: Row(
                    children: [
                      const Text('Active Time',
                          style: TextStyle(color: Colors.white, fontWeight: FontWeight.w800)),
                      const Spacer(),
                      Text(fmtHm(_active),
                          style: const TextStyle(color: Colors.white, fontWeight: FontWeight.w800)),
                    ],
                  ),
                ),
                const SizedBox(height: 12),
                Text(slot, style: TextStyle(color: Colors.grey.shade700)),
                const SizedBox(height: 18),
                Row(
                  children: [
                    _stat('\u20B9${_money((_s['earnings'] as num?) ?? 0)}', 'Earnings', _ink),
                    _stat('${_int('items_picked')}', 'Items picked', _green),
                    _stat('${_int('complaints')}', 'Complaints', Colors.red),
                  ],
                ),
                const SizedBox(height: 12),
                Text(
                  '${_int('orders_completed')} orders completed  |  ${_int('on_time_orders')} on time\n'
                  '\u20B9${_money((_s['pay_per_item'] as num?) ?? 0)} per item + speed bonus \u20B9${_money((_s['speed_bonus'] as num?) ?? 0)} + meter bonus \u20B9${_money((_s['meter_bonus'] as num?) ?? 0)}',
                  textAlign: TextAlign.center,
                  style: TextStyle(fontSize: 12, color: Colors.grey.shade600),
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          _bonusMeter(),
          const SizedBox(height: 16),
          _historyList(),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    Widget content;
    if (_loading) {
      content = const Center(child: CircularProgressIndicator());
    } else if (_error != null) {
      content = RefreshIndicator(
        onRefresh: _load,
        child: ListView(
          children: [
            const SizedBox(height: 120),
            Center(child: Text(_error!, style: const TextStyle(color: Colors.red))),
          ],
        ),
      );
    } else {
      content = _body();
    }
    return Scaffold(
      backgroundColor: Colors.white,
      appBar: AppBar(
        backgroundColor: Colors.white,
        foregroundColor: Colors.black,
        elevation: 0,
        title: const Text('Previous order details', style: TextStyle(fontWeight: FontWeight.w800, fontSize: 17)),
      ),
      body: SafeArea(child: content),
    );
  }
}