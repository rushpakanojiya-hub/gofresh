// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'package:flutter/material.dart';
import '../theme.dart';
import 'dashboard_screen.dart';
import 'tasks_screen.dart';
import 'slots_screen.dart';
import 'profile_screen.dart';

class ShellScreen extends StatefulWidget {
  const ShellScreen({super.key});

  @override
  State<ShellScreen> createState() => _ShellScreenState();
}

class _ShellScreenState extends State<ShellScreen> {
  int _tab = 0;
  int _rev = 0;
  String _filter = 'pending';

  Future<void> _openSlots() async {
    await Navigator.push(
      context,
      MaterialPageRoute(builder: (_) => Scaffold(backgroundColor: AppColors.bg, body: SlotsScreen())),
    );
    if (mounted) setState(() => _rev++);
  }

  void _openTasks(String key) => setState(() {
        _filter = key;
        _tab = 1;
      });

  Widget _page() {
    switch (_tab) {
      case 1:
        return TasksScreen(initial: _filter);
      case 2:
        return ProfileScreen();
      default:
        return DashboardScreen(onOpenSlots: _openSlots, onOpenTasks: _openTasks);
    }
  }

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: _tab == 0,
      onPopInvokedWithResult: (didPop, result) {
        if (!didPop) setState(() => _tab = 0);
      },
      child: _scaffold(),
    );
  }

  Widget _scaffold() {
    return Scaffold(
      backgroundColor: AppColors.bg,
      body: KeyedSubtree(key: ValueKey('$_tab-$_filter-$_rev'), child: _page()),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _tab,
        onDestinationSelected: (i) => setState(() => _tab = i),
        destinations: [
          NavigationDestination(icon: Icon(Icons.home_outlined), selectedIcon: Icon(Icons.home), label: 'Home'),
          NavigationDestination(icon: Icon(Icons.assignment_outlined), selectedIcon: Icon(Icons.assignment), label: 'Tasks'),
          NavigationDestination(icon: Icon(Icons.person_outline), selectedIcon: Icon(Icons.person), label: 'Profile'),
        ],
      ),
    );
  }
}