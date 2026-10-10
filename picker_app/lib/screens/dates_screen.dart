// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../utils/fmt.dart';
import 'stores_screen.dart';

class _DayInfo {
  final DateTime date;
  final int openSlots;
  final int myBooked;
  final int stores;
  const _DayInfo(this.date, this.openSlots, this.myBooked, this.stores);
}

class DatesScreen extends StatefulWidget {
  const DatesScreen({super.key});

  @override
  State<DatesScreen> createState() => _DatesScreenState();
}

class _DatesScreenState extends State<DatesScreen> {
  static const Color _blueDark = Color(0xFF0D47A1);
  static const Color _blue = Color(0xFF1976D2);
  static const Color _green = Color(0xFF16A34A);
  static const Color _grey = Color(0xFF9AA5B8);
  static const int _days = 7;

  bool _loading = true;
  String? _error;
  List<_DayInfo> _infos = [];

  @override
  void initState() {
    super.initState();
    _load();
  }

  bool _ended(Map s, bool isToday) {
    if (!isToday) return false;
    final p = (s['end_time'] ?? '').toString().split(':');
    if (p.length < 2) return false;
    final now = DateTime.now();
    final end = DateTime(now.year, now.month, now.day, int.tryParse(p[0]) ?? 0, int.tryParse(p[1]) ?? 0);
    return now.isAfter(end);
  }

  _DayInfo _summarize(DateTime d, Map<String, dynamic> r, bool isToday) {
    var open = 0;
    var mine = 0;
    var stores = 0;
    final list = r['stores'];
    if (list is List) {
      for (final st in list.whereType<Map>()) {
        var storeOpen = 0;
        final slots = st['slots'];
        if (slots is List) {
          for (final s in slots.whereType<Map>()) {
            if (s['booked_by_me'] == true) mine++;
            final cap = (s['capacity'] as num?)?.toInt() ?? 0;
            final booked = (s['booked'] as num?)?.toInt() ?? 0;
            final full = cap > 0 && booked >= cap;
            if (!full && !_ended(s, isToday)) storeOpen++;
          }
        }
        if (storeOpen > 0) {
          open += storeOpen;
          stores++;
        }
      }
    }
    return _DayInfo(d, open, mine, stores);
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final now = DateTime.now();
      final dates = List.generate(_days, (i) => DateTime(now.year, now.month, now.day + i));
      final results = await Future.wait(
        dates.map((d) => ApiService.getPickerStores(date: fmtDate(d))),
      );
      final infos = <_DayInfo>[];
      for (var i = 0; i < dates.length; i++) {
        final r = results[i];
        if (r['error'] != null) {
          if (!mounted) return;
          setState(() {
            _error = r['error'].toString();
            _loading = false;
          });
          return;
        }
        infos.add(_summarize(dates[i], r, i == 0));
      }
      if (!mounted) return;
      setState(() {
        _infos = infos;
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

  Future<void> _open(_DayInfo i) async {
    final ok = await Navigator.push<bool>(
      context,
      MaterialPageRoute(builder: (_) => StoresScreen(date: i.date)),
    );
    if (!mounted) return;
    if (ok == true) {
      Navigator.pop(context, true);
    } else {
      _load();
    }
  }

  Widget _monthLabel(String m) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 12),
      child: Row(
        children: [
          const Expanded(child: Divider()),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12),
            child: Text(m, style: TextStyle(color: Colors.grey.shade600, fontWeight: FontWeight.w600)),
          ),
          const Expanded(child: Divider()),
        ],
      ),
    );
  }

  Widget _dayCard(_DayInfo i, bool isToday) {
    final none = i.openSlots == 0 && i.myBooked == 0;
    final color = none ? _grey : (isToday ? _blue : _green);
    var title = none ? 'No slots open' : '${i.openSlots} Slots open';
    if (i.myBooked > 0) title += ', ${i.myBooked} Booked';
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: InkWell(
        borderRadius: BorderRadius.circular(12),
        onTap: none ? null : () => _open(i),
        child: Container(
          decoration: BoxDecoration(
            border: Border.all(color: const Color(0xFFD6E6FF)),
            borderRadius: BorderRadius.circular(12),
          ),
          child: Row(
            children: [
              Container(
                width: 64,
                height: 72,
                decoration: BoxDecoration(
                  color: color,
                  borderRadius: const BorderRadius.horizontal(left: Radius.circular(12)),
                ),
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Text('${i.date.day}',
                        style: const TextStyle(color: Colors.white, fontSize: 22, fontWeight: FontWeight.w800)),
                    Text(fmtDay(i.date), style: const TextStyle(color: Colors.white, fontWeight: FontWeight.w600)),
                  ],
                ),
              ),
              const SizedBox(width: 14),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(title, style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w800)),
                    const SizedBox(height: 4),
                    Row(
                      children: [
                        Icon(Icons.storefront_outlined, size: 14, color: Colors.grey.shade600),
                        const SizedBox(width: 4),
                        Text('${i.stores} Store(s) available',
                            style: TextStyle(fontSize: 12, color: Colors.grey.shade700)),
                      ],
                    ),
                  ],
                ),
              ),
              if (!none) const Icon(Icons.chevron_right),
              const SizedBox(width: 8),
            ],
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    Widget body;
    if (_loading) {
      body = const Center(child: CircularProgressIndicator());
    } else if (_error != null) {
      body = Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(_error!, style: const TextStyle(color: Colors.red)),
            const SizedBox(height: 8),
            TextButton(onPressed: _load, child: const Text('Retry')),
          ],
        ),
      );
    } else {
      final children = <Widget>[];
      int? lastMonth;
      for (var k = 0; k < _infos.length; k++) {
        final i = _infos[k];
        if (lastMonth != i.date.month) {
          children.add(_monthLabel(fmtMonth(i.date)));
          lastMonth = i.date.month;
        }
        children.add(_dayCard(i, k == 0));
      }
      body = RefreshIndicator(
        onRefresh: _load,
        child: ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.all(16),
          children: children,
        ),
      );
    }
    return Scaffold(
      backgroundColor: Colors.white,
      appBar: AppBar(
        backgroundColor: _blueDark,
        foregroundColor: Colors.white,
        title: const Text('Pick a date'),
      ),
      body: body,
    );
  }
}