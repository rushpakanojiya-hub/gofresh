import 'dart:async';
import 'package:flutter/material.dart';
import '../services/api_service.dart';

class TodayOrders extends StatefulWidget {
  const TodayOrders({super.key});

  @override
  State<TodayOrders> createState() => _TodayOrdersState();
}

class _TodayOrdersState extends State<TodayOrders> {
  List<Map<String, dynamic>> _pending = [];
  List<Map<String, dynamic>> _completed = [];
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _load();
    _timer = Timer.periodic(const Duration(seconds: 20), (_) => _load());
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  List<Map<String, dynamic>> _list(dynamic v) =>
      v is List ? v.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList() : <Map<String, dynamic>>[];

  Future<void> _load() async {
    try {
      final r = await ApiService.getPickerToday();
      if (!mounted || r['error'] != null) return;
      setState(() {
        _pending = _list(r['pending']);
        _completed = _list(r['completed']);
      });
    } catch (_) {}
  }

  Widget _row(Map<String, dynamic> o, bool done) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 5),
      child: Row(
        children: [
          Expanded(
            child: Text('Order #${o['order_id']}  |  ${o['items_picked']}/${o['items_needed']} items',
                style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700)),
          ),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
            decoration: BoxDecoration(
              color: done ? const Color(0xFFE8F9EE) : const Color(0xFFFFF4E5),
              borderRadius: BorderRadius.circular(20),
            ),
            child: Text(
              done ? 'Completed' : (o['status'] == 'in_progress' ? 'Picking' : 'Pending'),
              style: TextStyle(
                fontSize: 11,
                fontWeight: FontWeight.w700,
                color: done ? const Color(0xFF16A34A) : const Color(0xFFB45309),
              ),
            ),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: const BoxConstraints(maxHeight: 220),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        border: Border.all(color: const Color(0xFFD0D5DD)),
        borderRadius: BorderRadius.circular(10),
      ),
      child: ListView(
        shrinkWrap: true,
        children: [
          Text("Today's orders  (${_pending.length} pending, ${_completed.length} completed)",
              style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w800)),
          const SizedBox(height: 6),
          if (_pending.isEmpty && _completed.isEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 8),
              child: Text('No orders today', style: TextStyle(color: Colors.grey.shade600, fontSize: 12)),
            ),
          for (final o in _pending) _row(o, false),
          for (final o in _completed) _row(o, true),
        ],
      ),
    );
  }
}