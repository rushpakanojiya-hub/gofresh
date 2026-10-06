import 'dart:async';
import 'dart:math' as math;
import 'dart:ui' as ui;
import 'package:flutter/material.dart';
import 'package:geolocator/geolocator.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart';
import 'package:image_picker/image_picker.dart';
import 'package:url_launcher/url_launcher.dart';
import '../services/api_service.dart';
import '../widgets/swipe_confirm.dart';
import '../services/push_service.dart';

const Color _purple = Color(0xFF5B2A9E);

Future<BitmapDescriptor> _emojiMarker(String emoji, {double size = 60}) async {
  const double scale = 3.0;
  final double px = size * scale;
  final recorder = ui.PictureRecorder();
  final canvas = Canvas(recorder);
  final painter = TextPainter(
    text: TextSpan(text: emoji, style: TextStyle(fontSize: px * 0.8)),
    textDirection: TextDirection.ltr,
  )..layout();
  painter.paint(canvas, Offset((px - painter.width) / 2, (px - painter.height) / 2));
  final img = await recorder.endRecording().toImage(px.toInt(), px.toInt());
  final data = await img.toByteData(format: ui.ImageByteFormat.png);
  return BitmapDescriptor.bytes(data!.buffer.asUint8List(), width: size, height: size);
}

String _s(Map<String, dynamic> o, String k) {
  final v = o[k];
  return v == null ? '' : v.toString().trim();
}

LatLng? _ll(Map<String, dynamic> o, String latKey, String lngKey) {
  final lat = num.tryParse('${o[latKey] ?? ''}');
  final lng = num.tryParse('${o[lngKey] ?? ''}');
  if (lat == null || lng == null) return null;
  return LatLng(lat.toDouble(), lng.toDouble());
}

/// Shown after a return pickup is accepted: map to the customer, then
/// Reached customer -> Picked up (condition photo) -> Delivered to store.
class ReturnMapScreen extends StatefulWidget {
  final Map<String, dynamic> pickup;
  const ReturnMapScreen({super.key, required this.pickup});

  @override
  State<ReturnMapScreen> createState() => _ReturnMapScreenState();
}

class _ReturnMapScreenState extends State<ReturnMapScreen> {
  late Map<String, dynamic> _p;
  late final int _id;
  GoogleMapController? _map;
  LatLng? _me;
  BitmapDescriptor? _riderIcon;
  BitmapDescriptor? _homeIcon;
  BitmapDescriptor? _storeIcon;
  StreamSubscription<Position>? _posSub;
  bool _busy = false;
  bool _showMap = false;
  bool _uploading = false;
  String? _error;
  String? _photoUrl;
  final TextEditingController _notes = TextEditingController();

  String get _status => _s(_p, 'pickup_status');
  LatLng? get _customer => _ll(_p, 'delivery_lat', 'delivery_lng');
  LatLng? get _store => _ll(_p, 'pickup_lat', 'pickup_lng');
  LatLng? get _target => _status == 'picked_up' ? _store : _customer;

  @override
  void initState() {
    super.initState();
    _p = Map<String, dynamic>.from(widget.pickup);
    _id = (_p['return_request_id'] as num).toInt();
    _loadIcons();
    _locate();
    if (_status == 'accepted') _autoEnRoute();
  }

  @override
  void dispose() {
    _notes.dispose();
    _posSub?.cancel();
    PushService.dataRefresh.value++;
    _map?.dispose();
    super.dispose();
  }

  Future<void> _autoEnRoute() async {
    try {
      await ApiService.markReturnPickupEnRoute(_id);
      _setStatus('en_route');
    } catch (_) {
      // The swipe handles it again if this failed.
    }
  }

  void _setStatus(String s) {
    if (!mounted) return;
    setState(() => _p = {..._p, 'pickup_status': s});
    PushService.dataRefresh.value++;
    _fit();
  }

  Future<void> _loadIcons() async {
    final home = await _emojiMarker('\u{1F3E0}', size: 60);
    final store = await _emojiMarker('\u{1F3EC}', size: 60);
    final rider = await _emojiMarker('\u{1F6F5}', size: 42);
    if (!mounted) return;
    setState(() {
      _homeIcon = home;
      _storeIcon = store;
      _riderIcon = rider;
    });
  }

  Future<void> _locate() async {
    try {
      var perm = await Geolocator.checkPermission();
      if (perm == LocationPermission.denied) perm = await Geolocator.requestPermission();
      if (perm == LocationPermission.denied || perm == LocationPermission.deniedForever) return;
      final p = await Geolocator.getCurrentPosition();
      if (!mounted) return;
      setState(() => _me = LatLng(p.latitude, p.longitude));
      _fit();
      _posSub = Geolocator.getPositionStream(
        locationSettings: const LocationSettings(accuracy: LocationAccuracy.high, distanceFilter: 10),
      ).listen((pos) {
        if (!mounted) return;
        setState(() => _me = LatLng(pos.latitude, pos.longitude));
      });
    } catch (_) {}
  }

  Future<void> _fit() async {
    final c = _map;
    final t = _target;
    final m = _me;
    if (c == null || t == null) return;
    try {
      if (m == null) {
        await c.animateCamera(CameraUpdate.newLatLngZoom(t, 15));
        return;
      }
      final dist = Geolocator.distanceBetween(m.latitude, m.longitude, t.latitude, t.longitude);
      if (dist < 50) {
        await c.animateCamera(CameraUpdate.newLatLngZoom(t, 16));
        return;
      }
      final sw = LatLng(math.min(m.latitude, t.latitude), math.min(m.longitude, t.longitude));
      final ne = LatLng(math.max(m.latitude, t.latitude), math.max(m.longitude, t.longitude));
      await c.animateCamera(CameraUpdate.newLatLngBounds(LatLngBounds(southwest: sw, northeast: ne), 80));
    } catch (_) {}
  }

  Future<void> _run(Future<void> Function() fn) async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await fn();
    } catch (e) {
      if (mounted) setState(() => _error = e.toString().replaceFirst('Exception: ', ''));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _reached() => _run(() async {
        if (_status == 'accepted') {
          await ApiService.markReturnPickupEnRoute(_id);
          _setStatus('en_route');
        }
        await ApiService.markReturnPickupArrived(_id);
        _setStatus('arrived');
      });

  Future<void> _takePhoto() async {
    final photo = await ImagePicker().pickImage(source: ImageSource.camera, imageQuality: 80);
    if (photo == null) return;
    setState(() {
      _uploading = true;
      _error = null;
    });
    try {
      final url = await ApiService.uploadReturnPickupPhoto(photo.path);
      if (mounted) setState(() => _photoUrl = url);
    } catch (e) {
      if (mounted) setState(() => _error = e.toString().replaceFirst('Exception: ', ''));
    } finally {
      if (mounted) setState(() => _uploading = false);
    }
  }

  Future<void> _pickedUp() async {
    if (_photoUrl == null) {
      setState(() => _error = 'Take the product condition photo first');
      return;
    }
    await _run(() async {
      final notes = _notes.text.trim();
      await ApiService.confirmReturnPickedUp(_id, _photoUrl!, conditionNotes: notes.isEmpty ? null : notes);
      _setStatus('picked_up');
    });
  }

  Future<void> _handover() => _run(() async {
        await ApiService.handoverReturnToWarehouse(_id);
        _setStatus('handed_over');
      });

    Future<void> _call() async {
    final phone = _s(_p, 'customer_phone');
    if (phone.isEmpty) return;
    final messenger = ScaffoldMessenger.of(context);
    try {
      final ok = await launchUrl(Uri(scheme: 'tel', path: phone));
      if (!ok) messenger.showSnackBar(const SnackBar(content: Text('Could not start the call')));
    } catch (_) {
      messenger.showSnackBar(const SnackBar(content: Text('Could not start the call')));
    }
  }

  Widget _action() {
    switch (_status) {
      case 'accepted':
      case 'en_route':
        return SwipeConfirm(
          key: const ValueKey('reached'),
          label: 'Reached customer',
          enabled: !_busy,
          onConfirm: () => _reached(),
        );
      case 'arrived':
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            OutlinedButton.icon(
              onPressed: _uploading ? null : _takePhoto,
              icon: _uploading
                  ? const SizedBox(height: 16, width: 16, child: CircularProgressIndicator(strokeWidth: 2))
                  : Icon(_photoUrl == null ? Icons.camera_alt : Icons.check_circle,
                      color: _photoUrl == null ? null : Colors.green),
              label: Text(_uploading
                  ? 'Uploading...'
                  : (_photoUrl == null ? 'Take product condition photo' : 'Photo captured (retake)')),
              style: OutlinedButton.styleFrom(padding: const EdgeInsets.all(12)),
            ),
            const SizedBox(height: 12),
            SwipeConfirm(
              key: const ValueKey('picked'),
              label: 'Picked up',
              enabled: !_busy && _photoUrl != null,
              onConfirm: () => _pickedUp(),
            ),
          ],
        );
      case 'picked_up':
        return SwipeConfirm(
          key: const ValueKey('store'),
          label: 'Delivered to store',
          enabled: !_busy,
          onConfirm: () => _handover(),
        );
      default:
        return ElevatedButton(
          onPressed: () => Navigator.of(context).popUntil((r) => r.isFirst),
          style: ElevatedButton.styleFrom(
            backgroundColor: Colors.green,
            foregroundColor: Colors.white,
            padding: const EdgeInsets.all(16),
          ),
          child: const Text('Done', style: TextStyle(fontWeight: FontWeight.bold)),
        );
    }
  }

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: !_showMap,
      onPopInvokedWithResult: (didPop, result) {
        if (!didPop) setState(() => _showMap = false);
      },
      child: (_showMap || _status == 'picked_up') ? _buildMapView(context) : _buildDetails(context),
    );
  }

  Widget _detailSection({
    required IconData icon,
    required String title,
    String? subtitle,
    bool open = false,
    required List<Widget> children,
  }) {
    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      color: Colors.white,
      child: Theme(
        data: Theme.of(context).copyWith(dividerColor: Colors.transparent),
        child: ExpansionTile(
          key: ValueKey('$title-$_status'),
          initiallyExpanded: open,
          leading: Icon(icon, size: 20, color: Colors.black87),
          title: Text(title, style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 14)),
          subtitle: subtitle == null
              ? null
              : Text(subtitle, style: const TextStyle(fontSize: 12, color: Colors.black54)),
          childrenPadding: const EdgeInsets.fromLTRB(16, 0, 16, 14),
          expandedAlignment: Alignment.centerLeft,
          expandedCrossAxisAlignment: CrossAxisAlignment.start,
          children: children,
        ),
      ),
    );
  }

  Widget _callMapButtons({required bool showCall}) {
    final phone = _s(_p, 'customer_phone');
    return Padding(
      padding: const EdgeInsets.only(top: 12),
      child: Row(
        children: [
          if (showCall) ...[
            Expanded(
              child: OutlinedButton.icon(
                onPressed: phone.isEmpty ? null : _call,
                icon: const Icon(Icons.call, size: 16),
                label: const Text('Call'),
                style: OutlinedButton.styleFrom(
                  foregroundColor: Colors.black87,
                  padding: const EdgeInsets.symmetric(vertical: 12),
                ),
              ),
            ),
            const SizedBox(width: 10),
          ],
          Expanded(
            child: ElevatedButton.icon(
              onPressed: _target == null ? null : () => setState(() => _showMap = true),
              icon: const Icon(Icons.navigation, size: 16),
              label: const Text('Map'),
              style: ElevatedButton.styleFrom(
                backgroundColor: Colors.black,
                foregroundColor: Colors.white,
                padding: const EdgeInsets.symmetric(vertical: 12),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildDetails(BuildContext context) {
    final st = _status;
    final toStore = st == 'picked_up';
    final done = st == 'handed_over';
    final items = (_p['items'] is List) ? _p['items'] as List : const [];
    final name = _s(_p, 'customer_name');
    final phone = _s(_p, 'customer_phone');
    final addr = _s(_p, 'delivery_address');
    final storeName = _s(_p, 'pickup_name');
    final storeAddr = _s(_p, 'pickup_address');
    final reason = _s(_p, 'reason');
    final refund = num.tryParse('${_p['refund_amount'] ?? ''}');

    return Scaffold(
      backgroundColor: const Color(0xFFF7F1FB),
      appBar: AppBar(
        backgroundColor: Colors.white,
        elevation: 0,
        iconTheme: const IconThemeData(color: Colors.black87),
      ),
      body: SafeArea(
        child: Column(
          children: [
            Expanded(
              child: ListView(
                children: [
                  Container(
                    color: Colors.white,
                    padding: const EdgeInsets.only(bottom: 18),
                    margin: const EdgeInsets.only(bottom: 8),
                    child: Column(
                      children: [
                        const Text('RETURN PICKUP - ORDER ID',
                            style: TextStyle(fontSize: 11, color: Colors.black54)),
                        const SizedBox(height: 6),
                        Text('#${_s(_p, 'order_id')}',
                            style: const TextStyle(fontSize: 28, fontWeight: FontWeight.bold)),
                      ],
                    ),
                  ),
                  _detailSection(
                    icon: Icons.shopping_bag_outlined,
                    title: 'Order details',
                    subtitle: '${items.length} item${items.length == 1 ? '' : 's'}',
                    open: true,
                    children: [
                      for (final it in items)
                        Padding(
                          padding: const EdgeInsets.only(top: 6),
                          child: Text('${it is Map ? (it['quantity'] ?? 1) : 1}x  '
                              '${it is Map ? (it['product_name'] ?? '') : ''}',
                              style: const TextStyle(fontSize: 13)),
                        ),
                      if (reason.isNotEmpty)
                        Padding(
                          padding: const EdgeInsets.only(top: 8),
                          child: Text('Reason: $reason',
                              style: const TextStyle(fontSize: 12, color: Colors.black54)),
                        ),
                      if (refund != null)
                        Padding(
                          padding: const EdgeInsets.only(top: 2),
                          child: Text('Refund: \u20B9${refund.toStringAsFixed(2)}',
                              style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w600)),
                        ),
                    ],
                  ),
                  _detailSection(
                    icon: Icons.storefront_outlined,
                    title: 'Store details',
                    open: toStore,
                    children: [
                      if (storeName.isNotEmpty)
                        Text(storeName, style: const TextStyle(fontWeight: FontWeight.w600)),
                      if (storeAddr.isNotEmpty)
                        Padding(
                          padding: const EdgeInsets.only(top: 4),
                          child: Text(storeAddr,
                              style: const TextStyle(fontSize: 12, color: Colors.black54)),
                        ),
                      if (toStore && !done) _callMapButtons(showCall: false),
                    ],
                  ),
                  _detailSection(
                    icon: Icons.person_outline,
                    title: 'Customer details',
                    open: !toStore && !done,
                    children: [
                      if (name.isNotEmpty)
                        Text(name, style: const TextStyle(fontWeight: FontWeight.w600)),
                      if (phone.isNotEmpty)
                        Padding(
                          padding: const EdgeInsets.only(top: 4),
                          child: Text(phone, style: const TextStyle(fontSize: 13)),
                        ),
                      if (addr.isNotEmpty)
                        Padding(
                          padding: const EdgeInsets.only(top: 4),
                          child: Text(addr,
                              style: const TextStyle(fontSize: 12, color: Colors.black54)),
                        ),
                      if (!toStore && !done) _callMapButtons(showCall: true),
                    ],
                  ),
                ],
              ),
            ),
            ConstrainedBox(
              constraints: BoxConstraints(maxHeight: MediaQuery.of(context).size.height * 0.5),
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(16),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    if (_error != null)
                      Padding(
                        padding: const EdgeInsets.only(bottom: 8),
                        child: Text(_error!, style: const TextStyle(color: Colors.red)),
                      ),
                    _action(),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildMapView(BuildContext context) {
    final st = _status;
    final toStore = st == 'picked_up';
    final done = st == 'handed_over';
    final target = _target;
    final me = _me;
    final storeName = _s(_p, 'pickup_name');
    final name = toStore ? (storeName.isEmpty ? 'Deliver to the store' : storeName) : _s(_p, 'customer_name');
    final addr = toStore ? _s(_p, 'pickup_address') : _s(_p, 'delivery_address');
    final phone = _s(_p, 'customer_phone');
    final title = done ? 'Completed' : (toStore ? 'Store' : 'Pickup from customer');

    final markers = <Marker>{
      if (me != null && _riderIcon != null)
        Marker(
          markerId: const MarkerId('rider'),
          position: me,
          icon: _riderIcon!,
          anchor: const Offset(0.5, 0.5),
        ),
      if (target != null)
        Marker(
          markerId: const MarkerId('target'),
          position: target,
          icon: (toStore ? _storeIcon : _homeIcon) ?? BitmapDescriptor.defaultMarker,
          anchor: const Offset(0.5, 0.5),
          infoWindow: InfoWindow(title: name.isEmpty ? 'Drop' : name),
        ),
    };
    final lines = <Polyline>{
      if (target != null && me != null)
        Polyline(polylineId: const PolylineId('route'), points: [me, target], color: _purple, width: 4),
    };

    return Scaffold(
      body: Stack(
        children: [
          Positioned.fill(
            child: target == null
                ? Container(
                    color: const Color(0xFFE9E4F0),
                    alignment: Alignment.center,
                    child: const Text('Location not available', style: TextStyle(color: Colors.black54)),
                  )
                : GoogleMap(
                    initialCameraPosition: CameraPosition(target: target, zoom: 15),
                    onMapCreated: (c) {
                      _map = c;
                      Future.delayed(const Duration(milliseconds: 400), _fit);
                    },
                    markers: markers,
                    polylines: lines,
                    myLocationEnabled: false,
                    myLocationButtonEnabled: false,
                    zoomControlsEnabled: false,
                    mapToolbarEnabled: false,
                    padding: const EdgeInsets.only(bottom: 300),
                  ),
          ),
          Positioned(
            top: 0,
            left: 0,
            child: SafeArea(
              child: Padding(
                padding: const EdgeInsets.all(10),
                child: CircleAvatar(
                  backgroundColor: Colors.white,
                  child: IconButton(
                    icon: const Icon(Icons.arrow_back, color: Colors.black87),
                    onPressed: () {
                      if (_status == 'picked_up') {
                        Navigator.of(context).pop();
                      } else {
                        setState(() => _showMap = false);
                      }
                    },
                  ),
                ),
              ),
            ),
          ),
          Align(
            alignment: Alignment.bottomCenter,
            child: Container(
              width: double.infinity,
              decoration: const BoxDecoration(
                color: Colors.white,
                borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
                boxShadow: [BoxShadow(color: Colors.black26, blurRadius: 12)],
              ),
              child: SafeArea(
                top: false,
                child: SingleChildScrollView(
                  padding: const EdgeInsets.all(16),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Container(
                            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                            decoration: BoxDecoration(color: Colors.black87, borderRadius: BorderRadius.circular(4)),
                            child: Text(title,
                                style: const TextStyle(color: Colors.white, fontSize: 12, fontWeight: FontWeight.bold)),
                          ),
                          const SizedBox(width: 10),
                          Text('Order #${_s(_p, 'order_id')}',
                              style: const TextStyle(fontSize: 12, color: Colors.black54)),
                        ],
                      ),
                      const SizedBox(height: 10),
                      if (name.isNotEmpty)
                        Text(name, style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600)),
                      if (addr.isNotEmpty)
                        Padding(
                          padding: const EdgeInsets.only(top: 4),
                          child: Text(addr, style: const TextStyle(fontSize: 12, color: Colors.black54)),
                        ),
                      if ((_p['items'] as List?)?.isNotEmpty ?? false)
                        Padding(
                          padding: const EdgeInsets.only(top: 10),
                          child: Container(
                            width: double.infinity,
                            padding: const EdgeInsets.all(10),
                            decoration: BoxDecoration(
                              color: const Color(0xFFF7F1FB),
                              borderRadius: BorderRadius.circular(10),
                            ),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                const Text('Order details',
                                    style: TextStyle(fontSize: 12, fontWeight: FontWeight.bold)),
                                const SizedBox(height: 4),
                                for (final it in (_p['items'] as List))
                                  Text('${it['product_name']} x${it['quantity']}',
                                      style: const TextStyle(fontSize: 13)),
                                if (_s(_p, 'reason').isNotEmpty)
                                  Padding(
                                    padding: const EdgeInsets.only(top: 4),
                                    child: Text('Reason: ${_s(_p, 'reason')}',
                                        style: const TextStyle(fontSize: 12, color: Colors.black54)),
                                  ),
                                if (num.tryParse('${_p['refund_amount'] ?? ''}') != null)
                                  Padding(
                                    padding: const EdgeInsets.only(top: 2),
                                    child: Text(
                                        'Refund: \u20B9${num.parse('${_p['refund_amount']}').toStringAsFixed(2)}',
                                        style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w600)),
                                  ),
                              ],
                            ),
                          ),
                        ),
                      if (!done)
                        Padding(
                          padding: const EdgeInsets.only(top: 12),
                          child: Row(
                            children: [
                              if (!toStore) ...[
                                Expanded(
                                  child: OutlinedButton.icon(
                                    onPressed: phone.isEmpty ? null : _call,
                                    icon: const Icon(Icons.call, size: 16),
                                    label: const Text('Call'),
                                    style: OutlinedButton.styleFrom(
                                      foregroundColor: Colors.black87,
                                      padding: const EdgeInsets.symmetric(vertical: 12),
                                    ),
                                  ),
                                ),
                                const SizedBox(width: 10),
                              ],
                              Expanded(
                                child: ElevatedButton.icon(
                                  onPressed: _fit,
                                  icon: const Icon(Icons.navigation, size: 16),
                                  label: const Text('Map'),
                                  style: ElevatedButton.styleFrom(
                                    backgroundColor: Colors.black,
                                    foregroundColor: Colors.white,
                                    padding: const EdgeInsets.symmetric(vertical: 12),
                                  ),
                                ),
                              ),
                            ],
                          ),
                        ),
                      if (_error != null)
                        Padding(
                          padding: const EdgeInsets.only(top: 10),
                          child: Text(_error!, style: const TextStyle(color: Colors.red)),
                        ),
                      const SizedBox(height: 14),
                      _action(),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}