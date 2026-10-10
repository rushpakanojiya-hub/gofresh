// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../utils/fmt.dart';

class PayoutScreen extends StatefulWidget {
  const PayoutScreen({super.key});

  @override
  State<PayoutScreen> createState() => _PayoutScreenState();
}

class _PayoutScreenState extends State<PayoutScreen> {
  static const Color _green = Color(0xFF16A34A);
  static const List<String> _mon = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

  bool _weekly = false;
  DateTime _date = DateTime.now();
  bool _loading = true;
  String? _error;
  Map<String, dynamic> _d = {};

  @override
  void initState() {
    super.initState();
    _load();
  }

  String _two(int n) => n.toString().padLeft(2, '0');
  String _ymd(DateTime d) => '${d.year}-${_two(d.month)}-${_two(d.day)}';
  String _label(DateTime d) => '${d.day} ${_mon[d.month - 1]} \u2019${_two(d.year % 100)}';

  String _dateText() {
    if (!_weekly) return _label(_date);
    final s = _date.subtract(Duration(days: _date.weekday - 1));
    final e = s.add(Duration(days: 6));
    return '${_label(s)} - ${_label(e)}';
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final r = await ApiService.getPickerPayout(_weekly ? 'week' : 'day', _ymd(_date));
      if (!mounted) return;
      if (r['error'] != null) {
        setState(() {
          _error = r['error'].toString();
          _loading = false;
        });
        return;
      }
      setState(() {
        _d = r;
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

  Future<void> _pickDate() async {
    final p = await showDatePicker(
      context: context,
      initialDate: _date,
      firstDate: DateTime(2025, 1, 1),
      lastDate: DateTime.now(),
    );
    if (p != null) {
      _date = p;
      _load();
    }
  }

  num _n(String k) => (_d[k] as num?) ?? 0;
  String _money(num v) => v == v.roundToDouble() ? v.toInt().toString() : v.toStringAsFixed(2);

  Widget _row(String title, String amount, {String? sub, Color? color}) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 10),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: TextStyle(fontSize: 15, fontWeight: FontWeight.w700)),
                if (sub != null) Text(sub, style: TextStyle(fontSize: 12, color: Colors.grey.shade600)),
              ],
            ),
          ),
          Text(amount, style: TextStyle(fontSize: 15, fontWeight: FontWeight.w700, color: color)),
        ],
      ),
    );
  }

  Widget _perf(String value, String label, Color c) {
    return Expanded(
      child: Column(
        children: [
          Text(value, style: TextStyle(fontSize: 22, fontWeight: FontWeight.w800, color: c)),
          SizedBox(height: 2),
          Text(label, style: TextStyle(fontSize: 12, color: Colors.grey.shade600)),
        ],
      ),
    );
  }

  Widget _tab(String text, bool weekly) {
    final sel = _weekly == weekly;
    return Expanded(
      child: InkWell(
        onTap: () {
          if (_weekly != weekly) {
            _weekly = weekly;
            _load();
          }
        },
        child: Container(
          padding: const EdgeInsets.symmetric(vertical: 12),
          decoration: BoxDecoration(
            border: Border(bottom: BorderSide(color: sel ? Colors.black : Colors.transparent, width: 2)),
          ),
          child: Center(
            child: Text(text,
                style: TextStyle(fontWeight: FontWeight.w700, color: sel ? Colors.black : Colors.grey)),
          ),
        ),
      ),
    );
  }

  Widget _body() {
    final penalty = _n('penalty');
    final activeRaw = _d['active_seconds'];
    final active = activeRaw is num ? fmtHm(activeRaw.toInt()) : '--';
    final speed = _n('speed_bonus');
    final meter = _n('meter_bonus');
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.all(16),
        children: [
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: Colors.white,
              border: Border.all(color: const Color(0xFFE4E7EC)),
              borderRadius: BorderRadius.circular(14),
            ),
            child: Column(
              children: [
                _row('Total payout', '\u20B9${_money(_n('total'))}', color: _green),
                Divider(height: 1),
                _row('Item Picking', '\u20B9${_money(_n('item_picking'))}',
                    sub: '${_n('items_picked').toInt()} items'),
                Divider(height: 1),
                _row('Bonus', '\u20B9${_money(_n('bonus'))}',
                    sub: 'Speed \u20B9${_money(speed)}  |  Daily meter \u20B9${_money(meter)}'),
                if (penalty > 0) ...[
                  Divider(height: 1),
                  _row('Penalty', '-\u20B9${_money(penalty)}', sub: 'Complaint', color: Colors.red),
                ],
              ],
            ),
          ),
          SizedBox(height: 20),
          Text('Performance', style: TextStyle(fontSize: 17, fontWeight: FontWeight.w800)),
          SizedBox(height: 12),
          Container(
            padding: const EdgeInsets.symmetric(vertical: 18),
            decoration: BoxDecoration(
              color: Colors.white,
              border: Border.all(color: const Color(0xFFE4E7EC)),
              borderRadius: BorderRadius.circular(14),
            ),
            child: Column(
              children: [
                Row(children: [
                  _perf(active, 'Active hours', Colors.black87),
                  _perf('${_n('items_picked').toInt()}', 'Items picked', _green),
                ]),
                SizedBox(height: 20),
                Row(children: [
                  _perf('${_n('complaints').toInt()}', 'Complaints', Colors.red),
                  _perf('${_n('orders_completed').toInt()}', 'Orders', Colors.black87),
                ]),
              ],
            ),
          ),
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
        child: ListView(children: [
          const SizedBox(height: 120),
          Center(child: Text(_error!, style: const TextStyle(color: Colors.red))),
        ]),
      );
    } else {
      content = _body();
    }
    return Scaffold(
      backgroundColor: const Color(0xFFF4F6F8),
      appBar: AppBar(
        backgroundColor: Colors.white,
        foregroundColor: Colors.black,
        elevation: 0,
        title: const Text('Earnings', style: TextStyle(fontWeight: FontWeight.w800, fontSize: 18)),
      ),
      body: SafeArea(
        child: Column(
          children: [
            Container(
              color: Colors.white,
              child: Row(children: [_tab('Daily', false), _tab('Weekly', true)]),
            ),
            InkWell(
              onTap: _pickDate,
              child: Container(
                width: double.infinity,
                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
                color: const Color(0xFFE9EDF2),
                child: Row(children: [
                  Text(_dateText(), style: TextStyle(fontWeight: FontWeight.w700)),
                  Icon(Icons.arrow_drop_down),
                ]),
              ),
            ),
            Expanded(child: content),
          ],
        ),
      ),
    );
  }
}