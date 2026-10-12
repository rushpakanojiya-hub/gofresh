// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'dart:async';
import 'package:flutter/material.dart';
import 'package:lottie/lottie.dart';
import '../services/api_service.dart';
import '../theme.dart';
import 'login_screen.dart';

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
  final VoidCallback onOpenSlots;
  const DashboardScreen({super.key, required this.onOpenSlots});

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
      final running = asInt(asMap(_d['counts'])['in_progress']);
      final ok = await showDialog<bool>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: Text('End shift?'),
          content: Text(running > 0
              ? 'You still have $running task(s) in progress. End your shift anyway?'
              : 'Your shift time will stop counting.'),
          actions: [
            TextButton(onPressed: () => Navigator.pop(ctx, false), child: Text('Cancel')),
            TextButton(
              onPressed: () => Navigator.pop(ctx, true),
              child: Text('End shift', style: TextStyle(color: Colors.red)),
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

  String _greeting() {
    final h = DateTime.now().hour;
    if (h < 12) return 'Good morning';
    if (h < 17) return 'Good afternoon';
    return 'Good evening';
  }

  Widget _hdrChip(IconData? icon, String text) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: 10, vertical: 5),
      decoration: BoxDecoration(color: Colors.white.withAlpha(36), borderRadius: BorderRadius.circular(20)),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (icon != null) ...[Icon(icon, size: 13, color: Colors.white), SizedBox(width: 5)],
          Text(text, style: TextStyle(color: Colors.white, fontSize: 12, fontWeight: FontWeight.w700)),
        ],
      ),
    );
  }

  Widget _header(Map<String, dynamic> staff, Map<String, dynamic> store, bool active) {
    final name = (staff['name'] ?? '').toString();
    final initial = name.trim().isEmpty ? '?' : name.trim()[0].toUpperCase();
    final storeName = (store['name'] ?? '').toString();
    return GradientHeader(
      child: Row(
        children: [
          CircleAvatar(
            radius: 27,
            backgroundColor: Colors.white,
            child: Text(initial, style: TextStyle(fontSize: 24, fontWeight: FontWeight.w800, color: AppColors.blue)),
          ),
          SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(_greeting(), style: TextStyle(color: Colors.white70, fontSize: 13, fontWeight: FontWeight.w600)),
                Text(name, style: TextStyle(color: Colors.white, fontSize: 22, fontWeight: FontWeight.w800)),
                SizedBox(height: 8),
                Wrap(
                  spacing: 8,
                  runSpacing: 6,
                  children: [
                    _hdrChip(Icons.badge_outlined, (staff['employee_id'] ?? '-').toString()),
                    if (storeName.isNotEmpty) _hdrChip(Icons.storefront_outlined, storeName),
                  ],
                ),
              ],
            ),
          ),
          Container(
            padding: EdgeInsets.symmetric(horizontal: 10, vertical: 6),
            decoration: BoxDecoration(color: Colors.white.withAlpha(36), borderRadius: BorderRadius.circular(20)),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.circle, size: 8, color: active ? Color(0xFF4ADE80) : Colors.white54),
                SizedBox(width: 6),
                Text(active ? 'On shift' : 'Off shift',
                    style: TextStyle(color: Colors.white, fontSize: 12, fontWeight: FontWeight.w700)),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _shiftCard(Map<String, dynamic> shift, Map<String, dynamic> booking) {
    final active = shift['active'] == true;
    final started = DateTime.tryParse('${shift['started_at'] ?? ''}')?.toLocal();
    final canStart = shift['can_start'] == true;
    final hint = (shift['hint'] ?? '').toString();
    final slotOver = shift['slot_over'] == true;
    final hasBooking = booking.isNotEmpty;

    final Widget button;
    if (active) {
      button = PrimaryButton(
        label: 'End shift',
        icon: Icons.stop_circle_outlined,
        color: AppColors.red,
        busy: _busy,
        onPressed: () => _toggleShift(true),
      );
    } else if (!hasBooking) {
      button = PrimaryButton(
        label: 'Book a slot',
        icon: Icons.event_available_outlined,
        color: AppColors.blue,
        onPressed: widget.onOpenSlots,
      );
    } else {
      button = PrimaryButton(
        label: 'Start shift',
        icon: Icons.play_circle_outline,
        busy: _busy,
        onPressed: canStart ? () => _toggleShift(false) : null,
      );
    }

    return Container(
      padding: EdgeInsets.all(18),
      decoration: cardDecoration(radius: 20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 10,
                height: 10,
                decoration: BoxDecoration(color: active ? AppColors.green : Color(0xFF98A2B3), shape: BoxShape.circle),
              ),
              SizedBox(width: 8),
              Text(active ? 'On shift' : 'Off shift',
                  style: TextStyle(fontSize: 16, fontWeight: FontWeight.w800, color: AppColors.ink)),
              Spacer(),
              Text('Today: ${hm(asInt(shift['today_seconds']))}', style: TextStyle(fontSize: 12, color: AppColors.muted)),
            ],
          ),
          if (active && started != null) ...[
            SizedBox(height: 12),
            _ShiftClock(start: started),
            SizedBox(height: 2),
            Text('Started at ${clock12(started)}', style: TextStyle(fontSize: 12, color: AppColors.muted)),
          ],
          if (hasBooking) ...[
            SizedBox(height: 14),
            Container(
              width: double.infinity,
              padding: EdgeInsets.symmetric(horizontal: 12, vertical: 10),
              decoration: BoxDecoration(color: Color(0xFFEFF5FF), borderRadius: BorderRadius.circular(12)),
              child: Row(
                children: [
                  Icon(Icons.event_available_outlined, size: 20, color: AppColors.blue),
                  SizedBox(width: 10),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text("Today's slot", style: TextStyle(fontSize: 11, color: AppColors.muted)),
                        Text(
                          '${time12((booking['start_time'] ?? '').toString())} - ${time12((booking['end_time'] ?? '').toString())}',
                          style: TextStyle(fontSize: 14, fontWeight: FontWeight.w800, color: AppColors.blue),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ],
          SizedBox(height: 16),
          button,
          if (!active && hint.isNotEmpty)
            Padding(
              padding: EdgeInsets.only(top: 10),
              child: Text(hint, style: TextStyle(fontSize: 12, color: AppColors.muted)),
            ),
          if (active && slotOver)
            Padding(
              padding: EdgeInsets.only(top: 10),
              child: Text('Your slot has ended. Please end your shift.',
                  style: TextStyle(fontSize: 12, color: AppColors.orange)),
            ),
        ],
      ),
    );
  }

  Widget _stat(String label, int value, String key, Color color, IconData icon) {
    final on = _tab == key;
    return Expanded(
      child: InkWell(
        borderRadius: BorderRadius.circular(16),
        onTap: () => setState(() => _tab = key),
        child: Container(
          padding: EdgeInsets.symmetric(vertical: 14, horizontal: 8),
          decoration: BoxDecoration(
            color: on ? color.withAlpha(22) : Colors.white,
            borderRadius: BorderRadius.circular(16),
            border: Border.all(color: on ? color : AppColors.line, width: on ? 1.6 : 1),
          ),
          child: Column(
            children: [
              Icon(icon, size: 20, color: color),
              SizedBox(height: 6),
              Text('$value', style: TextStyle(fontSize: 22, fontWeight: FontWeight.w800, color: color)),
              SizedBox(height: 2),
              Text(label, style: TextStyle(fontSize: 11, color: AppColors.muted, fontWeight: FontWeight.w600)),
            ],
          ),
        ),
      ),
    );
  }

  Widget _taskTile(Map<String, dynamic> t) {
    final product = asMap(t['product']);
    final name = (product['name'] ?? 'Product #${t['product_id'] ?? ''}').toString();
    String img = (product['image_url'] ?? '').toString();
    if (img.isNotEmpty && !img.startsWith('http')) {
      img = '$kOrigin${img.startsWith('/') ? '' : '/'}$img';
    }
    final status = (t['status'] ?? '').toString();
    final done = status == 'completed';
    final bin = done ? _binLabel(t['actual_bin'], t['actual_bin_id']) : _binLabel(t['suggested_bin'], t['suggested_bin_id']);
    final raw = done ? t['completed_at'] : (t['started_at'] ?? t['assigned_at'] ?? t['created_at']);
    final when = DateTime.tryParse('$raw')?.toLocal();
    final color = done ? AppColors.green : (status == 'in_progress' ? AppColors.blue : AppColors.orange);
    final label = done ? 'Completed' : (status == 'in_progress' ? 'In progress' : 'Pending');

    Widget box(IconData ic) => Container(
          width: 54,
          height: 54,
          color: Color(0xFFF2F4F7),
          child: Icon(ic, size: 22, color: Colors.grey),
        );

    return Container(
      margin: EdgeInsets.only(bottom: 10),
      padding: EdgeInsets.all(12),
      decoration: cardDecoration(),
      child: Row(
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(12),
            child: img.isEmpty
                ? box(Icons.inventory_2_outlined)
                : Image.network(
                    img,
                    width: 54,
                    height: 54,
                    fit: BoxFit.cover,
                    errorBuilder: (c, e, s) => box(Icons.broken_image_outlined),
                  ),
          ),
          SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(name, maxLines: 2, overflow: TextOverflow.ellipsis,
                    style: TextStyle(fontSize: 14, fontWeight: FontWeight.w800, color: AppColors.ink)),
                SizedBox(height: 6),
                Wrap(
                  spacing: 6,
                  runSpacing: 4,
                  children: [
                    Container(
                      padding: EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                      decoration: BoxDecoration(color: Color(0xFFF2F4F7), borderRadius: BorderRadius.circular(8)),
                      child: Text('Qty ${asInt(t['quantity'])}',
                          style: TextStyle(fontSize: 11, fontWeight: FontWeight.w700, color: AppColors.ink)),
                    ),
                    Container(
                      padding: EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                      decoration: BoxDecoration(color: Color(0xFFEFF5FF), borderRadius: BorderRadius.circular(8)),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(Icons.place_outlined, size: 12, color: AppColors.blue),
                          SizedBox(width: 3),
                          Text(bin, style: TextStyle(fontSize: 11, fontWeight: FontWeight.w700, color: AppColors.blue)),
                        ],
                      ),
                    ),
                  ],
                ),
                if (when != null)
                  Padding(
                    padding: EdgeInsets.only(top: 4),
                    child: Text(clock12(when), style: TextStyle(fontSize: 11, color: AppColors.muted)),
                  ),
                if (t['wrong_location'] == true)
                  Padding(
                    padding: EdgeInsets.only(top: 2),
                    child: Text('Different bin used', style: TextStyle(fontSize: 11, color: AppColors.orange)),
                  ),
              ],
            ),
          ),
          SizedBox(width: 8),
          StatusPill(label, color),
        ],
      ),
    );
  }

  Widget _empty(String text) {
    return Padding(
      padding: EdgeInsets.symmetric(vertical: 8),
      child: Column(
        children: [
          Lottie.asset(
            'assets/lottie/search.json',
            width: 170,
            height: 170,
            errorBuilder: (c, e, s) => SizedBox(height: 40),
          ),
          Text(text, style: TextStyle(color: AppColors.muted, fontWeight: FontWeight.w600)),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final staff = asMap(_d['staff']);
    final store = asMap(_d['store']);
    final shift = asMap(_d['shift']);
    final counts = asMap(_d['counts']);
    final booking = asMap(_d['today_booking']);
    final active = shift['active'] == true;
    final tasks = asList(_d[_tab]);
    final title = _tab == 'pending' ? 'Pending tasks' : (_tab == 'in_progress' ? 'In progress' : 'Completed today');
    final emptyText = _tab == 'pending'
        ? 'No pending tasks assigned to you'
        : (_tab == 'in_progress' ? 'No tasks in progress' : 'No tasks completed today');

    Widget body;
    if (_loading) {
      body = Center(child: CircularProgressIndicator());
    } else if (_error != null && _d.isEmpty) {
      body = ListView(
        physics: AlwaysScrollableScrollPhysics(),
        padding: EdgeInsets.fromLTRB(24, 140, 24, 24),
        children: [
          Center(child: Text(_error!, style: TextStyle(color: AppColors.muted))),
          SizedBox(height: 16),
          Center(child: OutlinedButton(onPressed: () => _load(), child: Text('Retry'))),
        ],
      );
    } else {
      body = ListView(
        physics: AlwaysScrollableScrollPhysics(),
        padding: EdgeInsets.zero,
        children: [
          _header(staff, store, active),
          Padding(
            padding: EdgeInsets.fromLTRB(16, 16, 16, 24),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _shiftCard(shift, booking),
                SizedBox(height: 18),
                Row(
                  children: [
                    _stat('Pending', asInt(counts['pending']), 'pending', AppColors.orange, Icons.hourglass_top_rounded),
                    SizedBox(width: 10),
                    _stat('In progress', asInt(counts['in_progress']), 'in_progress', AppColors.blue, Icons.sync_rounded),
                    SizedBox(width: 10),
                    _stat('Completed', asInt(counts['completed_today']), 'completed', AppColors.green, Icons.task_alt_rounded),
                  ],
                ),
                SizedBox(height: 22),
                Row(
                  children: [
                    Text(title, style: TextStyle(fontSize: 16, fontWeight: FontWeight.w800, color: AppColors.ink)),
                    Spacer(),
                    Text('${tasks.length}', style: TextStyle(fontSize: 13, color: AppColors.muted, fontWeight: FontWeight.w700)),
                  ],
                ),
                SizedBox(height: 12),
                if (tasks.isEmpty) _empty(emptyText),
                for (final t in tasks) _taskTile(t),
              ],
            ),
          ),
        ],
      );
    }
    return RefreshIndicator(onRefresh: () => _load(), child: body);
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
    return Text(
      '${two(secs ~/ 3600)}:${two((secs % 3600) ~/ 60)}:${two(secs % 60)}',
      style: TextStyle(fontSize: 32, fontWeight: FontWeight.w800, color: AppColors.ink),
    );
  }
}