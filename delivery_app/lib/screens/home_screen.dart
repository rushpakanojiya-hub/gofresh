import 'dart:async';
import 'package:flutter/material.dart';
import '../services/api_service.dart';
import 'order_detail_screen.dart';
import 'return_pickup_detail_screen.dart';
import '../services/location_service.dart';
import '../services/push_service.dart';
import 'new_order_sheet.dart';

class HomeScreen extends StatefulWidget {
  final void Function(int)? onSwitchTab;
  const HomeScreen({super.key, this.onSwitchTab});

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
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
  List<dynamic> _orders = [];
  List<dynamic> _returns = [];
  final Set<int> _actingIds = {};

  Timer? _pollTimer;

  @override
  void initState() {
    super.initState();
    _loadAll();
    _loadAvailability();
    _pollTimer = Timer.periodic(const Duration(seconds: 15), (_) {
      if (mounted) _loadAll(silent: true);
    });
  }

  @override
  void dispose() {
    _pollTimer?.cancel();
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
    if (target == null) return;
    PushService.popupShown.add(_idOf(target));
    PushService.sheetOpen = true;
    try {
      await NewOrderSheet.show(context, Map<String, dynamic>.from(target as Map));
    } finally {
      PushService.sheetOpen = false;
    }
    if (mounted) _loadAll(silent: true);
  }

  Future<void> _loadAvailability() async {
    try {
      final data = await ApiService.getAvailability();
      if (mounted) setState(() => _isOnline = data['is_online'] == true);
    } catch (_) {
      // Badge just won't show a definite state yet.
    }
  }

  Future<void> _toggleOnline() async {
    final next = !(_isOnline ?? false);
    setState(() => _togglingOnline = true);
    try {
      final data = await ApiService.updateAvailability(next);
      if (mounted) setState(() => _isOnline = data['is_online'] == true);
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
      Navigator.push(
        context,
        MaterialPageRoute(builder: (_) => OrderDetailScreen(order: item)),
      ).then((_) => _loadAll());
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
                            GestureDetector(
                              onTap: _togglingOnline ? null : _toggleOnline,
                              child: Container(
                                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                                decoration: BoxDecoration(
                                  color: _isOnline == true ? const Color(0xFFE1F5E6) : const Color(0xFFFDEAEA),
                                  borderRadius: BorderRadius.circular(20),
                                ),
                                child: Row(
                                  mainAxisSize: MainAxisSize.min,
                                  children: [
                                    if (_togglingOnline)
                                      const SizedBox(
                                        width: 10,
                                        height: 10,
                                        child: CircularProgressIndicator(strokeWidth: 2),
                                      )
                                    else
                                      Container(
                                        width: 8,
                                        height: 8,
                                        decoration: BoxDecoration(
                                          shape: BoxShape.circle,
                                          color: _isOnline == true ? Colors.green : Colors.red,
                                        ),
                                      ),
                                    const SizedBox(width: 6),
                                    Text(
                                      _isOnline == true ? 'ONLINE' : 'OFFLINE',
                                      style: TextStyle(
                                        fontSize: 12,
                                        fontWeight: FontWeight.bold,
                                        color: _isOnline == true ? Colors.green[800] : Colors.red[800],
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                            ),
                          ],
                        ),
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