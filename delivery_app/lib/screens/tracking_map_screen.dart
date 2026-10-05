import 'dart:async';
import 'dart:ui' as ui;
import 'package:flutter/material.dart';
import 'package:geolocator/geolocator.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart';
import 'package:url_launcher/url_launcher.dart';

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

class TrackingMapScreen extends StatefulWidget {
  final String title;
  final LatLng destination;
  final Stream<LatLng> partnerStream;
  final Widget bottom;
  const TrackingMapScreen({
    super.key,
    required this.title,
    required this.destination,
    required this.partnerStream,
    required this.bottom,
  });

  @override
  State<TrackingMapScreen> createState() => _TrackingMapScreenState();
}

class _TrackingMapScreenState extends State<TrackingMapScreen> {
  GoogleMapController? _ctl;
  LatLng? _partner;
  StreamSubscription<LatLng>? _sub;
  BitmapDescriptor? _homeIcon;
  BitmapDescriptor? _riderIcon;

  @override
  void initState() {
    super.initState();
    _loadIcons();
    _sub = widget.partnerStream.listen((p) {
      if (!mounted) return;
      setState(() => _partner = p);
      _fit();
    });
  }

  Future<void> _loadIcons() async {
    final home = await _emojiMarker('\u{1F3E0}', size: 60);
    final rider = await _emojiMarker('\u{1F6F5}', size: 42);
    if (mounted) setState(() { _homeIcon = home; _riderIcon = rider; });
  }

  @override
  void dispose() {
    _sub?.cancel();
    _ctl?.dispose();
    super.dispose();
  }

  Future<void> _fit() async {
    final c = _ctl, p = _partner, d = widget.destination;
    if (c == null || p == null) return;
    try {
      final sw = LatLng(p.latitude < d.latitude ? p.latitude : d.latitude,
          p.longitude < d.longitude ? p.longitude : d.longitude);
      final ne = LatLng(p.latitude > d.latitude ? p.latitude : d.latitude,
          p.longitude > d.longitude ? p.longitude : d.longitude);
      await c.animateCamera(CameraUpdate.newLatLngBounds(LatLngBounds(southwest: sw, northeast: ne), 80));
    } catch (_) {}
  }

  @override
  Widget build(BuildContext context) {
    final p = _partner, d = widget.destination;
    final meters = p == null
        ? null
        : Geolocator.distanceBetween(p.latitude, p.longitude, d.latitude, d.longitude);
    final etaMin = meters == null ? null : (meters / 5.5 / 60).ceil();

    return Scaffold(
      appBar: AppBar(title: Text(widget.title)),
      body: Column(children: [
        Expanded(
          child: Stack(children: [
            GoogleMap(
              initialCameraPosition: CameraPosition(target: p ?? d, zoom: 15),
              markers: {
                Marker(
                  markerId: const MarkerId('dest'),
                  position: d,
                  icon: _homeIcon ?? BitmapDescriptor.defaultMarker,
                ),
                if (p != null)
                  Marker(
                    markerId: const MarkerId('partner'),
                    position: p,
                    icon: _riderIcon ?? BitmapDescriptor.defaultMarkerWithHue(BitmapDescriptor.hueGreen),
                  ),
              },
              onMapCreated: (c) {
                _ctl = c;
                Future.delayed(const Duration(milliseconds: 300), _fit);
              },
              myLocationButtonEnabled: false,
              zoomControlsEnabled: false,
            ),
            if (meters != null)
              Positioned(
                top: 12,
                left: 12,
                right: 12,
                child: Card(
                  child: Padding(
                    padding: const EdgeInsets.all(12),
                    child: Row(mainAxisAlignment: MainAxisAlignment.spaceAround, children: [
                      Text('${(meters / 1000).toStringAsFixed(1)} km'),
                      Text('~$etaMin min'),
                    ]),
                  ),
                ),
              ),
            Positioned(
              right: 12,
              bottom: 12,
              child: FloatingActionButton.small(onPressed: _fit, child: const Icon(Icons.my_location)),
            ),
          ]),
        ),
        SafeArea(child: widget.bottom),
      ]),
    );
  }
}

Future<void> callNumber(String phone) => launchUrl(Uri.parse('tel:$phone'));

Future<void> openGoogleMapsNav(LatLng to) => launchUrl(
      Uri.parse('https://www.google.com/maps/dir/?api=1&destination=${to.latitude},${to.longitude}&travelmode=driving'),
      mode: LaunchMode.externalApplication,
    );