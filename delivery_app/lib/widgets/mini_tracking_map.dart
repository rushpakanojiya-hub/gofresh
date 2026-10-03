import 'dart:math';
import 'dart:typed_data';
import 'dart:ui' as ui;
import 'package:flutter/material.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart';

Future<BitmapDescriptor> _emojiMarker(String emoji, {double size = 100}) async {
  final recorder = ui.PictureRecorder();
  final canvas = Canvas(recorder);
  final painter = TextPainter(textDirection: TextDirection.ltr);
  painter.text = TextSpan(text: emoji, style: TextStyle(fontSize: size * 0.78));
  painter.layout();
  painter.paint(canvas, Offset((size - painter.width) / 2, (size - painter.height) / 2));
  final picture = recorder.endRecording();
  final image = await picture.toImage(size.toInt(), size.toInt());
  final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
  return BitmapDescriptor.bytes(bytes!.buffer.asUint8List());
}

class MiniTrackingMap extends StatefulWidget {
  final LatLng destination;
  final LatLng? partner;
  final VoidCallback onTap;
  const MiniTrackingMap({super.key, required this.destination, this.partner, required this.onTap});

  @override
  State<MiniTrackingMap> createState() => _MiniTrackingMapState();
}

class _MiniTrackingMapState extends State<MiniTrackingMap> {
  GoogleMapController? _ctl;
  BitmapDescriptor? _homeIcon;
  BitmapDescriptor? _riderIcon;

  @override
  void initState() {
    super.initState();
    _loadIcons();
  }

  Future<void> _loadIcons() async {
    final home = await _emojiMarker('\u{1F3E0}', size: 90);
    final rider = await _emojiMarker('\u{1F6F5}', size: 62);
    if (mounted) setState(() { _homeIcon = home; _riderIcon = rider; });
  }

  Future<void> _fit() async {
    final c = _ctl;
    final p = widget.partner;
    final d = widget.destination;
    if (c == null) return;
    try {
      if (p == null || (p.latitude == d.latitude && p.longitude == d.longitude)) {
        await c.animateCamera(CameraUpdate.newLatLngZoom(d, 16));
        return;
      }
      final sw = LatLng(min(p.latitude, d.latitude), min(p.longitude, d.longitude));
      final ne = LatLng(max(p.latitude, d.latitude), max(p.longitude, d.longitude));
      await c.animateCamera(CameraUpdate.newLatLngBounds(LatLngBounds(southwest: sw, northeast: ne), 48));
    } catch (_) {}
  }

  @override
  void didUpdateWidget(covariant MiniTrackingMap old) {
    super.didUpdateWidget(old);
    if (old.partner != widget.partner) _fit();
  }

  @override
  Widget build(BuildContext context) {
    final markers = <Marker>{
      Marker(
        markerId: const MarkerId('dest'),
        position: widget.destination,
        icon: _homeIcon ?? BitmapDescriptor.defaultMarker,
      ),
      if (widget.partner != null)
        Marker(
          markerId: const MarkerId('partner'),
          position: widget.partner!,
          icon: _riderIcon ?? BitmapDescriptor.defaultMarkerWithHue(BitmapDescriptor.hueGreen),
        ),
    };
    return ClipRRect(
      borderRadius: BorderRadius.circular(16),
      child: SizedBox(
        height: 160,
        child: Stack(children: [
          IgnorePointer(
            child: GoogleMap(
              initialCameraPosition: CameraPosition(target: widget.partner ?? widget.destination, zoom: 15),
              markers: markers,
              onMapCreated: (c) {
                _ctl = c;
                Future.delayed(const Duration(milliseconds: 300), _fit);
              },
              zoomControlsEnabled: false,
              myLocationButtonEnabled: false,
              mapToolbarEnabled: false,
              compassEnabled: false,
            ),
          ),
          Positioned.fill(
            child: Material(color: Colors.transparent, child: InkWell(onTap: widget.onTap)),
          ),
          const Positioned(right: 8, bottom: 8, child: Icon(Icons.open_in_full, size: 20)),
        ]),
      ),
    );
  }
}