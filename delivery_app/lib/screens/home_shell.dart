import 'package:flutter/material.dart';
import 'home_screen.dart';
import 'orders_screen.dart';
import 'earnings_screen.dart';
import 'profile_screen.dart';
import '../services/push_service.dart';
import 'qr_scan_screen.dart';
import 'package:geolocator/geolocator.dart';
import '../services/api_service.dart';

class HomeShell extends StatefulWidget {
  const HomeShell({super.key});

  @override
  State<HomeShell> createState() => _HomeShellState();
}

class _HomeShellState extends State<HomeShell> {
  int _index = 0;
  static const Color primaryPurple = Color(0xFF5B2A9E);

  void switchTab(int i) {
    if (i != _index) PushService.dataRefresh.value++;
    setState(() => _index = i);
  }

  Future<void> _openScanner() async {
    final code = await Navigator.of(context).push<String>(
      MaterialPageRoute(builder: (_) => const QrScanScreen()),
    );
    if (code == null || !mounted) return;
    final messenger = ScaffoldMessenger.of(context);
    try {
      double? lat;
      double? lng;
      try {
        final pos = await Geolocator.getCurrentPosition(
          locationSettings: const LocationSettings(timeLimit: Duration(seconds: 8)),
        );
        lat = pos.latitude;
        lng = pos.longitude;
      } catch (_) {}
      final res = await ApiService.checkInToStore(code, lat, lng);
      messenger.showSnackBar(
        SnackBar(content: Text('Checked in at ${res['warehouse_name'] ?? 'store'}')),
      );
    } catch (e) {
      messenger.showSnackBar(
        SnackBar(content: Text(e.toString().replaceFirst('Exception: ', ''))),
      );
    }
  }

  void _onPushTab() {
    final t = PushService.tabRequest.value;
    if (t == null) return;
    PushService.tabRequest.value = null;
    if (mounted) switchTab(t);
  }

  @override
  void initState() {
    super.initState();
    PushService.tabRequest.addListener(_onPushTab);
    PushService.start();
  }

  @override
  void dispose() {
    PushService.tabRequest.removeListener(_onPushTab);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final screens = [
      HomeScreen(onSwitchTab: switchTab),
      OrdersScreen(onSwitchTab: switchTab),
      const EarningsScreen(),
      const ProfileScreen(),
    ];

    return PopScope(       canPop: _index == 0,       onPopInvokedWithResult: (didPop, result) {         if (!didPop) {           switchTab(0);         }       },       child: Scaffold(
      floatingActionButtonLocation: FloatingActionButtonLocation.centerFloat,
      floatingActionButton: FloatingActionButton(
        onPressed: _openScanner,
        backgroundColor: primaryPurple,
        foregroundColor: Colors.white,
        child: const Icon(Icons.qr_code_scanner),
      ),
      body: IndexedStack(index: _index, children: screens),
      bottomNavigationBar: BottomNavigationBar(
        currentIndex: _index,
        onTap: switchTab,
        type: BottomNavigationBarType.fixed,
        selectedItemColor: primaryPurple,
        unselectedItemColor: Colors.black45,
        showUnselectedLabels: true,
        items: const [
          BottomNavigationBarItem(icon: Icon(Icons.home_outlined), label: 'Home'),
          BottomNavigationBarItem(icon: Icon(Icons.local_shipping_outlined), label: 'Orders'),
          BottomNavigationBarItem(icon: Icon(Icons.account_balance_wallet_outlined), label: 'Earnings'),
          BottomNavigationBarItem(icon: Icon(Icons.person_outline), label: 'Profile'),
        ],
      ),
    ),     );
  }
}
