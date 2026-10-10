// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'package:mobile_scanner/mobile_scanner.dart';

const Color _kInk = Color(0xFF111111);
const Color _kGreen = Color(0xFF16A34A);

// ---------------------------------------------------------------- 1. Scan
class PutterScanScreen extends StatefulWidget {
  const PutterScanScreen({super.key});

  @override
  State<PutterScanScreen> createState() => _PutterScanScreenState();
}

class _PutterScanScreenState extends State<PutterScanScreen> {
  bool _done = false;
  bool _warned = false;

  void _onDetect(BarcodeCapture cap) {
    if (_done) return;
    final v = cap.barcodes.isNotEmpty ? cap.barcodes.first.rawValue : null;
    if (v == null || v.isEmpty) return;
    if (!v.startsWith('PUTTER-')) {
      if (!_warned) {
        _warned = true;
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Invalid QR code')));
        Future.delayed(Duration(seconds: 2), () => _warned = false);
      }
      return;
    }
    _done = true;
    Navigator.pushReplacement(context, MaterialPageRoute(builder: (_) => const PutterInboundScreen()));
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        foregroundColor: Colors.white,
        elevation: 0,
        title: Text('Scan QR code', style: TextStyle(fontWeight: FontWeight.w700)),
      ),
      body: Stack(
        children: [
          MobileScanner(onDetect: _onDetect),
          Center(
            child: Container(
              width: 240,
              height: 240,
              decoration: BoxDecoration(
                border: Border.all(color: Colors.white, width: 3),
                borderRadius: BorderRadius.circular(16),
              ),
            ),
          ),
          Positioned(
            left: 0,
            right: 0,
            bottom: 60,
            child: Text('Scan the truck QR code to start',
                textAlign: TextAlign.center, style: TextStyle(color: Colors.white, fontSize: 15)),
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------- 2. Inbound tasks
class PutterInboundScreen extends StatelessWidget {
  const PutterInboundScreen({super.key});

  Widget _row(String k, String v) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 12),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(k, style: TextStyle(fontSize: 14)),
          Text(v, style: TextStyle(fontSize: 14, fontWeight: FontWeight.w600)),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xFFF2F3F5),
      appBar: AppBar(
        backgroundColor: Colors.white,
        foregroundColor: Colors.black,
        elevation: 0,
        title: Text('Inbound Tasks', style: TextStyle(fontWeight: FontWeight.w700, fontSize: 17)),
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          Text('Tasks Count : 1', style: TextStyle(fontSize: 12, color: Colors.black54)),
          SizedBox(height: 10),
          Container(
            padding: const EdgeInsets.all(14),
            decoration: BoxDecoration(color: Colors.white, borderRadius: BorderRadius.circular(14)),
            child: Column(
              children: [
                _row('Truck Id', 'WB19L0994'),
                Divider(height: 1),
                _row('Consignment Id', '133011'),
                Divider(height: 1),
                _row('Invoice Count', '84'),
                Divider(height: 1),
                _row('Total Crates', '286'),
                Divider(height: 1),
                _row('Arrival Time', 'Truck not arrived'),
                Divider(height: 1),
                Padding(
                  padding: const EdgeInsets.symmetric(vertical: 12),
                  child: Text('Main Warehouse', style: TextStyle(fontSize: 13)),
                ),
                SizedBox(
                  width: double.infinity,
                  child: ElevatedButton(
                    onPressed: () => Navigator.push(
                        context, MaterialPageRoute(builder: (_) => const PutterTruckPhotoScreen())),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: _kInk,
                      foregroundColor: Colors.white,
                      padding: const EdgeInsets.symmetric(vertical: 16),
                      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                    ),
                    child: Text('Start Task', style: TextStyle(fontSize: 15, fontWeight: FontWeight.w700)),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------- 3. Truck photos
class PutterTruckPhotoScreen extends StatefulWidget {
  const PutterTruckPhotoScreen({super.key});

  @override
  State<PutterTruckPhotoScreen> createState() => _PutterTruckPhotoScreenState();
}

class _PutterTruckPhotoScreenState extends State<PutterTruckPhotoScreen> {
  final ImagePicker _picker = ImagePicker();
  final TextEditingController _lockCtl = TextEditingController();
  String? _lockPhoto;
  String? _doorsPhoto;

  @override
  void dispose() {
    _lockCtl.dispose();
    super.dispose();
  }

  Future<void> _pick(bool lock) async {
    final src = await showModalBottomSheet<ImageSource>(
      context: context,
      builder: (_) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: Icon(Icons.photo_camera_outlined),
              title: Text('Camera'),
              onTap: () => Navigator.pop(context, ImageSource.camera),
            ),
            ListTile(
              leading: Icon(Icons.photo_library_outlined),
              title: Text('Gallery'),
              onTap: () => Navigator.pop(context, ImageSource.gallery),
            ),
          ],
        ),
      ),
    );
    if (src == null) return;
    try {
      final x = await _picker.pickImage(source: src, imageQuality: 70);
      if (x == null || !mounted) return;
      setState(() {
        if (lock) {
          _lockPhoto = x.path;
        } else {
          _doorsPhoto = x.path;
        }
      });
    } catch (_) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Could not open camera/gallery')));
    }
  }

  Widget _tile(String title, String step, String? path, VoidCallback onTap) {
    return InkWell(
      onTap: onTap,
      child: Container(
        margin: const EdgeInsets.only(bottom: 12),
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: const Color(0xFFE4E7EC)),
        ),
        child: Row(
          children: [
            Container(
              width: 48,
              height: 48,
              decoration: BoxDecoration(color: const Color(0xFFF2F3F5), borderRadius: BorderRadius.circular(8)),
              clipBehavior: Clip.antiAlias,
              child: path == null
                  ? Icon(Icons.photo_camera_outlined, color: Colors.black54)
                  : Image.file(File(path), fit: BoxFit.cover),
            ),
            SizedBox(width: 14),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title, style: TextStyle(fontSize: 15, fontWeight: FontWeight.w700)),
                  Text(step, style: TextStyle(fontSize: 12, color: Colors.black54)),
                ],
              ),
            ),
            if (path != null) Icon(Icons.check_circle, color: _kGreen),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final photo = _doorsPhoto ?? _lockPhoto;
    final canContinue = photo != null;
    return Scaffold(
      backgroundColor: const Color(0xFFF2F3F5),
      appBar: AppBar(
        backgroundColor: Colors.white,
        foregroundColor: Colors.black,
        elevation: 0,
        leading: IconButton(icon: Icon(Icons.close), onPressed: () => Navigator.pop(context)),
        title: Text('Unloading Truck Details', style: TextStyle(fontWeight: FontWeight.w700, fontSize: 17)),
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text('Truck Number', style: TextStyle(fontWeight: FontWeight.w600)),
              Text('WB19L0994', style: TextStyle(fontWeight: FontWeight.w800)),
            ],
          ),
          SizedBox(height: 16),
          _tile('Add photo - Truck Lock', 'Step 01', _lockPhoto, () => _pick(true)),
          Container(
            margin: const EdgeInsets.only(bottom: 12),
            padding: const EdgeInsets.all(14),
            decoration: BoxDecoration(
              color: Colors.white,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: const Color(0xFFE4E7EC)),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('Enter lock id to unlock consignment in truck',
                    style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600)),
                SizedBox(height: 8),
                TextField(
                  controller: _lockCtl,
                  decoration: InputDecoration(
                    hintText: 'Lock id (optional for testing)',
                    border: OutlineInputBorder(),
                    isDense: true,
                  ),
                ),
              ],
            ),
          ),
          _tile('Add photo - Open truck doors', 'Step 03', _doorsPhoto, () => _pick(false)),
          SizedBox(height: 8),
          ElevatedButton(
            onPressed: canContinue
                ? () => Navigator.push(
                    context, MaterialPageRoute(builder: (_) => PutterUploadedScreen(photoPath: photo)))
                : null,
            style: ElevatedButton.styleFrom(
              backgroundColor: _kInk,
              foregroundColor: Colors.white,
              disabledBackgroundColor: Colors.grey.shade400,
              disabledForegroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(vertical: 16),
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
            ),
            child: Text('Continue', style: TextStyle(fontSize: 15, fontWeight: FontWeight.w700)),
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------- 4. Uploaded
class PutterUploadedScreen extends StatelessWidget {
  final String photoPath;
  const PutterUploadedScreen({super.key, required this.photoPath});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      body: SafeArea(
        child: Column(
          children: [
            Expanded(
              child: Stack(
                children: [
                  Positioned.fill(child: Image.file(File(photoPath), fit: BoxFit.cover)),
                  Positioned(
                    left: 14,
                    top: 14,
                    child: InkWell(
                      onTap: () => Navigator.pop(context),
                      child: CircleAvatar(
                        backgroundColor: Colors.white,
                        child: Icon(Icons.close, color: Colors.black),
                      ),
                    ),
                  ),
                ],
              ),
            ),
            Container(
              color: Colors.white,
              padding: const EdgeInsets.all(16),
              child: SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  onPressed: () => Navigator.push(
                      context, MaterialPageRoute(builder: (_) => const PutterUnlockScreen())),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: const Color(0xFF22C55E),
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(vertical: 16),
                    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(6)),
                  ),
                  child: Text('Uploaded', style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600)),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------- 5. Unlocking details
class PutterUnlockScreen extends StatelessWidget {
  const PutterUnlockScreen({super.key});

  Widget _row(String k, String v) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 14),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(k, style: TextStyle(fontSize: 14, fontWeight: FontWeight.w600)),
          Text(v, style: TextStyle(fontSize: 14, fontWeight: FontWeight.w800)),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black54,
      body: SafeArea(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.end,
          children: [
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(18),
              decoration: BoxDecoration(
                color: Colors.white,
                borderRadius: BorderRadius.vertical(top: Radius.circular(18)),
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text('Unlocking Truck Details', style: TextStyle(fontSize: 16, fontWeight: FontWeight.w800)),
                      InkWell(onTap: () => Navigator.pop(context), child: Icon(Icons.close, size: 20)),
                    ],
                  ),
                  SizedBox(height: 6),
                  _row('Truck ID', 'WB19L0994'),
                  _row('Consignment ID', '133011'),
                  _row('Invoice count', '84'),
                  _row('Total crates', '286'),
                  _row('Date', '8 Oct, 11:00 AM'),
                  SizedBox(height: 8),
                  SizedBox(
                    width: double.infinity,
                    child: ElevatedButton(
                      onPressed: () {
                        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Unloading started')));
                        Navigator.popUntil(context, (r) => r.isFirst);
                      },
                      style: ElevatedButton.styleFrom(
                        backgroundColor: _kInk,
                        foregroundColor: Colors.white,
                        padding: const EdgeInsets.symmetric(vertical: 16),
                        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                      ),
                      child: Text('Start Unloading', style: TextStyle(fontSize: 15, fontWeight: FontWeight.w700)),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}