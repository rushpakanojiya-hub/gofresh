import 'package:flutter/material.dart';
import 'package:geolocator/geolocator.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart';
import 'package:image_picker/image_picker.dart';
import '../services/api_service.dart';
import '../services/location_service.dart';
import '../services/push_service.dart';
import 'home_shell.dart';
import 'selfie_camera_screen.dart';
import 'login_screen.dart';

const Color _purple = Color(0xFF5B2A9E);
const Color _bg = Color(0xFFF7F1FB);

class OnboardingFlow extends StatefulWidget {
  final Map<String, dynamic> initial;
  const OnboardingFlow({super.key, required this.initial});

  @override
  State<OnboardingFlow> createState() => _OnboardingFlowState();
}

class _OnboardingFlowState extends State<OnboardingFlow> {
  late Map<String, dynamic> _d;
  int _screen = 0; // 0 vehicle, 1 store, 2 documents, 3 selfie, 4 money
  bool _busy = false;
  String? _error;
  String? _uploading;

  String _vehicle = '';
  bool _noVehicle = false;
  final _vehicleNo = TextEditingController();
  List<dynamic> _stores = [];
  bool _storesLoading = true;
  int? _storeId;
  Position? _pos;
  final _storeSearch = TextEditingController();
  final Set<int> _expanded = {};
  String _licence = '';
  String _docType = 'aadhaar';
  String _voter = '';
  String _pan = '';
  String _aadhaar = '';
  String _selfie = '';
  bool _payMethodScreen = false;
  String _plan = 'full';
  final _upi = TextEditingController();
  final _holder = TextEditingController();
  final _acct = TextEditingController();
  final _ifsc = TextEditingController();

  final List<Map<String, Object>> _vehicles = [
    {'k': 'motorcycle', 'l': 'Motorcycle', 'i': Icons.two_wheeler},
    {'k': 'bicycle', 'l': 'Bicycle', 'i': Icons.pedal_bike},
    {'k': 'electric_scooter', 'l': 'Electric scooter', 'i': Icons.electric_scooter},
  ];

  String get _status => (_d['approval_status'] ?? 'onboarding').toString();

  @override
  void initState() {
    super.initState();
    _applyData(widget.initial);
    final step = (_d['onboarding_step'] is num) ? (_d['onboarding_step'] as num).toInt() : 0;
    _screen = _status == 'rejected' ? 0 : step.clamp(0, 4).toInt();
    _loadStores();
    _locate();
  }

  @override
  void dispose() {
    _vehicleNo.dispose();
    _upi.dispose();
    _storeSearch.dispose();
    _holder.dispose();
    _acct.dispose();
    _ifsc.dispose();
    super.dispose();
  }

  void _applyData(Map<String, dynamic> m) {
    _d = Map<String, dynamic>.from(m);
    _vehicle = (m['vehicle_type'] ?? '').toString();
    _vehicleNo.text = (m['vehicle_number'] ?? '').toString();
    _storeId = (m['warehouse_id'] as num?)?.toInt();
    _licence = (m['licence_url'] ?? '').toString();
    final dt = (m['id_doc_type'] ?? '').toString();
    if (dt.isNotEmpty) _docType = dt;
    _voter = (m['voter_url'] ?? '').toString();
    _pan = (m['pan_url'] ?? '').toString();
    _aadhaar = (m['aadhaar_url'] ?? '').toString();
    _selfie = (m['selfie_url'] ?? '').toString();
  }

  Future<void> _loadStores() async {
    try {
      final s = await ApiService.getStores();
      if (!mounted) return;
      setState(() {
        _stores = s;
        _storeId ??= _bestId();
        _storesLoading = false;
      });
    } catch (_) {
      if (mounted) setState(() => _storesLoading = false);
    }
  }

  Future<void> _run(Future<void> Function() job) async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await job();
    } catch (e) {
      _error = e.toString().replaceFirst('Exception: ', '');
    }
    if (mounted) setState(() => _busy = false);
  }

  Future<void> _save(String path, Map<String, dynamic> body) => _run(() async {
        final saved = await ApiService.saveOnboarding(path, body);
        if (!mounted) return;
        if (saved['approval_status'] == 'approved') {
          _enterApp();
          return;
        }
        setState(() {
          _applyData(saved);
          if (_screen < 4) _screen++;
        });
      });

  Future<void> _pick(String field, {bool selfie = false}) async {
    String? selfiePath;
    if (selfie) {
      selfiePath = await Navigator.push<String>(
        context,
        MaterialPageRoute(builder: (_) => const SelfieCameraScreen()),
      );
      if (selfiePath == null) return;
    }
    ImageSource? src = ImageSource.camera;
    if (!mounted) return;
    if (!selfie) {
      src = await showModalBottomSheet<ImageSource>(
        context: context,
        builder: (ctx) => SafeArea(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              ListTile(
                leading: const Icon(Icons.photo_camera),
                title: const Text('Camera'),
                onTap: () => Navigator.pop(ctx, ImageSource.camera),
              ),
              ListTile(
                leading: const Icon(Icons.photo_library),
                title: const Text('Gallery'),
                onTap: () => Navigator.pop(ctx, ImageSource.gallery),
              ),
            ],
          ),
        ),
      );
    }
    String? path = selfiePath;
    if (path == null) {
      if (src == null) return;
      final x = await ImagePicker().pickImage(
        source: src,
        imageQuality: 70,
        maxWidth: 1600,
        preferredCameraDevice: CameraDevice.rear,
      );
      if (x == null) return;
      path = x.path;
    }
    setState(() {
      _uploading = field;
      _error = null;
    });
    try {
      final url = await ApiService.uploadOnboardingImage(path);
      if (!mounted) return;
      setState(() {
        if (field == 'licence') _licence = url;
        if (field == 'voter') _voter = url;
        if (field == 'pan') _pan = url;
        if (field == 'aadhaar') _aadhaar = url;
        if (field == 'selfie') _selfie = url;
      });
    } catch (e) {
      _error = e.toString().replaceFirst('Exception: ', '');
    }
    if (mounted) setState(() => _uploading = null);
  }

  Future<void> _refresh() => _run(() async {
        final d = await ApiService.getOnboarding();
        if (!mounted) return;
        if (d['approval_status'] == 'approved') {
          _enterApp();
          return;
        }
        setState(() {
          _applyData(d);
          if (_status == 'rejected') _screen = 0;
        });
      });

  void _enterApp() {
    LocationService.startTracking();
    PushService.registerCurrentToken();
    Navigator.pushAndRemoveUntil(
      context,
      MaterialPageRoute(builder: (_) => const HomeShell()),
      (route) => false,
    );
  }

  Future<void> _logout() async {
    await ApiService.clearToken();
    if (!mounted) return;
    Navigator.pushAndRemoveUntil(
      context,
      MaterialPageRoute(builder: (_) => const LoginScreen()),
      (route) => false,
    );
  }

  // ---------- UI helpers ----------
  Widget _title(String t, String s) => Padding(
        padding: const EdgeInsets.only(bottom: 16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(t, style: const TextStyle(fontSize: 22, fontWeight: FontWeight.bold)),
            const SizedBox(height: 4),
            Text(s, style: const TextStyle(color: Colors.black54)),
          ],
        ),
      );

  Widget _primary(String text, VoidCallback? onTap) => Padding(
        padding: const EdgeInsets.only(top: 16),
        child: SizedBox(
          width: double.infinity,
          child: ElevatedButton(
            onPressed: (_busy || onTap == null) ? null : onTap,
            style: ElevatedButton.styleFrom(
              backgroundColor: _purple,
              foregroundColor: Colors.white,
              padding: const EdgeInsets.all(16),
            ),
            child: _busy
                ? const SizedBox(
                    height: 20,
                    width: 20,
                    child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                  )
                : Text(text),
          ),
        ),
      );

  Widget _choiceCard({
    required IconData icon,
    required String label,
    required bool selected,
    required VoidCallback onTap,
    String? subtitle,
  }) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        margin: const EdgeInsets.only(bottom: 12),
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.circular(14),
          border: Border.all(color: selected ? _purple : Colors.black12, width: selected ? 2 : 1),
        ),
        child: Row(
          children: [
            Icon(icon, size: 32, color: _purple),
            const SizedBox(width: 14),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(label, style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600)),
                  if (subtitle != null && subtitle.isNotEmpty)
                    Text(subtitle, style: const TextStyle(fontSize: 12, color: Colors.black54)),
                ],
              ),
            ),
            Icon(
              selected ? Icons.radio_button_checked : Icons.radio_button_unchecked,
              color: selected ? _purple : Colors.black38,
            ),
          ],
        ),
      ),
    );
  }

  Widget _docTile(String label, String field, String url) {
    final busy = _uploading == field;
    return Container(
      margin: const EdgeInsets.only(bottom: 10),
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(color: Colors.white, borderRadius: BorderRadius.circular(14)),
      child: Row(
        children: [
          Icon(url.isNotEmpty ? Icons.check_circle : Icons.upload_file,
              color: url.isNotEmpty ? Colors.green : _purple),
          const SizedBox(width: 12),
          Expanded(child: Text(label, style: const TextStyle(fontWeight: FontWeight.w600))),
          if (busy)
            const SizedBox(height: 20, width: 20, child: CircularProgressIndicator(strokeWidth: 2))
          else
            TextButton(
              onPressed: (_busy || _uploading != null) ? null : () => _pick(field),
              child: Text(url.isEmpty ? 'Upload' : 'Retake'),
            ),
        ],
      ),
    );
  }

  // ---------- steps ----------
  Widget _vehicleStep() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _title('Select your vehicle', 'Choose the vehicle you will use for deliveries'),
        for (final v in _vehicles)
          _choiceCard(
            icon: v['i'] as IconData,
            label: v['l'] as String,
            selected: _vehicle == v['k'] && !_noVehicle,
            onTap: () => setState(() {
              _vehicle = v['k'] as String;
              _noVehicle = false;
            }),
          ),
        _choiceCard(
          icon: Icons.no_transfer,
          label: "I don't have a vehicle",
          selected: _noVehicle,
          onTap: () => setState(() {
            _noVehicle = true;
            _vehicle = '';
          }),
        ),
        if (_noVehicle)
          const Padding(
            padding: EdgeInsets.only(bottom: 8),
            child: Text('You need a vehicle to deliver. Arrange one and come back.',
                style: TextStyle(color: Colors.black54)),
          ),
        if (_vehicle.isNotEmpty && _vehicle != 'bicycle')
          Padding(
            padding: const EdgeInsets.only(top: 4),
            child: TextField(
              controller: _vehicleNo,
              textCapitalization: TextCapitalization.characters,
              decoration: const InputDecoration(
                labelText: 'Vehicle number',
                border: OutlineInputBorder(),
                filled: true,
                fillColor: Colors.white,
              ),
            ),
          ),
        _primary(
          'Continue',
          (_vehicle.isEmpty || _noVehicle)
              ? null
              : () => _save('vehicle', {
                    'vehicle_type': _vehicle,
                    'vehicle_number': _vehicle == 'bicycle' ? '' : _vehicleNo.text.trim(),
                  }),
        ),
      ],
    );
  }

  Future<void> _locate() async {
    try {
      var perm = await Geolocator.checkPermission();
      if (perm == LocationPermission.denied) {
        perm = await Geolocator.requestPermission();
      }
      if (perm == LocationPermission.denied || perm == LocationPermission.deniedForever) return;
      final pos = await Geolocator.getCurrentPosition(
        locationSettings: const LocationSettings(
          accuracy: LocationAccuracy.medium,
          timeLimit: Duration(seconds: 10),
        ),
      );
      if (!mounted) return;
      setState(() {
        _pos = pos;
        _storeId ??= _bestId();
      });
    } catch (_) {}
  }

  LatLng? _ll(dynamic s) {
    final lat = (s['lat'] as num?)?.toDouble();
    final lng = (s['lng'] as num?)?.toDouble();
    if (lat == null || lng == null || (lat == 0 && lng == 0)) return null;
    return LatLng(lat, lng);
  }

  double? _km(dynamic s) {
    final p = _pos;
    final ll = _ll(s);
    if (p == null || ll == null) return null;
    return Geolocator.distanceBetween(p.latitude, p.longitude, ll.latitude, ll.longitude) / 1000;
  }

  int? _bestId() {
    int? id;
    double? best;
    for (final s in _stores) {
      final d = _km(s);
      if (d != null && (best == null || d < best)) {
        best = d;
        id = (s['id'] as num).toInt();
      }
    }
    return id;
  }

  List<dynamic> _visibleStores() {
    final q = _storeSearch.text.trim().toLowerCase();
    final list = _stores.where((s) {
      if (q.isEmpty) return true;
      return '${s['name']} ${s['city']} ${s['address']}'.toLowerCase().contains(q);
    }).toList();
    list.sort((a, b) {
      final da = _km(a);
      final db = _km(b);
      if (da == null && db == null) return 0;
      if (da == null) return 1;
      if (db == null) return -1;
      return da.compareTo(db);
    });
    return list;
  }

  Widget _stepper() {
    Widget dot(int i) {
      final done = i < _screen;
      final cur = i == _screen;
      return Container(
        width: 32,
        height: 32,
        alignment: Alignment.center,
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          color: done ? _purple : (cur ? Colors.white : Colors.black12),
          border: cur ? Border.all(color: _purple, width: 2) : null,
        ),
        child: done
            ? const Icon(Icons.check, size: 18, color: Colors.white)
            : Text('${i + 1}',
                style: TextStyle(fontWeight: FontWeight.bold, color: cur ? _purple : Colors.black45)),
      );
    }

    final kids = <Widget>[];
    for (var i = 0; i < 5; i++) {
      if (i > 0) {
        kids.add(Expanded(child: Container(height: 2, color: i <= _screen ? _purple : Colors.black12)));
      }
      kids.add(dot(i));
    }
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
      child: Row(children: kids),
    );
  }

  Widget _storeMap() {
    final markers = <Marker>{};
    LatLng? center;
    for (final s in _stores) {
      final ll = _ll(s);
      if (ll == null) continue;
      final id = (s['id'] as num).toInt();
      final sel = id == _storeId;
      if (sel) center = ll;
      center ??= ll;
      markers.add(Marker(
        markerId: MarkerId('$id'),
        position: ll,
        icon: BitmapDescriptor.defaultMarkerWithHue(
            sel ? BitmapDescriptor.hueGreen : BitmapDescriptor.hueViolet),
        infoWindow: InfoWindow(title: (s['name'] ?? '').toString()),
      ));
    }
    final c = center;
    if (c == null) return const SizedBox.shrink();
    return ClipRRect(
      borderRadius: BorderRadius.circular(14),
      child: SizedBox(
        height: 200,
        child: GoogleMap(
          key: ValueKey('map$_storeId'),
          initialCameraPosition: CameraPosition(target: c, zoom: 14),
          markers: markers,
          liteModeEnabled: true,
          zoomControlsEnabled: false,
          myLocationButtonEnabled: false,
          mapToolbarEnabled: false,
        ),
      ),
    );
  }

  Widget _storeCard(dynamic s, bool best) {
    final id = (s['id'] as num).toInt();
    final selected = _storeId == id;
    final km = _km(s);
    final open = _expanded.contains(id);
    final addr = (s['address'] ?? '').toString();
    final city = (s['city'] ?? '').toString();
    return GestureDetector(
      onTap: () => setState(() => _storeId = id),
      child: Container(
        margin: const EdgeInsets.only(bottom: 12),
        decoration: BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.circular(14),
          border: Border.all(color: selected ? _purple : Colors.black12, width: selected ? 2 : 1),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (best)
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                decoration: const BoxDecoration(
                  color: _purple,
                  borderRadius: BorderRadius.only(
                    topLeft: Radius.circular(12),
                    bottomRight: Radius.circular(12),
                  ),
                ),
                child: const Text('★ BEST STORE',
                    style: TextStyle(color: Colors.white, fontSize: 11, fontWeight: FontWeight.bold)),
              ),
            Padding(
              padding: const EdgeInsets.all(14),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Column(
                    children: [
                      const Icon(Icons.storefront, size: 32, color: _purple),
                      if (km != null)
                        Padding(
                          padding: const EdgeInsets.only(top: 4),
                          child: Text('${km.toStringAsFixed(2)} KM',
                              style: const TextStyle(fontSize: 11, color: Colors.black54)),
                        ),
                    ],
                  ),
                  const SizedBox(width: 14),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text((s['name'] ?? '').toString(),
                            style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600)),
                        const SizedBox(height: 4),
                        Text(
                          addr.isEmpty ? city : addr,
                          maxLines: open ? null : 1,
                          overflow: open ? TextOverflow.visible : TextOverflow.ellipsis,
                          style: const TextStyle(fontSize: 12, color: Colors.black54),
                        ),
                        GestureDetector(
                          onTap: () => setState(() {
                            if (open) {
                              _expanded.remove(id);
                            } else {
                              _expanded.add(id);
                            }
                          }),
                          child: Padding(
                            padding: const EdgeInsets.only(top: 6),
                            child: Text(open ? 'Hide' : 'View',
                                style: const TextStyle(
                                    fontSize: 12, fontWeight: FontWeight.bold, color: _purple)),
                          ),
                        ),
                      ],
                    ),
                  ),
                  Icon(selected ? Icons.radio_button_checked : Icons.radio_button_unchecked,
                      color: selected ? _purple : Colors.black38),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _storeStep() {
    final best = _bestId();
    final list = _visibleStores();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _title('Select your store', 'Pick the store you will deliver from'),
        _storeMap(),
        const SizedBox(height: 12),
        TextField(
          controller: _storeSearch,
          onChanged: (_) => setState(() {}),
          decoration: const InputDecoration(
            prefixIcon: Icon(Icons.search),
            hintText: 'Search store',
            border: OutlineInputBorder(),
            filled: true,
            fillColor: Colors.white,
          ),
        ),
        if (_pos == null && !_storesLoading)
          const Padding(
            padding: EdgeInsets.only(top: 8),
            child: Text('Turn on location to see distance and the best store.',
                style: TextStyle(fontSize: 12, color: Colors.black54)),
          ),
        const SizedBox(height: 12),
        if (_storesLoading)
          const Center(child: Padding(padding: EdgeInsets.all(24), child: CircularProgressIndicator()))
        else if (list.isEmpty)
          const Text('No stores found.', style: TextStyle(color: Colors.black54)),
        for (final s in list) _storeCard(s, (s['id'] as num).toInt() == best),
        _primary('Next', _storeId == null ? null : () => _save('store', {'warehouse_id': _storeId})),
      ],
    );
  }
  Widget _docsStep() {
    final bool ready;
    if (_docType == 'aadhaar') {
      ready = _aadhaar.isNotEmpty;
    } else if (_docType == 'driving_licence') {
      ready = _licence.isNotEmpty;
    } else {
      ready = _voter.isNotEmpty && _pan.isNotEmpty;
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _title('Upload any one of these documents', 'Clear photos of your original documents'),
        _choiceCard(
          icon: Icons.fingerprint,
          label: 'Aadhaar Card',
          subtitle: 'Faster verification',
          selected: _docType == 'aadhaar',
          onTap: () => setState(() => _docType = 'aadhaar'),
        ),
        _choiceCard(
          icon: Icons.badge_outlined,
          label: 'Driving licence',
          selected: _docType == 'driving_licence',
          onTap: () => setState(() => _docType = 'driving_licence'),
        ),
        _choiceCard(
          icon: Icons.how_to_vote_outlined,
          label: 'Voter ID + PAN card',
          selected: _docType == 'voter_pan',
          onTap: () => setState(() => _docType = 'voter_pan'),
        ),
        const SizedBox(height: 4),
        if (_docType == 'aadhaar') _docTile('Aadhaar card', 'aadhaar', _aadhaar),
        if (_docType == 'driving_licence') _docTile('Driving licence', 'licence', _licence),
        if (_docType == 'voter_pan') ...[
          _docTile('Voter ID', 'voter', _voter),
          _docTile('PAN card', 'pan', _pan),
        ],
        _primary(
          'Next',
          (!ready || _uploading != null)
              ? null
              : () => _save('documents', {
                    'id_doc_type': _docType,
                    'aadhaar_url': _aadhaar,
                    'licence_url': _licence,
                    'voter_url': _voter,
                    'pan_url': _pan,
                  }),
        ),
      ],
    );
  }
  Widget _selfieTip(IconData icon, bool ok, String text) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 14),
      child: Row(
        children: [
          Stack(
            children: [
              Container(
                width: 72,
                height: 72,
                decoration: BoxDecoration(
                  color: Colors.white,
                  borderRadius: BorderRadius.circular(14),
                  border: Border.all(color: Colors.black12),
                ),
                child: Icon(icon, size: 40, color: _purple),
              ),
              Positioned(
                top: 4,
                left: 4,
                child: Container(
                  width: 20,
                  height: 20,
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    color: ok ? Colors.green : Colors.red,
                  ),
                  child: Icon(ok ? Icons.check : Icons.close, size: 14, color: Colors.white),
                ),
              ),
            ],
          ),
          const SizedBox(width: 16),
          Expanded(child: Text(text, style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w500))),
        ],
      ),
    );
  }

  Widget _selfieStep() {
    if (_selfie.isEmpty) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _title('Take a selfie', 'Follow these tips for a quick approval'),
          _selfieTip(Icons.face, true, 'Complete face should be visible and inside frame'),
          const Divider(height: 1),
          _selfieTip(Icons.accessibility_new, true, 'Sit straight and wear proper clothes'),
          const Divider(height: 1),
          _selfieTip(Icons.masks, false, 'No mask or helmet'),
          if (_uploading == 'selfie')
            const Padding(
              padding: EdgeInsets.only(top: 16),
              child: Center(child: CircularProgressIndicator()),
            ),
          _primary(
            'Take selfie',
            _uploading != null ? null : () => _pick('selfie', selfie: true),
          ),
        ],
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _title('Check your selfie', 'Face clearly visible, good light, no mask or helmet'),
        Center(
          child: Padding(
            padding: const EdgeInsets.symmetric(vertical: 12),
            child: ClipOval(
              child: Image.network(
                _selfie,
                height: 180,
                width: 180,
                fit: BoxFit.cover,
                errorBuilder: (_, _, _) => const CircleAvatar(
                  radius: 90,
                  backgroundColor: Colors.white,
                  child: Icon(Icons.check_circle, size: 80, color: Colors.green),
                ),
              ),
            ),
          ),
        ),
        Center(
          child: _uploading == 'selfie'
              ? const CircularProgressIndicator()
              : OutlinedButton.icon(
                  onPressed: (_busy || _uploading != null) ? null : () => _pick('selfie', selfie: true),
                  icon: const Icon(Icons.photo_camera),
                  label: const Text('Retake'),
                ),
        ),
        _primary(
          'Next',
          _uploading != null ? null : () => _save('selfie', {'selfie_url': _selfie}),
        ),
      ],
    );
  }
  Widget _planCard({
    required String title,
    required String price,
    String? strike,
    String? chip,
    required String note,
    String? badge,
    required bool selected,
    required VoidCallback onTap,
  }) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        margin: const EdgeInsets.only(bottom: 12),
        decoration: BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.circular(14),
          border: Border.all(color: selected ? _purple : Colors.black12, width: selected ? 2 : 1),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (badge != null)
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                decoration: const BoxDecoration(
                  color: Color(0xFF1B8A4B),
                  borderRadius: BorderRadius.only(
                    topLeft: Radius.circular(12),
                    bottomRight: Radius.circular(12),
                  ),
                ),
                child: Text(badge,
                    style: const TextStyle(color: Colors.white, fontSize: 11, fontWeight: FontWeight.bold)),
              ),
            Padding(
              padding: const EdgeInsets.all(14),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(title, style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600)),
                        const SizedBox(height: 6),
                        Wrap(
                          crossAxisAlignment: WrapCrossAlignment.center,
                          spacing: 8,
                          runSpacing: 4,
                          children: [
                            Text(price, style: const TextStyle(fontSize: 22, fontWeight: FontWeight.bold)),
                            if (strike != null)
                              Text(strike,
                                  style: const TextStyle(
                                      color: Colors.black45, decoration: TextDecoration.lineThrough)),
                            if (chip != null)
                              Container(
                                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                                decoration: BoxDecoration(
                                  color: const Color(0xFFE3F5EA),
                                  borderRadius: BorderRadius.circular(6),
                                ),
                                child: Text(chip,
                                    style: const TextStyle(fontSize: 12, color: Color(0xFF1B8A4B))),
                              ),
                          ],
                        ),
                        const SizedBox(height: 8),
                        Text(note, style: const TextStyle(fontSize: 12, color: Colors.black54)),
                      ],
                    ),
                  ),
                  Icon(selected ? Icons.radio_button_checked : Icons.radio_button_unchecked,
                      color: selected ? _purple : Colors.black38),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _payRow(IconData icon, String label, {Widget? trailing}) {
    return InkWell(
      onTap: _busy ? null : () => _save('payout', <String, dynamic>{}),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 14),
        child: Row(
          children: [
            Container(
              width: 44,
              height: 44,
              decoration: BoxDecoration(
                color: _bg,
                borderRadius: BorderRadius.circular(10),
              ),
              child: Icon(icon, color: _purple),
            ),
            const SizedBox(width: 14),
            Expanded(
              child: Text(label, style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600)),
            ),
            trailing ?? const Icon(Icons.chevron_right, color: Colors.black45),
          ],
        ),
      ),
    );
  }

  Widget _paySection(String label) {
    return Padding(
      padding: const EdgeInsets.only(top: 20, bottom: 12),
      child: Center(
        child: Text(
          label,
          style: const TextStyle(
              fontSize: 13, letterSpacing: 2, color: Colors.black45, fontWeight: FontWeight.w600),
        ),
      ),
    );
  }

  Widget _payMethodStep() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            IconButton(
              icon: const Icon(Icons.arrow_back),
              onPressed: () => setState(() => _payMethodScreen = false),
            ),
            const Text('Select Payment Method',
                style: TextStyle(fontSize: 18, fontWeight: FontWeight.w600)),
          ],
        ),
        _paySection('RECOMMENDED'),
        Container(
          decoration: BoxDecoration(color: Colors.white, borderRadius: BorderRadius.circular(16)),
          child: Column(
            children: [
              _payRow(Icons.account_balance_wallet_outlined, 'Google Pay UPI'),
              const Divider(height: 1, indent: 14, endIndent: 14),
              _payRow(Icons.account_balance_wallet_outlined, 'PhonePe UPI'),
              const Divider(height: 1, indent: 14, endIndent: 14),
              _payRow(Icons.account_balance_wallet_outlined, 'WhatsApp UPI'),
            ],
          ),
        ),
        _paySection('CARDS'),
        Container(
          decoration: BoxDecoration(color: Colors.white, borderRadius: BorderRadius.circular(16)),
          child: _payRow(
            Icons.credit_card,
            'Add credit or debit cards',
            trailing: const Text('ADD',
                style: TextStyle(color: _purple, fontWeight: FontWeight.bold)),
          ),
        ),
      ],
    );
  }
  Widget _planStep() {
    final full = _plan == 'full';
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Container(
          width: double.infinity,
          padding: const EdgeInsets.symmetric(vertical: 24, horizontal: 16),
          decoration: BoxDecoration(
            color: _purple,
            borderRadius: BorderRadius.circular(16),
          ),
          child: const Column(
            children: [
              Icon(Icons.savings_outlined, size: 56, color: Colors.white),
              SizedBox(height: 10),
              Text('Join the team and start earning',
                  textAlign: TextAlign.center,
                  style: TextStyle(color: Colors.white, fontSize: 16, fontWeight: FontWeight.w600)),
            ],
          ),
        ),
        const SizedBox(height: 20),
        const Center(
          child: Text('Select your plan', style: TextStyle(fontSize: 18, fontWeight: FontWeight.w600)),
        ),
        const SizedBox(height: 14),
        _planCard(
          title: 'Pay full',
          price: '\u20B91506',
          strike: '\u20B92151',
          chip: 'Save \u20B9645',
          note: 'No extra charges later',
          badge: '\u2605 HIGHER SAVINGS',
          selected: full,
          onTap: () => setState(() => _plan = 'full'),
        ),
        _planCard(
          title: 'Pay in instalments',
          price: '\u20B9535/week',
          chip: 'for 4 weeks',
          note: 'Pay instalments from your earnings | Total: \u20B92151',
          selected: !full,
          onTap: () => setState(() => _plan = 'instalments'),
        ),
        _primary(
          full ? 'Join for \u20B91506' : 'Join for \u20B9535/week',
          () => setState(() => _payMethodScreen = true),
        ),
      ],
    );
  }

  Widget _moneyStep() {
    if (_payMethodScreen) return _payMethodStep();
    return _planStep();
  }

  Widget _stepBody() {
    switch (_screen) {
      case 0:
        return _vehicleStep();
      case 1:
        return _storeStep();
      case 2:
        return _docsStep();
      case 3:
        return _selfieStep();
      default:
        return _moneyStep();
    }
  }

  Widget _review() {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.hourglass_top, size: 64, color: _purple),
            const SizedBox(height: 16),
            const Text('Application under review',
                style: TextStyle(fontSize: 20, fontWeight: FontWeight.bold)),
            const SizedBox(height: 8),
            const Text('We will notify you once your account is approved.', textAlign: TextAlign.center),
            if (_error != null) ...[
              const SizedBox(height: 12),
              Text(_error!, style: const TextStyle(color: Colors.red)),
            ],
            const SizedBox(height: 24),
            ElevatedButton(
              onPressed: _busy ? null : _refresh,
              style: ElevatedButton.styleFrom(backgroundColor: _purple, foregroundColor: Colors.white),
              child: _busy
                  ? const SizedBox(
                      height: 18,
                      width: 18,
                      child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                    )
                  : const Text('Check status'),
            ),
            TextButton(onPressed: _logout, child: const Text('Log out')),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final review = _status == 'pending';
    return PopScope(
      canPop: review || _screen == 0,
      onPopInvokedWithResult: (didPop, result) {
        if (!didPop && !_busy) setState(() => _screen--);
      },
      child: Scaffold(
        backgroundColor: _bg,
        appBar: AppBar(
          backgroundColor: _bg,
          elevation: 0,
          automaticallyImplyLeading: false,
          leading: (!review && _screen > 0)
              ? IconButton(
                  icon: const Icon(Icons.arrow_back),
                  onPressed: _busy ? null : () => setState(() => _screen--),
                )
              : null,
          title: Text(review ? 'Application' : 'Step ${_screen + 1} of 5',
              style: const TextStyle(color: Colors.black87)),
          iconTheme: const IconThemeData(color: Colors.black87),
        ),
        body: SafeArea(
          child: review
              ? _review()
              : Column(
                  children: [
                    _stepper(),
                    Expanded(
                      child: SingleChildScrollView(
                        padding: const EdgeInsets.all(20),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            if (_status == 'rejected')
                              Container(
                                width: double.infinity,
                                margin: const EdgeInsets.only(bottom: 16),
                                padding: const EdgeInsets.all(12),
                                decoration: BoxDecoration(
                                  color: Colors.red.shade50,
                                  borderRadius: BorderRadius.circular(10),
                                ),
                                child: const Text(
                                  'Your application was rejected. Please update your details and submit again.',
                                  style: TextStyle(color: Colors.red),
                                ),
                              ),
                            _stepBody(),
                            if (_error != null)
                              Padding(
                                padding: const EdgeInsets.only(top: 12),
                                child: Text(_error!, style: const TextStyle(color: Colors.red)),
                              ),
                            Center(child: TextButton(onPressed: _logout, child: const Text('Log out'))),
                          ],
                        ),
                      ),
                    ),
                  ],
                ),
        ),
      ),
    );
  }
}