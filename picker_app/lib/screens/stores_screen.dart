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

  bool _loading = true;
  bool _booking = false;
  bool _locDenied = false;
  String? _error;
  double? _lat;
  double? _lng;
  int? _selSlotId;
  List<Map<String, dynamic>> _stores = [];

  @override
  void initState() {
    super.initState();
    _init();
  }

  String get _date => fmtDate(widget.date);

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
      _selSlotId = null;
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

  bool _slotEnded(Map<String, dynamic> s) {
    if (fmtDate(DateTime.now()) != _date) return false;
    final p = (s['end_time'] ?? '').toString().split(':');
    if (p.length < 2) return false;
    final now = DateTime.now();
    final end = DateTime(now.year, now.month, now.day, int.tryParse(p[0]) ?? 0, int.tryParse(p[1]) ?? 0);
    return now.isAfter(end);
  }

  Future<void> _book() async {
    final id = _selSlotId;
    if (id == null || _booking) return;
    setState(() => _booking = true);
    try {
      final r = await ApiService.bookSlot(id, _date);
      if (!mounted) return;
      if (r['error'] != null) {
        setState(() => _booking = false);
        _snack(r['error'].toString());
        await _load();
        return;
      }
      Navigator.pop(context, true);
    } catch (_) {
      if (!mounted) return;
      setState(() => _booking = false);
      _snack('Network error. Please try again.');
    }
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
            'Book your slot',
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

  Widget _slotRow(Map<String, dynamic> s) {
    final id = (s['id'] as num).toInt();
    final cap = (s['capacity'] as num?)?.toInt() ?? 0;
    final booked = (s['booked'] as num?)?.toInt() ?? 0;
    final mine = s['booked_by_me'] == true;
    final full = cap > 0 && booked >= cap;
    final ended = _slotEnded(s);
    final disabled = mine || full || ended;
    final selected = _selSlotId == id;
    String note = '$booked/$cap booked';
    if (mine) note = 'Already booked by you';
    if (full) note = 'Slot full';
    if (ended) note = 'Slot over';
    return InkWell(
      onTap: disabled ? null : () => setState(() => _selSlotId = selected ? null : id),
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 10, horizontal: 12),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    '${fmtTime((s['start_time'] ?? '').toString())} - ${fmtTime((s['end_time'] ?? '').toString())}',
                    style: TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w800,
                      color: disabled ? Colors.grey : Colors.black87,
                    ),
                  ),
                  const SizedBox(height: 2),
                  Text(note, style: const TextStyle(fontSize: 12, color: _orange, fontWeight: FontWeight.w600)),
                ],
              ),
            ),
            Icon(
              mine
                  ? Icons.check_circle
                  : (selected ? Icons.check_box : Icons.check_box_outline_blank),
              color: disabled && !mine ? Colors.grey.shade400 : _green,
            ),
          ],
        ),
      ),
    );
  }

  Widget _storeCard(Map<String, dynamic> st) {
    final dist = (st['distance_km'] as num?)?.toDouble() ?? 0;
    final slotsRaw = st['slots'];
    final slots = slotsRaw is List
        ? slotsRaw.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList()
        : <Map<String, dynamic>>[];
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
                      Text(
                        (st['name'] ?? '').toString(),
                        style: const TextStyle(fontWeight: FontWeight.w800, fontSize: 16),
                      ),
                      Text(
                        (st['address'] ?? '').toString(),
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(fontSize: 12, color: Colors.grey.shade700),
                      ),
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
            child: slots.isEmpty
                ? const Padding(
                    padding: EdgeInsets.all(16),
                    child: Text('No slots available'),
                  )
                : Column(children: slots.map(_slotRow).toList()),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.white,
      appBar: AppBar(
        backgroundColor: _blueDark,
        foregroundColor: Colors.white,
        title: const Text('Choose a store'),
      ),
      body: Column(
        children: [
          _banner(),
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
                        : ListView(
                            padding: const EdgeInsets.all(16),
                            children: _stores.map(_storeCard).toList(),
                          ),
          ),
          SafeArea(
            top: false,
            child: Padding(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 12),
              child: SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  onPressed: (_selSlotId == null || _booking) ? null : _book,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: Colors.black,
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
                      : const Text('Book slot', style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700)),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}