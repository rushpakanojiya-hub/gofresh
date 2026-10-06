import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../services/push_service.dart';

const Color _pocketBg = Color(0xFFF7F1FB);
const Color _pocketPurple = Color(0xFF5B2A9E);

class PocketScreen extends StatefulWidget {
  final void Function(int)? onSwitchTab;
  const PocketScreen({super.key, this.onSwitchTab});

  @override
  State<PocketScreen> createState() => _PocketScreenState();
}

class _PocketScreenState extends State<PocketScreen> {
  static const List<String> _months = [
    'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
    'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec',
  ];

  List<dynamic> _entries = [];
  bool _loading = true;

  // TODO: pocket balance and tips need a backend API. Shown as 0 for now.
  final num _pocketBalance = 0;
  final num _cashLimit = 1500;
  final num _tipsBalance = 0;

  @override
  void initState() {
    super.initState();
    PushService.dataRefresh.addListener(_onDataRefresh);
    _load();
  }

  @override
  void dispose() {
    PushService.dataRefresh.removeListener(_onDataRefresh);
    super.dispose();
  }

  void _onDataRefresh() {
    Future.microtask(() {
      if (mounted) _load(silent: true);
    });
  }

  Future<void> _load({bool silent = false}) async {
    if (!silent) setState(() => _loading = true);
    try {
      final data = await ApiService.getEarnings();
      if (!mounted) return;
      setState(() {
        _entries = (data['entries'] as List?) ?? [];
        _loading = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() => _loading = false);
    }
  }

  static DateTime _mondayOf(DateTime d) => DateTime(d.year, d.month, d.day - (d.weekday - 1));
  static DateTime _addDays(DateTime d, int n) => DateTime(d.year, d.month, d.day + n);
  static String _fmt(DateTime d) => '${d.day.toString().padLeft(2, '0')} ${_months[d.month - 1]}';
  static String _rs(num v) => v == v.roundToDouble() ? v.toInt().toString() : v.toStringAsFixed(2);

  num _weekAmount(DateTime start) {
    final end = _addDays(start, 7);
    num total = 0;
    for (final e in _entries) {
      if (e is! Map) continue;
      final t = DateTime.tryParse('${e['delivered_at'] ?? ''}')?.toLocal();
      if (t == null || t.isBefore(start) || !t.isBefore(end)) continue;
      total += (e['amount'] is num) ? e['amount'] as num : 0;
    }
    return total;
  }

  void _soon(String what) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('$what is coming soon')));
  }

  Future<void> _showDepositOptions() async {
    Widget option(BuildContext ctx, IconData icon, String label) {
      return InkWell(
        onTap: () {
          Navigator.pop(ctx);
          _soon(label);
        },
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
          decoration: const BoxDecoration(
            border: Border(top: BorderSide(color: Color(0xFFEDEDED))),
          ),
          child: Row(
            children: [
              CircleAvatar(
                radius: 20,
                backgroundColor: const Color(0xFFF1F1F1),
                child: Icon(icon, color: Colors.black87, size: 20),
              ),
              const SizedBox(width: 14),
              Expanded(child: Text(label, style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 15))),
              const Icon(Icons.chevron_right, color: Colors.black54),
            ],
          ),
        ),
      );
    }

    await showModalBottomSheet(
      context: context,
      backgroundColor: Colors.white,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(16))),
      builder: (ctx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 14, 8, 10),
              child: Row(
                children: [
                  const Expanded(
                    child: Text('Deposit options', style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16)),
                  ),
                  IconButton(icon: const Icon(Icons.close), onPressed: () => Navigator.pop(ctx)),
                ],
              ),
            ),
            option(ctx, Icons.account_balance_wallet_outlined, 'UPI'),
            option(ctx, Icons.credit_card, 'Debit card/Net banking'),
            option(ctx, Icons.storefront_outlined, 'Cash deposit at store'),
            option(ctx, Icons.store_mall_directory_outlined, 'Airtel store'),
          ],
        ),
      ),
    );
  }

  Widget _label(String t) => Padding(
        padding: const EdgeInsets.fromLTRB(4, 18, 4, 8),
        child: Center(
          child: Text(t, style: const TextStyle(fontSize: 11, letterSpacing: 1, color: Colors.black45, fontWeight: FontWeight.w600)),
        ),
      );

  Widget _row(String title, String value, {VoidCallback? onTap}) {
    return InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 12),
        child: Row(
          children: [
            Expanded(child: Text(title, style: const TextStyle(fontSize: 14))),
            Text(value, style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w600)),
            const SizedBox(width: 4),
            const Icon(Icons.chevron_right, size: 20, color: Colors.black54),
          ],
        ),
      ),
    );
  }

  Widget _tile({required Widget top, required String title, String? sub}) {
    return Expanded(
      child: Container(
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(color: Colors.white, borderRadius: BorderRadius.circular(12)),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            top,
            const SizedBox(height: 10),
            Text(title, style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 13)),
            if (sub != null)
              Padding(
                padding: const EdgeInsets.only(top: 2),
                child: Text(sub, style: const TextStyle(fontSize: 11, color: Colors.black54)),
              ),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final thisWeek = _mondayOf(DateTime.now());
    final lastWeek = _addDays(thisWeek, -7);
    final weekAmount = _weekAmount(thisWeek);
    final lastWeekAmount = _weekAmount(lastWeek);

    return Scaffold(
      backgroundColor: _pocketBg,
      body: SafeArea(
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : RefreshIndicator(
                onRefresh: _load,
                child: ListView(
                  padding: const EdgeInsets.fromLTRB(16, 12, 16, 110),
                  children: [
                    InkWell(
                      onTap: () => widget.onSwitchTab?.call(2),
                      borderRadius: BorderRadius.circular(14),
                      child: Container(
                        padding: const EdgeInsets.symmetric(vertical: 18),
                        decoration: BoxDecoration(color: Colors.white, borderRadius: BorderRadius.circular(14)),
                        child: Column(
                          children: [
                            Text(
                              'Earnings: ${_fmt(thisWeek)} - ${_fmt(_addDays(thisWeek, 6))} \u2192',
                              style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w600),
                            ),
                            const SizedBox(height: 10),
                            Text('\u20B9${_rs(weekAmount)}',
                                style: const TextStyle(fontSize: 28, fontWeight: FontWeight.bold)),
                          ],
                        ),
                      ),
                    ),
                    _label('POCKET'),
                    Container(
                      padding: const EdgeInsets.fromLTRB(14, 4, 14, 14),
                      decoration: BoxDecoration(color: Colors.white, borderRadius: BorderRadius.circular(14)),
                      child: Column(
                        children: [
                          _row('Pocket balance', '\u20B9${_rs(_pocketBalance)}'),
                          const Divider(height: 1),
                          _row('Available cash limit', '\u20B9${_rs(_cashLimit)}'),
                          const SizedBox(height: 8),
                          Row(
                            children: [
                              Expanded(
                                child: OutlinedButton(
                                  onPressed: _showDepositOptions,
                                  style: OutlinedButton.styleFrom(
                                    foregroundColor: Colors.black87,
                                    side: const BorderSide(color: Colors.black87),
                                    padding: const EdgeInsets.symmetric(vertical: 14),
                                    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
                                  ),
                                  child: const Text('Deposit', style: TextStyle(fontWeight: FontWeight.w600)),
                                ),
                              ),
                              const SizedBox(width: 12),
                              Expanded(
                                child: ElevatedButton(
                                  onPressed: null,
                                  style: ElevatedButton.styleFrom(
                                    disabledBackgroundColor: const Color(0xFFE3E3E3),
                                    disabledForegroundColor: Colors.black38,
                                    padding: const EdgeInsets.symmetric(vertical: 14),
                                    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
                                  ),
                                  child: const Text('Withdraw', style: TextStyle(fontWeight: FontWeight.w600)),
                                ),
                              ),
                            ],
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(height: 10),
                    Container(
                      padding: const EdgeInsets.symmetric(horizontal: 14),
                      decoration: BoxDecoration(color: Colors.white, borderRadius: BorderRadius.circular(14)),
                      child: _row('Customer tips balance', '\u20B9${_rs(_tipsBalance)}', onTap: () => _soon('Customer tips')),
                    ),
                    _label('MORE SERVICES'),
                    Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        _tile(
                          top: Row(
                            children: [
                              Text('\u20B9${_rs(lastWeekAmount)}',
                                  style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
                              const SizedBox(width: 6),
                              const Icon(Icons.check_circle, color: Colors.green, size: 16),
                            ],
                          ),
                          title: 'Payout',
                          sub: '${_fmt(lastWeek)} - ${_fmt(_addDays(lastWeek, 6))}',
                        ),
                        const SizedBox(width: 12),
                        _tile(
                          top: const Icon(Icons.receipt_long_outlined, size: 24, color: _pocketPurple),
                          title: 'Customer tips statement',
                        ),
                      ],
                    ),
                  ],
                ),
              ),
      ),
    );
  }
}