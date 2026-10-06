import 'package:flutter/material.dart';
import '../services/api_service.dart';

const Color _gBg = Color(0xFFF7F1FB);
const Color _gGreen = Color(0xFF1ED760);

class GigsScreen extends StatefulWidget {
  const GigsScreen({super.key});

  @override
  State<GigsScreen> createState() => _GigsScreenState();
}

class _GigsScreenState extends State<GigsScreen> {
  static const List<String> _months = [
    'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
    'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec',
  ];

  int _dayOffset = 0;
  bool _loading = true;
  bool _saving = false;
  String? _error;
  String _filter = 'all';
  Map<String, dynamic>? _data;
  final Set<String> _selected = {};

  DateTime get _day => DateTime.now().toUtc().add(Duration(hours: 5, minutes: 30, days: _dayOffset));
  String get _date =>
      '${_day.year}-${_day.month.toString().padLeft(2, '0')}-${_day.day.toString().padLeft(2, '0')}';

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
      _selected.clear();
    });
    try {
      final data = await ApiService.getGigs(_date);
      if (!mounted) return;
      setState(() {
        _data = data;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.toString().replaceFirst('Exception: ', '');
        _loading = false;
      });
    }
  }

  void _toast(String m) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(m)));
  }

  Future<void> _book() async {
    if (_selected.isEmpty || _saving) return;
    setState(() => _saving = true);
    try {
      await ApiService.bookGigs(_date, _selected.toList());
      _toast('Gigs booked');
      await _load();
    } catch (e) {
      _toast(e.toString().replaceFirst('Exception: ', ''));
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  Future<void> _cancel(String key, String label) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Cancel this gig?'),
        content: Text(label),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('No')),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Cancel gig', style: TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );
    if (ok != true) return;
    setState(() => _saving = true);
    try {
      await ApiService.cancelGig(_date, key);
      _toast('Gig cancelled');
      await _load();
    } catch (e) {
      _toast(e.toString().replaceFirst('Exception: ', ''));
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  String _rs(num v) => v.round().toString();

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

  Widget _slotRow(dynamic s) {
    final key = '${s['key']}';
    final label = '${s['label']}';
    final booked = s['booked'] == true;
    final started = s['started'] == true;
    final ended = s['ended'] == true;
    final minR = (_data?['per_hour_min'] as num?) ?? 0;
    final maxR = (_data?['per_hour_max'] as num?) ?? 0;
    final canPick = !booked && !ended;

    Widget trailing;
    if (booked) {
      trailing = started
          ? const Padding(padding: EdgeInsets.all(12), child: Icon(Icons.check_circle, color: Colors.green))
          : IconButton(
              icon: const Icon(Icons.cancel_outlined),
              onPressed: _saving ? null : () => _cancel(key, label),
            );
    } else if (ended) {
      trailing = const Padding(
        padding: EdgeInsets.all(12),
        child: Text('Closed', style: TextStyle(fontSize: 12, color: Colors.black45)),
      );
    } else {
      trailing = Checkbox(
        value: _selected.contains(key),
        activeColor: Colors.green,
        onChanged: _saving
            ? null
            : (v) => setState(() {
                  if (v == true) {
                    _selected.add(key);
                  } else {
                    _selected.remove(key);
                  }
                }),
      );
    }

    return InkWell(
      onTap: !canPick || _saving
          ? null
          : () => setState(() {
                if (!_selected.add(key)) _selected.remove(key);
              }),
      child: Container(
        padding: const EdgeInsets.fromLTRB(14, 8, 6, 8),
        decoration: BoxDecoration(
          color: Colors.white,
          border: Border(
            left: BorderSide(color: booked ? const Color(0xFF5C6BC0) : Colors.transparent, width: 3),
            bottom: const BorderSide(color: Color(0xFFEDEDED)),
          ),
        ),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(label,
                      style: TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w600,
                        color: ended && !booked ? Colors.black38 : Colors.black87,
                      )),
                  const SizedBox(height: 2),
                  Text('\u20B9${_rs(minR)} - \u20B9${_rs(maxR)} per hour',
                      style: const TextStyle(fontSize: 12, color: Colors.black54)),
                  if (booked)
                    const Padding(
                      padding: EdgeInsets.only(top: 2),
                      child: Text('Booked', style: TextStyle(fontSize: 12, color: Color(0xFF3F51B5))),
                    ),
                ],
              ),
            ),
            trailing,
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final groups = (_data?['groups'] as List?) ?? [];
    final list = <Widget>[];
    var selCount = 0;
    var selHours = 0;
    for (final g in groups) {
      final all = (g['slots'] as List?) ?? [];
      for (final s in all) {
        if (_selected.contains('${s['key']}')) {
          selCount++;
          selHours += ((s['hours'] as num?) ?? 0).toInt();
        }
      }
      final shown = all.where((s) {
        final booked = s['booked'] == true;
        final ended = s['ended'] == true;
        if (_filter == 'open') return !booked && !ended;
        if (_filter == 'booked') return booked;
        return true;
      }).toList();
      if (shown.isEmpty) continue;
      list.add(Container(
        margin: const EdgeInsets.only(top: 12),
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        decoration: const BoxDecoration(
          color: Color(0xFF5F6368),
          borderRadius: BorderRadius.vertical(top: Radius.circular(10)),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('${g['name']}', style: const TextStyle(color: Colors.white, fontWeight: FontWeight.bold, fontSize: 14)),
            const SizedBox(height: 2),
            Text('${g['start_label']} - ${g['end_label']} \u2022 ${g['slot_count']} Gigs',
                style: const TextStyle(color: Colors.white70, fontSize: 12)),
          ],
        ),
      ));
      for (final s in shown) {
        list.add(_slotRow(s));
      }
    }

    final minR = (_data?['per_hour_min'] as num?) ?? 0;
    final maxR = (_data?['per_hour_max'] as num?) ?? 0;
    final showSelected = selCount > 0;
    final count = showSelected ? selCount : ((_data?['booked_count'] as num?) ?? 0).toInt();
    final hours = showSelected ? selHours : ((_data?['booked_hours'] as num?) ?? 0).toInt();

    return Scaffold(
      backgroundColor: _gBg,
      appBar: AppBar(
        backgroundColor: Colors.white,
        elevation: 0,
        iconTheme: const IconThemeData(color: Colors.black87),
        title: Text('Gigs, ${_day.day} ${_months[_day.month - 1]}',
            style: const TextStyle(color: Colors.black87, fontSize: 18, fontWeight: FontWeight.bold)),
        actions: [
          TextButton(
            onPressed: _loading
                ? null
                : () {
                    setState(() => _dayOffset = _dayOffset == 0 ? 1 : 0);
                    _load();
                  },
            child: Text(_dayOffset == 0 ? 'Tomorrow' : 'Today'),
          ),
        ],
      ),
      body: SafeArea(
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 10, 16, 0),
              child: Row(children: [_chip('all', 'All'), _chip('open', 'Open'), _chip('booked', 'Booked')]),
            ),
            Expanded(
              child: _loading
                  ? const Center(child: CircularProgressIndicator())
                  : _error != null
                      ? Center(child: Text(_error!))
                      : RefreshIndicator(
                          onRefresh: _load,
                          child: ListView(
                            padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
                            children: list.isEmpty
                                ? const [Padding(padding: EdgeInsets.all(40), child: Center(child: Text('No gigs here')))]
                                : list,
                          ),
                        ),
            ),
            Container(
              width: double.infinity,
              color: const Color(0xFF1E88E5),
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
              child: Row(
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const Text('Estimated payout',
                            style: TextStyle(color: Colors.white, fontWeight: FontWeight.bold, fontSize: 12)),
                        Text(showSelected ? 'Selected Gigs' : 'Booked Gigs',
                            style: const TextStyle(color: Colors.white, fontSize: 12)),
                      ],
                    ),
                  ),
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: [
                      Text('\u20B9${_rs(minR * hours)} - \u20B9${_rs(maxR * hours)}',
                          style: const TextStyle(color: Colors.white, fontWeight: FontWeight.bold, fontSize: 14)),
                      Text('$count Gig${count == 1 ? '' : 's'} ($hours hours)',
                          style: const TextStyle(color: Colors.white, fontSize: 12)),
                    ],
                  ),
                ],
              ),
            ),
            Padding(
              padding: const EdgeInsets.all(12),
              child: SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  onPressed: (_selected.isEmpty || _saving) ? null : _book,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: _gGreen,
                    foregroundColor: Colors.black87,
                    padding: const EdgeInsets.symmetric(vertical: 14),
                    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                  ),
                  child: _saving
                      ? const SizedBox(height: 18, width: 18, child: CircularProgressIndicator(strokeWidth: 2))
                      : const Text('Book', style: TextStyle(fontWeight: FontWeight.bold, fontSize: 15)),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}