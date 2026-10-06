import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import '../services/push_service.dart';
import 'ratings_screen.dart';
import '../services/api_service.dart';
import '../services/location_service.dart';
import 'login_screen.dart';
import 'notifications_screen.dart';
import 'support_screen.dart';

class ProfileScreen extends StatefulWidget {
  const ProfileScreen({super.key});

  @override
  State<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends State<ProfileScreen> {
  static const Color primaryPurple = Color(0xFF5B2A9E);
  static const Color pageBg = Color(0xFFF7F1FB);

  Map<String, dynamic>? _profile;
  Map<String, dynamic>? _rating;
  bool _loading = true;
  bool _uploadingPhoto = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    PushService.ratingRefresh.addListener(_load);
    _load();
  }

  @override
  void dispose() {
    PushService.ratingRefresh.removeListener(_load);
    super.dispose();
  }

  Future<void> _editName(String current) async {
    final ctrl = TextEditingController(text: current == 'Delivery Partner' ? '' : current);
    final newName = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Edit name'),
        content: TextField(
          controller: ctrl,
          autofocus: true,
          textCapitalization: TextCapitalization.words,
          decoration: const InputDecoration(hintText: 'Your name'),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Cancel')),
          TextButton(onPressed: () => Navigator.pop(ctx, ctrl.text.trim()), child: const Text('Save')),
        ],
      ),
    );
    if (newName == null || newName.isEmpty || newName == current) return;
    try {
      await ApiService.updateProfile({'name': newName});
      if (!mounted) return;
      await _load();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(e.toString().replaceFirst('Exception: ', ''))),
      );
    }
  }
  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final data = await ApiService.getProfile();
      try { _rating = await ApiService.getMyRating(); } catch (_) {}
      setState(() {
        _profile = data;
        _loading = false;
      });
    } catch (e) {
      setState(() {
        _error = 'Failed to load profile';
        _loading = false;
      });
    }
  }

  Future<void> _changePhoto() async {
    final source = await showModalBottomSheet<ImageSource>(
      context: context,
      builder: (ctx) => SafeArea(
        child: Wrap(
          children: [
            ListTile(
              leading: const Icon(Icons.photo_camera_outlined),
              title: const Text('Take a photo'),
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
    if (source == null) return;
    final picked = await ImagePicker().pickImage(source: source, maxWidth: 1024, imageQuality: 80);
    if (picked == null) return;
    setState(() => _uploadingPhoto = true);
    try {
      final url = await ApiService.uploadProfilePhoto(picked.path);
      await ApiService.saveProfilePhoto(url);
      if (!mounted) return;
      await _load();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(e.toString().replaceFirst('Exception: ', ''))),
      );
    } finally {
      if (mounted) setState(() => _uploadingPhoto = false);
    }
  }

  Future<void> _logout(BuildContext context) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Logout'),
        content: const Text('Are you sure you want to logout?'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('No')),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Yes', style: TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    await ApiService.clearToken();
    LocationService.stopTracking();
    if (!context.mounted) return;
    Navigator.pushAndRemoveUntil(
      context,
      MaterialPageRoute(builder: (_) => const LoginScreen()),
      (route) => false,
    );
  }

  Widget _tile(IconData icon, String title, {String? subtitle, VoidCallback? onTap}) {
    return Container(
      margin: const EdgeInsets.only(bottom: 10),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: const Color(0xFFEDE6F7)),
      ),
      child: ListTile(
        leading: Icon(icon, color: primaryPurple),
        title: Text(title),
        subtitle: subtitle != null ? Text(subtitle) : null,
        trailing: onTap != null ? const Icon(Icons.chevron_right, size: 18) : null,
        onTap: onTap,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final name = _profile?['name'] ?? 'Delivery Partner';
    final phone = _profile?['phone'] ?? '';
    final vehicleType = _profile?['vehicle_type'];
    final vehicleNumber = _profile?['vehicle_number'];
    final String? photoUrl = _profile?['profile_photo_url']?.toString();

    return Scaffold(
      backgroundColor: pageBg,
      body: SafeArea(
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : RefreshIndicator(
                onRefresh: _load,
                child: ListView(
                  padding: const EdgeInsets.all(16),
                  children: [
                    const Text(
                      'Profile',
                      style: TextStyle(fontSize: 20, fontWeight: FontWeight.bold, color: Colors.black87),
                    ),
                    const SizedBox(height: 24),
                    Center(
                      child: GestureDetector(
                        onTap: _uploadingPhoto ? null : _changePhoto,
                        child: Stack(
                          children: [
                            CircleAvatar(
                              radius: 42,
                              backgroundColor: const Color(0xFFEDE6F7),
                              backgroundImage: (photoUrl != null && photoUrl.isNotEmpty) ? NetworkImage(photoUrl) : null,
                              child: (photoUrl == null || photoUrl.isEmpty)
                                  ? const Icon(Icons.person, color: primaryPurple, size: 44)
                                  : null,
                            ),
                            Positioned(
                              right: 0,
                              bottom: 0,
                              child: Container(
                                padding: const EdgeInsets.all(6),
                                decoration: const BoxDecoration(color: primaryPurple, shape: BoxShape.circle),
                                child: _uploadingPhoto
                                    ? const SizedBox(
                                        width: 14,
                                        height: 14,
                                        child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                                      )
                                    : const Icon(Icons.camera_alt, size: 14, color: Colors.white),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                    const SizedBox(height: 12),
                    Center(
                      child: Text(name, style: const TextStyle(fontSize: 17, fontWeight: FontWeight.w600)),
                    ),
                    if (phone.toString().isNotEmpty)
                      Center(
                        child: Text(phone.toString(), style: const TextStyle(fontSize: 13, color: Colors.black54)),
                      ),
                    const SizedBox(height: 28),
                    if (_error != null) ...[
                      Text(_error!, style: const TextStyle(color: Colors.red)),
                      const SizedBox(height: 12),
                    ],
                    const Text(
                      'ACCOUNT',
                      style: TextStyle(fontWeight: FontWeight.bold, fontSize: 12, color: Colors.black45, letterSpacing: 0.5),
                    ),
                    const SizedBox(height: 8),
                    _tile(Icons.person_outline, 'Name', subtitle: name.toString(), onTap: () => _editName(name.toString())),
                    _tile(Icons.phone_outlined, 'Mobile', subtitle: phone.toString().isNotEmpty ? phone.toString() : '-'),
                    _tile(Icons.star_outline, 'Rating', subtitle: (_rating != null && ((_rating!['rating_count'] ?? 0) as num) > 0) ? '${_rating!['avg_rating']} / 5  (${_rating!['rating_count']} ratings)' : 'No ratings yet', onTap: () => Navigator.push(context, MaterialPageRoute(builder: (_) => const RatingsScreen())).then((_) => _load())),
                    _tile(
                      Icons.two_wheeler_outlined,
                      'Vehicle',
                      subtitle: vehicleType != null
                          ? '$vehicleType${vehicleNumber != null ? ' \u00B7 $vehicleNumber' : ''}'
                          : 'Not set',
                    ),
                    const SizedBox(height: 20),
                    const Text(
                      'MORE',
                      style: TextStyle(fontWeight: FontWeight.bold, fontSize: 12, color: Colors.black45, letterSpacing: 0.5),
                    ),
                    const SizedBox(height: 8),
                    _tile(
                      Icons.notifications_none,
                      'Notifications',
                      onTap: () {
                        Navigator.push(context, MaterialPageRoute(builder: (_) => const NotificationsScreen()));
                      },
                    ),

                    _tile(
                      Icons.support_agent_outlined,
                      'Support',
                      onTap: () {
                        Navigator.push(context, MaterialPageRoute(builder: (_) => const SupportScreen()));
                      },
                    ),
                    const SizedBox(height: 20),
                    Container(
                      decoration: BoxDecoration(
                        color: Colors.white,
                        borderRadius: BorderRadius.circular(14),
                        border: Border.all(color: const Color(0xFFEDE6F7)),
                      ),
                      child: ListTile(
                        leading: const Icon(Icons.logout, color: Colors.redAccent),
                        title: const Text('Logout'),
                        onTap: () => _logout(context),
                      ),
                    ),
                  ],
                ),
              ),
      ),
    );
  }
}
