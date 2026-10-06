import 'dart:math' as math;
import 'package:flutter/material.dart';
import 'package:geolocator/geolocator.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart';
import 'package:image_picker/image_picker.dart';
import 'package:url_launcher/url_launcher.dart';
import '../services/api_service.dart';
import '../widgets/swipe_confirm.dart';

const Color _purple = Color(0xFF5B2A9E);

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
  bool _busy = false;
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
    _locate();
    if (_status == 'accepted') _autoEnRoute();
  }

  @override
  void dispose() {
    _notes.dispose();
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
    _fit();
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

  Future<void> _openMaps() async {
    final t = _target;
    final addr = _status == 'picked_up' ? _s(_p, 'pickup_address') : _s(_p, 'delivery_address');
    final uri = t != null
        ? Uri.parse('https://www.google.com/maps/dir/?api=1&destination=${t.latitude},${t.longitude}&travelmode=driving')
        : Uri.parse('https://www.google.com/maps/search/?api=1&query=${Uri.encodeComponent(addr)}');
    final messenger = ScaffoldMessenger.of(context);
    try {
      final ok = await launchUrl(uri, mode: LaunchMode.externalApplication);
      if (!ok) messenger.showSnackBar(const SnackBar(content: Text('Could not open Google Maps')));
    } catch (_) {
      messenger.showSnackBar(const SnackBar(content: Text('Could not open Google Maps')));
    }
  }

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
            TextField(
              controller: _notes,
              maxLines: 2,
              decoration: const InputDecoration(
                labelText: 'Condition notes (optional)',
                border: OutlineInputBorder(),
                isDense: true,
              ),
            ),
            const SizedBox(height: 10),
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
      if (me != null)
        Marker(
          markerId: const MarkerId('rider'),
          position: me,
          icon: BitmapDescriptor.defaultMarkerWithHue(BitmapDescriptor.hueAzure),
        ),
      if (target != null)
        Marker(
          markerId: const MarkerId('target'),
          position: target,
          icon: BitmapDescriptor.defaultMarkerWithHue(BitmapDescriptor.hueViolet),
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
          SafeArea(
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Material(
                color: Colors.white,
                elevation: 3,
                borderRadius: BorderRadius.circular(24),
                child: InkWell(
                  borderRadius: BorderRadius.circular(24),
                  onTap: () => Navigator.of(context).maybePop(),
                  child: const Padding(
                    padding: EdgeInsets.symmetric(horizontal: 14, vertical: 10),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.arrow_back, size: 18),
                        SizedBox(width: 8),
                        Text('Return pickup', style: TextStyle(fontWeight: FontWeight.w600)),
                      ],
                    ),
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
                                  onPressed: _openMaps,
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