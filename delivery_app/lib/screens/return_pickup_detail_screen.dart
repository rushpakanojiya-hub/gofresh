import 'dart:async';
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'package:url_launcher/url_launcher.dart';
import '../services/api_service.dart';
import 'return_map_screen.dart';

class ReturnPickupDetailScreen extends StatefulWidget {
  final Map<String, dynamic> pickup;
  const ReturnPickupDetailScreen({super.key, required this.pickup});

  @override
  State<ReturnPickupDetailScreen> createState() => _ReturnPickupDetailScreenState();
}

class _ReturnPickupDetailScreenState extends State<ReturnPickupDetailScreen> {
  late Map<String, dynamic> _pickup;
  late final int _returnRequestId;
  bool _loading = false;
  String? _error;
  bool _uploadingPhoto = false;
  String? _capturedPhotoUrl;
  final TextEditingController _notesController = TextEditingController();

  static const Color primaryPurple = Color(0xFF5B2A9E);

  @override
  void initState() {
    super.initState();
    _pickup = widget.pickup;
    _returnRequestId = (_pickup['return_request_id'] as num).toInt();
    final st = _pickup['pickup_status']?.toString();
    if (st == 'accepted' || st == 'en_route' || st == 'arrived' || st == 'picked_up') {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        Navigator.of(context).pushReplacement(
          MaterialPageRoute(builder: (_) => ReturnMapScreen(pickup: _pickup)),
        );
      });
    }
  }

  @override
  void dispose() {
    _notesController.dispose();
    super.dispose();
  }

  Future<void> _callCustomer(String phone) async {
    final uri = Uri(scheme: 'tel', path: phone);
    if (await canLaunchUrl(uri)) {
      await launchUrl(uri);
    } else if (mounted) {
      setState(() => _error = 'Could not open dialer');
    }
  }

  void _applyPickupUpdate(String newPickupStatus) {
    setState(() {
      _pickup = {..._pickup, 'pickup_status': newPickupStatus};
    });
  }

  Future<void> _accept() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      await ApiService.acceptReturnPickup(_returnRequestId);
      _applyPickupUpdate('accepted');
      setState(() => _loading = false);
      if (!mounted) return;
      Navigator.of(context).pushReplacement(
        MaterialPageRoute(builder: (_) => ReturnMapScreen(pickup: {..._pickup, 'pickup_status': 'accepted'})),
      );
    } catch (e) {
      setState(() {
        _error = e.toString().replaceFirst('Exception: ', '');
        _loading = false;
      });
    }
  }

  Future<void> _reject() async {
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
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      await ApiService.rejectReturnPickup(_returnRequestId);
      if (mounted) Navigator.pop(context);
    } catch (e) {
      setState(() {
        _error = e.toString().replaceFirst('Exception: ', '');
        _loading = false;
      });
    }
  }

  Future<void> _markEnRoute() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      await ApiService.markReturnPickupEnRoute(_returnRequestId);
      _applyPickupUpdate('en_route');
      setState(() => _loading = false);
    } catch (e) {
      setState(() {
        _error = e.toString().replaceFirst('Exception: ', '');
        _loading = false;
      });
    }
  }

  Future<void> _markArrived() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      await ApiService.markReturnPickupArrived(_returnRequestId);
      _applyPickupUpdate('arrived');
      setState(() => _loading = false);
    } catch (e) {
      setState(() {
        _error = e.toString().replaceFirst('Exception: ', '');
        _loading = false;
      });
    }
  }

  Future<void> _pickConditionPhoto() async {
    final picker = ImagePicker();
    final photo = await picker.pickImage(source: ImageSource.camera, imageQuality: 80);
    if (photo == null) return;
    setState(() {
      _uploadingPhoto = true;
      _error = null;
    });
    try {
      final url = await ApiService.uploadReturnPickupPhoto(photo.path);
      setState(() {
        _capturedPhotoUrl = url;
        _uploadingPhoto = false;
      });
    } catch (e) {
      setState(() {
        _error = e.toString().replaceFirst('Exception: ', '');
        _uploadingPhoto = false;
      });
    }
  }

  Future<void> _confirmPickedUp() async {
    if (_capturedPhotoUrl == null) return;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      await ApiService.confirmReturnPickedUp(
        _returnRequestId,
        _capturedPhotoUrl!,
        conditionNotes: _notesController.text.trim().isEmpty ? null : _notesController.text.trim(),
      );
      _applyPickupUpdate('picked_up');
      setState(() => _loading = false);
    } catch (e) {
      setState(() {
        _error = e.toString().replaceFirst('Exception: ', '');
        _loading = false;
      });
    }
  }

  Future<void> _handover() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      await ApiService.handoverReturnToWarehouse(_returnRequestId);
      _applyPickupUpdate('handed_over');
      setState(() => _loading = false);
    } catch (e) {
      setState(() {
        _error = e.toString().replaceFirst('Exception: ', '');
        _loading = false;
      });
    }
  }

  static const List<Map<String, String>> _stepDefs = [
    {'key': 'accepted', 'label': 'Accepted'},
    {'key': 'en_route', 'label': 'On the Way'},
    {'key': 'arrived', 'label': 'Arrived at Customer'},
    {'key': 'picked_up', 'label': 'Picked Up'},
    {'key': 'handed_over', 'label': 'Handed Over to Warehouse'},
  ];

  int _currentStepIndex(String? pickupStatus) {
    if (pickupStatus == null || pickupStatus == 'assigned') return -1;
    return _stepDefs.indexWhere((s) => s['key'] == pickupStatus);
  }

  Widget _buildStepper(String? pickupStatus) {
    final currentIndex = _currentStepIndex(pickupStatus);
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
                        color: isDone || isCurrent ? primaryPurple : Colors.white,
                        border: Border.all(
                          color: isDone || isCurrent ? primaryPurple : const Color(0xFFE0DAF0),
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
                          color: lineDone ? primaryPurple : const Color(0xFFE0DAF0),
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
    final pickupStatus = _pickup['pickup_status']?.toString();
    final deliveryAddress = _pickup['delivery_address'] ?? '';
    final customerName = _pickup['customer_name'] ?? '';
    final customerPhone = _pickup['customer_phone'] ?? '';
    final reason = _pickup['reason'] ?? '';
    final refundAmount = _pickup['refund_amount'];
    final itemCount = _pickup['item_count'] ?? 0;
    final orderId = _pickup['order_id'];

    final isPendingResponse = pickupStatus == 'assigned';

    return Scaffold(
      appBar: AppBar(title: Text('Return Pickup - Order #$orderId')),
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
                  const Row(
                    children: [
                      Icon(Icons.assignment_return_outlined, color: Colors.orange, size: 20),
                      SizedBox(width: 8),
                      Text('New return pickup request', style: TextStyle(fontWeight: FontWeight.bold, fontSize: 15)),
                    ],
                  ),
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      Expanded(
                        child: OutlinedButton(
                          onPressed: _loading ? null : _reject,
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
                          onPressed: _loading ? null : _accept,
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
          Card(
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(customerName, style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
                  const SizedBox(height: 4),
                  Row(
                    children: [
                      Text(customerPhone),
                      const SizedBox(width: 8),
                      if (customerPhone.toString().isNotEmpty)
                        InkWell(
                          onTap: () => _callCustomer(customerPhone.toString()),
                          borderRadius: BorderRadius.circular(20),
                          child: const Padding(
                            padding: EdgeInsets.all(4),
                            child: Icon(Icons.call, color: primaryPurple, size: 20),
                          ),
                        ),
                    ],
                  ),
                  const SizedBox(height: 8),
                  Text(deliveryAddress),
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
                  Text('Items to Pick Up ($itemCount)', style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
                  const Divider(),
                  ...(_pickup['items'] as List<dynamic>? ?? []).map((item) => Padding(
                        padding: const EdgeInsets.symmetric(vertical: 4),
                        child: Text("${item['product_name']} x${item['quantity']}"),
                      )),
                  const Divider(),
                  Text('Reason: $reason'),
                  const SizedBox(height: 4),
                  Text('Refund amount: \u20B9${_money(refundAmount)}', style: const TextStyle(fontWeight: FontWeight.bold)),
                ],
              ),
            ),
          ),
          if (_error != null) ...[
            const SizedBox(height: 12),
            Text(_error!, style: const TextStyle(color: Colors.red)),
          ],
          if (!isPendingResponse) _buildStepper(pickupStatus),
          const SizedBox(height: 12),
          if (pickupStatus == 'accepted')
            ElevatedButton(
              onPressed: _loading ? null : _markEnRoute,
              style: ElevatedButton.styleFrom(padding: const EdgeInsets.all(16)),
              child: _loading
                  ? const SizedBox(height: 20, width: 20, child: CircularProgressIndicator(strokeWidth: 2))
                  : const Text('Start - On the Way'),
            )
          else if (pickupStatus == 'en_route')
            ElevatedButton(
              onPressed: _loading ? null : _markArrived,
              style: ElevatedButton.styleFrom(padding: const EdgeInsets.all(16)),
              child: _loading
                  ? const SizedBox(height: 20, width: 20, child: CircularProgressIndicator(strokeWidth: 2))
                  : const Text('Arrived at Customer'),
            )
          else if (pickupStatus == 'arrived')
            Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                TextField(
                  controller: _notesController,
                  decoration: const InputDecoration(
                    labelText: 'Condition notes (optional)',
                    border: OutlineInputBorder(),
                  ),
                  maxLines: 2,
                ),
                const SizedBox(height: 12),
                if (_capturedPhotoUrl == null)
                  OutlinedButton.icon(
                    onPressed: _uploadingPhoto ? null : _pickConditionPhoto,
                    icon: _uploadingPhoto
                        ? const SizedBox(height: 16, width: 16, child: CircularProgressIndicator(strokeWidth: 2))
                        : const Icon(Icons.camera_alt),
                    label: Text(_uploadingPhoto ? 'Uploading...' : 'Take Product Condition Photo'),
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
                        Text('Condition photo captured'),
                      ],
                    ),
                  ),
                const SizedBox(height: 8),
                ElevatedButton(
                  onPressed: (_loading || _capturedPhotoUrl == null) ? null : _confirmPickedUp,
                  style: ElevatedButton.styleFrom(padding: const EdgeInsets.all(16), backgroundColor: Colors.green),
                  child: _loading
                      ? const SizedBox(height: 20, width: 20, child: CircularProgressIndicator(strokeWidth: 2))
                      : const Text('Confirm Pickup'),
                ),
              ],
            )
          else if (pickupStatus == 'picked_up')
            ElevatedButton(
              onPressed: _loading ? null : _handover,
              style: ElevatedButton.styleFrom(padding: const EdgeInsets.all(16)),
              child: _loading
                  ? const SizedBox(height: 20, width: 20, child: CircularProgressIndicator(strokeWidth: 2))
                  : const Text('Handover to Warehouse'),
            )
          else if (pickupStatus == 'handed_over')
            const Center(
              child: Padding(
                padding: EdgeInsets.symmetric(vertical: 16),
                child: Text('Handed over to warehouse', style: TextStyle(color: Colors.green, fontSize: 18, fontWeight: FontWeight.bold)),
              ),
            ),
        ],
      ),
    );
  }
}

String _money(dynamic v) {
  final n = v is num ? v : num.tryParse('${v ?? ''}');
  if (n == null) return '-';
  return n.toStringAsFixed(2);
}
