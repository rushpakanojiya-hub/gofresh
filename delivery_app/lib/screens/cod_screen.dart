import 'package:flutter/material.dart';
import '../services/api_service.dart';

class CODScreen extends StatefulWidget {
  const CODScreen({super.key});

  @override
  State<CODScreen> createState() => _CODScreenState();
}

class _CODScreenState extends State<CODScreen> {
  static const Color primaryPurple = Color(0xFF5B2A9E);
  static const Color pageBg = Color(0xFFF7F1FB);
  static const Color bannerBg = Color(0xFFEDE6F7);

  Map<String, dynamic>? _summary;
  List<dynamic> _settlements = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final summary = await ApiService.getCODSummary();
      final settlements = await ApiService.getCODSettlements();
      setState(() {
        _summary = summary;
        _settlements = settlements;
        _loading = false;
      });
    } catch (e) {
      setState(() {
        _error = 'Failed to load COD data';
        _loading = false;
      });
    }
  }

  String _fmt(dynamic value) {
    if (value == null) return '—';
    final n = (value is num) ? value : num.tryParse(value.toString());
    if (n == null) return '—';
    return '₹${n.toStringAsFixed(0)}';
  }

  String? _formatDate(dynamic raw) {
    if (raw == null) return null;
    try {
      final dt = DateTime.parse(raw.toString()).toLocal();
      const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
      return '${dt.day} ${months[dt.month - 1]}';
    } catch (_) {
      return null;
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: pageBg,
      body: SafeArea(
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : _error != null
                ? Center(child: Text(_error!))
                : RefreshIndicator(
                    onRefresh: _load,
                    child: ListView(
                      padding: const EdgeInsets.fromLTRB(16, 16, 16, 16),
                      children: [
                        const Text(
                          'Cash Management',
                          style: TextStyle(fontSize: 20, fontWeight: FontWeight.bold, color: Colors.black87),
                        ),
                        const SizedBox(height: 16),
                        Container(
                          width: double.infinity,
                          padding: const EdgeInsets.all(18),
                          decoration: BoxDecoration(color: bannerBg, borderRadius: BorderRadius.circular(20)),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Row(
                                children: [
                                  Expanded(child: _statTile("Today's COD", _fmt(_summary?['today_collected']))),
                                  Expanded(child: _statTile('Total Collected', _fmt(_summary?['total_collected']))),
                                ],
                              ),
                              const SizedBox(height: 10),
                              Row(
                                children: [
                                  Expanded(child: _statTile('Settled', _fmt(_summary?['total_deposited']))),
                                  Expanded(
                                    child: _statTile(
                                      'Pending',
                                      _fmt(_summary?['pending_settlement']),
                                      highlight: true,
                                    ),
                                  ),
                                ],
                              ),
                            ],
                          ),
                        ),
                        const SizedBox(height: 24),
                        const Text(
                          'SETTLEMENT HISTORY',
                          style: TextStyle(fontWeight: FontWeight.bold, fontSize: 13, color: Colors.black54, letterSpacing: 0.5),
                        ),
                        const SizedBox(height: 8),
                        if (_settlements.isEmpty)
                          Container(
                            width: double.infinity,
                            padding: const EdgeInsets.all(20),
                            decoration: BoxDecoration(color: Colors.white, borderRadius: BorderRadius.circular(14)),
                            child: const Center(
                              child: Text('No settlements yet', style: TextStyle(color: Colors.black45)),
                            ),
                          )
                        else
                          ...(_settlements.map((s) {
                            final isVerified = s['status'] == 'verified';
                            return Container(
                              margin: const EdgeInsets.only(bottom: 8),
                              padding: const EdgeInsets.all(14),
                              decoration: BoxDecoration(color: Colors.white, borderRadius: BorderRadius.circular(14)),
                              child: Row(
                                children: [
                                  Expanded(
                                    child: Column(
                                      crossAxisAlignment: CrossAxisAlignment.start,
                                      children: [
                                        Text(
                                          _formatDate(s['deposit_date']) ?? '-',
                                          style: const TextStyle(fontSize: 12, color: Colors.black45),
                                        ),
                                        const SizedBox(height: 2),
                                        Text(
                                          _fmt(s['amount']),
                                          style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 16),
                                        ),
                                      ],
                                    ),
                                  ),
                                  Container(
                                    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
                                    decoration: BoxDecoration(
                                      color: isVerified ? const Color(0xFFE1F5E6) : const Color(0xFFFFF3D6),
                                      borderRadius: BorderRadius.circular(10),
                                    ),
                                    child: Text(
                                      isVerified ? 'Settled ✓' : 'Pending',
                                      style: TextStyle(
                                        fontSize: 12,
                                        fontWeight: FontWeight.w600,
                                        color: isVerified ? Colors.green[800] : Colors.orange[800],
                                      ),
                                    ),
                                  ),
                                ],
                              ),
                            );
                          })),
                      ],
                    ),
                  ),
      ),
    );
  }

  Widget _statTile(String label, String value, {bool highlight = false}) {
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 4),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(color: Colors.white, borderRadius: BorderRadius.circular(14)),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label, style: const TextStyle(fontSize: 11, color: Colors.black45)),
          const SizedBox(height: 4),
          Text(
            value,
            style: TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.bold,
              color: highlight ? primaryPurple : Colors.black87,
            ),
          ),
        ],
      ),
    );
  }
}
