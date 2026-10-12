// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import '../services/api_service.dart';
import '../theme.dart';
import 'login_screen.dart';
import 'putaway_home_screen.dart';
import 'movement_screen.dart';

class DashboardScreen extends StatefulWidget {
  final VoidCallback onOpenSlots;
  final void Function(String key) onOpenTasks;
  const DashboardScreen({super.key, required this.onOpenSlots, required this.onOpenTasks});

  @override
  State<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends State<DashboardScreen> {
  Map<String, dynamic> _d = {};
  bool _loading = true;
  bool _busy = false;
  String? _error;
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
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(m)));
  }

  Future<void> _forceLogout() async {
    await ApiService.logout();
    if (!mounted) return;
    Navigator.pushAndRemoveUntil(context, MaterialPageRoute(builder: (_) => const LoginScreen()), (route) => false);
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

  Future<void> _openMovement() async {
    final on = asMap(_d['shift'])['active'] == true;
    await Navigator.push(context, MaterialPageRoute(builder: (_) => MovementScreen(shiftActive: on)));
    if (mounted) _load(silent: true);
  }

  Future<void> _openPutaway() async {
    await Navigator.push(context, MaterialPageRoute(builder: (_) => PutawayHomeScreen()));
    if (mounted) _load(silent: true);
  }

  Widget _topBar(Map<String, dynamic> store, bool active) {
    final storeName = (store['name'] ?? '').toString();
    return Row(
      children: [
        Icon(Icons.location_on_outlined, size: 28, color: AppColors.ink),
        SizedBox(width: 10),
        Expanded(
          child: Text(storeName.isEmpty ? 'GoFresh' : storeName,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(fontSize: 20, fontWeight: FontWeight.w800, color: AppColors.ink)),
        ),
        Container(
          padding: EdgeInsets.symmetric(horizontal: 12, vertical: 6),
          decoration: BoxDecoration(
            color: active ? Color(0xFFE8F9EE) : Color(0xFFF2F4F7),
            borderRadius: BorderRadius.circular(20),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.circle, size: 8, color: active ? AppColors.green : Color(0xFF98A2B3)),
              SizedBox(width: 6),
              Text(active ? 'On shift' : 'Off shift',
                  style: TextStyle(fontSize: 12, fontWeight: FontWeight.w700, color: active ? AppColors.green : AppColors.muted)),
            ],
          ),
        ),
      ],
    );
  }

  Widget _welcome(String name, bool active) {
    return Row(
      children: [
        Container(
          width: 16,
          height: 16,
          decoration: BoxDecoration(color: active ? AppColors.green : Color(0xFF98A2B3), shape: BoxShape.circle),
        ),
        SizedBox(width: 12),
        Expanded(
          child: Text('Welcome $name!',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(fontSize: 26, fontWeight: FontWeight.w800, color: AppColors.ink)),
        ),
      ],
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
              Text('My shift', style: TextStyle(fontSize: 16, fontWeight: FontWeight.w800, color: AppColors.ink)),
              Spacer(),
              Text('Today: ${hm(asInt(shift['today_seconds']))}', style: TextStyle(fontSize: 12, color: AppColors.muted)),
            ],
          ),
          if (active && started != null) ...[
            SizedBox(height: 10),
            _ShiftClock(start: started),
            SizedBox(height: 2),
            Text('Started at ${clock12(started)}', style: TextStyle(fontSize: 12, color: AppColors.muted)),
          ],
          if (hasBooking) ...[
            SizedBox(height: 12),
            InkWell(
              borderRadius: BorderRadius.circular(12),
              onTap: widget.onOpenSlots,
              child: Container(
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
                    Icon(Icons.chevron_right, color: AppColors.muted),
                  ],
                ),
              ),
            ),
          ],
          SizedBox(height: 14),
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

  Widget _art(IconData main, IconData small, bool enabled) {
    final c = enabled ? AppColors.blue : Color(0xFF98A2B3);
    return Container(
      height: 96,
      width: double.infinity,
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(18),
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: enabled ? [Color(0xFFE6EDFF), Color(0xFFF6F8FF)] : [Color(0xFFF2F4F7), Color(0xFFFAFAFB)],
        ),
      ),
      child: Stack(
        alignment: Alignment.center,
        children: [
          Icon(main, size: 58, color: c),
          Positioned(right: 18, bottom: 10, child: Icon(small, size: 26, color: enabled ? Color(0xFFD6A25C) : Color(0xFFB0B7C3))),
        ],
      ),
    );
  }

  Widget _tile(String title, String desc, IconData main, IconData small, {required bool enabled, int badge = 0, VoidCallback? onTap}) {
    return InkWell(
      borderRadius: BorderRadius.circular(22),
      onTap: onTap ?? () => _snack('$title is not available for the putter role yet'),
      child: Container(
        padding: EdgeInsets.all(14),
        decoration: cardDecoration(radius: 22),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  child: Text(title,
                      style: TextStyle(fontSize: 17, fontWeight: FontWeight.w800, color: enabled ? AppColors.ink : AppColors.muted)),
                ),
                if (badge > 0)
                  Container(
                    padding: EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                    decoration: BoxDecoration(color: AppColors.orange, borderRadius: BorderRadius.circular(12)),
                    child: Text('$badge', style: TextStyle(color: Colors.white, fontSize: 11, fontWeight: FontWeight.w800)),
                  ),
                if (!enabled) StatusPill('Soon', AppColors.muted),
              ],
            ),
            SizedBox(height: 6),
            Text(desc, style: TextStyle(fontSize: 12, color: AppColors.muted, height: 1.3)),
            Spacer(),
            _art(main, small, enabled),
          ],
        ),
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
    final name = (staff['name'] ?? '').toString();
    final top = MediaQuery.of(context).padding.top;

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
        padding: EdgeInsets.fromLTRB(18, top + 16, 18, 24),
        children: [
          _topBar(store, active),
          SizedBox(height: 22),
          _welcome(name, active),
          SizedBox(height: 18),
          _shiftCard(shift, booking),
          SizedBox(height: 10),
          GridView.count(
            shrinkWrap: true,
            physics: NeverScrollableScrollPhysics(),
            crossAxisCount: 2,
            mainAxisSpacing: 10,
            crossAxisSpacing: 10,
            padding: EdgeInsets.zero,
            childAspectRatio: 1.2,
            children: [
              _tile('Put-Away', 'Use this for put-away bins and items', Icons.inventory_2_rounded, Icons.move_to_inbox_rounded,
                  enabled: true, badge: asInt(counts['pending']), onTap: _openPutaway),
              _tile('Consolidation', 'Consolidate order items at one place', Icons.layers_rounded, Icons.all_inbox_rounded, enabled: false),
              _tile('Inventory Movement', 'Move SKUs from one location or bin to another', Icons.alt_route_rounded, Icons.pin_drop_rounded, enabled: true, onTap: _openMovement),
            ],
          ),
        ],
      );
    }
    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: SystemUiOverlayStyle(statusBarColor: Colors.transparent, statusBarIconBrightness: Brightness.dark),
      child: Container(color: Colors.white, child: RefreshIndicator(onRefresh: () => _load(), child: body)),
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
    return Text(
      '${two(secs ~/ 3600)}:${two((secs % 3600) ~/ 60)}:${two(secs % 60)}',
      style: TextStyle(fontSize: 32, fontWeight: FontWeight.w800, color: AppColors.ink),
    );
  }
}