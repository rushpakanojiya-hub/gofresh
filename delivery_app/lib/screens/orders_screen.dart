import 'dart:async';
import 'package:flutter/material.dart';
import '../services/api_service.dart';
import 'notifications_screen.dart';
import 'order_detail_screen.dart';
import 'return_pickup_detail_screen.dart';

class OrdersScreen extends StatefulWidget {
  final void Function(int)? onSwitchTab;
  const OrdersScreen({super.key, this.onSwitchTab});

  @override
  State<OrdersScreen> createState() => _OrdersScreenState();
}

class _OrdersScreenState extends State<OrdersScreen> with WidgetsBindingObserver {
  List<dynamic> _orders = [];
  List<dynamic> _returns = [];
  bool _loading = true;
  String? _error;
  String _filter = 'active';
  final Set<int> _actingIds = {};
  Timer? _refreshTimer;

  static const Color primaryPurple = Color(0xFF5B2A9E);
  static const Color pageBg = Color(0xFFF7F1FB);
  static const Color cardBg = Color(0xFFF3EDFA);

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _refreshTimer = Timer.periodic(const Duration(seconds: 20), (_) {
      if (mounted) _loadAll(silent: true);
    });
    _loadAll();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) _loadAll(silent: true);
  }

  @override
  void dispose() {
    _refreshTimer?.cancel();
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  Future<void> _loadAll({bool silent = false}) async {
    setState(() {
      if (!silent) _loading = true;
      _error = null;
    });
    try {
      final orders = await ApiService.getMyDeliveries();
      List<dynamic> returns = [];
      try {
        returns = await ApiService.getMyReturnPickups();
      } catch (_) {
        returns = [];
      }
      if (!mounted) return;
      setState(() {
        _orders = orders;
        _returns = returns;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        if (!silent) _error = 'Failed to load orders';
        _loading = false;
      });
    }
  }

  // Merged list: every item is either an order map (no 'type' key) or a
  // return-pickup map (type == 'return_pickup'). Sorted by created_at desc.
  List<dynamic> get _merged {
    final all = [..._orders, ..._returns];
    all.sort((a, b) {
      final da = DateTime.tryParse((a['created_at'] ?? '').toString()) ?? DateTime(1970);
      final db = DateTime.tryParse((b['created_at'] ?? '').toString()) ?? DateTime(1970);
      return db.compareTo(da);
    });
    return all;
  }

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
    return status == 'confirmed' && (item['delivery_status'] == null || item['delivery_status'] == 'assigned');
  }

  bool _isCompleted(dynamic item) =>
      _isReturn(item) ? _statusOf(item) == 'handed_over' : _statusOf(item) == 'delivered';

  bool _isCancelled(dynamic item) => !_isReturn(item) && _statusOf(item) == 'cancelled';

  List<dynamic> get _filteredItems {
    final merged = _merged;
    if (_filter == 'new') return merged.where(_isNew).toList();
    if (_filter == 'active') {
      return merged.where((i) => !_isCompleted(i) && !_isCancelled(i)).toList();
    }
    if (_filter == 'completed') return merged.where(_isCompleted).toList();
    return merged;
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

  Color _statusColor(String status) {
    switch (status) {
      case 'confirmed':
      case 'accepted':
        return const Color(0xFF3B82F6);
      case 'shipped':
      case 'en_route':
        return const Color(0xFFF59E0B);
      case 'delivered':
      case 'handed_over':
        return const Color(0xFF22C55E);
      default:
        return Colors.grey;
    }
  }

  IconData _statusIcon(dynamic item) {
    if (_isReturn(item)) return Icons.assignment_return_outlined;
    final status = _statusOf(item);
    if (status == 'delivered') return Icons.check_circle_outline;
    if (status == 'shipped') return Icons.local_shipping_outlined;
    return Icons.assignment_outlined;
  }

  String? _formatTime(dynamic raw) {
    if (raw == null) return null;
    try {
      final dt = DateTime.parse(raw.toString()).toLocal();
      final now = DateTime.now();
      final isToday = dt.year == now.year && dt.month == now.month && dt.day == now.day;
      final hour = dt.hour % 12 == 0 ? 12 : dt.hour % 12;
      final min = dt.minute.toString().padLeft(2, '0');
      final ampm = dt.hour >= 12 ? 'PM' : 'AM';
      final time = '$hour:$min $ampm';
      return isToday ? 'Today, $time' : '${dt.day}/${dt.month}, $time';
    } catch (_) {
      return null;
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
    final items = _filteredItems;
    return Scaffold(
      backgroundColor: pageBg,
      body: SafeArea(
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 0),
              child: Row(
                children: [
                  const Expanded(
                    child: Text(
                      'My Deliveries',
                      style: TextStyle(fontSize: 20, fontWeight: FontWeight.bold, color: Colors.black87),
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.notifications_none, color: Colors.black87),
                    onPressed: () {
                      Navigator.push(
                        context,
                        MaterialPageRoute(builder: (_) => const NotificationsScreen()),
                      );
                    },
                  ),
                ],
              ),
            ),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              child: Row(
                children: [
                  _filterChip('New', 'new'),
                  const SizedBox(width: 8),
                  _filterChip('Active', 'active'),
                  const SizedBox(width: 8),
                  _filterChip('Completed', 'completed'),
                  const SizedBox(width: 8),
                  _filterChip('All', 'all'),
                ],
              ),
            ),
            Expanded(
              child: _loading
                  ? const Center(child: CircularProgressIndicator())
                  : _error != null
                      ? Center(child: Text(_error!))
                      : items.isEmpty
                          ? const Center(child: Text('No deliveries here', style: TextStyle(color: Colors.black45)))
                          : RefreshIndicator(
                              onRefresh: _loadAll,
                              child: ListView.builder(
                                padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
                                itemCount: items.length,
                                itemBuilder: (context, index) {
                                  final item = items[index];
                                  final isReturn = _isReturn(item);
                                  final id = _idOf(item);
                                  final status = _statusOf(item);
                                  final isActing = _actingIds.contains(id);
                                  final isNewItem = _isNew(item);

                                  return GestureDetector(
                                    onTap: () => _openDetail(item),
                                    child: Container(
                                      margin: const EdgeInsets.only(bottom: 10),
                                      padding: const EdgeInsets.all(14),
                                      decoration: BoxDecoration(color: cardBg, borderRadius: BorderRadius.circular(16)),
                                      child: Column(
                                        crossAxisAlignment: CrossAxisAlignment.start,
                                        children: [
                                          Row(
                                            children: [
                                              Container(
                                                padding: const EdgeInsets.all(8),
                                                decoration: BoxDecoration(
                                                  color: Colors.white,
                                                  borderRadius: BorderRadius.circular(10),
                                                ),
                                                child: Icon(_statusIcon(item), size: 18, color: _statusColor(status)),
                                              ),
                                              const SizedBox(width: 10),
                                              Expanded(
                                                child: Column(
                                                  crossAxisAlignment: CrossAxisAlignment.start,
                                                  children: [
                                                    Text(
                                                      isReturn ? 'Return - Order #${item['order_id']}' : 'Order #$id',
                                                      style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 15),
                                                    ),
                                                    if (_formatTime(item['created_at']) != null)
                                                      Text(
                                                        _formatTime(item['created_at'])!,
                                                        style: const TextStyle(fontSize: 11, color: Colors.black45),
                                                      ),
                                                  ],
                                                ),
                                              ),
                                              Container(
                                                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                                                decoration: BoxDecoration(
                                                  color: _statusColor(status).withValues(alpha: 0.12),
                                                  borderRadius: BorderRadius.circular(10),
                                                ),
                                                child: Text(
                                                  (isReturn ? (status.isEmpty ? 'ASSIGNED' : status) : status).toUpperCase(),
                                                  style: TextStyle(fontSize: 10, fontWeight: FontWeight.bold, color: _statusColor(status)),
                                                ),
                                              ),
                                            ],
                                          ),
                                          if (!isReturn && item['total_amount'] != null) ...[
                                            const SizedBox(height: 8),
                                            Text(
                                              '\u20B9${_money(item['total_amount'])} \u00B7 ${(item['payment_method'] ?? '').toString().toUpperCase()}',
                                              style: const TextStyle(fontSize: 13, color: Colors.black54),
                                            ),
                                          ],
                                          if (isReturn && item['refund_amount'] != null) ...[
                                            const SizedBox(height: 8),
                                            Text(
                                              'Refund: \u20B9${_money(item['refund_amount'])}',
                                              style: const TextStyle(fontSize: 13, color: Colors.black54),
                                            ),
                                          ],
                                          if (isNewItem) ...[
                                            const SizedBox(height: 10),
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
                                            ),
                                          ],
                                        ],
                                      ),
                                    ),
                                  );
                                },
                              ),
                            ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _filterChip(String label, String value) {
    final selected = _filter == value;
    return GestureDetector(
      onTap: () => setState(() => _filter = value),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
        decoration: BoxDecoration(
          color: selected ? primaryPurple : Colors.white,
          borderRadius: BorderRadius.circular(20),
        ),
        child: Text(
          label,
          style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: selected ? Colors.white : Colors.black54),
        ),
      ),
    );
  }
}

String _money(dynamic v) {
  final n = v is num ? v : num.tryParse('${v ?? ''}');
  if (n == null) return '-';
  return n.toStringAsFixed(2);
}
