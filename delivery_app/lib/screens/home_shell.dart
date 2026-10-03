import 'package:flutter/material.dart';
import 'home_screen.dart';
import 'orders_screen.dart';
import 'earnings_screen.dart';
import 'profile_screen.dart';
import '../services/push_service.dart';

class HomeShell extends StatefulWidget {
  const HomeShell({super.key});

  @override
  State<HomeShell> createState() => _HomeShellState();
}

class _HomeShellState extends State<HomeShell> {
  int _index = 0;
  static const Color primaryPurple = Color(0xFF5B2A9E);

  void switchTab(int i) => setState(() => _index = i);

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
