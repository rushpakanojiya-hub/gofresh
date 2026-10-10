// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'dart:async';
import 'package:flutter/material.dart';
import 'today_orders.dart';
import '../services/api_service.dart';
import '../utils/fmt.dart';
import 'dates_screen.dart';
import 'login_screen.dart';
import 'picking_screen.dart';
import 'summary_screen.dart';
import 'payout_screen.dart';
import 'putter_flow.dart';

class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key});

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  static const Color _blueDark = Color(0xFF0D47A1);
  static const Color _blueLight = Color(0xFF1976D2);
  static const Color _yellow = Color(0xFFFFC107);
  static const Color _ink = Color(0xFF1B2A1F);
  static const Color _green = Color(0xFF16A34A);

  int _tab = 0;
  String _name = 'Picker';
  String _sub = 'GoFresh Picker';
  List<Map<String, dynamic>> _bookings = [];
  bool _online = false;
  bool _busy = false;
  String _workflow = '';
  String _presenceStore = '';
  Map<String, dynamic> _sum = {};

  Map<String, dynamic>? _task;
  int _shownTaskId = 0;
  bool _dialogOpen = false;
  bool _pickingOpen = false;
  bool _polling = false;
  int _tick = 0;
  Timer? _timer;
  int _activeBase = 0;
  int _loginBase = 0;
  DateTime _statsAt = DateTime.now();

  @override
  void initState() {
    super.initState();
    _loadStaff();
    _loadProfileSummary();
    _refresh().then((_) => _pollTask());
    _timer = Timer.periodic(const Duration(seconds: 5), (_) {
      _tick++;
      _pollTask();
      if (_tick % 6 == 0) _refresh();
    });
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  Future<void> _loadProfileSummary() async {
    try {
      final r = await ApiService.getPickerSummary();
      if (!mounted || r['error'] != null) return;
      setState(() => _sum = r);
    } catch (_) {}
  }

  String _roleLabel() {
    if (_workflow.isEmpty || _workflow == 'picker') return 'PICKER';
    if (_workflow == 'packer') return 'PUTTER';
    return _workflow.toUpperCase();
  }

  int _liveSecs(int base) => base + (_online ? DateTime.now().difference(_statsAt).inSeconds : 0);

  String _pHm(int s) {
    if (s < 0) s = 0;
    return '${(s ~/ 3600).toString().padLeft(2, '0')}h ${((s % 3600) ~/ 60).toString().padLeft(2, '0')}m';
  }

  String _pMoney(num v) => v == v.roundToDouble() ? v.toInt().toString() : v.toStringAsFixed(2);

  Future<void> _confirmLogout() async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        title: const Text('Logout?'),
        content: const Text('You will need to login again with OTP.'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Cancel')),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Logout', style: TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );
    if (ok == true) await _logout();
  }

  Widget _pStat(String value, String label, Color c) {
    return Expanded(
      child: Container(
        padding: const EdgeInsets.symmetric(vertical: 14),
        decoration: BoxDecoration(
          color: const Color(0xFFF5F7FA),
          borderRadius: BorderRadius.circular(14),
        ),
        child: Column(
          children: [
            Text(value, style: TextStyle(fontSize: 18, fontWeight: FontWeight.w800, color: c)),
            const SizedBox(height: 2),
            Text(label, style: TextStyle(fontSize: 11, color: Colors.grey.shade600)),
          ],
        ),
      ),
    );
  }
  Future<void> _loadStaff() async {
    final s = await ApiService.getSavedStaff();
    final n = s?['name']?.toString();
    final ph = s?['phone']?.toString();
    if (!mounted) return;
    setState(() {
      if (n != null && n.isNotEmpty) _name = n;
      if (ph != null && ph.isNotEmpty) _sub = ph;
    });
  }

  void _applyPresence(Map<String, dynamic> p) {
    _online = p['is_online'] == true;
    _workflow = (p['workflow'] ?? '').toString();
    _presenceStore = (p['warehouse_name'] ?? '').toString();
    _activeBase = (p['active_seconds'] as num?)?.toInt() ?? 0;
    _loginBase = (p['login_seconds'] as num?)?.toInt() ?? 0;
    _statsAt = DateTime.now();
    if (!_online) _task = null;
  }

  Future<void> _refresh() async {
    try {
      final b = await ApiService.getMyBookings();
      final p = await ApiService.getPresence();
      if (!mounted) return;
      final list = b['bookings'];
      setState(() {
        _bookings = list is List
            ? list.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList()
            : [];
        if (p['error'] == null) _applyPresence(p);
      });
    } catch (_) {}
  }

  int get _activeSecs => _activeBase + (_online ? DateTime.now().difference(_statsAt).inSeconds : 0);
  int get _loginSecs =>
      _loginBase + ((_online || _loginBase > 0) ? DateTime.now().difference(_statsAt).inSeconds : 0);

  Future<void> _pollTask() async {
    if (!_online || _pickingOpen || _polling) return;
    _polling = true;
    try {
      final r = await ApiService.getPickerTask();
      if (!mounted) return;
      if (r['error'] != null) return;
      final t = r['task'];
      final task = t is Map ? Map<String, dynamic>.from(t) : null;
      setState(() => _task = task);
      if (task != null) {
        final id = (task['task_id'] as num?)?.toInt() ?? 0;
        if (id != 0 && _shownTaskId != id) {
          _shownTaskId = id;
          _showTaskPopup(task);
        }
      }
    } catch (_) {
    } finally {
      _polling = false;
    }
  }

  void _showTaskPopup(Map<String, dynamic> task) {
    if (_dialogOpen || !mounted) return;
    _dialogOpen = true;
    showDialog<void>(
      context: context,
      builder: (_) => Dialog(
        backgroundColor: Colors.transparent,
        insetPadding: const EdgeInsets.all(20),
        child: _orderCard(task, popup: true),
      ),
    ).whenComplete(() => _dialogOpen = false);
  }

  Future<void> _openPicking(Map<String, dynamic> t) async {
    _pickingOpen = true;
    final done = await Navigator.push<bool>(
      context,
      MaterialPageRoute(
        builder: (_) => PickingScreen(
          orderId: (t['order_id'] as num).toInt(),
          allottedSeconds: (t['allotted_seconds'] as num?)?.toInt() ?? 120,
        ),
      ),
    );
    _pickingOpen = false;
    if (!mounted) return;
    setState(() => _task = null);
    await _refresh();
    if (done == true) _snack('Order handed over');
    _pollTask();
  }

  Map<String, dynamic>? get _todayBooking {
    final t = fmtDate(DateTime.now());
    for (final b in _bookings) {
      if (b['date'] == t) return b;
    }
    return null;
  }

  Future<void> _logout() async {
    await ApiService.logout();
    if (!mounted) return;
    Navigator.pushAndRemoveUntil(
      context,
      MaterialPageRoute(builder: (_) => const LoginScreen()),
      (route) => false,
    );
  }

  void _snack(String m) {
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(m)));
  }

  Future<void> _openStores() async {
    final ok = await Navigator.push<bool>(
      context,
      MaterialPageRoute(builder: (_) => const DatesScreen()),
    );
    if (ok == true) {
      await _refresh();
      if (mounted) _snack('Slot booked');
    }
  }

  Future<void> _onToggle() async {
    if (_busy) return;
    if (_online) {
      await _setPresence(false, _workflow);
      return;
    }
    if (_todayBooking == null) {
      _snack('Choose a store and book a slot first');
      return;
    }
    final wf = await showModalBottomSheet<String>(
      context: context,
      backgroundColor: Colors.white,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (_) => const _WorkflowSheet(),
    );
    if (wf == null) return;
    await _setPresence(true, wf);
    if (wf == 'packer' && mounted && _online) {
      Navigator.push(context, MaterialPageRoute(builder: (_) => const PutterScanScreen()));
    }
  }

  Future<void> _setPresence(bool online, String wf) async {
    setState(() => _busy = true);
    try {
      final r = await ApiService.setPresence(online, wf);
      if (!mounted) return;
      if (r['error'] != null) {
        _snack(r['error'].toString());
      } else {
        setState(() => _applyPresence(r));
        if (_online) _pollTask();
      }
    } catch (_) {
      if (mounted) _snack('Network error. Please try again.');
    }
    if (mounted) setState(() => _busy = false);
  }

  Future<void> _cancel(int id) async {
    try {
      final r = await ApiService.cancelBooking(id);
      if (!mounted) return;
      if (r['error'] != null) {
        _snack(r['error'].toString());
        return;
      }
      await _refresh();
    } catch (_) {
      if (mounted) _snack('Network error. Please try again.');
    }
  }

  Widget _onlineToggle() {
    final on = _online;
    return GestureDetector(
      onTap: _onToggle,
      child: Container(
        height: 34,
        padding: const EdgeInsets.symmetric(horizontal: 4),
        decoration: BoxDecoration(
          color: on ? _green : const Color(0xFF6B7A99),
          borderRadius: BorderRadius.circular(20),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (!on) const CircleAvatar(radius: 13, backgroundColor: Colors.white),
            SizedBox(width: on ? 10 : 8),
            Text(
              on ? 'ONLINE' : 'OFFLINE',
              style: const TextStyle(color: Colors.white, fontSize: 12, fontWeight: FontWeight.w700),
            ),
            SizedBox(width: on ? 8 : 10),
            if (on) const CircleAvatar(radius: 13, backgroundColor: Colors.white),
          ],
        ),
      ),
    );
  }

  Widget _header() {
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 14, 16, 12),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Hi, $_name',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w800),
                ),
                const SizedBox(height: 2),
                Text(_sub, style: TextStyle(fontSize: 12, color: Colors.grey.shade600)),
              ],
            ),
          ),
          _onlineToggle(),
          const SizedBox(width: 8),
          IconButton(
            onPressed: () => _snack('No notifications yet'),
            icon: const Icon(Icons.notifications_none),
          ),
        ],
      ),
    );
  }

  Widget _statsRow() {
    Widget cell(IconData icon, String value, String label, Color c) {
      return Expanded(
        child: Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(icon, color: c),
            const SizedBox(width: 8),
            Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(value, style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w800)),
                Text(label, style: TextStyle(fontSize: 11, color: Colors.grey.shade600)),
              ],
            ),
          ],
        ),
      );
    }

    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 0, 16, 12),
      child: Row(
        children: [
          cell(Icons.directions_run, fmtHm(_activeSecs), 'Active hours', _green),
          cell(Icons.access_time, fmtHm(_loginSecs), 'Login hours', _blueLight),
        ],
      ),
    );
  }

  Widget _orderCard(Map<String, dynamic> t, {bool popup = false}) {
    final qty = (t['quantity'] as num?)?.toInt() ?? 0;
    final orderId = (t['order_id'] as num?)?.toInt() ?? 0;
    final secs = (t['allotted_seconds'] as num?)?.toInt() ?? 120;
    final started = t['status'] == 'in_progress';
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(18),
      decoration: BoxDecoration(color: const Color(0xFFE6F7E9), borderRadius: BorderRadius.circular(20)),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              Container(
                padding: const EdgeInsets.all(10),
                decoration: BoxDecoration(color: Colors.white, borderRadius: BorderRadius.circular(12)),
                child: const Icon(Icons.shopping_basket_outlined),
              ),
              const SizedBox(width: 12),
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Text('Order Picking', style: TextStyle(fontSize: 18, fontWeight: FontWeight.w800)),
                  const SizedBox(height: 4),
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                    decoration: BoxDecoration(color: const Color(0xFFE53935), borderRadius: BorderRadius.circular(12)),
                    child: const Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.timer_outlined, size: 12, color: Colors.white),
                        SizedBox(width: 4),
                        Text('Urgent', style: TextStyle(color: Colors.white, fontSize: 11, fontWeight: FontWeight.w700)),
                      ],
                    ),
                  ),
                ],
              ),
            ],
          ),
          const SizedBox(height: 22),
          const Text('Quantity', style: TextStyle(fontSize: 14)),
          Text(qty.toString().padLeft(2, '0'), style: const TextStyle(fontSize: 56, fontWeight: FontWeight.w800)),
          const SizedBox(height: 18),
          Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('Order ID', style: TextStyle(fontSize: 12, color: Colors.grey.shade600)),
                    Text('$orderId', style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w800)),
                  ],
                ),
              ),
              _TimeBox((secs ~/ 60).toString().padLeft(2, '0'), 'MIN'),
              const SizedBox(width: 6),
              _TimeBox((secs % 60).toString().padLeft(2, '0'), 'SEC'),
            ],
          ),
          const SizedBox(height: 16),
          SizedBox(
            width: double.infinity,
            child: ElevatedButton(
              onPressed: () {
                if (popup) Navigator.of(context).pop();
                _openPicking(t);
              },
              style: ElevatedButton.styleFrom(
                backgroundColor: _ink,
                foregroundColor: Colors.white,
                padding: const EdgeInsets.symmetric(vertical: 16),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
              ),
              child: Text(
                started ? 'Continue picking' : 'Start picking',
                style: const TextStyle(fontSize: 17, fontWeight: FontWeight.w800),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _mainCard() {
    final tb = _todayBooking;
    final String title;
    final String msg;
    final String btn;
    final VoidCallback onTap;
    if (_online) {
      final store = _presenceStore.isNotEmpty ? _presenceStore : (tb?['warehouse_name'] ?? '').toString();
      title = 'YOU\u2019RE\nONLINE';
      msg = '$store\nWaiting for orders...';
      btn = 'Go offline';
      onTap = _onToggle;
    } else if (tb != null) {
      title = 'YOU\u2019RE\nALL SET';
      msg = '${tb['warehouse_name']}\n${fmtTime((tb['start_time'] ?? '').toString())} - ${fmtTime((tb['end_time'] ?? '').toString())}';
      btn = 'View my slots';
      onTap = () => setState(() => _tab = 1);
    } else {
      title = 'WELCOME\nTO GOFRESH';
      msg = 'Before you start, lets\nchoose a store for you!';
      btn = 'Choose a store';
      onTap = _openStores;
    }
    return Container(
      width: double.infinity,
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(20),
        gradient: const LinearGradient(
          begin: Alignment.topCenter,
          end: Alignment.bottomCenter,
          colors: [_blueLight, _blueDark],
        ),
      ),
      padding: const EdgeInsets.all(20),
      child: Column(
        children: [
          const Spacer(flex: 3),
          Text(
            title,
            textAlign: TextAlign.center,
            style: const TextStyle(
              color: Colors.white,
              fontSize: 40,
              height: 1.05,
              fontWeight: FontWeight.w900,
              shadows: [
                Shadow(color: _ink, offset: Offset(0, 3), blurRadius: 0),
                Shadow(color: _ink, offset: Offset(2, 0), blurRadius: 0),
                Shadow(color: _ink, offset: Offset(-2, 0), blurRadius: 0),
              ],
            ),
          ),
          const SizedBox(height: 18),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 22),
            decoration: BoxDecoration(
              color: _yellow,
              borderRadius: BorderRadius.circular(14),
              border: Border.all(color: _ink, width: 3),
            ),
            child: Text(
              msg,
              textAlign: TextAlign.center,
              style: const TextStyle(color: _ink, fontSize: 17, fontWeight: FontWeight.w800),
            ),
          ),
          const Spacer(flex: 4),
          SizedBox(
            width: double.infinity,
            child: ElevatedButton(
              onPressed: onTap,
              style: ElevatedButton.styleFrom(
                backgroundColor: Colors.white,
                foregroundColor: _ink,
                padding: const EdgeInsets.symmetric(vertical: 16),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
              ),
              child: Text(btn, style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w800)),
            ),
          ),
        ],
      ),
    );
  }

  Widget _taskTab() {
    final t = _task;
    final showStats = _online || _loginBase > 0;
    return Column(
      children: [
        _header(),
        if (showStats) _statsRow(),
        Expanded(
          child: (t != null && _online)
              ? Center(
                  child: SingleChildScrollView(
                    padding: const EdgeInsets.fromLTRB(16, 4, 16, 16),
                    child: _orderCard(t),
                  ),
                )
              : Padding(
                  padding: const EdgeInsets.fromLTRB(16, 4, 16, 16),
                  child: _online ? _waitingView() : _mainCard(),
                ),
        ),
      ],
    );
  }

  Widget _waitingView() {
    final wf = _workflow.isEmpty ? 'PICKER' : (_workflow == 'packer' ? 'PUTTER' : _workflow.toUpperCase());
    final tb = _todayBooking;
    final store = _presenceStore.isNotEmpty ? _presenceStore : (tb?['warehouse_name'] ?? '').toString();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Container(
          padding: const EdgeInsets.symmetric(vertical: 12, horizontal: 16),
          decoration: BoxDecoration(
            border: Border.all(color: Colors.black87),
            borderRadius: BorderRadius.circular(10),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Text(wf, style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w800)),
              const Icon(Icons.arrow_right),
            ],
          ),
        ),
        const SizedBox(height: 12),
        InkWell(
          onTap: () => Navigator.push(context, MaterialPageRoute(builder: (_) => const SummaryScreen())),
          child: Container(
            padding: const EdgeInsets.symmetric(vertical: 12, horizontal: 14),
            decoration: BoxDecoration(color: const Color(0xFFF2F4F7), borderRadius: BorderRadius.circular(8)),
            child: Row(
              children: [
                const Icon(Icons.history, size: 20, color: Colors.black54),
                const SizedBox(width: 10),
                const Expanded(
                  child: Text('Previous order details', style: TextStyle(fontSize: 14, fontWeight: FontWeight.w800)),
                ),
                Text('View details', style: TextStyle(fontSize: 12, color: Colors.grey.shade700)),
                const Icon(Icons.arrow_right, size: 20),
              ],
            ),
          ),
        ),
        const SizedBox(height: 12),
        const TodayOrders(),
        Expanded(
          child: Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Container(
                  width: 130,
                  height: 130,
                  decoration: BoxDecoration(color: const Color(0xFFFFF3D0), borderRadius: BorderRadius.circular(26)),
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      const Icon(Icons.shopping_basket_outlined, size: 46, color: _green),
                      const SizedBox(height: 6),
                      Text.rich(
                        const TextSpan(
                          children: [
                            TextSpan(text: 'go', style: TextStyle(color: _ink)),
                            TextSpan(text: 'fresh', style: TextStyle(color: _green)),
                          ],
                        ),
                        style: const TextStyle(fontSize: 24, fontWeight: FontWeight.w900),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 28),
                const Text('Searching for order ...', style: TextStyle(fontSize: 17, fontWeight: FontWeight.w800)),
                if (store.isNotEmpty) ...[
                  const SizedBox(height: 6),
                  Text(store, style: TextStyle(fontSize: 13, color: Colors.grey.shade600)),
                ],
              ],
            ),
          ),
        ),
        OutlinedButton(
          onPressed: () async {
            await _pollTask();
            if (mounted && _task == null) _snack('No order available right now');
          },
          style: OutlinedButton.styleFrom(
            foregroundColor: Colors.black,
            side: const BorderSide(color: Colors.black87),
            padding: const EdgeInsets.symmetric(vertical: 16),
            shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
          ),
          child: const Text('Fetch order', style: TextStyle(fontSize: 16, fontWeight: FontWeight.w800)),
        ),
      ],
    );
  }
  Widget _bookingTile(Map<String, dynamic> b) {
    final id = (b['id'] as num).toInt();
    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        border: Border.all(color: const Color(0xFFD6E6FF)),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  (b['warehouse_name'] ?? '').toString(),
                  style: const TextStyle(fontWeight: FontWeight.w800, fontSize: 15),
                ),
                const SizedBox(height: 4),
                Text(
                  '${b['date']}  |  ${fmtTime((b['start_time'] ?? '').toString())} - ${fmtTime((b['end_time'] ?? '').toString())}',
                  style: TextStyle(color: Colors.grey.shade700),
                ),
              ],
            ),
          ),
          TextButton(
            onPressed: () => _cancel(id),
            child: const Text('Cancel', style: TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );
  }

  Widget _slotsTab() {
    return RefreshIndicator(
      onRefresh: _refresh,
      child: ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.all(16),
        children: [
          const Text('My slots', style: TextStyle(fontSize: 20, fontWeight: FontWeight.w800)),
          const SizedBox(height: 12),
          if (_bookings.isEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 24),
              child: Text(
                'No slots booked yet',
                textAlign: TextAlign.center,
                style: TextStyle(color: Colors.grey.shade600),
              ),
            ),
          for (final b in _bookings) _bookingTile(b),
          const SizedBox(height: 8),
          ElevatedButton(
            onPressed: _openStores,
            style: ElevatedButton.styleFrom(
              backgroundColor: Colors.black,
              foregroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(vertical: 14),
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
            ),
            child: const Text('Book a slot', style: TextStyle(fontWeight: FontWeight.w700)),
          ),
        ],
      ),
    );
  }

  Widget _placeholder(String title) {
    return Center(
      child: Text('$title (coming soon)', style: TextStyle(fontSize: 16, color: Colors.grey.shade600)),
    );
  }

  Widget _profileTab() {
    final isPicker = _workflow.isEmpty || _workflow == 'picker';
    final initial = _name.trim().isEmpty ? '?' : _name.trim()[0].toUpperCase();
    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 24, 20, 24),
      children: [
        Center(
          child: CircleAvatar(
            radius: 38,
            backgroundColor: const Color(0xFFE3F2FD),
            child: Text(initial, style: const TextStyle(fontSize: 32, fontWeight: FontWeight.w800, color: _blueDark)),
          ),
        ),
        const SizedBox(height: 14),
        Text(_name, textAlign: TextAlign.center, style: const TextStyle(fontSize: 20, fontWeight: FontWeight.w800)),
        const SizedBox(height: 4),
        Text(_sub, textAlign: TextAlign.center, style: TextStyle(color: Colors.grey.shade600)),
        const SizedBox(height: 14),
        Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 6),
              decoration: BoxDecoration(color: Colors.black, borderRadius: BorderRadius.circular(20)),
              child: Text(_roleLabel(),
                  style: const TextStyle(color: Colors.white, fontSize: 12, fontWeight: FontWeight.w800, letterSpacing: 0.8)),
            ),
            const SizedBox(width: 8),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
              decoration: BoxDecoration(
                color: _online ? const Color(0xFFE8F9EE) : const Color(0xFFF1F1F1),
                borderRadius: BorderRadius.circular(20),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.circle, size: 8, color: _online ? _green : Colors.grey),
                  const SizedBox(width: 6),
                  Text(_online ? 'Online' : 'Offline',
                      style: TextStyle(fontSize: 12, fontWeight: FontWeight.w700, color: _online ? _green : Colors.grey.shade700)),
                ],
              ),
            ),
          ],
        ),
        if (_presenceStore.isNotEmpty) ...[
          const SizedBox(height: 10),
          Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(Icons.storefront_outlined, size: 16, color: Colors.grey.shade600),
              const SizedBox(width: 6),
              Text(_presenceStore, style: TextStyle(fontSize: 13, color: Colors.grey.shade700)),
            ],
          ),
        ],
        const SizedBox(height: 26),
        const Text('Today', style: TextStyle(fontSize: 15, fontWeight: FontWeight.w800)),
        const SizedBox(height: 10),
        Row(
          children: [
            _pStat(_pHm(_liveSecs(_activeBase)), 'Active time', _ink),
            const SizedBox(width: 10),
            _pStat(_pHm(_liveSecs(_loginBase)), 'Login time', _ink),
          ],
        ),
        if (isPicker) ...[
          const SizedBox(height: 10),
          Row(
            children: [
              _pStat('${(_sum['orders_completed'] as num?)?.toInt() ?? 0}', 'Orders', _ink),
              const SizedBox(width: 10),
              _pStat('${(_sum['items_picked'] as num?)?.toInt() ?? 0}', 'Items picked', _green),
              const SizedBox(width: 10),
              _pStat('\u20B9${_pMoney((_sum['earnings'] as num?) ?? 0)}', 'Earnings', _ink),
            ],
          ),
        ],
        const SizedBox(height: 32),
        OutlinedButton(
          onPressed: _confirmLogout,
          style: OutlinedButton.styleFrom(
            foregroundColor: Colors.red,
            padding: const EdgeInsets.symmetric(vertical: 14),
            side: const BorderSide(color: Colors.red),
            shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
          ),
          child: const Text('Logout', style: TextStyle(fontWeight: FontWeight.w700)),
        ),
      ],
    );
  }
  @override
  Widget build(BuildContext context) {
    final tabs = <Widget>[
      _taskTab(),
      _slotsTab(),
      const PayoutScreen(),
      _profileTab(),
    ];
    return Scaffold(
      backgroundColor: Colors.white,
      body: SafeArea(child: tabs[_tab]),
      bottomNavigationBar: BottomNavigationBar(
        currentIndex: _tab,
        onTap: (i) {
          setState(() => _tab = i);
          if (i == 3) _loadProfileSummary();
        },
        type: BottomNavigationBarType.fixed,
        backgroundColor: Colors.white,
        selectedItemColor: Colors.black,
        unselectedItemColor: Colors.black45,
        selectedLabelStyle: const TextStyle(fontWeight: FontWeight.w700),
        items: const [
          BottomNavigationBarItem(icon: Icon(Icons.assignment_outlined), label: 'Task'),
          BottomNavigationBarItem(icon: Icon(Icons.event_note_outlined), label: 'Slots'),
          BottomNavigationBarItem(icon: Icon(Icons.currency_rupee), label: 'Payouts'),
          BottomNavigationBarItem(icon: Icon(Icons.person_outline), label: 'Profile'),
        ],
      ),
    );
  }
}

class _TimeBox extends StatelessWidget {
  final String value;
  final String label;
  const _TimeBox(this.value, this.label);

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 48,
      padding: const EdgeInsets.symmetric(vertical: 6),
      decoration: BoxDecoration(color: const Color(0xFF16A34A), borderRadius: BorderRadius.circular(8)),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(value, style: const TextStyle(color: Colors.white, fontSize: 18, fontWeight: FontWeight.w800)),
          Text(label, style: const TextStyle(color: Colors.white, fontSize: 9, fontWeight: FontWeight.w700)),
        ],
      ),
    );
  }
}

class _WorkflowSheet extends StatefulWidget {
  const _WorkflowSheet();

  @override
  State<_WorkflowSheet> createState() => _WorkflowSheetState();
}

class _WorkflowSheetState extends State<_WorkflowSheet> {
  String _sel = 'picker';

  Widget _item(String value, String label, IconData icon) {
    final on = _sel == value;
    return InkWell(
      onTap: () => setState(() => _sel = value),
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 14),
        child: Row(
          children: [
            Icon(icon, size: 32, color: Colors.black87),
            const SizedBox(width: 16),
            Expanded(child: Text(label, style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w800))),
            Icon(
              on ? Icons.radio_button_checked : Icons.radio_button_unchecked,
              color: const Color(0xFF16A34A),
            ),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 20, 20, 16),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            const Text('Choose Workflow', style: TextStyle(fontSize: 18, fontWeight: FontWeight.w800)),
            const Divider(height: 24),
            _item('picker', 'PICKER', Icons.shopping_basket_outlined),
            _item('packer', 'PUTTER', Icons.inventory_2_outlined),
            _item('auditor', 'AUDITOR', Icons.fact_check_outlined),
            _item('fnv', 'FNV', Icons.eco_outlined),
            const SizedBox(height: 12),
            ElevatedButton(
              onPressed: () => Navigator.pop(context, _sel),
              style: ElevatedButton.styleFrom(
                backgroundColor: Colors.black,
                foregroundColor: Colors.white,
                padding: const EdgeInsets.symmetric(vertical: 16),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
              ),
              child: const Text('Proceed', style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700)),
            ),
          ],
        ),
      ),
    );
  }
}