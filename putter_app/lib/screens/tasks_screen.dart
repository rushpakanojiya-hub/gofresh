// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'dart:async';
import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../theme.dart';
import 'login_screen.dart';

String _binLabel(dynamic bin, dynamic id) {
  if (bin is Map) {
    for (final k in ['code', 'bin_code', 'name', 'label']) {
      final v = bin[k];
      if (v != null && '$v'.trim().isNotEmpty) return '$v';
    }
  }
  return id == null ? '-' : 'Bin #$id';
}

class TasksScreen extends StatefulWidget {
  final String initial;
  const TasksScreen({super.key, this.initial = 'pending'});

  @override
  State<TasksScreen> createState() => _TasksScreenState();
}

class _TasksScreenState extends State<TasksScreen> {
  Map<String, dynamic> _d = {};
  bool _loading = true;
  String? _error;
  late String _tab;
  Timer? _poll;

  @override
  void initState() {
    super.initState();
    _tab = widget.initial;
    _load();
    _poll = Timer.periodic(const Duration(seconds: 30), (_) => _load(silent: true));
  }

  @override
  void dispose() {
    _poll?.cancel();
    super.dispose();
  }

  Future<void> _load({bool silent = false}) async {
    try {
      final r = await ApiService.getDashboard();
      if (!mounted) return;
      final code = r['_http'];
      if (code == 401 || code == 403) {
        await ApiService.logout();
        if (!mounted) return;
        Navigator.pushAndRemoveUntil(context, MaterialPageRoute(builder: (_) => const LoginScreen()), (route) => false);
        return;
      }
      setState(() {
        if (r['error'] != null) {
          if (!silent || _d.isEmpty) _error = r['error'].toString();
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

  Widget _stat(String label, int value, String key, Color color, IconData icon) {
    final on = _tab == key;
    return Expanded(
      child: InkWell(
        borderRadius: BorderRadius.circular(16),
        onTap: () => setState(() => _tab = key),
        child: Container(
          padding: EdgeInsets.symmetric(vertical: 14, horizontal: 8),
          decoration: BoxDecoration(
            color: on ? color.withAlpha(22) : Colors.white,
            borderRadius: BorderRadius.circular(16),
            border: Border.all(color: on ? color : AppColors.line, width: on ? 1.6 : 1),
          ),
          child: Column(
            children: [
              Icon(icon, size: 20, color: color),
              SizedBox(height: 6),
              Text('$value', style: TextStyle(fontSize: 22, fontWeight: FontWeight.w800, color: color)),
              SizedBox(height: 2),
              Text(label, style: TextStyle(fontSize: 11, color: AppColors.muted, fontWeight: FontWeight.w600)),
            ],
          ),
        ),
      ),
    );
  }

  Widget _taskTile(Map<String, dynamic> t) {
    final product = asMap(t['product']);
    final name = (product['name'] ?? 'Product #${t['product_id'] ?? ''}').toString();
    String img = (product['image_url'] ?? '').toString();
    if (img.isNotEmpty && !img.startsWith('http')) {
      img = '$kOrigin${img.startsWith('/') ? '' : '/'}$img';
    }
    final status = (t['status'] ?? '').toString();
    final done = status == 'completed';
    final bin = done ? _binLabel(t['actual_bin'], t['actual_bin_id']) : _binLabel(t['suggested_bin'], t['suggested_bin_id']);
    final raw = done ? t['completed_at'] : (t['started_at'] ?? t['assigned_at'] ?? t['created_at']);
    final when = DateTime.tryParse('$raw')?.toLocal();
    final color = done ? AppColors.green : (status == 'in_progress' ? AppColors.blue : AppColors.orange);
    final label = done ? 'Completed' : (status == 'in_progress' ? 'In progress' : 'Pending');

    Widget box(IconData ic) => Container(width: 54, height: 54, color: Color(0xFFF2F4F7), child: Icon(ic, size: 22, color: Colors.grey));

    return Container(
      margin: EdgeInsets.only(bottom: 10),
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
                Text(name, maxLines: 2, overflow: TextOverflow.ellipsis,
                    style: TextStyle(fontSize: 14, fontWeight: FontWeight.w800, color: AppColors.ink)),
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
                if (when != null)
                  Padding(padding: EdgeInsets.only(top: 4), child: Text(clock12(when), style: TextStyle(fontSize: 11, color: AppColors.muted))),
                if (t['wrong_location'] == true)
                  Padding(padding: EdgeInsets.only(top: 2), child: Text('Different bin used', style: TextStyle(fontSize: 11, color: AppColors.orange))),
              ],
            ),
          ),
          SizedBox(width: 8),
          StatusPill(label, color),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final counts = asMap(_d['counts']);
    final tasks = asList(_d[_tab]);
    final title = _tab == 'pending' ? 'Pending tasks' : (_tab == 'in_progress' ? 'In progress' : 'Completed today');
    final emptyText = _tab == 'pending'
        ? 'No pending tasks assigned to you'
        : (_tab == 'in_progress' ? 'No tasks in progress' : 'No tasks completed today');

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
          Center(child: OutlinedButton(onPressed: () => _load(), child: Text('Retry'))),
        ],
      );
    } else {
      body = ListView(
        physics: AlwaysScrollableScrollPhysics(),
        padding: EdgeInsets.fromLTRB(16, 16, 16, 24),
        children: [
          Row(
            children: [
              _stat('Pending', asInt(counts['pending']), 'pending', AppColors.orange, Icons.hourglass_top_rounded),
              SizedBox(width: 10),
              _stat('In progress', asInt(counts['in_progress']), 'in_progress', AppColors.blue, Icons.sync_rounded),
              SizedBox(width: 10),
              _stat('Completed', asInt(counts['completed_today']), 'completed', AppColors.green, Icons.task_alt_rounded),
            ],
          ),
          SizedBox(height: 22),
          Row(
            children: [
              Text(title, style: TextStyle(fontSize: 16, fontWeight: FontWeight.w800, color: AppColors.ink)),
              Spacer(),
              Text('${tasks.length}', style: TextStyle(fontSize: 13, color: AppColors.muted, fontWeight: FontWeight.w700)),
            ],
          ),
          SizedBox(height: 12),
          if (tasks.isEmpty)
            Padding(
              padding: EdgeInsets.symmetric(vertical: 28),
              child: Center(child: Text(emptyText, style: TextStyle(color: AppColors.muted, fontWeight: FontWeight.w600))),
            ),
          for (final t in tasks) _taskTile(t),
        ],
      );
    }

    return Column(
      children: [
        GradientHeader(
          child: Align(
            alignment: Alignment.centerLeft,
            child: Text('Putaway tasks', style: TextStyle(color: Colors.white, fontSize: 24, fontWeight: FontWeight.w800)),
          ),
        ),
        Expanded(child: RefreshIndicator(onRefresh: () => _load(), child: body)),
      ],
    );
  }
}