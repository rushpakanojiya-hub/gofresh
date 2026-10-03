import 'dart:math' as math;
import 'package:flutter/material.dart';
import 'package:geolocator/geolocator.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart';
import 'package:url_launcher/url_launcher.dart';
import 'delivery_handover_screen.dart';

const Color _green = Color(0xFF1ED760);
const Color _purple = Color(0xFF5B2A9E);

String _pick(Map<String, dynamic> o, List<String> keys) {
  for (final k in keys) {
    final v = o[k];
    if (v != null && v.toString().trim().isNotEmpty) return v.toString();
  }
  return '';
}

String _money(dynamic v) {
  final n = v is num ? v : num.tryParse('${v ?? ''}');
  if (n == null) return '-';
  return n.toStringAsFixed(2);
}

/// Shown after "Order picked": map to the customer's drop point, call and
/// navigate buttons, and a swipe to confirm the partner has reached.
class DeliveryMapScreen extends StatefulWidget {
  final Map<String, dynamic> order;
  const DeliveryMapScreen({super.key, required this.order});

  @override
  State<DeliveryMapScreen> createState() => _DeliveryMapScreenState();
}

class _DeliveryMapScreenState extends State<DeliveryMapScreen> {
  GoogleMapController? _map;
  LatLng? _drop;
  LatLng? _me;

  @override
  void initState() {
    super.initState();
    final lat = num.tryParse('${widget.order['delivery_lat'] ?? ''}');
    final lng = num.tryParse('${widget.order['delivery_lng'] ?? ''}');
    if (lat != null && lng != null) _drop = LatLng(lat.toDouble(), lng.toDouble());
    _locate();
  }

  @override
  void dispose() {
    _map?.dispose();
    super.dispose();
  }

  Future<void> _locate() async {
    try {
      var perm = await Geolocator.checkPermission();
      if (perm == LocationPermission.denied) {
        perm = await Geolocator.requestPermission();
      }
      if (perm == LocationPermission.denied || perm == LocationPermission.deniedForever) return;
      final p = await Geolocator.getCurrentPosition();
      if (!mounted) return;
      setState(() => _me = LatLng(p.latitude, p.longitude));
      _fit();
    } catch (_) {
      // Map still works with just the drop marker.
    }
  }

  Future<void> _fit() async {
    final c = _map;
    final d = _drop;
    final m = _me;
    if (c == null || d == null) return;
    try {
      if (m == null) {
        await c.animateCamera(CameraUpdate.newLatLngZoom(d, 15));
        return;
      }
      final dist = Geolocator.distanceBetween(m.latitude, m.longitude, d.latitude, d.longitude);
      if (dist < 50) {
        await c.animateCamera(CameraUpdate.newLatLngZoom(d, 16));
        return;
      }
      final sw = LatLng(math.min(m.latitude, d.latitude), math.min(m.longitude, d.longitude));
      final ne = LatLng(math.max(m.latitude, d.latitude), math.max(m.longitude, d.longitude));
      await c.animateCamera(CameraUpdate.newLatLngBounds(LatLngBounds(southwest: sw, northeast: ne), 80));
    } catch (_) {
      // Map not laid out yet; ignore.
    }
  }

  Future<void> _openMaps() async {
    final d = _drop;
    final addr = _pick(widget.order, ['delivery_address']);
    final uri = d != null
        ? Uri.parse(
            'https://www.google.com/maps/dir/?api=1&destination=${d.latitude},${d.longitude}&travelmode=driving')
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
    final phone = _pick(widget.order, ['customer_phone']);
    if (phone.isEmpty) return;
    final messenger = ScaffoldMessenger.of(context);
    try {
      final ok = await launchUrl(Uri(scheme: 'tel', path: phone));
      if (!ok) messenger.showSnackBar(const SnackBar(content: Text('Could not start the call')));
    } catch (_) {
      messenger.showSnackBar(const SnackBar(content: Text('Could not start the call')));
    }
  }

  void _reached() {
    final updated = Map<String, dynamic>.from(widget.order)..['status'] = 'shipped';
    Navigator.of(context).pushReplacement(
      MaterialPageRoute(builder: (_) => DeliveryHandoverScreen(order: updated)),
    );
  }

  @override
  Widget build(BuildContext context) {
    final o = widget.order;
    final id = _pick(o, ['order_id', 'id']);
    final name = _pick(o, ['customer_name']);
    final addr = _pick(o, ['delivery_address']);
    final phone = _pick(o, ['customer_phone']);
    final isCod = _pick(o, ['payment_method']).toLowerCase() == 'cod';
    final drop = _drop;
    final me = _me;

    final markers = <Marker>{
      if (drop != null)
        Marker(
          markerId: const MarkerId('drop'),
          position: drop,
          infoWindow: InfoWindow(title: name.isEmpty ? 'Drop' : name),
        ),
    };
    final lines = <Polyline>{
      if (drop != null && me != null)
        Polyline(
          polylineId: const PolylineId('route'),
          points: [me, drop],
          color: _purple,
          width: 4,
        ),
    };

    return Scaffold(
      body: Stack(
        children: [
          Positioned.fill(
            child: drop == null
                ? Container(
                    color: const Color(0xFFE9E4F0),
                    alignment: Alignment.center,
                    child: const Text('Customer location not available',
                        style: TextStyle(color: Colors.black54)),
                  )
                : GoogleMap(
                    initialCameraPosition: CameraPosition(target: drop, zoom: 15),
                    onMapCreated: (c) {
                      _map = c;
                      Future.delayed(const Duration(milliseconds: 400), _fit);
                    },
                    markers: markers,
                    polylines: lines,
                    myLocationEnabled: me != null,
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
                        Text('Reach drop', style: TextStyle(fontWeight: FontWeight.w600)),
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
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(16, 16, 16, 16),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Container(
                            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                            decoration: BoxDecoration(
                              color: Colors.black87,
                              borderRadius: BorderRadius.circular(4),
                            ),
                            child: const Text('Drop',
                                style: TextStyle(
                                    color: Colors.white, fontSize: 12, fontWeight: FontWeight.bold)),
                          ),
                          const SizedBox(width: 10),
                          Text('Order #$id', style: const TextStyle(fontSize: 12, color: Colors.black54)),
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
                      if (isCod)
                        Padding(
                          padding: const EdgeInsets.only(top: 8),
                          child: Text('COD: \u20B9${_money(o['total_amount'])}',
                              style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w600)),
                        ),
                      const SizedBox(height: 14),
                      Row(
                        children: [
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
                      const SizedBox(height: 14),
                      _SwipeButton(label: 'Reached drop', onConfirm: _reached),
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

/// Green pill with a black knob the partner drags to the right to confirm.
class _SwipeButton extends StatefulWidget {
  final String label;
  final VoidCallback onConfirm;
  const _SwipeButton({required this.label, required this.onConfirm});

  @override
  State<_SwipeButton> createState() => _SwipeButtonState();
}

class _SwipeButtonState extends State<_SwipeButton> {
  static const double _knob = 52;
  double _dx = 0;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(builder: (context, c) {
      final maxDx = c.maxWidth - _knob - 8;
      return Container(
        height: _knob + 8,
        decoration: BoxDecoration(
          color: _green,
          borderRadius: BorderRadius.circular(40),
        ),
        child: Stack(
          alignment: Alignment.centerLeft,
          children: [
            Center(
              child: Text(widget.label,
                  style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 15)),
            ),
            Positioned(
              left: 4 + _dx,
              child: GestureDetector(
                onHorizontalDragUpdate: (d) =>
                    setState(() => _dx = (_dx + d.delta.dx).clamp(0.0, maxDx).toDouble()),
                onHorizontalDragEnd: (_) {
                  if (_dx >= maxDx * 0.85) {
                    setState(() => _dx = maxDx);
                    widget.onConfirm();
                  } else {
                    setState(() => _dx = 0);
                  }
                },
                child: Container(
                  width: _knob,
                  height: _knob,
                  decoration: const BoxDecoration(color: Colors.black, shape: BoxShape.circle),
                  child: const Icon(Icons.arrow_forward, color: Colors.white),
                ),
              ),
            ),
          ],
        ),
      );
    });
  }
}