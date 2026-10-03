import 'dart:async';
import 'package:flutter/material.dart';
import 'delivery_complete_screen.dart';
import 'package:image_picker/image_picker.dart';
import '../services/api_service.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart';
import '../widgets/mini_tracking_map.dart';
import 'tracking_map_screen.dart';
import 'package:geolocator/geolocator.dart';

class OrderDetailScreen extends StatefulWidget {
  final Map<String, dynamic> order;
  const OrderDetailScreen({super.key, required this.order});

  @override
  State<OrderDetailScreen> createState() => _OrderDetailScreenState();
}

class _OrderDetailScreenState extends State<OrderDetailScreen> {
  late Map<String, dynamic> _order;
  late final int _orderId;
  bool _loading = false;
  String? _error;
  bool _uploadingProof = false;
  Map<String, dynamic>? _rating;

  Future<void> _loadRating() async {
    try {
      final id = int.tryParse('${widget.order['order_id'] ?? widget.order['id']}');
      if (id == null) return;
      final d = await ApiService.getOrderRating(id);
      if (mounted && d['rated'] == true) setState(() => _rating = d);
    } catch (_) {}
  }

  @override
  void initState() {
    super.initState();
    _order = widget.order;
    _loadRating();
    // Cache the id from the initial list summary (keyed "order_id") once.
    // Action responses below return the raw Order model instead (keyed
    // "id" with a different field set entirely), so re-reading
    // _order['order_id'] after any action would be null - see BUG-37.
    _orderId = (_order['order_id'] as num).toInt();
  }

  Future<void> _callCustomer(String phone) async {
    final uri = Uri(scheme: 'tel', path: phone);
    if (await canLaunchUrl(uri)) {
      await launchUrl(uri);
    } else if (mounted) {
      setState(() => _error = 'Could not open dialer');
    }
  }
  // Merges fields from a raw Order response (returned by the action
  // endpoints below) into the existing summary-shaped _order map,
  // translating field names that differ between the two shapes
  // (e.g. "delivery_assignment_status" -> "assignment_status").
  // Never replaces _order wholesale - see BUG-37.
  Future<void> _pickAndUploadProof() async {
    final picker = ImagePicker();
    final photo = await picker.pickImage(source: ImageSource.camera, imageQuality: 80);
    if (photo == null) return;
    setState(() {
      _uploadingProof = true;
      _error = null;
    });
    try {
      final data = await ApiService.uploadDeliveryProof(_orderId, photo.path);
      setState(() {
        _order = {..._order, 'delivery_proof_url': data['delivery_proof_url']};
        _uploadingProof = false;
      });
    } catch (e) {
      setState(() {
        _error = e.toString().replaceFirst('Exception: ', '');
        _uploadingProof = false;
      });
    }
  }

  void _applyOrderUpdate(Map<String, dynamic> rawOrder) {
    setState(() {
      _order = {
        ..._order,
        'status': rawOrder['status'] ?? _order['status'],
        'assignment_status': rawOrder['delivery_assignment_status'],
        'rejection_reason': rawOrder['delivery_rejection_reason'],
        'assignment_expires_at': rawOrder['delivery_assignment_expires_at'],
        'delivery_status': rawOrder['delivery_status'],
      };
    });
  }

  Future<void> _markShipped() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final data = await ApiService.markShipped(_orderId);
      _applyOrderUpdate(data['order']);
      setState(() {
        _loading = false;
      });
    } catch (e) {
      setState(() {
        _error = e.toString().replaceFirst('Exception: ', '');
        _loading = false;
      });
    }
  }

  Future<void> _advanceDeliveryStatus(String status, {String? otp}) async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final data = await ApiService.updateDeliveryStatus(_orderId, status, otp: otp);
      _applyOrderUpdate(data['order']);
      if (status == 'delivered') {
        final confirmData = await ApiService.confirmDelivery(_orderId);
        _applyOrderUpdate(confirmData['order']);
        if (!mounted) return;
        Navigator.of(context).pushReplacement(
          MaterialPageRoute(builder: (_) => DeliveryCompleteScreen(order: _order)),
        );
        return;
      }
      setState(() => _loading = false);
    } catch (e) {
      setState(() {
        _error = e.toString().replaceFirst('Exception: ', '');
        _loading = false;
      });
    }
  }

  Future<void> _markDeliveryFailed() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Mark delivery as failed?'),
        content: const Text('Use this only if the customer refused, was unavailable, or delivery genuinely could not be completed.'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Cancel')),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Yes, mark failed', style: TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    await _advanceDeliveryStatus('failed_delivery');
  }

  Future<void> _resolveFailedDelivery(String action) async {
    final reasonController = TextEditingController();
    final reason = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(action == 'retry' ? 'Retry delivery' : 'Return to store'),
        content: TextField(
          controller: reasonController,
          decoration: const InputDecoration(hintText: 'Reason (required)'),
          maxLines: 2,
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Cancel')),
          TextButton(
            onPressed: () => Navigator.pop(ctx, reasonController.text.trim()),
            child: Text(action == 'retry' ? 'Retry' : 'Return'),
          ),
        ],
      ),
    );
    if (reason == null || reason.isEmpty) return;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final data = await ApiService.resolveFailedDelivery(_orderId, action, reason);
      _applyOrderUpdate(data['order']);
      setState(() => _loading = false);
    } catch (e) {
      setState(() {
        _error = e.toString().replaceFirst('Exception: ', '');
        _loading = false;
      });
    }
  }

  Future<void> _acceptAssignment() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final data = await ApiService.acceptAssignment(_orderId);
      _applyOrderUpdate(data['order']);
      setState(() {
        _loading = false;
      });
    } catch (e) {
      setState(() {
        _error = e.toString().replaceFirst('Exception: ', '');
        _loading = false;
      });
    }
  }

  Future<void> _rejectAssignment() async {
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

    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final data = await ApiService.rejectAssignment(_orderId, reason: reasonController.text);
      _applyOrderUpdate(data['order']);
      setState(() {
        _loading = false;
      });
    } catch (e) {
      setState(() {
        _error = e.toString().replaceFirst('Exception: ', '');
        _loading = false;
      });
    }
  }

    static const List<Map<String, String>> _stepDefs = [
    {'key': 'going_to_store', 'label': 'Going to Store'},
    {'key': 'arrived_at_store', 'label': 'Arrived at Store'},
    {'key': 'picked_up', 'label': 'Picked Up'},
    {'key': 'out_for_delivery', 'label': 'Out for Delivery'},
    {'key': 'arrived_at_customer', 'label': 'Arrived at Customer'},
    {'key': 'delivered', 'label': 'Delivered'},
  ];

  int _currentStepIndex(String? deliveryStatus, String orderStatus) {
    if (orderStatus == 'delivered') return _stepDefs.length - 1;
    if (deliveryStatus == null) return -1;
    // Legacy "arrived" maps to the same visual position as
    // "arrived_at_customer" - both mean the courier is at the customer.
    final effective = deliveryStatus == 'arrived' ? 'arrived_at_customer' : deliveryStatus;
    return _stepDefs.indexWhere((s) => s['key'] == effective);
  }

  Widget _buildStepper(String? deliveryStatus, String orderStatus) {
    final currentIndex = _currentStepIndex(deliveryStatus, orderStatus);
    if (currentIndex < 0) return const SizedBox.shrink();

    return Container(
      margin: const EdgeInsets.only(bottom: 16),
      padding: const EdgeInsets.symmetric(vertical: 8, horizontal: 4),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: List.generate(_stepDefs.length, (stepIndex) {
          final isDone = stepIndex < currentIndex;
          final isCurrent = stepIndex == currentIndex;
          final isLast = stepIndex == _stepDefs.length - 1;
          final label = _stepDefs[stepIndex]['label']!;
          final lineDone = stepIndex < currentIndex;

          return IntrinsicHeight(
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Column(
                  children: [
                    Container(
                      width: 26,
                      height: 26,
                      decoration: BoxDecoration(
                        shape: BoxShape.circle,
                        color: isDone || isCurrent ? const Color(0xFF5B2A9E) : Colors.white,
                        border: Border.all(
                          color: isDone || isCurrent ? const Color(0xFF5B2A9E) : const Color(0xFFE0DAF0),
                          width: 2,
                        ),
                      ),
                      child: isDone
                          ? const Icon(Icons.check, size: 14, color: Colors.white)
                          : isCurrent
                              ? Center(
                                  child: Container(
                                    width: 8,
                                    height: 8,
                                    decoration: const BoxDecoration(shape: BoxShape.circle, color: Colors.white),
                                  ),
                                )
                              : null,
                    ),
                    if (!isLast)
                      Expanded(
                        child: Container(
                          width: 2,
                          margin: const EdgeInsets.symmetric(vertical: 2),
                          color: lineDone ? const Color(0xFF5B2A9E) : const Color(0xFFE0DAF0),
                        ),
                      ),
                  ],
                ),
                const SizedBox(width: 12),
                Padding(
                  padding: const EdgeInsets.only(bottom: 20, top: 3),
                  child: Text(
                    label,
                    style: TextStyle(
                      fontSize: 14,
                      fontWeight: isCurrent ? FontWeight.bold : FontWeight.normal,
                      color: isDone || isCurrent ? Colors.black87 : Colors.black45,
                    ),
                  ),
                ),
              ],
            ),
          );
        }),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final status = _order['status'] ?? '';
    final assignmentStatus = _order['assignment_status'];
    final rejectionReason = _order['rejection_reason'];
    final expiresAt = _order['assignment_expires_at'];
    final deliveryAddress = _order['delivery_address'] ?? '';
    final customerName = _order['customer_name'] ?? '';
    final customerPhone = _order['customer_phone'] ?? '';
    final itemCount = _order['item_count'] ?? 0;

    final isPendingResponse = assignmentStatus == 'assigned';
    final isRejected = assignmentStatus == 'rejected';
    final isExpired = assignmentStatus == 'expired';
    final canProgressOrder = assignmentStatus == null || assignmentStatus == 'accepted';

    return Scaffold(
      appBar: AppBar(title: Text('Order #$_orderId')),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          if (isPendingResponse)
            Container(
              margin: const EdgeInsets.only(bottom: 12),
              padding: const EdgeInsets.all(14),
              decoration: BoxDecoration(
                color: const Color(0xFFFFF4E5),
                borderRadius: BorderRadius.circular(12),
                border: Border.all(color: Colors.orange.shade200),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      const Icon(Icons.timer_outlined, color: Colors.orange, size: 20),
                      const SizedBox(width: 8),
                      const Text(
                        'New delivery request',
                        style: TextStyle(fontWeight: FontWeight.bold, fontSize: 15),
                      ),
                      const Spacer(),
                      _CountdownText(expiresAt: expiresAt?.toString()),
                    ],
                  ),
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      Expanded(
                        child: OutlinedButton(
                          onPressed: _loading ? null : _rejectAssignment,
                          style: OutlinedButton.styleFrom(
                            foregroundColor: Colors.red,
                            side: const BorderSide(color: Colors.red),
                            padding: const EdgeInsets.symmetric(vertical: 12),
                          ),
                          child: const Text('Reject'),
                        ),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: ElevatedButton(
                          onPressed: _loading ? null : _acceptAssignment,
                          style: ElevatedButton.styleFrom(
                            backgroundColor: Colors.green,
                            padding: const EdgeInsets.symmetric(vertical: 12),
                          ),
                          child: _loading
                              ? const SizedBox(
                                  height: 18,
                                  width: 18,
                                  child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                                )
                              : const Text('Accept'),
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          if (isRejected)
            Container(
              margin: const EdgeInsets.only(bottom: 12),
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: const Color(0xFFFDEAEA),
                borderRadius: BorderRadius.circular(12),
              ),
              child: Text(
                rejectionReason != null && rejectionReason.toString().isNotEmpty
                    ? 'You rejected this delivery: $rejectionReason'
                    : 'You rejected this delivery.',
                style: const TextStyle(color: Colors.red),
              ),
            ),
          if (isExpired)
            Container(
              margin: const EdgeInsets.only(bottom: 12),
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: const Color(0xFFF0F0F0),
                borderRadius: BorderRadius.circular(12),
              ),
              child: const Text(
                'This assignment expired without a response.',
                style: TextStyle(color: Colors.black54),
              ),
            ),
          Card(
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    customerName,
                    style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 16),
                  ),
                  const SizedBox(height: 4),
                  Row(                     children: [                       Text(customerPhone),                       const SizedBox(width: 8),                       if (customerPhone.toString().isNotEmpty)                         InkWell(                           onTap: () => _callCustomer(customerPhone.toString()),                           borderRadius: BorderRadius.circular(20),                           child: const Padding(                             padding: EdgeInsets.all(4),                             child: Icon(Icons.call, color: Color(0xFF5B2A9E), size: 20),                           ),                         ),                     ],                   ),
                  const SizedBox(height: 8),
                  Text(deliveryAddress),
                    if (_order['delivery_lat'] != null && _order['delivery_lng'] != null) ...[
                      const SizedBox(height: 8),
                      MiniTrackingMap(
                        destination: LatLng(
                          (_order['delivery_lat'] as num).toDouble(),
                          (_order['delivery_lng'] as num).toDouble(),
                        ),
                        partner: null,
                        onTap: () => Navigator.push(context, MaterialPageRoute(
                          builder: (_) => TrackingMapScreen(
                            title: 'Navigate to customer',
                            destination: LatLng(
                              (_order['delivery_lat'] as num).toDouble(),
                              (_order['delivery_lng'] as num).toDouble(),
                            ),
                            partnerStream: Geolocator.getPositionStream(
                              locationSettings: const LocationSettings(
                                accuracy: LocationAccuracy.high,
                                distanceFilter: 10,
                              ),
                            ).map((p) => LatLng(p.latitude, p.longitude)),
                            bottom: Padding(
                              padding: const EdgeInsets.all(16),
                              child: ElevatedButton.icon(
                                onPressed: () => _callCustomer(customerPhone.toString()),
                                icon: const Icon(Icons.phone),
                                label: const Text('Call customer'),
                              ),
                            ),
                          ),
                        )),
                      ),
                    ],
                ],
              ),
            ),
          ),
          const SizedBox(height: 12),
          Card(
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('Items ($itemCount)', style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
                  const Divider(),
                  ...(_order['items'] as List<dynamic>? ?? []).map((item) => Padding(
                    padding: const EdgeInsets.symmetric(vertical: 4),
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        Expanded(child: Text("${item['product_name']} x${item['quantity']}")),
                        Text("\u20B9${item['price']}"),
                      ],
                    ),
                  )),

                  const Divider(),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      const Text('Total', style: TextStyle(fontWeight: FontWeight.bold)),
                      Text(
                        '\u20B9${_money(_order['total_amount'])}',
                        style: const TextStyle(fontWeight: FontWeight.bold),
                      ),
                    ],
                  ),
                  Text('Payment: ${_order['payment_method']?.toString().toUpperCase() ?? ''}'),
                ],
              ),
            ),
          ),
          if (_error != null) ...[
            const SizedBox(height: 12),
            Text(_error!, style: const TextStyle(color: Colors.red)),
          ],
          if (status == 'delivered' ||
              (_order['delivery_status'] != null && _order['delivery_status'] != 'assigned'))
            _buildStepper(_order['delivery_status']?.toString(), status),
          if (_rating != null) ...[
            const SizedBox(height: 12),
            Card(
              child: Padding(
                padding: const EdgeInsets.all(12),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text('Customer Rating', style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
                    const SizedBox(height: 6),
                    Row(
                      children: List.generate(
                        5,
                        (i) => Icon(
                          i < ((_rating!['rating'] as num?)?.toInt() ?? 0) ? Icons.star : Icons.star_border,
                          color: Colors.amber,
                          size: 22,
                        ),
                      ),
                    ),
                    if ((_rating!['review'] ?? '').toString().isNotEmpty) ...[
                      const SizedBox(height: 6),
                      Text(_rating!['review'].toString()),
                    ],
                  ],
                ),
              ),
            ),
          ],
          const SizedBox(height: 20),
          if (canProgressOrder &&
              status == 'handed_over' &&
              (_order['delivery_status'] == null || _order['delivery_status'] == 'assigned' || _order['delivery_status'] == 'accepted'))
            ElevatedButton(
              onPressed: _loading ? null : () => _advanceDeliveryStatus('going_to_store'),
              style: ElevatedButton.styleFrom(padding: const EdgeInsets.all(16)),
              child: _loading
                  ? const SizedBox(height: 20, width: 20, child: CircularProgressIndicator(strokeWidth: 2))
                  : const Text('Start Delivery (Going to Store)'),
            )
          else if (canProgressOrder &&
              _order['delivery_status'] != null &&
              _order['delivery_status'] != 'assigned' &&
              _order['delivery_status'] != 'delivered')
            Builder(builder: (context) {
              final deliveryStatus = _order['delivery_status'];
              switch (deliveryStatus) {
                case 'going_to_store':
                  return ElevatedButton(
                    onPressed: _loading ? null : () => _advanceDeliveryStatus('arrived_at_store'),
                    style: ElevatedButton.styleFrom(padding: const EdgeInsets.all(16)),
                    child: _loading
                        ? const SizedBox(height: 20, width: 20, child: CircularProgressIndicator(strokeWidth: 2))
                        : const Text('Arrived at Store'),
                  );
                case 'arrived_at_store':
                  return ElevatedButton(
                    onPressed: _loading ? null : () => _advanceDeliveryStatus('picked_up'),
                    style: ElevatedButton.styleFrom(padding: const EdgeInsets.all(16)),
                    child: _loading
                        ? const SizedBox(height: 20, width: 20, child: CircularProgressIndicator(strokeWidth: 2))
                        : const Text('Confirm Pickup'),
                  );
                case 'picked_up':
                  return ElevatedButton(
                    onPressed: _loading ? null : () => _advanceDeliveryStatus('out_for_delivery'),
                    style: ElevatedButton.styleFrom(padding: const EdgeInsets.all(16)),
                    child: _loading
                        ? const SizedBox(height: 20, width: 20, child: CircularProgressIndicator(strokeWidth: 2))
                        : const Text('Out for Delivery'),
                  );
                case 'out_for_delivery':
                  return ElevatedButton(
                    onPressed: _loading ? null : () => _advanceDeliveryStatus('arrived_at_customer'),
                    style: ElevatedButton.styleFrom(padding: const EdgeInsets.all(16)),
                    child: _loading
                        ? const SizedBox(height: 20, width: 20, child: CircularProgressIndicator(strokeWidth: 2))
                        : const Text('Arrived at Location'),
                  );
                case 'arrived_at_customer':
                  final isCOD = (_order['payment_method']?.toString().toLowerCase() ?? '') == 'cod';
                  final amount = _order['total_amount'];
                  return Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Container(
                        padding: const EdgeInsets.all(14),
                        margin: const EdgeInsets.only(bottom: 12),
                        decoration: BoxDecoration(
                          color: isCOD ? const Color(0xFFFFF4E5) : const Color(0xFFE1F5E6),
                          borderRadius: BorderRadius.circular(12),
                        ),
                        child: isCOD
                            ? Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  const Text('CASH ON DELIVERY', style: TextStyle(fontWeight: FontWeight.bold, fontSize: 13)),
                                  const SizedBox(height: 6),
                                  Text('Amount to Collect: \u20B9${_money(amount)}'),
                                ],
                              )
                            : Row(
                                children: [
                                  const Icon(Icons.check_circle, color: Colors.green, size: 20),
                                  const SizedBox(width: 8),
                                  Expanded(child: Text('Payment already received (\u20B9${_money(amount)})')),
                                ],
                              ),
                      ),
                      if ((_order['delivery_proof_url'] as String?) == null || (_order['delivery_proof_url'] as String).isEmpty)
                        OutlinedButton.icon(
                          onPressed: _uploadingProof ? null : _pickAndUploadProof,
                          icon: _uploadingProof
                              ? const SizedBox(height: 16, width: 16, child: CircularProgressIndicator(strokeWidth: 2))
                              : const Icon(Icons.camera_alt),
                          label: Text(_uploadingProof ? 'Uploading...' : 'Take Delivery Proof Photo'),
                          style: OutlinedButton.styleFrom(padding: const EdgeInsets.all(14)),
                        )
                      else
                        Container(
                          padding: const EdgeInsets.all(12),
                          margin: const EdgeInsets.only(bottom: 4),
                          decoration: BoxDecoration(color: const Color(0xFFE1F5E6), borderRadius: BorderRadius.circular(10)),
                          child: Row(
                            children: const [
                              Icon(Icons.check_circle, color: Colors.green, size: 18),
                              SizedBox(width: 8),
                              Text('Delivery proof photo captured'),
                            ],
                          ),
                        ),
                      const SizedBox(height: 8),
                      ElevatedButton(
                        onPressed: (_loading || (_order['delivery_proof_url'] as String?) == null || (_order['delivery_proof_url'] as String).isEmpty)
                            ? null
                            : () => _advanceDeliveryStatus('delivered'),
                        style: ElevatedButton.styleFrom(padding: const EdgeInsets.all(16), backgroundColor: Colors.green),
                        child: _loading
                            ? const SizedBox(height: 20, width: 20, child: CircularProgressIndicator(strokeWidth: 2))
                            : const Text('Confirm Delivery'),
                      ),
                      const SizedBox(height: 8),
                      OutlinedButton(
                        onPressed: _loading ? null : _markDeliveryFailed,
                        style: OutlinedButton.styleFrom(foregroundColor: Colors.red, padding: const EdgeInsets.all(14)),
                        child: const Text('Delivery Failed'),
                      ),
                    ],
                  );
                case 'failed_delivery':
                  return Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Container(
                        padding: const EdgeInsets.all(14),
                        margin: const EdgeInsets.only(bottom: 12),
                        decoration: BoxDecoration(color: const Color(0xFFFDEAEA), borderRadius: BorderRadius.circular(12)),
                        child: const Text('This delivery could not be completed. Choose how to proceed.'),
                      ),
                      ElevatedButton(
                        onPressed: _loading ? null : () => _resolveFailedDelivery('retry'),
                        style: ElevatedButton.styleFrom(padding: const EdgeInsets.all(16)),
                        child: const Text('Retry Delivery'),
                      ),
                      const SizedBox(height: 8),
                      OutlinedButton(
                        onPressed: _loading ? null : () => _resolveFailedDelivery('return'),
                        style: OutlinedButton.styleFrom(padding: const EdgeInsets.all(14)),
                        child: const Text('Return to Store'),
                      ),
                    ],
                  );
                case 'returned':
                  return const Center(
                    child: Padding(
                      padding: EdgeInsets.symmetric(vertical: 16),
                      child: Text('Returned to store', style: TextStyle(color: Colors.orange, fontSize: 18, fontWeight: FontWeight.bold)),
                    ),
                  );
                default:
                  return ElevatedButton(
                    onPressed: _loading ? null : () => _advanceDeliveryStatus('going_to_store'),
                    style: ElevatedButton.styleFrom(padding: const EdgeInsets.all(16)),
                    child: _loading
                        ? const SizedBox(height: 20, width: 20, child: CircularProgressIndicator(strokeWidth: 2))
                        : const Text('Going to Store'),
                  );
              }
            })
          else if (canProgressOrder && status != 'delivered')
            const Center(
              child: Padding(
                padding: EdgeInsets.symmetric(vertical: 16),
                child: Text(
                  'Order accepted. Waiting for the store to hand it over.',
                  style: TextStyle(color: Colors.black54, fontSize: 15),
                ),
              ),
            )
          else if (status == 'delivered')
            const Center(
              child: Text('Delivered', style: TextStyle(color: Colors.green, fontSize: 18)),
            )
          else if (_order['delivery_status'] == 'returned')
            const Center(
              child: Text('Returned to store', style: TextStyle(color: Colors.orange, fontSize: 18)),
            ),
        ],
      ),
    );
  }
}

class _CountdownText extends StatefulWidget {
  final String? expiresAt;
  const _CountdownText({required this.expiresAt});

  @override
  State<_CountdownText> createState() => _CountdownTextState();
}

class _CountdownTextState extends State<_CountdownText> {
  Timer? _timer;
  Duration? _remaining;

  @override
  void initState() {
    super.initState();
    _tick();
    _timer = Timer.periodic(const Duration(seconds: 1), (_) => _tick());
  }

  void _tick() {
    if (widget.expiresAt == null) {
      if (mounted) setState(() => _remaining = null);
      return;
    }
    try {
      final expiry = DateTime.parse(widget.expiresAt!).toLocal();
      final diff = expiry.difference(DateTime.now());
      if (mounted) setState(() => _remaining = diff.isNegative ? Duration.zero : diff);
    } catch (_) {
      if (mounted) setState(() => _remaining = null);
    }
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (_remaining == null) {
      return const Text(
        'Awaiting response',
        style: TextStyle(fontSize: 12, color: Colors.orange, fontWeight: FontWeight.w600),
      );
    }
    final m = _remaining!.inMinutes;
    final s = _remaining!.inSeconds % 60;
    final label = _remaining! == Duration.zero ? 'Expiring...' : '${m}m ${s.toString().padLeft(2, '0')}s left';
    return Text(
      label,
      style: TextStyle(
        fontSize: 12,
        fontWeight: FontWeight.w700,
        color: _remaining!.inSeconds < 30 ? Colors.red : Colors.orange,
      ),
    );
  }
}

String _money(dynamic v) {
  final n = v is num ? v : num.tryParse('${v ?? ''}');
  if (n == null) return '-';
  return n.toStringAsFixed(2);
}
