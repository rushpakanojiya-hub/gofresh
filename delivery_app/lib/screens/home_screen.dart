import 'dart:async';
import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:geolocator/geolocator.dart';
import 'order_pickup_screen.dart';
import 'delivery_map_screen.dart';
import 'delivery_handover_screen.dart';
import '../services/api_service.dart';
import 'return_pickup_detail_screen.dart';
import 'return_offer_sheet.dart';
import '../services/location_service.dart';
import '../services/push_service.dart';
import 'new_order_sheet.dart';
import 'gigs_screen.dart';

class HomeScreen extends StatefulWidget {
  final void Function(int)? onSwitchTab;
  const HomeScreen({super.key, this.onSwitchTab});

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> with WidgetsBindingObserver {
  static const Color primaryPurple = Color(0xFF5B2A9E);
  static const Color pageBg = Color(0xFFF7F1FB);
  static const Color bannerBg = Color(0xFFEDE6F7);
  static const Color cardBg = Color(0xFFF3EDFA);

  bool? _isOnline;
  bool _togglingOnline = false;

  bool _loading = true;
  String? _error;

  Map<String, dynamic>? _earnings;
  Map<String, dynamic>? _codSummary;
  Map<String, dynamic>? _checkin;
  List<dynamic> _orders = [];
  List<dynamic> _returns = [];
  final Set<int> _actingIds = {};

  Timer? _pollTimer;

  @override
  void initState() {
    super.initState();
    PushService.dataRefresh.addListener(_onDataRefresh);
    WidgetsBinding.instance.addObserver(this);
    _loadAll();
    _loadAvailability();
    _loadGigs();
    _pollTimer = Timer.periodic(const Duration(seconds: 4), (_) {
      if (mounted) _loadAll(silent: true);
    });
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed && mounted) {
      _loadAll(silent: true).then((_) {
        if (mounted) _maybeShowNewOrderPopup();
      });
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _pollTimer?.cancel();
    PushService.dataRefresh.removeListener(_onDataRefresh);
    super.dispose();
  }

  Future<void> _loadAll({bool silent = false}) async {
    if (!silent) {
      setState(() {
        _loading = true;
        _error = null;
      });
    }
    try {
      final orders = await ApiService.getMyDeliveries();
      List<dynamic> returns = [];
      try {
        returns = await ApiService.getMyReturnPickups();
      } catch (_) {
        returns = [];
      }
      Map<String, dynamic>? checkin;
      try {
        checkin = await ApiService.getCheckin();
      } catch (_) {
        checkin = null;
      }
      Map<String, dynamic>? earnings;
      Map<String, dynamic>? codSummary;
      try {
        earnings = await ApiService.getEarnings();
      } catch (_) {
        earnings = null;
      }
      try {
        codSummary = await ApiService.getCODSummary();
      } catch (_) {
        codSummary = null;
      }
      setState(() {
        _orders = orders;
        _checkin = checkin;
        _returns = returns;
        _earnings = earnings;
        _codSummary = codSummary;
        _loading = false;
        // A successful refresh clears any earlier error so a transient
        // background-poll failure doesn't permanently hide the dashboard.
        _error = null;
      });
      _maybeShowNewOrderPopup();
    } catch (e) {
      // Only a foreground (non-silent) load failure replaces the whole
      // screen with an error - a silent background poll hiccup must never
      // wipe out already-loaded data (new orders/returns disappearing).
      if (!silent) {
        setState(() {
          _error = 'Failed to load dashboard';
          _loading = false;
        });
      }
    }
  }

  /// Opens the half-screen accept popup for a pending assignment the partner
  /// has not been shown yet (app opened late, resumed, or push was missed).
  Future<void> _maybeShowNewOrderPopup() async {
    if (!mounted || PushService.sheetOpen) return;
    final route = ModalRoute.of(context);
    if (route == null || !route.isCurrent) return;
    dynamic target;
    for (final o in _orders) {
      if (_isReturn(o) || !_isNew(o)) continue;
      if (o['assignment_status']?.toString() != 'assigned') continue;
      final exp = DateTime.tryParse((o['assignment_expires_at'] ?? '').toString());
      if (exp != null && exp.isBefore(DateTime.now())) continue;
      if (PushService.popupShown.contains(_idOf(o))) continue;
      target = o;
      break;
    }
    if (target == null) {
      dynamic rt;
      for (final r in _returns) {
        if (r['pickup_status']?.toString() != 'assigned') continue;
        final rexp = DateTime.tryParse((r['pickup_assignment_expires_at'] ?? '').toString());
        if (rexp != null && rexp.isBefore(DateTime.now())) continue;
        final rid = (r['return_request_id'] as num).toInt();
        if (PushService.returnPopupShown.contains(rid)) continue;
        rt = r;
        break;
      }
      if (rt == null) return;
      PushService.returnPopupShown.add((rt['return_request_id'] as num).toInt());
      PushService.sheetOpen = true;
      try {
        await ReturnOfferSheet.show(context, Map<String, dynamic>.from(rt as Map));
      } finally {
        PushService.sheetOpen = false;
      }
      if (mounted) _loadAll(silent: true);
      return;
    }
    PushService.popupShown.add(_idOf(target));
    PushService.sheetOpen = true;
    try {
      await NewOrderSheet.show(context, Map<String, dynamic>.from(target as Map));
    } finally {
      PushService.sheetOpen = false;
    }
    if (mounted) _loadAll(silent: true);
  }

  void _onDataRefresh() {
    Future.microtask(() {
      if (mounted) _loadAll(silent: true);
    });
  }

  Future<void> _loadAvailability() async {
    try {
      final data = await ApiService.getAvailability();
      if (mounted) { setState(() => _isOnline = data['is_online'] == true); _syncWorking(data['is_online'] == true); }
    } catch (_) {
      // Badge just won't show a definite state yet.
    }
  }

  Future<bool> _ensureLocationOn() async {
    if (!await Geolocator.isLocationServiceEnabled()) {
      if (!mounted) return false;
      final open = await showDialog<bool>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('Location is off'),
          content: const Text('Turn on location to go online.'),
          actions: [
            TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Cancel')),
            TextButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('Open settings')),
          ],
        ),
      );
      if (open == true) await Geolocator.openLocationSettings();
      return false;
    }
    if (await LocationService.requestPermission()) return true;
    if (!mounted) return false;
    final open = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Location permission needed'),
        content: const Text('Allow location access (Allow all the time) to go online.'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Cancel')),
          TextButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('Open settings')),
        ],
      ),
    );
    if (open == true) await Geolocator.openAppSettings();
    return false;
  }

  Future<void> _toggleOnline([bool? target]) async {
    final next = target ?? !(_isOnline ?? false);
    if (_isOnline == next) return;
    if (next && !await _ensureLocationOn()) return;
    setState(() => _togglingOnline = true);
    try {
      final data = await ApiService.updateAvailability(next);
      if (mounted) { setState(() => _isOnline = data['is_online'] == true); _syncWorking(data['is_online'] == true); }
      if (data['is_online'] == true) {
        LocationService.startTracking();
      } else {
        LocationService.stopTracking();
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(e.toString().replaceFirst('Exception: ', ''))),
        );
      }
    } finally {
      if (mounted) setState(() => _togglingOnline = false);
    }
  }

  String _money(dynamic v) {
    final n = v is num ? v : num.tryParse('${v ?? ''}');
    if (n == null) return '-';
    return n.toStringAsFixed(2);
  }

  String _greeting() {
    final h = DateTime.now().hour;
    if (h < 12) return 'Good Morning, Partner!';
    if (h < 17) return 'Good Afternoon, Partner!';
    return 'Good Evening, Partner!';
  }

  // ---- Merged orders + return-pickups, mirroring OrdersScreen's logic ----

  bool _isReturn(dynamic item) => item['type'] == 'return_pickup';

  int _idOf(dynamic item) =>
      _isReturn(item) ? item['return_request_id'] as int : (item['order_id'] ?? item['id']) as int;

  String _statusOf(dynamic item) =>
      (_isReturn(item) ? item['pickup_status'] : item['status'])?.toString() ?? '';

  bool _isNew(dynamic item) {
    if (_isReturn(item)) {
      final s = item['pickup_status'];
      return s == null || s == 'assigned';
    }
    final status = (item['status'] ?? '').toString();
    return status == 'confirmed' &&
        (item['delivery_status'] == null || item['delivery_status'] == 'assigned');
  }

  bool _isFinished(dynamic item) =>
      _isReturn(item) ? _statusOf(item) == 'handed_over' : _statusOf(item) == 'delivered';

  bool _isCancelled(dynamic item) => !_isReturn(item) && _statusOf(item) == 'cancelled';

  List<dynamic> get _activeItems {
    final all = [..._orders, ..._returns]
        .where((i) => !_isFinished(i) && !_isCancelled(i))
        .toList();
    // Newest first, and anything awaiting accept/reject floats to the top
    // so the partner sees it immediately instead of having to scroll.
    all.sort((a, b) {
      final aNew = _isNew(a) ? 0 : 1;
      final bNew = _isNew(b) ? 0 : 1;
      if (aNew != bNew) return aNew - bNew;
      final da = DateTime.tryParse((a['created_at'] ?? '').toString()) ?? DateTime(1970);
      final db = DateTime.tryParse((b['created_at'] ?? '').toString()) ?? DateTime(1970);
      return db.compareTo(da);
    });
    return all;
  }

  int get _todayCompletedCount {
    final now = DateTime.now();
    return _orders.where((o) {
      if ((o['status'] ?? '') != 'delivered') return false;
      final raw = o['delivered_at'] ?? o['updated_at'];
      if (raw == null) return false;
      try {
        final dt = DateTime.parse(raw.toString()).toLocal();
        return dt.year == now.year && dt.month == now.month && dt.day == now.day;
      } catch (_) {
        return false;
      }
    }).length;
  }

  Future<void> _acceptOrder(int orderId) async {
    setState(() => _actingIds.add(orderId));
    try {
      await ApiService.acceptAssignment(orderId);
      await _loadAll();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(e.toString().replaceFirst('Exception: ', ''))),
        );
      }
    } finally {
      if (mounted) setState(() => _actingIds.remove(orderId));
    }
  }

  Future<void> _rejectOrder(int orderId) async {
    final reasonController = TextEditingController();
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Reject this delivery?'),
        content: TextField(
          controller: reasonController,
          decoration: const InputDecoration(hintText: 'Reason (optional)'),
          maxLines: 2,
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Cancel')),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Reject', style: TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    setState(() => _actingIds.add(orderId));
    try {
      await ApiService.rejectAssignment(orderId, reason: reasonController.text);
      await _loadAll();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(e.toString().replaceFirst('Exception: ', ''))),
        );
      }
    } finally {
      if (mounted) setState(() => _actingIds.remove(orderId));
    }
  }

  Future<void> _acceptReturn(int returnRequestId) async {
    setState(() => _actingIds.add(returnRequestId));
    try {
      await ApiService.acceptReturnPickup(returnRequestId);
      await _loadAll();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(e.toString().replaceFirst('Exception: ', ''))),
        );
      }
    } finally {
      if (mounted) setState(() => _actingIds.remove(returnRequestId));
    }
  }

  Future<void> _rejectReturn(int returnRequestId) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Reject this pickup?'),
        content: const Text('This will open the pickup for reassignment to another partner.'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Cancel')),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Reject', style: TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    setState(() => _actingIds.add(returnRequestId));
    try {
      await ApiService.rejectReturnPickup(returnRequestId);
      await _loadAll();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(e.toString().replaceFirst('Exception: ', ''))),
        );
      }
    } finally {
      if (mounted) setState(() => _actingIds.remove(returnRequestId));
    }
  }

  void _openDetail(dynamic item) {
    if (_isReturn(item)) {
      Navigator.push(
        context,
        MaterialPageRoute(builder: (_) => ReturnPickupDetailScreen(pickup: item)),
      ).then((_) => _loadAll());
    } else {
      final m = Map<String, dynamic>.from(item as Map);
      final ds = m['delivery_status']?.toString();
      Widget? next;
      if (ds == 'accepted' || ds == 'going_to_store' || ds == 'arrived_at_store') {
        next = OrderPickupScreen(order: m);
      } else if (ds == 'picked_up' || ds == 'out_for_delivery') {
        next = DeliveryMapScreen(order: m);
      } else if (ds == 'arrived' || ds == 'arrived_at_customer') {
        next = DeliveryHandoverScreen(order: m);
      }
      if (next != null) {
        final screen = next;
        Navigator.push(context, MaterialPageRoute(builder: (_) => screen))
            .then((_) => _loadAll());
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final todayEarnings = _earnings?['today_earnings'];
    final todayDeliveries = _earnings?['today_deliveries'] ?? _todayCompletedCount;
    final codCollectedToday = _codSummary?['today_collected'];
    final pendingSettlement = _codSummary?['pending_settlement'];
    final activeItems = _activeItems.where((i) => _isReturn(i)).toList();

    return Scaffold(
      backgroundColor: pageBg,
      body: SafeArea(
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : _error != null
                ? Center(child: Text(_error!))
                : RefreshIndicator(
                    onRefresh: _loadAll,
                    child: ListView(
                      padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
                      children: [
                        Row(
                          children: [
                            Expanded(
                              child: Text(
                                _greeting(),
                                style: const TextStyle(fontSize: 20, fontWeight: FontWeight.bold, color: Colors.black87),
                              ),
                            ),
                            Container(
                              padding: const EdgeInsets.all(3),
                              decoration: BoxDecoration(
                                color: const Color(0xFFEDEDED),
                                borderRadius: BorderRadius.circular(20),
                              ),
                              child: Row(
                                mainAxisSize: MainAxisSize.min,
                                children: [
                                  for (final online in [false, true])
                                    GestureDetector(
                                      onTap: _togglingOnline ? null : () => _toggleOnline(online),
                                      child: AnimatedContainer(
                                        duration: const Duration(milliseconds: 200),
                                        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
                                        decoration: BoxDecoration(
                                          color: (_isOnline == true) == online
                                              ? (online ? const Color(0xFF4CAF50) : const Color(0xFFE53935))
                                              : Colors.transparent,
                                          borderRadius: BorderRadius.circular(17),
                                        ),
                                        child: Text(
                                          online ? 'ONLINE' : 'OFFLINE',
                                          style: TextStyle(
                                            fontSize: 11,
                                            fontWeight: FontWeight.bold,
                                            color: (_isOnline == true) == online ? Colors.white : Colors.black54,
                                          ),
                                        ),
                                      ),
                                    ),
                                ],
                              ),
                            ),
                          ],
                        ),
                        const SizedBox(height: 16),

                        _checkinBanner(),
                        _gigCard(),
                        const SizedBox(height: 12),
                        _summaryCard(todayDeliveries, todayEarnings),
                        const SizedBox(height: 12),
                        _cashCard(pendingSettlement),
                        const SizedBox(height: 16),

                        // Active delivery (orders + return pickups)
                        if (activeItems.isNotEmpty) ...[
                          const Text(
                            'ACTIVE DELIVERY',
                            style: TextStyle(fontWeight: FontWeight.bold, fontSize: 13, color: Colors.black54, letterSpacing: 0.5),
                          ),
                          const SizedBox(height: 8),
                          ...activeItems.map((item) {
                            final isReturn = _isReturn(item);
                            final id = _idOf(item);
                            final isNewItem = _isNew(item);
                            final isActing = _actingIds.contains(id);
                            final status = _statusOf(item);
                            return Padding(
                              padding: const EdgeInsets.only(bottom: 12),
                              child: Container(
                                width: double.infinity,
                                padding: const EdgeInsets.all(16),
                                decoration: BoxDecoration(
                                  color: cardBg,
                                  borderRadius: BorderRadius.circular(16),
                                  border: Border.all(color: primaryPurple.withValues(alpha: 0.15)),
                                ),
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Row(
                                      children: [
                                        Text(
                                          isReturn ? 'Return - Order #${item['order_id']}' : 'Order #$id',
                                          style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 16),
                                        ),
                                        const Spacer(),
                                        Container(
                                          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                                          decoration: BoxDecoration(
                                            color: (isNewItem ? Colors.orange : primaryPurple).withValues(alpha: 0.12),
                                            borderRadius: BorderRadius.circular(10),
                                          ),
                                          child: Text(
                                            isNewItem
                                                ? 'NEW'
                                                : (status.isEmpty ? 'ASSIGNED' : status).replaceAll('_', ' ').toUpperCase(),
                                            style: TextStyle(
                                              fontSize: 10,
                                              fontWeight: FontWeight.bold,
                                              color: isNewItem ? Colors.orange[800] : primaryPurple,
                                            ),
                                          ),
                                        ),
                                      ],
                                    ),
                                    const SizedBox(height: 10),
                                    if (!isReturn && item['payment_method'] == 'cod')
                                      Text(
                                        'COD: ₹${_money(item['total_amount'])}',
                                        style: const TextStyle(color: Colors.black54, fontSize: 13),
                                      ),
                                    if (isReturn && item['refund_amount'] != null)
                                      Text(
                                        'Refund: ₹${_money(item['refund_amount'])}',
                                        style: const TextStyle(color: Colors.black54, fontSize: 13),
                                      ),
                                    const SizedBox(height: 12),
                                    if (isNewItem)
                                      Row(
                                        children: [
                                          Expanded(
                                            child: OutlinedButton(
                                              onPressed: isActing
                                                  ? null
                                                  : () => isReturn ? _rejectReturn(id) : _rejectOrder(id),
                                              style: OutlinedButton.styleFrom(foregroundColor: Colors.red),
                                              child: const Text('Reject'),
                                            ),
                                          ),
                                          const SizedBox(width: 8),
                                          Expanded(
                                            child: ElevatedButton(
                                              onPressed: isActing
                                                  ? null
                                                  : () => isReturn ? _acceptReturn(id) : _acceptOrder(id),
                                              style: ElevatedButton.styleFrom(
                                                backgroundColor: primaryPurple,
                                                foregroundColor: Colors.white,
                                                padding: const EdgeInsets.symmetric(vertical: 12),
                                                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
                                              ),
                                              child: isActing
                                                  ? const SizedBox(
                                                      height: 16,
                                                      width: 16,
                                                      child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                                                    )
                                                  : const Text('Accept'),
                                            ),
                                          ),
                                        ],
                                      )
                                    else
                                      SizedBox(
                                        width: double.infinity,
                                        child: ElevatedButton(
                                          onPressed: () => _openDetail(item),
                                          style: ElevatedButton.styleFrom(
                                            backgroundColor: primaryPurple,
                                            foregroundColor: Colors.white,
                                            padding: const EdgeInsets.symmetric(vertical: 12),
                                            shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
                                          ),
                                          child: Text(isReturn ? 'View Pickup' : 'View Delivery'),
                                        ),
                                      ),
                                  ],
                                ),
                              ),
                            );
                          }),
                        ] else
                          Container(
                            width: double.infinity,
                            padding: const EdgeInsets.all(20),
                            decoration: BoxDecoration(color: cardBg, borderRadius: BorderRadius.circular(16)),
                            child: const Center(
                              child: Text('No active delivery right now', style: TextStyle(color: Colors.black45)),
                            ),
                          ),
                      ],
                    ),
                  ),
      ),
    );
  }

  int _workAccumMs = 0;
  int _workSinceMs = 0;

  String _dayKey(DateTime d) => '${d.year}-${d.month}-${d.day}';

  Future<void> _syncWorking(bool online) async {
    try {
      final p = await SharedPreferences.getInstance();
      final now = DateTime.now();
      final nowMs = now.millisecondsSinceEpoch;
      final midnight = DateTime(now.year, now.month, now.day).millisecondsSinceEpoch;
      var accum = p.getInt('work_accum_ms') ?? 0;
      var since = p.getInt('work_since_ms') ?? 0;
      if ((p.getString('work_day') ?? '') != _dayKey(now)) {
        accum = 0;
        if (since > 0) since = midnight;
      }
      if (online && since == 0) since = nowMs;
      if (!online && since > 0) {
        accum += nowMs - since;
        since = 0;
      }
      await p.setString('work_day', _dayKey(now));
      await p.setInt('work_accum_ms', accum);
      await p.setInt('work_since_ms', since);
      if (mounted) {
        setState(() {
          _workAccumMs = accum;
          _workSinceMs = since;
        });
      }
    } catch (_) {}
  }

  String get _workingLabel {
    var ms = _workAccumMs;
    if (_workSinceMs > 0) ms += DateTime.now().millisecondsSinceEpoch - _workSinceMs;
    final mins = (ms / 60000).floor();
    return '${mins ~/ 60}h ${mins % 60}m';
  }

  List<Map<String, dynamic>> _gigSlots = [];

  Future<void> _loadGigs() async {
    try {
      final out = <Map<String, dynamic>>[];
      final ist = DateTime.now().toUtc().add(const Duration(hours: 5, minutes: 30));
      for (var i = 0; i < 2; i++) {
        final d = ist.add(Duration(days: i));
        final ds = '${d.year}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';
        final data = await ApiService.getGigs(ds);
        for (final g in (data['groups'] as List? ?? [])) {
          for (final s in (g['slots'] as List? ?? [])) {
            if (s['booked'] == true) {
              out.add({...Map<String, dynamic>.from(s as Map), 'day_offset': i});
            }
          }
        }
      }
      if (mounted) setState(() => _gigSlots = out);
    } catch (_) {}
  }

  Future<void> _openGigs() async {
    await Navigator.push(context, MaterialPageRoute(builder: (_) => const GigsScreen()));
    if (mounted) _loadGigs();
  }

  Widget _gigCard() {
    final now = DateTime.now();
    Map<String, dynamic>? cur;
    Map<String, dynamic>? next;
    for (final s in _gigSlots) {
      final st = DateTime.tryParse('${s['start_at']}');
      final en = DateTime.tryParse('${s['end_at']}');
      if (st == null || en == null) continue;
      if (!now.isBefore(st) && now.isBefore(en)) {
        cur ??= s;
      } else if (st.isAfter(now)) {
        final nx = next == null ? null : DateTime.tryParse('${next['start_at']}');
        if (nx == null || st.isBefore(nx)) next = s;
      }
    }
    String line;
    if (cur != null) {
      line = 'Current gig: ${cur['label']}';
    } else if (next != null) {
      line = 'Next gig: ${next['day_offset'] == 0 ? 'Today' : 'Tomorrow'}, ${next['label']}';
    } else {
      line = 'No gig booked';
    }
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: Colors.black12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Row(
            children: [
              Icon(Icons.event_available, size: 20),
              SizedBox(width: 8),
              Text('Gig details', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
            ],
          ),
          const SizedBox(height: 10),
          Text(line, style: const TextStyle(fontSize: 14, color: Colors.black87)),
          const SizedBox(height: 12),
          Row(
            children: [
              if (cur != null && _isOnline != true) ...[
                Expanded(
                  child: ElevatedButton(
                    onPressed: _togglingOnline ? null : () => _toggleOnline(true),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: const Color(0xFF4CAF50),
                      foregroundColor: Colors.white,
                      padding: const EdgeInsets.symmetric(vertical: 12),
                    ),
                    child: const Text('Go Online'),
                  ),
                ),
                const SizedBox(width: 10),
              ],
              Expanded(
                child: ElevatedButton(
                  onPressed: _openGigs,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: Colors.black,
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(vertical: 12),
                  ),
                  child: const Text('Book gigs'),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _checkinBanner() {
    if (_isOnline != true) return const SizedBox.shrink();
    final checkedIn = _checkin?['checked_in'] == true;
    final color = checkedIn ? const Color(0xFF2E7D32) : const Color(0xFFEF6C00);
    final text = checkedIn
        ? 'Checked in at ${_checkin?['warehouse_name'] ?? 'store'}'
        : 'Scan store QR to receive orders';
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Container(
        width: double.infinity,
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.12),
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: color.withValues(alpha: 0.4)),
        ),
        child: Row(
          children: [
            Icon(checkedIn ? Icons.check_circle : Icons.qr_code_scanner, color: color, size: 20),
            const SizedBox(width: 10),
            Expanded(
              child: Text(
                text,
                style: TextStyle(color: color, fontWeight: FontWeight.w600, fontSize: 13),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _summaryItem(IconData icon, Color color, String label, String value) {
    return Expanded(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 8),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Container(
              padding: const EdgeInsets.all(6),
              decoration: BoxDecoration(color: color.withValues(alpha: 0.15), shape: BoxShape.circle),
              child: Icon(icon, size: 18, color: color),
            ),
            const SizedBox(height: 8),
            Text(label, style: const TextStyle(fontSize: 11, color: Colors.black54)),
            const SizedBox(height: 4),
            Text(value, style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold, color: Colors.black87)),
          ],
        ),
      ),
    );
  }

  Widget _summaryCard(dynamic deliveries, dynamic earnings) {
    final count = deliveries is num ? deliveries.toInt() : 0;
    final amount = earnings is num ? earnings.toStringAsFixed(0) : '0';
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: Colors.black12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text("Today's Summary", style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
          const SizedBox(height: 12),
          IntrinsicHeight(
            child: Row(
              children: [
                _summaryItem(Icons.inventory_2_outlined, Colors.blue, 'Completed Orders', '$count'),
                Container(width: 1, color: Colors.black12),
                _summaryItem(Icons.currency_rupee, Colors.green, 'Earnings', '\u20B9$amount'),
                Container(width: 1, color: Colors.black12),
                _summaryItem(Icons.access_time, Colors.deepPurple, 'Working Hours', _workingLabel),
              ],
            ),
          ),
        ],
      ),
    );
  }
  static const double _cashLimit = 1500;

  Widget _cashCard(dynamic pending) {
    final held = pending is num ? pending.toDouble() : 0.0;
    final ratio = (held / _cashLimit).clamp(0.0, 1.0).toDouble();
    final full = held >= _cashLimit;
    final warn = held >= _cashLimit * 0.8;
    final color = full ? Colors.red : (warn ? Colors.orange : Colors.green);
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: full ? Colors.red.withValues(alpha: 0.4) : Colors.black12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Icon(Icons.payments_outlined, size: 20),
              const SizedBox(width: 8),
              const Expanded(
                child: Text('Cash in hand', style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold)),
              ),
              Text(
                '\u20B9${held.toStringAsFixed(0)} / \u20B9${_cashLimit.toStringAsFixed(0)}',
                style: TextStyle(fontSize: 14, fontWeight: FontWeight.bold, color: color),
              ),
            ],
          ),
          const SizedBox(height: 10),
          ClipRRect(
            borderRadius: BorderRadius.circular(6),
            child: LinearProgressIndicator(
              value: ratio,
              minHeight: 8,
              backgroundColor: Colors.black12,
              valueColor: AlwaysStoppedAnimation<Color>(color),
            ),
          ),
          if (warn) ...[
            const SizedBox(height: 8),
            Text(
              full
                  ? 'Cash limit reached. Deposit your cash with admin to receive new COD orders.'
                  : 'Close to the limit. Please deposit your cash with admin soon.',
              style: TextStyle(fontSize: 12, color: color),
            ),
          ],
        ],
      ),
    );
  }
  Widget _statTile(String label, String value) {
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 4),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(color: Colors.white, borderRadius: BorderRadius.circular(14)),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label, style: const TextStyle(fontSize: 11, color: Colors.black45)),
          const SizedBox(height: 4),
          Text(value, style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold, color: Colors.black87)),
        ],
      ),
    );
  }
}