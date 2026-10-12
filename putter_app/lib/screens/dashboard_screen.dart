import 'dart:async';
import 'package:flutter/material.dart';
import '../services/api_service.dart';
import 'login_screen.dart';

const Color _blue = Color(0xFF0D47A1);
const Color _green = Color(0xFF16A34A);
const Color _orange = Color(0xFFB45309);

Map<String, dynamic> _m(dynamic v) => v is Map ? Map<String, dynamic>.from(v) : <String, dynamic>{};

List<Map<String, dynamic>> _list(dynamic v) =>
    v is List ? v.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList() : <Map<String, dynamic>>[];

int _i(dynamic v) => (v as num?)?.toInt() ?? 0;

String _hm(int secs) {
  final h = secs ~/ 3600;
  final m = (secs % 3600) ~/ 60;
  return '${h}h ${m.toString().padLeft(2, '0')}m';
}

String _clock(DateTime d) {
  final h = d.hour % 12 == 0 ? 12 : d.hour % 12;
  final mm = d.minute.toString().padLeft(2, '0');
  return '$h:$mm ${d.hour >= 12 ? 'PM' : 'AM'}';
}

String _binLabel(dynamic bin, dynamic id) {
  if (bin is Map) {
    for (final k in ['code', 'bin_code', 'name', 'label']) {
      final v = bin[k];
      if (v != null && '$v'.trim().isNotEmpty) return '$v';
    }
  }
  return id == null ? '-' : 'Bin #$id';
}

class DashboardScreen extends StatefulWidget {
  const DashboardScreen({super.key});

  @override
  State<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends State<DashboardScreen> {
  Map<String, dynamic> _d = {};
  bool _loading = true;
  bool _busy = false;
  String? _error;
  String _tab = 'pending';
  Timer? _poll;

  @override
  void initState() {
    super.initState();
    _load();
    _poll = Timer.periodic(const Duration(seconds: 30), (_) => _load(silent: true));
  }

  @override
  void dispose() {
    _poll?.cancel();
    super.dispose();
  }

  void _snack(String m) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(m)));
  }

  Future<void> _forceLogout() async {
    await ApiService.logout();
    if (!mounted) return;
    Navigator.pushAndRemoveUntil(
      context,
      MaterialPageRoute(builder: (_) => const LoginScreen()),
      (route) => false,
    );
  }

  Future<void> _confirmLogout() async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Logout?'),
        content: const Text('You will need to log in again with an OTP.'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Cancel')),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Logout', style: TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );
    if (ok == true) await _forceLogout();
  }

  Future<void> _load({bool silent = false}) async {
    try {
      final r = await ApiService.getDashboard();
      if (!mounted) return;
      final code = r['_http'];
      if (code == 401 || code == 403) {
        await _forceLogout();
        return;
      }
      setState(() {
        if (r['error'] != null) {
          if (!silent || _d.isEmpty) _error = r['error'].toString();
        } else {
          _d = r;
          _error = null;
        }
        _loading = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        if (_d.isEmpty) _error = 'Network error. Pull down to retry.';
        _loading = false;
      });
    }
  }

  Future<void> _toggleShift(bool active) async {
    if (_busy) return;
    if (active) {
      final running = _i(_m(_d['counts'])['in_progress']);
      final ok = await showDialog<bool>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('End shift?'),
          content: Text(running > 0
              ? 'You still have $running task(s) in progress. End your shift anyway?'
              : 'Your shift time will stop counting.'),
          actions: [
            TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Cancel')),
            TextButton(
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('End shift', style: TextStyle(color: Colors.red)),
            ),
          ],
        ),
      );
      if (ok != true) return;
    }
    setState(() => _busy = true);
    try {
      final r = active ? await ApiService.endShift() : await ApiService.startShift();
      if (r['error'] != null) _snack(r['error'].toString());
      await _load(silent: true);
    } catch (_) {
      _snack('Network error. Please try again.');
    }
    if (mounted) setState(() => _busy = false);
  }

  Widget _chip(String text, {bool dark = true}) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: dark ? Colors.black : const Color(0xFFEAECF0),
        borderRadius: BorderRadius.circular(20),
      ),
      child: Text(
        text,
        style: TextStyle(
          fontSize: 12,
          fontWeight: FontWeight.w700,
          color: dark ? Colors.white : Colors.black87,
        ),
      ),
    );
  }

  Widget _header(Map<String, dynamic> staff, Map<String, dynamic> store) {
    final name = (staff['name'] ?? '').toString();
    final initial = name.trim().isEmpty ? '?' : name.trim()[0].toUpperCase();
    final storeName = (store['name'] ?? '').toString();
    return Row(
      children: [
        CircleAvatar(
          radius: 24,
          backgroundColor: const Color(0xFFE3F2FD),
          child: Text(initial, style: const TextStyle(fontSize: 22, fontWeight: FontWeight.w800, color: _blue)),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('Hi, $name', style: const TextStyle(fontSize: 20, fontWeight: FontWeight.w800)),
              const SizedBox(height: 6),
              Wrap(
                spacing: 8,
                runSpacing: 6,
                children: [
                  _chip((staff['employee_id'] ?? '-').toString()),
                  if (storeName.isNotEmpty) _chip(storeName, dark: false),
                ],
              ),
            ],
          ),
        ),
        IconButton(onPressed: _confirmLogout, icon: const Icon(Icons.logout)),
      ],
    );
  }

  Widget _shiftCard(Map<String, dynamic> shift) {
    final active = shift['active'] == true;
    final started = DateTime.tryParse('${shift['started_at'] ?? ''}')?.toLocal();
    return Container(
      margin: const EdgeInsets.only(top: 18),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: const Color(0xFFD0D5DD)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.circle, size: 10, color: active ? _green : Colors.grey),
              const SizedBox(width: 8),
              Text(active ? 'On shift' : 'Off shift', style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w800)),
              const Spacer(),
              Text('Today: ${_hm(_i(shift['today_seconds']))}', style: TextStyle(fontSize: 12, color: Colors.grey.shade600)),
            ],
          ),
          if (active && started != null) ...[
            const SizedBox(height: 10),
            _ShiftClock(start: started),
            const SizedBox(height: 2),
            Text('Started at ${_clock(started)}', style: TextStyle(fontSize: 12, color: Colors.grey.shade600)),
          ],
          const SizedBox(height: 14),
          SizedBox(
            width: double.infinity,
            child: ElevatedButton(
              onPressed: _busy ? null : () => _toggleShift(active),
              style: ElevatedButton.styleFrom(
                backgroundColor: active ? Colors.red : Colors.black,
                foregroundColor: Colors.white,
                padding: const EdgeInsets.symmetric(vertical: 14),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
              ),
              child: _busy
                  ? const SizedBox(
                      height: 20,
                      width: 20,
                      child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                    )
                  : Text(active ? 'End shift' : 'Start shift', style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w700)),
            ),
          ),
        ],
      ),
    );
  }

  Widget _stat(String label, int value, String tabKey, Color color) {
    final on = _tab == tabKey;
    return Expanded(
      child: InkWell(
        borderRadius: BorderRadius.circular(14),
        onTap: () => setState(() => _tab = tabKey),
        child: Container(
          padding: const EdgeInsets.symmetric(vertical: 14),
          decoration: BoxDecoration(
            color: Colors.white,
            borderRadius: BorderRadius.circular(14),
            border: Border.all(color: on ? Colors.black : const Color(0xFFD0D5DD), width: on ? 2 : 1),
          ),
          child: Column(
            children: [
              Text('$value', style: TextStyle(fontSize: 22, fontWeight: FontWeight.w800, color: color)),
              const SizedBox(height: 2),
              Text(label, style: TextStyle(fontSize: 12, color: Colors.grey.shade600)),
            ],
          ),
        ),
      ),
    );
  }

  Widget _taskTile(Map<String, dynamic> t) {
    final product = _m(t['product']);
    final name = (product['name'] ?? 'Product #${t['product_id'] ?? ''}').toString();
    final status = (t['status'] ?? '').toString();
    final done = status == 'completed';
    final bin = done ? _binLabel(t['actual_bin'], t['actual_bin_id']) : _binLabel(t['suggested_bin'], t['suggested_bin_id']);
    final raw = done ? t['completed_at'] : (t['started_at'] ?? t['assigned_at'] ?? t['created_at']);
    final when = DateTime.tryParse('$raw')?.toLocal();
    final color = done ? _green : (status == 'in_progress' ? _blue : _orange);
    final label = done ? 'Completed' : (status == 'in_progress' ? 'In progress' : 'Pending');
    return Container(
      margin: const EdgeInsets.only(bottom: 10),
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: const Color(0xFFD0D5DD)),
      ),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(name, style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w800)),
                const SizedBox(height: 4),
                Text(
                  'Qty ${_i(t['quantity'])}  |  ${done ? 'Put in' : 'Suggested'}: $bin',
                  style: TextStyle(fontSize: 12, color: Colors.grey.shade700),
                ),
                if (when != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 2),
                    child: Text(_clock(when), style: TextStyle(fontSize: 12, color: Colors.grey.shade500)),
                  ),
                if (t['wrong_location'] == true)
                  const Padding(
                    padding: EdgeInsets.only(top: 2),
                    child: Text('Different bin used', style: TextStyle(fontSize: 12, color: _orange)),
                  ),
              ],
            ),
          ),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
            decoration: BoxDecoration(
              color: color.withAlpha(30),
              borderRadius: BorderRadius.circular(20),
            ),
            child: Text(label, style: TextStyle(fontSize: 11, fontWeight: FontWeight.w700, color: color)),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final staff = _m(_d['staff']);
    final store = _m(_d['store']);
    final shift = _m(_d['shift']);
    final counts = _m(_d['counts']);
    final active = shift['active'] == true;
    final tasks = _list(_d[_tab]);
    final emptyText = _tab == 'pending'
        ? 'No pending tasks assigned to you'
        : (_tab == 'in_progress' ? 'No tasks in progress' : 'No tasks completed today');

    Widget body;
    if (_loading) {
      body = const Center(child: CircularProgressIndicator());
    } else if (_error != null && _d.isEmpty) {
      body = ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.all(24),
        children: [
          const SizedBox(height: 80),
          Center(child: Text(_error!, style: TextStyle(color: Colors.grey.shade700))),
          const SizedBox(height: 16),
          Center(child: OutlinedButton(onPressed: () => _load(), child: const Text('Retry'))),
        ],
      );
    } else {
      body = ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(20, 16, 20, 24),
        children: [
          _header(staff, store),
          _shiftCard(shift),
          if (!active)
            Padding(
              padding: const EdgeInsets.only(top: 10),
              child: Text('Start your shift to begin putaway tasks.', style: TextStyle(fontSize: 12, color: Colors.grey.shade600)),
            ),
          const SizedBox(height: 18),
          Row(
            children: [
              _stat('Pending', _i(counts['pending']), 'pending', _orange),
              const SizedBox(width: 10),
              _stat('In progress', _i(counts['in_progress']), 'in_progress', _blue),
              const SizedBox(width: 10),
              _stat('Completed', _i(counts['completed_today']), 'completed', _green),
            ],
          ),
          const SizedBox(height: 18),
          if (tasks.isEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 28),
              child: Center(child: Text(emptyText, style: TextStyle(color: Colors.grey.shade600))),
            ),
          for (final t in tasks) _taskTile(t),
        ],
      );
    }

    return Scaffold(
      backgroundColor: const Color(0xFFF7F8FA),
      body: SafeArea(child: RefreshIndicator(onRefresh: () => _load(), child: body)),
    );
  }
}

class _ShiftClock extends StatefulWidget {
  final DateTime start;
  const _ShiftClock({required this.start});

  @override
  State<_ShiftClock> createState() => _ShiftClockState();
}

class _ShiftClockState extends State<_ShiftClock> {
  Timer? _t;

  @override
  void initState() {
    super.initState();
    _t = Timer.periodic(const Duration(seconds: 1), (_) {
      if (mounted) setState(() {});
    });
  }

  @override
  void dispose() {
    _t?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final s = DateTime.now().difference(widget.start).inSeconds;
    final secs = s < 0 ? 0 : s;
    String two(int n) => n.toString().padLeft(2, '0');
    return Text(
      '${two(secs ~/ 3600)}:${two((secs % 3600) ~/ 60)}:${two(secs % 60)}',
      style: const TextStyle(fontSize: 30, fontWeight: FontWeight.w800),
    );
  }
}