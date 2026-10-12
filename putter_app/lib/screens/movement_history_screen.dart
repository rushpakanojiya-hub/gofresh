// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../theme.dart';
import 'putaway_common.dart';

class MovementHistoryScreen extends StatefulWidget {
  const MovementHistoryScreen({super.key});

  @override
  State<MovementHistoryScreen> createState() => _MovementHistoryScreenState();
}

class _MovementHistoryScreenState extends State<MovementHistoryScreen> {
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
      final r = await ApiService.movementHistory();
      if (!mounted) return;
      setState(() {
        if (r['error'] != null) {
          _error = r['error'].toString();
        } else {
          _rows = asList(r['movements']);
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

  Widget _tile(Map<String, dynamic> m) {
    final product = asMap(m['product']);
    final name = (product['name'] ?? 'Product #${m['product_id'] ?? ''}').toString();
    return Padding(
      padding: EdgeInsets.only(bottom: 10),
      child: Container(
        padding: EdgeInsets.all(14),
        decoration: cardDecoration(),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(name, maxLines: 2, overflow: TextOverflow.ellipsis, style: TextStyle(fontSize: 14, fontWeight: FontWeight.w800, color: AppColors.ink)),
            SizedBox(height: 6),
            Row(
              children: [
                Icon(Icons.alt_route_rounded, size: 14, color: AppColors.blue),
                SizedBox(width: 6),
                Expanded(child: Text((m['notes'] ?? '').toString(), style: TextStyle(fontSize: 12, fontWeight: FontWeight.w700, color: AppColors.blue))),
              ],
            ),
            SizedBox(height: 4),
            Text('Stock ${asInt(m['new_qty'])}  |  ${fmtDateTime(m['created_at'])}', style: TextStyle(fontSize: 11, color: AppColors.muted)),
          ],
        ),
      ),
    );
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
          Text('Last 30 days - ${_rows.length} moves', style: TextStyle(color: AppColors.muted, fontWeight: FontWeight.w600)),
          SizedBox(height: 12),
          if (_rows.isEmpty)
            Padding(
              padding: EdgeInsets.symmetric(vertical: 40),
              child: Center(child: Text('No movements yet', style: TextStyle(color: AppColors.muted, fontWeight: FontWeight.w600))),
            ),
          for (final m in _rows) _tile(m),
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
              Text('Movement History', style: TextStyle(color: Colors.white, fontSize: 22, fontWeight: FontWeight.w800)),
            ]),
          ),
          Expanded(child: RefreshIndicator(onRefresh: _load, child: body)),
        ],
      ),
    );
  }
}