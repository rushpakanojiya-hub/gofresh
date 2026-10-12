import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import '../services/api_service.dart';

/// Profile avatar: shows the uploaded photo (or the name initial) and lets the
/// picker change it from camera or gallery.
class ProfilePhotoAvatar extends StatefulWidget {
  final String initial;
  const ProfilePhotoAvatar({super.key, required this.initial});

  @override
  State<ProfilePhotoAvatar> createState() => _ProfilePhotoAvatarState();
}

class _ProfilePhotoAvatarState extends State<ProfilePhotoAvatar> {
  static const String _origin = 'https://gofresh-evl7.onrender.com';
  String _url = '';
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  String _abs(String u) {
    if (u.isEmpty || u.startsWith('http')) return u;
    return '$_origin${u.startsWith('/') ? '' : '/'}$u';
  }

  Future<void> _load() async {
    try {
      final r = await ApiService.getPickerProfile();
      if (!mounted || r['error'] != null) return;
      setState(() => _url = _abs((r['photo_url'] ?? '').toString()));
    } catch (_) {}
  }

  void _snack(String m) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(m)));
  }

  Future<void> _choose() async {
    final src = await showModalBottomSheet<ImageSource>(
      context: context,
      builder: (ctx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.photo_camera_outlined),
              title: const Text('Take photo'),
              onTap: () => Navigator.pop(ctx, ImageSource.camera),
            ),
            ListTile(
              leading: const Icon(Icons.photo_library_outlined),
              title: const Text('Choose from gallery'),
              onTap: () => Navigator.pop(ctx, ImageSource.gallery),
            ),
          ],
        ),
      ),
    );
    if (src == null) return;
    try {
      final x = await ImagePicker().pickImage(source: src, imageQuality: 70, maxWidth: 1024, maxHeight: 1024);
      if (x == null || !mounted) return;
      setState(() => _busy = true);
      final r = await ApiService.uploadProfilePhoto(x.path);
      if (!mounted) return;
      if (r['error'] != null) {
        setState(() => _busy = false);
        _snack(r['error'].toString());
        return;
      }
      setState(() {
        _url = _abs((r['image_url'] ?? '').toString());
        _busy = false;
      });
      _snack('Profile photo updated');
    } catch (_) {
      if (mounted) setState(() => _busy = false);
      _snack('Could not upload the photo. Please try again.');
    }
  }

  @override
  Widget build(BuildContext context) {
    return Center(
      child: GestureDetector(
        onTap: _busy ? null : _choose,
        child: Stack(
          children: [
            CircleAvatar(
              radius: 38,
              backgroundColor: const Color(0xFFE3F2FD),
              backgroundImage: _url.isEmpty ? null : NetworkImage(_url),
              child: _url.isEmpty
                  ? Text(widget.initial,
                      style: const TextStyle(fontSize: 32, fontWeight: FontWeight.w800, color: Color(0xFF0D47A1)))
                  : null,
            ),
            if (_busy)
              const Positioned.fill(
                child: DecoratedBox(
                  decoration: BoxDecoration(color: Color(0x66000000), shape: BoxShape.circle),
                  child: Center(
                    child: SizedBox(
                      width: 22,
                      height: 22,
                      child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                    ),
                  ),
                ),
              ),
            Positioned(
              right: 0,
              bottom: 0,
              child: Container(
                padding: const EdgeInsets.all(5),
                decoration: const BoxDecoration(color: Colors.black, shape: BoxShape.circle),
                child: const Icon(Icons.photo_camera, size: 14, color: Colors.white),
              ),
            ),
          ],
        ),
      ),
    );
  }
}