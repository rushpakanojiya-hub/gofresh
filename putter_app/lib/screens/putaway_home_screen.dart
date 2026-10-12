// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../theme.dart';
import 'putaway_list_screen.dart';
import 'putaway_history_screen.dart';

class PutawayHomeScreen extends StatefulWidget {
  const PutawayHomeScreen({super.key});

  @override
  State<PutawayHomeScreen> createState() => _PutawayHomeScreenState();
}

class _PutawayHomeScreenState extends State<PutawayHomeScreen> {
  Map<String, dynamic> _d = {};
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final r = await ApiService.getDashboard();
      if (!mounted) return;
      setState(() {
        if (r['error'] != null) {
          if (_d.isEmpty) _error = r['error'].toString();
        } else {
          _d = r;
          _error = null;
        }
        _loading = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        if (_d.isEmpty) _error = 'Network error. Pull down to retry.';
        _loading = false;
      });
    }
  }

  Future<void> _openList(String tab) async {
    await Navigator.push(context, MaterialPageRoute(builder: (_) => PutawayListScreen(initial: tab)));
    if (mounted) _load();
  }

  Future<void> _openHistory() async {
    await Navigator.push(context, MaterialPageRoute(builder: (_) => PutawayHistoryScreen()));
  }

  Widget _count(String label, int v, Color color, IconData icon, String tab) {
    return Expanded(
      child: InkWell(
        borderRadius: BorderRadius.circular(16),
        onTap: () => _openList(tab),
        child: Container(
          padding: EdgeInsets.symmetric(vertical: 14),
          decoration: cardDecoration(),
          child: Column(
            children: [
              Icon(icon, color: color, size: 20),
              SizedBox(height: 6),
              Text('$v', style: TextStyle(fontSize: 22, fontWeight: FontWeight.w800, color: color)),
              SizedBox(height: 2),
              Text(label, style: TextStyle(fontSize: 11, color: AppColors.muted, fontWeight: FontWeight.w600)),
            ],
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final counts = asMap(_d['counts']);
    final shift = asMap(_d['shift']);
    final store = asMap(_d['store']);
    final active = shift['active'] == true;

    Widget body;
    if (_loading) {
      body = Center(child: CircularProgressIndicator());
    } else if (_error != null && _d.isEmpty) {
      body = ListView(
        physics: AlwaysScrollableScrollPhysics(),
        padding: EdgeInsets.fromLTRB(24, 100, 24, 24),
        children: [
          Center(child: Text(_error!, style: TextStyle(color: AppColors.muted))),
          SizedBox(height: 16),
          Center(child: OutlinedButton(onPressed: _load, child: Text('Retry'))),
        ],
      );
    } else {
      body = ListView(
        physics: AlwaysScrollableScrollPhysics(),
        padding: EdgeInsets.fromLTRB(16, 16, 16, 24),
        children: [
          Container(
            padding: EdgeInsets.all(14),
            decoration: BoxDecoration(
              color: active ? Color(0xFFE8F9EE) : Color(0xFFFFF4E5),
              borderRadius: BorderRadius.circular(14),
            ),
            child: Row(
              children: [
                Icon(active ? Icons.check_circle_outline : Icons.info_outline, color: active ? AppColors.green : AppColors.orange),
                SizedBox(width: 10),
                Expanded(
                  child: Text(
                    active ? 'You are on shift. You can start put-away.' : 'Start your shift from Home to begin a put-away.',
                    style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: active ? AppColors.green : AppColors.orange),
                  ),
                ),
              ],
            ),
          ),
          SizedBox(height: 16),
          Row(
            children: [
              _count('Pending', asInt(counts['pending']), AppColors.orange, Icons.hourglass_top_rounded, 'pending'),
              SizedBox(width: 10),
              _count('In progress', asInt(counts['in_progress']), AppColors.blue, Icons.sync_rounded, 'in_progress'),
              SizedBox(width: 10),
              _count('Done today', asInt(counts['completed_today']), AppColors.green, Icons.task_alt_rounded, 'pending'),
            ],
          ),
          SizedBox(height: 22),
          PrimaryButton(label: 'Start put-away', icon: Icons.inventory_2_outlined, color: AppColors.blue, onPressed: () => _openList('pending')),
          SizedBox(height: 12),
          SizedBox(
            width: double.infinity,
            child: OutlinedButton.icon(
              onPressed: _openHistory,
              icon: Icon(Icons.history),
              label: Text('Put-away history', style: TextStyle(fontWeight: FontWeight.w700)),
              style: OutlinedButton.styleFrom(
                foregroundColor: AppColors.blue,
                side: BorderSide(color: AppColors.blue),
                padding: EdgeInsets.symmetric(vertical: 14),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
              ),
            ),
          ),
        ],
      );
    }

    final storeName = (store['name'] ?? '').toString();
    return Scaffold(
      backgroundColor: AppColors.bg,
      body: Column(
        children: [
          GradientHeader(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(children: [
                  InkWell(onTap: () => Navigator.pop(context), child: Icon(Icons.arrow_back, color: Colors.white)),
                  SizedBox(width: 12),
                  Text('Put-Away', style: TextStyle(color: Colors.white, fontSize: 24, fontWeight: FontWeight.w800)),
                ]),
                SizedBox(height: 6),
                Text(storeName.isEmpty ? 'Store' : storeName, style: TextStyle(color: Colors.white70, fontSize: 13, fontWeight: FontWeight.w600)),
              ],
            ),
          ),
          Expanded(child: RefreshIndicator(onRefresh: _load, child: body)),
        ],
      ),
    );
  }
}