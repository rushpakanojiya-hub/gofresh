// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'package:flutter/material.dart';
import 'package:geolocator/geolocator.dart';
import '../services/api_service.dart';
import '../utils/fmt.dart';

class StoresScreen extends StatefulWidget {
  final DateTime date;
  const StoresScreen({super.key, required this.date});

  @override
  State<StoresScreen> createState() => _StoresScreenState();
}

class _StoresScreenState extends State<StoresScreen> {
  static const Color _blueDark = Color(0xFF0D47A1);
  static const Color _green = Color(0xFF16A34A);
  static const Color _orange = Color(0xFFEA580C);
  static const Color _indigo = Color(0xFF3F51B5);

  bool _loading = true;
  bool _booking = false;
  bool _locDenied = false;
  String? _error;
  double? _lat;
  double? _lng;
  String _filter = 'all';
  final Set<int> _sel = {};
  List<Map<String, dynamic>> _stores = [];

  @override
  void initState() {
    super.initState();
    _init();
  }

  String get _date => fmtDate(widget.date);
  bool get _isToday => fmtDate(DateTime.now()) == _date;

  void _snack(String m) {
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(m)));
  }

  Future<void> _init() async {
    await _locate();
    await _load();
  }

  Future<void> _locate() async {
    try {
      var perm = await Geolocator.checkPermission();
      if (perm == LocationPermission.denied) {
        perm = await Geolocator.requestPermission();
      }
      if (perm == LocationPermission.denied || perm == LocationPermission.deniedForever) {
        _locDenied = true;
        return;
      }
      if (!await Geolocator.isLocationServiceEnabled()) {
        _locDenied = true;
        return;
      }
      final pos = await Geolocator.getCurrentPosition(
        locationSettings: const LocationSettings(
          accuracy: LocationAccuracy.medium,
          timeLimit: Duration(seconds: 10),
        ),
      );
      _lat = pos.latitude;
      _lng = pos.longitude;
    } catch (_) {
      _locDenied = true;
    }
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
      _sel.clear();
    });
    try {
      final r = await ApiService.getPickerStores(lat: _lat, lng: _lng, date: _date);
      if (!mounted) return;
      if (r['error'] != null) {
        setState(() {
          _error = r['error'].toString();
          _loading = false;
        });
        return;
      }
      final list = r['stores'];
      setState(() {
        _stores = list is List
            ? list.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList()
            : [];
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

  int _mins(String t) {
    final p = t.split(':');
    if (p.length < 2) return 0;
    return (int.tryParse(p[0]) ?? 0) * 60 + (int.tryParse(p[1]) ?? 0);
  }

  int get _nowMins {
    final n = DateTime.now();
    return n.hour * 60 + n.minute;
  }

  String _st(Map<String, dynamic> s) => (s['start_time'] ?? '').toString();
  String _en(Map<String, dynamic> s) => (s['end_time'] ?? '').toString();
  bool _ended(Map<String, dynamic> s) => _isToday && _nowMins >= _mins(_en(s));
  bool _started(Map<String, dynamic> s) => _isToday && _nowMins >= _mins(_st(s));
  bool _mine(Map<String, dynamic> s) => s['booked_by_me'] == true;
  bool _full(Map<String, dynamic> s) {
    final cap = (s['capacity'] as num?)?.toInt() ?? 0;
    final booked = (s['booked'] as num?)?.toInt() ?? 0;
    return cap > 0 && booked >= cap;
  }

  bool _open(Map<String, dynamic> s) => !_mine(s) && !_full(s) && !_ended(s);

  bool _passes(Map<String, dynamic> s) {
    if (_filter == 'open') return _open(s);
    if (_filter == 'booked') return _mine(s);
    return true;
  }

  String _groupOf(Map<String, dynamic> s) {
    final m = _mins(_st(s));
    if (m < 720) return 'Morning';
    if (m < 1020) return 'Afternoon';
    return 'Evening';
  }

  List<Map<String, dynamic>> _slotsOf(Map<String, dynamic> st) {
    final r = st['slots'];
    return r is List ? r.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList() : <Map<String, dynamic>>[];
  }

  String _hrs(double h) => h == h.roundToDouble() ? '${h.round()}' : h.toStringAsFixed(1);

  Future<void> _book() async {
    if (_sel.isEmpty || _booking) return;
    setState(() => _booking = true);
    var ok = 0;
    String? err;
    for (final id in _sel.toList()) {
      try {
        final r = await ApiService.bookSlot(id, _date);
        if (r['error'] != null) {
          err = r['error'].toString();
        } else {
          ok++;
        }
      } catch (_) {
        err = 'Network error. Please try again.';
      }
    }
    if (!mounted) return;
    if (ok > 0) {
      if (err != null) _snack('$ok booked. $err');
      Navigator.pop(context, true);
      return;
    }
    setState(() => _booking = false);
    if (err != null) _snack(err);
    await _load();
  }

  Widget _banner() {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.fromLTRB(16, 4, 16, 16),
      color: _blueDark,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Book your slots',
            style: TextStyle(color: Colors.white, fontSize: 22, fontWeight: FontWeight.w800),
          ),
          const SizedBox(height: 12),
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(color: const Color(0xFFE8EEF9), borderRadius: BorderRadius.circular(12)),
            child: Row(
              children: [
                const Icon(Icons.location_on, color: Colors.redAccent),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('${_stores.length} Stores', style: const TextStyle(fontWeight: FontWeight.w800)),
                      Text(
                        _locDenied ? 'Location off - distance unavailable' : 'Near you',
                        style: TextStyle(fontSize: 12, color: Colors.grey.shade700),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 10),
          Text(fmtDateLong(widget.date), style: const TextStyle(color: Colors.white70, fontWeight: FontWeight.w600)),
        ],
      ),
    );
  }

  Widget _chip(String id, String label) {
    final sel = _filter == id;
    return Padding(
      padding: const EdgeInsets.only(right: 8),
      child: ChoiceChip(
        label: Text(label, style: TextStyle(color: sel ? Colors.white : Colors.black87, fontSize: 12)),
        selected: sel,
        selectedColor: const Color(0xFF455A64),
        backgroundColor: Colors.white,
        showCheckmark: false,
        onSelected: (_) => setState(() => _filter = id),
      ),
    );
  }

  Widget _slotRow(Map<String, dynamic> s) {
    final id = (s['id'] as num).toInt();
    final cap = (s['capacity'] as num?)?.toInt() ?? 0;
    final booked = (s['booked'] as num?)?.toInt() ?? 0;
    final mine = _mine(s);
    final ended = _ended(s);
    final full = _full(s);
    final started = _started(s);
    final canPick = _open(s);
    final dim = ended && !mine;

    Widget trailing;
    if (mine) {
      trailing = Padding(
        padding: const EdgeInsets.all(12),
        child: Icon(started && !ended ? Icons.play_circle_fill : Icons.check_circle, color: _green),
      );
    } else if (ended) {
      trailing = const Padding(
        padding: EdgeInsets.all(12),
        child: Text('Closed', style: TextStyle(fontSize: 12, color: Colors.black45)),
      );
    } else if (full) {
      trailing = const Padding(
        padding: EdgeInsets.all(12),
        child: Text('Full', style: TextStyle(fontSize: 12, color: Colors.black45)),
      );
    } else {
      trailing = Checkbox(
        value: _sel.contains(id),
        activeColor: _green,
        onChanged: _booking ? null : (_) => setState(() { if (!_sel.add(id)) _sel.remove(id); }),
      );
    }

    return InkWell(
      onTap: (!canPick || _booking) ? null : () => setState(() { if (!_sel.add(id)) _sel.remove(id); }),
      child: Container(
        padding: const EdgeInsets.fromLTRB(14, 8, 6, 8),
        decoration: BoxDecoration(
          color: Colors.white,
          border: Border(
            left: BorderSide(color: mine ? _indigo : Colors.transparent, width: 3),
            bottom: const BorderSide(color: Color(0xFFEDEDED)),
          ),
        ),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    '${fmtTime(_st(s))} - ${fmtTime(_en(s))}',
                    style: TextStyle(
                      fontSize: 15,
                      fontWeight: FontWeight.w700,
                      color: dim ? Colors.black38 : Colors.black87,
                    ),
                  ),
                  const SizedBox(height: 2),
                  if (mine)
                    Text(started && !ended ? 'Booked - running now' : 'Booked',
                        style: const TextStyle(fontSize: 12, color: _indigo, fontWeight: FontWeight.w600))
                  else
                    Text('$booked/$cap booked',
                        style: TextStyle(fontSize: 12, color: dim ? Colors.black38 : _orange, fontWeight: FontWeight.w600)),
                ],
              ),
            ),
            trailing,
          ],
        ),
      ),
    );
  }

  List<Widget> _groups(List<Map<String, dynamic>> slots) {
    final out = <Widget>[];
    for (final g in const ['Morning', 'Afternoon', 'Evening']) {
      final all = slots.where((s) => _groupOf(s) == g).toList();
      final shown = all.where(_passes).toList();
      if (shown.isEmpty) continue;
      out.add(Container(
        width: double.infinity,
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        color: const Color(0xFF5F6368),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('$g slots', style: const TextStyle(color: Colors.white, fontWeight: FontWeight.bold, fontSize: 14)),
            const SizedBox(height: 2),
            Text('${fmtTime(_st(all.first))} - ${fmtTime(_en(all.last))} \u2022 ${all.length} Slots',
                style: const TextStyle(color: Colors.white70, fontSize: 12)),
          ],
        ),
      ));
      for (final s in shown) {
        out.add(_slotRow(s));
      }
    }
    if (out.isEmpty) {
      out.add(const Padding(padding: EdgeInsets.all(16), child: Text('No slots here')));
    }
    return out;
  }

  Widget _storeCard(Map<String, dynamic> st) {
    final dist = (st['distance_km'] as num?)?.toDouble() ?? 0;
    final slots = _slotsOf(st);
    return Padding(
      padding: const EdgeInsets.only(bottom: 16),
      child: Column(
        children: [
          Container(
            padding: const EdgeInsets.all(12),
            decoration: const BoxDecoration(
              color: Color(0xFFD6E6FF),
              borderRadius: BorderRadius.vertical(top: Radius.circular(12)),
            ),
            child: Row(
              children: [
                if (dist > 0)
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
                    margin: const EdgeInsets.only(right: 12),
                    decoration: BoxDecoration(color: Colors.white54, borderRadius: BorderRadius.circular(8)),
                    child: Text(
                      '$dist\nkms',
                      textAlign: TextAlign.center,
                      style: const TextStyle(fontWeight: FontWeight.w800, fontSize: 13),
                    ),
                  ),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text((st['name'] ?? '').toString(),
                          style: const TextStyle(fontWeight: FontWeight.w800, fontSize: 16)),
                      Text((st['address'] ?? '').toString(),
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(fontSize: 12, color: Colors.grey.shade700)),
                    ],
                  ),
                ),
              ],
            ),
          ),
          Container(
            decoration: BoxDecoration(
              color: Colors.white,
              border: Border.all(color: const Color(0xFFD6E6FF)),
              borderRadius: const BorderRadius.vertical(bottom: Radius.circular(12)),
            ),
            clipBehavior: Clip.antiAlias,
            child: slots.isEmpty
                ? const Padding(padding: EdgeInsets.all(16), child: Text('No slots available'))
                : Column(children: _groups(slots)),
          ),
        ],
      ),
    );
  }

  Widget _summaryBar() {
    final all = <Map<String, dynamic>>[for (final st in _stores) ..._slotsOf(st)];
    final chosen = _sel.isNotEmpty
        ? all.where((s) => _sel.contains((s['id'] as num).toInt())).toList()
        : all.where(_mine).toList();
    var hours = 0.0;
    for (final s in chosen) {
      hours += (_mins(_en(s)) - _mins(_st(s))) / 60.0;
    }
    final n = chosen.length;
    return Container(
      width: double.infinity,
      color: const Color(0xFF1E88E5),
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      child: Row(
        children: [
          Expanded(
            child: Text(_sel.isNotEmpty ? 'Selected slots' : 'Booked slots',
                style: const TextStyle(color: Colors.white, fontWeight: FontWeight.bold, fontSize: 13)),
          ),
          Text('$n Slot${n == 1 ? '' : 's'} (${_hrs(hours)} hours)',
              style: const TextStyle(color: Colors.white, fontSize: 13)),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xFFF3F7FF),
      appBar: AppBar(
        backgroundColor: _blueDark,
        foregroundColor: Colors.white,
        title: const Text('Choose a store'),
      ),
      body: Column(
        children: [
          _banner(),
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 10, 16, 0),
            child: Row(children: [_chip('all', 'All'), _chip('open', 'Open'), _chip('booked', 'Booked')]),
          ),
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : _error != null
                    ? Center(
                        child: Column(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Text(_error!, style: const TextStyle(color: Colors.red)),
                            const SizedBox(height: 8),
                            TextButton(onPressed: _load, child: const Text('Retry')),
                          ],
                        ),
                      )
                    : _stores.isEmpty
                        ? const Center(child: Text('No stores found'))
                        : RefreshIndicator(
                            onRefresh: _load,
                            child: ListView(
                              padding: const EdgeInsets.all(16),
                              children: _stores.map(_storeCard).toList(),
                            ),
                          ),
          ),
          _summaryBar(),
          SafeArea(
            top: false,
            child: Padding(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 12),
              child: SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  onPressed: (_sel.isEmpty || _booking) ? null : _book,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: _green,
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(vertical: 16),
                    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
                  ),
                  child: _booking
                      ? const SizedBox(
                          height: 20,
                          width: 20,
                          child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                        )
                      : Text(_sel.length > 1 ? 'Book ${_sel.length} slots' : 'Book slot',
                          style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w700)),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}