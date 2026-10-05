import 'dart:async';
import 'package:flutter/material.dart';
import '../services/api_service.dart';
import 'delivery_complete_screen.dart';
import 'delivery_handover_screen.dart';
import 'delivery_map_screen.dart';
import 'order_pickup_screen.dart';
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

  List<dynamic> _itemsFor(String filter) {
    final merged = _merged;
    if (filter == 'new') return merged.where(_isNew).toList();
    if (filter == 'active') {
      return merged.where((i) => !_isCompleted(i) && !_isCancelled(i)).toList();
    }
    if (filter == 'completed') return merged.where(_isCompleted).toList();
    return merged;
  }

  List<dynamic> get _filteredItems => _itemsFor(_filter);

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
      case 'cancelled':
        return const Color(0xFFEF4444);
      default:
        return Colors.grey;
    }
  }

  IconData _statusIcon(dynamic item) {
    if (_isReturn(item)) return Icons.assignment_return_outlined;
    final status = _statusOf(item);
    if (status == 'delivered') return Icons.check_circle_outline;
    if (status == 'shipped') return Icons.local_shipping_outlined;
    if (status == 'cancelled') return Icons.cancel_outlined;
    return Icons.assignment_outlined;
  }

  DateTime? _dateOf(dynamic item) {
    final raw = item['created_at'];
    if (raw == null) return null;
    return DateTime.tryParse(raw.toString())?.toLocal();
  }

  bool _isToday(DateTime? dt) {
    if (dt == null) return false;
    final now = DateTime.now();
    return dt.year == now.year && dt.month == now.month && dt.day == now.day;
  }

  String? _formatTime(dynamic raw) {
    if (raw == null) return null;
    try {
      final dt = DateTime.parse(raw.toString()).toLocal();
      final hour = dt.hour % 12 == 0 ? 12 : dt.hour % 12;
      final min = dt.minute.toString().padLeft(2, '0');
      final ampm = dt.hour >= 12 ? 'PM' : 'AM';
      final time = '$hour:$min $ampm';
      return _isToday(dt) ? 'Today, $time' : '${dt.day}/${dt.month}, $time';
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
      final m = Map<String, dynamic>.from(item as Map);
      final ds = m['delivery_status']?.toString();
      Widget? next;
      if (ds == 'accepted' || ds == 'going_to_store' || ds == 'arrived_at_store') {
        next = OrderPickupScreen(order: m);
      } else if (ds == 'picked_up' || ds == 'out_for_delivery') {
        next = DeliveryMapScreen(order: m);
      } else if (ds == 'arrived' || ds == 'arrived_at_customer') {
        next = DeliveryHandoverScreen(order: m);
      } else if (m['status']?.toString() == 'delivered') {
        next = DeliveryCompleteScreen(order: m);
      } else if (ds == 'delivered') {
        next = DeliveryHandoverScreen(order: m);
      }
      if (next != null) {
        final screen = next;
        Navigator.push(context, MaterialPageRoute(builder: (_) => screen))
            .then((_) => _loadAll());
      }
    }
  }

  // Flat list: String entries are section headers, others are items.
  List<dynamic> _withHeaders(List<dynamic> items) {
    final today = items.where((i) => _isToday(_dateOf(i))).toList();
    final earlier = items.where((i) => !_isToday(_dateOf(i))).toList();
    return [
      if (today.isNotEmpty) ...['Today', ...today],
      if (earlier.isNotEmpty) ...[today.isNotEmpty ? 'Earlier' : 'All orders', ...earlier],
    ];
  }

  @override
  Widget build(BuildContext context) {
    final entries = _withHeaders(_filteredItems);
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
                      style: TextStyle(fontSize: 24, fontWeight: FontWeight.bold, color: Colors.black87),
                    ),
                  ),
                ],
              ),
            ),
            SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 8),
              child: Row(
                children: [
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
                      : RefreshIndicator(
                          onRefresh: _loadAll,
                          child: entries.isEmpty
                              ? ListView(
                                  children: [
                                    SizedBox(height: MediaQuery.of(context).size.height * 0.2),
                                    const Icon(Icons.inbox_outlined, size: 64, color: Colors.black26),
                                    const SizedBox(height: 12),
                                    const Center(
                                      child: Text('No orders here',
                                          style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600, color: Colors.black45)),
                                    ),
                                    const SizedBox(height: 4),
                                    const Center(
                                      child: Text('Pull down to refresh', style: TextStyle(fontSize: 12, color: Colors.black38)),
                                    ),
                                  ],
                                )
                              : ListView.builder(
                                  padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
                                  itemCount: entries.length,
                                  itemBuilder: (context, index) {
                                    final entry = entries[index];
                                    if (entry is String) {
                                      return Padding(
                                        padding: const EdgeInsets.fromLTRB(2, 12, 0, 8),
                                        child: Text(
                                          entry.toUpperCase(),
                                          style: const TextStyle(
                                              fontSize: 12, fontWeight: FontWeight.bold, color: Colors.black45, letterSpacing: 0.8),
                                        ),
                                      );
                                    }
                                    return _orderCard(entry);
                                  },
                                ),
                        ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _chip(String text, Color color) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(text, style: TextStyle(fontSize: 10, fontWeight: FontWeight.bold, color: color)),
    );
  }

  Widget _orderCard(dynamic item) {
    final isReturn = _isReturn(item);
    final id = _idOf(item);
    final status = _statusOf(item);
    final color = _statusColor(status);
    final isActing = _actingIds.contains(id);
    final isNewItem = _isNew(item);
    final statusLabel = (status.isEmpty ? 'assigned' : status).replaceAll('_', ' ').toUpperCase();
    final pay = (item['payment_method'] ?? '').toString().toLowerCase();

    return GestureDetector(
      onTap: () => _openDetail(item),
      child: Container(
        margin: const EdgeInsets.only(bottom: 12),
        decoration: BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.circular(16),
          boxShadow: [
            BoxShadow(color: Colors.black.withValues(alpha: 0.05), blurRadius: 10, offset: const Offset(0, 3)),
          ],
        ),
        child: ClipRRect(
          borderRadius: BorderRadius.circular(16),
          child: IntrinsicHeight(
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Container(width: 5, color: color),
                Expanded(
                  child: Padding(
                    padding: const EdgeInsets.all(14),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Container(
                              padding: const EdgeInsets.all(8),
                              decoration: BoxDecoration(
                                color: color.withValues(alpha: 0.12),
                                borderRadius: BorderRadius.circular(10),
                              ),
                              child: Icon(_statusIcon(item), size: 20, color: color),
                            ),
                            const SizedBox(width: 12),
                            Expanded(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(
                                    isReturn ? 'Return - Order #${item['order_id']}' : 'Order #$id',
                                    style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 16),
                                  ),
                                  if (_formatTime(item['created_at']) != null)
                                    Text(
                                      _formatTime(item['created_at'])!,
                                      style: const TextStyle(fontSize: 12, color: Colors.black45),
                                    ),
                                ],
                              ),
                            ),
                            _chip(statusLabel, color),
                          ],
                        ),
                        if (!isReturn && item['total_amount'] != null) ...[
                          const SizedBox(height: 12),
                          Row(
                            children: [
                              Text(
                                '\u20B9${_money(item['total_amount'])}',
                                style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold, color: Colors.black87),
                              ),
                              const SizedBox(width: 8),
                              if (pay == 'cod')
                                _chip('COD', const Color(0xFFF59E0B))
                              else if (pay.isNotEmpty)
                                _chip('PAID', const Color(0xFF22C55E)),
                              const Spacer(),
                              const Icon(Icons.chevron_right, color: Colors.black26),
                            ],
                          ),
                        ],
                        if (isReturn && item['refund_amount'] != null) ...[
                          const SizedBox(height: 12),
                          Row(
                            children: [
                              Text(
                                'Refund \u20B9${_money(item['refund_amount'])}',
                                style: const TextStyle(fontSize: 16, fontWeight: FontWeight.bold, color: Colors.black87),
                              ),
                              const Spacer(),
                              const Icon(Icons.chevron_right, color: Colors.black26),
                            ],
                          ),
                        ],
                        if (isNewItem) ...[
                          const SizedBox(height: 12),
                          Row(
                            children: [
                              Expanded(
                                child: OutlinedButton(
                                  onPressed: isActing
                                      ? null
                                      : () => isReturn ? _rejectReturn(id) : _rejectOrder(id),
                                  style: OutlinedButton.styleFrom(
                                    foregroundColor: Colors.red,
                                    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
                                  ),
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
                          ),
                        ],
                      ],
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _filterChip(String label, String value) {
    final selected = _filter == value;
    final count = _itemsFor(value).length;
    return GestureDetector(
      onTap: () => setState(() => _filter = value),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
        decoration: BoxDecoration(
          color: selected ? primaryPurple : Colors.white,
          borderRadius: BorderRadius.circular(20),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              label,
              style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: selected ? Colors.white : Colors.black54),
            ),
            if (count > 0) ...[
              const SizedBox(width: 6),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 1),
                decoration: BoxDecoration(
                  color: selected ? Colors.white.withValues(alpha: 0.25) : primaryPurple.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(10),
                ),
                child: Text(
                  '$count',
                  style: TextStyle(fontSize: 11, fontWeight: FontWeight.bold, color: selected ? Colors.white : primaryPurple),
                ),
              ),
            ],
          ],
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