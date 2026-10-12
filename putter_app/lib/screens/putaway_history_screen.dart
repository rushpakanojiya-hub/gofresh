// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../theme.dart';
import 'putaway_common.dart';

class PutawayHistoryScreen extends StatefulWidget {
  const PutawayHistoryScreen({super.key});

  @override
  State<PutawayHistoryScreen> createState() => _PutawayHistoryScreenState();
}

class _PutawayHistoryScreenState extends State<PutawayHistoryScreen> {
  List<Map<String, dynamic>> _rows = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final r = await ApiService.putawayHistory();
      if (!mounted) return;
      setState(() {
        if (r['error'] != null) {
          _error = r['error'].toString();
        } else {
          _rows = asList(r['history']);
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

  @override
  Widget build(BuildContext context) {
    Widget body;
    if (_loading) {
      body = Center(child: CircularProgressIndicator());
    } else if (_error != null) {
      body = ListView(
        physics: AlwaysScrollableScrollPhysics(),
        padding: EdgeInsets.fromLTRB(24, 100, 24, 24),
        children: [Center(child: Text(_error!, style: TextStyle(color: AppColors.muted)))],
      );
    } else {
      body = ListView(
        physics: AlwaysScrollableScrollPhysics(),
        padding: EdgeInsets.fromLTRB(16, 16, 16, 24),
        children: [
          Text('Last 30 days - ${_rows.length} completed', style: TextStyle(color: AppColors.muted, fontWeight: FontWeight.w600)),
          SizedBox(height: 12),
          if (_rows.isEmpty)
            Padding(
              padding: EdgeInsets.symmetric(vertical: 40),
              child: Center(child: Text('No completed put-away yet', style: TextStyle(color: AppColors.muted, fontWeight: FontWeight.w600))),
            ),
          for (final t in _rows) putawayCard(t),
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
              Text('Put-Away History', style: TextStyle(color: Colors.white, fontSize: 22, fontWeight: FontWeight.w800)),
            ]),
          ),
          Expanded(child: RefreshIndicator(onRefresh: _load, child: body)),
        ],
      ),
    );
  }
}