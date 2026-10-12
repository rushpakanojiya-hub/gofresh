// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../theme.dart';
import 'login_screen.dart';

class ProfileScreen extends StatefulWidget {
  const ProfileScreen({super.key});

  @override
  State<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends State<ProfileScreen> {
  Map<String, dynamic> _d = {};
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _logout() async {
    await ApiService.logout();
    if (!mounted) return;
    Navigator.pushAndRemoveUntil(
      context,
      MaterialPageRoute(builder: (_) => const LoginScreen()),
      (route) => false,
    );
  }

  Future<void> _confirmLogout() async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('Logout?'),
        content: Text('You will need to log in again with an OTP.'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: Text('Cancel')),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: Text('Logout', style: TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );
    if (ok == true) await _logout();
  }

  Future<void> _load() async {
    try {
      final r = await ApiService.getDashboard();
      if (!mounted) return;
      final code = r['_http'];
      if (code == 401 || code == 403) {
        await _logout();
        return;
      }
      setState(() {
        if (r['error'] != null) {
          _error = r['error'].toString();
        } else {
          _d = r;
          _error = null;
        }
        _loading = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _error = 'Network error. Pull down to retry.';
        _loading = false;
      });
    }
  }

  Widget _row(IconData icon, String label, String value) {
    return Padding(
      padding: EdgeInsets.symmetric(vertical: 10),
      child: Row(
        children: [
          Container(
            width: 38,
            height: 38,
            decoration: BoxDecoration(color: Color(0xFFEFF5FF), borderRadius: BorderRadius.circular(12)),
            child: Icon(icon, size: 19, color: AppColors.blue),
          ),
          SizedBox(width: 12),
          Expanded(child: Text(label, style: TextStyle(fontSize: 13, color: AppColors.muted))),
          Text(value.isEmpty ? '-' : value,
              style: TextStyle(fontSize: 14, fontWeight: FontWeight.w700, color: AppColors.ink)),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final staff = asMap(_d['staff']);
    final store = asMap(_d['store']);
    final shift = asMap(_d['shift']);
    final name = (staff['name'] ?? '').toString();
    final initial = name.trim().isEmpty ? '?' : name.trim()[0].toUpperCase();

    Widget body;
    if (_loading) {
      body = Center(child: CircularProgressIndicator());
    } else if (_error != null && _d.isEmpty) {
      body = ListView(
        physics: AlwaysScrollableScrollPhysics(),
        padding: EdgeInsets.fromLTRB(24, 140, 24, 24),
        children: [Center(child: Text(_error!, style: TextStyle(color: AppColors.muted)))],
      );
    } else {
      body = ListView(
        physics: AlwaysScrollableScrollPhysics(),
        padding: EdgeInsets.zero,
        children: [
          GradientHeader(
            child: Column(
              children: [
                CircleAvatar(
                  radius: 40,
                  backgroundColor: Colors.white,
                  child: Text(initial, style: TextStyle(fontSize: 34, fontWeight: FontWeight.w800, color: AppColors.blue)),
                ),
                SizedBox(height: 12),
                Text(name, style: TextStyle(color: Colors.white, fontSize: 22, fontWeight: FontWeight.w800)),
                SizedBox(height: 4),
                Text('+91 ${staff['phone'] ?? ''}', style: TextStyle(color: Colors.white70, fontSize: 13)),
                SizedBox(height: 12),
                Container(
                  padding: EdgeInsets.symmetric(horizontal: 14, vertical: 6),
                  decoration: BoxDecoration(color: Colors.white.withAlpha(36), borderRadius: BorderRadius.circular(20)),
                  child: Text('PUTTER',
                      style: TextStyle(color: Colors.white, fontSize: 12, fontWeight: FontWeight.w800, letterSpacing: 1)),
                ),
              ],
            ),
          ),
          Padding(
            padding: EdgeInsets.fromLTRB(16, 16, 16, 24),
            child: Column(
              children: [
                Container(
                  padding: EdgeInsets.symmetric(horizontal: 16, vertical: 6),
                  decoration: cardDecoration(radius: 20),
                  child: Column(
                    children: [
                      _row(Icons.badge_outlined, 'Employee ID', (staff['employee_id'] ?? '').toString()),
                      Divider(height: 1, color: AppColors.line),
                      _row(Icons.storefront_outlined, 'Store', (store['name'] ?? '').toString()),
                      Divider(height: 1, color: AppColors.line),
                      _row(Icons.location_city_outlined, 'City', (store['city'] ?? '').toString()),
                      Divider(height: 1, color: AppColors.line),
                      _row(Icons.timer_outlined, 'Shift time today', hm(asInt(shift['today_seconds']))),
                    ],
                  ),
                ),
                SizedBox(height: 20),
                SizedBox(
                  width: double.infinity,
                  child: OutlinedButton.icon(
                    onPressed: _confirmLogout,
                    icon: Icon(Icons.logout),
                    label: Text('Logout', style: TextStyle(fontWeight: FontWeight.w700)),
                    style: OutlinedButton.styleFrom(
                      foregroundColor: AppColors.red,
                      side: BorderSide(color: AppColors.red),
                      padding: EdgeInsets.symmetric(vertical: 14),
                      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
      );
    }
    return RefreshIndicator(onRefresh: _load, child: body);
  }
}