// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'package:flutter/material.dart';
import 'package:mobile_scanner/mobile_scanner.dart';
import '../theme.dart';

String fmtDateTime(dynamic raw) {
  final d = DateTime.tryParse('${raw ?? ''}')?.toLocal();
  if (d == null) return '-';
  return '${longDate(d)} ${d.year}, ${clock12(d)}';
}

String fmtDay(dynamic raw) {
  final d = DateTime.tryParse('${raw ?? ''}')?.toLocal();
  if (d == null) return '-';
  return '${longDate(d)} ${d.year}';
}

String binText(dynamic bin) {
  final m = asMap(bin);
  if (m.isEmpty) return '-';
  final rack = asMap(m['rack']);
  final zone = asMap(rack['zone']);
  final parts = [zone['name'], rack['name'], m['name']]
      .map((e) => (e ?? '').toString().trim())
      .where((e) => e.isNotEmpty)
      .toList();
  return parts.isEmpty ? 'Bin #${m['id']}' : parts.join(' - ');
}

String imgUrl(dynamic raw) {
  final s = (raw ?? '').toString();
  if (s.isEmpty) return '';
  if (s.startsWith('http')) return s;
  return '$kOrigin${s.startsWith('/') ? '' : '/'}$s';
}

Widget putawayCard(Map<String, dynamic> t, {VoidCallback? onTap}) {
  final product = asMap(t['product']);
  final name = (product['name'] ?? 'Product #${t['product_id'] ?? ''}').toString();
  final img = imgUrl(product['image_url']);
  final status = (t['status'] ?? '').toString();
  final done = status == 'completed';
  final bin = binText(done ? t['actual_bin'] : t['suggested_bin']);
  final when = fmtDateTime(done ? t['completed_at'] : (t['started_at'] ?? t['assigned_at'] ?? t['created_at']));
  final color = done ? AppColors.green : (status == 'in_progress' ? AppColors.blue : AppColors.orange);
  final label = done ? 'Completed' : (status == 'in_progress' ? 'In progress' : 'Pending');
  Widget box(IconData ic) => Container(width: 54, height: 54, color: Color(0xFFF2F4F7), child: Icon(ic, size: 22, color: Colors.grey));
  return Padding(
    padding: EdgeInsets.only(bottom: 10),
    child: InkWell(
      borderRadius: BorderRadius.circular(16),
      onTap: onTap,
      child: Container(
        padding: EdgeInsets.all(12),
        decoration: cardDecoration(),
        child: Row(
          children: [
            ClipRRect(
              borderRadius: BorderRadius.circular(12),
              child: img.isEmpty
                  ? box(Icons.inventory_2_outlined)
                  : Image.network(img, width: 54, height: 54, fit: BoxFit.cover, errorBuilder: (c, e, s) => box(Icons.broken_image_outlined)),
            ),
            SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(name, maxLines: 2, overflow: TextOverflow.ellipsis, style: TextStyle(fontSize: 14, fontWeight: FontWeight.w800, color: AppColors.ink)),
                  SizedBox(height: 6),
                  Wrap(
                    spacing: 6,
                    runSpacing: 4,
                    children: [
                      Container(
                        padding: EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                        decoration: BoxDecoration(color: Color(0xFFF2F4F7), borderRadius: BorderRadius.circular(8)),
                        child: Text('Qty ${asInt(t['quantity'])}', style: TextStyle(fontSize: 11, fontWeight: FontWeight.w700, color: AppColors.ink)),
                      ),
                      Container(
                        padding: EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                        decoration: BoxDecoration(color: Color(0xFFEFF5FF), borderRadius: BorderRadius.circular(8)),
                        child: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Icon(Icons.place_outlined, size: 12, color: AppColors.blue),
                            SizedBox(width: 3),
                            Text(bin, style: TextStyle(fontSize: 11, fontWeight: FontWeight.w700, color: AppColors.blue)),
                          ],
                        ),
                      ),
                    ],
                  ),
                  SizedBox(height: 4),
                  Text(when, style: TextStyle(fontSize: 11, color: AppColors.muted)),
                  if (t['wrong_location'] == true)
                    Padding(padding: EdgeInsets.only(top: 2), child: Text('Different bin used', style: TextStyle(fontSize: 11, color: AppColors.orange))),
                ],
              ),
            ),
            SizedBox(width: 8),
            StatusPill(label, color),
          ],
        ),
      ),
    ),
  );
}

Future<String?> askCode(BuildContext context, String title, String hint) {
  final c = TextEditingController();
  return showDialog<String>(
    context: context,
    builder: (ctx) => AlertDialog(
      title: Text(title),
      content: TextField(controller: c, autofocus: true, decoration: InputDecoration(hintText: hint)),
      actions: [
        TextButton(onPressed: () => Navigator.pop(ctx), child: Text('Cancel')),
        TextButton(onPressed: () => Navigator.pop(ctx, c.text.trim()), child: Text('OK')),
      ],
    ),
  );
}

Future<String?> scanCode(BuildContext context, String title, String hint) {
  return Navigator.push<String>(context, MaterialPageRoute(builder: (_) => ScannerPage(title: title, hint: hint)));
}

class ScannerPage extends StatefulWidget {
  final String title;
  final String hint;
  const ScannerPage({super.key, required this.title, required this.hint});

  @override
  State<ScannerPage> createState() => _ScannerPageState();
}

class _ScannerPageState extends State<ScannerPage> {
  final MobileScannerController _c = MobileScannerController(detectionSpeed: DetectionSpeed.noDuplicates);
  bool _done = false;

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  void _onDetect(BarcodeCapture cap) {
    if (_done || cap.barcodes.isEmpty) return;
    final v = cap.barcodes.first.rawValue;
    if (v == null || v.trim().isEmpty) return;
    _done = true;
    Navigator.pop(context, v.trim());
  }

  Future<void> _manual() async {
    final v = await askCode(context, widget.title, widget.hint);
    if (v == null || v.isEmpty || !mounted) return;
    _done = true;
    Navigator.pop(context, v);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      body: Stack(
        children: [
          MobileScanner(controller: _c, onDetect: _onDetect),
          Center(
            child: Container(
              width: 260,
              height: 260,
              decoration: BoxDecoration(border: Border.all(color: Colors.white, width: 3), borderRadius: BorderRadius.circular(20)),
            ),
          ),
          SafeArea(
            child: Padding(
              padding: EdgeInsets.all(8),
              child: Row(
                children: [
                  IconButton(onPressed: () => Navigator.pop(context), icon: Icon(Icons.arrow_back, color: Colors.white)),
                  Expanded(child: Text(widget.title, style: TextStyle(color: Colors.white, fontSize: 18, fontWeight: FontWeight.w800))),
                  IconButton(onPressed: () => _c.toggleTorch(), icon: Icon(Icons.flashlight_on_outlined, color: Colors.white)),
                ],
              ),
            ),
          ),
          Positioned(
            left: 20,
            right: 20,
            bottom: 36,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text('Point the camera at the code', style: TextStyle(color: Colors.white70)),
                SizedBox(height: 12),
                PrimaryButton(label: 'Enter manually', icon: Icons.keyboard_outlined, color: AppColors.blue, onPressed: _manual),
              ],
            ),
          ),
        ],
      ),
    );
  }
}