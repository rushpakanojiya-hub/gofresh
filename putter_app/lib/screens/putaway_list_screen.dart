// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../theme.dart';
import 'putaway_common.dart';
import 'putaway_task_screen.dart';

class PutawayListScreen extends StatefulWidget {
  final String initial;
  const PutawayListScreen({super.key, this.initial = 'pending'});

  @override
  State<PutawayListScreen> createState() => _PutawayListScreenState();
}

class _PutawayListScreenState extends State<PutawayListScreen> {
  Map<String, dynamic> _d = {};
  bool _loading = true;
  String? _error;
  late String _tab;

  @override
  void initState() {
    super.initState();
    _tab = widget.initial == 'in_progress' ? 'in_progress' : 'pending';
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

  Future<void> _open(Map<String, dynamic> t) async {
    final shiftOn = asMap(_d['shift'])['active'] == true;
    await Navigator.push(
      context,
      MaterialPageRoute(builder: (_) => PutawayTaskScreen(taskId: asInt(t['id']), shiftActive: shiftOn)),
    );
    if (mounted) _load();
  }

  Widget _chip(String label, String key, int n) {
    final on = _tab == key;
    return Expanded(
      child: InkWell(
        borderRadius: BorderRadius.circular(12),
        onTap: () => setState(() => _tab = key),
        child: Container(
          padding: EdgeInsets.symmetric(vertical: 12),
          decoration: BoxDecoration(
            color: on ? AppColors.blue : Colors.white,
            borderRadius: BorderRadius.circular(12),
            border: Border.all(color: on ? AppColors.blue : AppColors.line),
          ),
          child: Center(
            child: Text('$label ($n)', style: TextStyle(fontWeight: FontWeight.w700, color: on ? Colors.white : AppColors.ink)),
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final counts = asMap(_d['counts']);
    final tasks = asList(_d[_tab]);
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
          Row(
            children: [
              _chip('Pending', 'pending', asInt(counts['pending'])),
              SizedBox(width: 10),
              _chip('In progress', 'in_progress', asInt(counts['in_progress'])),
            ],
          ),
          SizedBox(height: 16),
          if (tasks.isEmpty)
            Padding(
              padding: EdgeInsets.symmetric(vertical: 40),
              child: Center(
                child: Text(_tab == 'pending' ? 'No pending tasks assigned to you' : 'No tasks in progress',
                    style: TextStyle(color: AppColors.muted, fontWeight: FontWeight.w600)),
              ),
            ),
          for (final t in tasks) putawayCard(t, onTap: () => _open(t)),
        ],
      );
    }
    return Scaffold(
      backgroundColor: AppColors.bg,
      body: Column(
        children: [
          GradientHeader(
            child: Row(children: [
              InkWell(onTap: () => Navigator.pop(context), child: Icon(Icons.arrow_back, color: Colors.white)),
              SizedBox(width: 12),
              Text('Put-Away Task List', style: TextStyle(color: Colors.white, fontSize: 22, fontWeight: FontWeight.w800)),
            ]),
          ),
          Expanded(child: RefreshIndicator(onRefresh: _load, child: body)),
        ],
      ),
    );
  }
}