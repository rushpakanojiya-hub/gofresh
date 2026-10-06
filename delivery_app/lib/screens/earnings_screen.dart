import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../services/push_service.dart';

class EarningsScreen extends StatefulWidget {
  final void Function(int)? onSwitchTab;
  const EarningsScreen({super.key, this.onSwitchTab});

  @override
  State<EarningsScreen> createState() => _EarningsScreenState();
}

class _EarningsScreenState extends State<EarningsScreen> {
  Map<String, dynamic>? _data;
  bool _loading = true;
  String? _error;

  static const Color primaryPurple = Color(0xFF5B2A9E);
  static const Color pageBg = Color(0xFFF7F1FB);
  static const Color todayCardBg = Color(0xFFDCD3EC);
  static const Color totalCardBg = Color(0xFFCFE0D4);
  static const Color historyCardBg = Color(0xFFF3EDFA);
  static const Color motivationBg = Color(0xFFEDE6F7);

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
    setState(() {
      if (!silent) _loading = true;
      _error = null;
    });
    try {
      final data = await ApiService.getEarnings();
      setState(() {
        _data = data;
        _loading = false;
      });
    } catch (e) {
      setState(() {
        _error = 'Failed to load earnings: $e';
        _loading = false;
      });
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
                      padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
                      children: [
                        Row(
                          mainAxisAlignment: MainAxisAlignment.spaceBetween,
                          children: [
                            IconButton(
                              icon: const Icon(Icons.arrow_back, color: Colors.black87),
                              onPressed: () => widget.onSwitchTab?.call(0),
                            ),
                            const Text(
                              'Earnings',
                              style: TextStyle(fontSize: 20, fontWeight: FontWeight.bold, color: Colors.black87),
                            ),
                            const SizedBox(width: 48),
                          ],
                        ),
                        const SizedBox(height: 8),
                        Row(
                          children: [
                            Expanded(
                              child: _SummaryCard(
                                title: "Today's Earnings",
                                amount: _data?['today_earnings'] ?? 0,
                                subtitle: '${_data?['today_deliveries'] ?? 0} deliveries',
                                bgColor: todayCardBg,
                                textColor: primaryPurple,
                              ),
                            ),
                            const SizedBox(width: 12),
                            Expanded(
                              child: _SummaryCard(
                                title: 'Total Earnings',
                                amount: _data?['total_earnings'] ?? 0,
                                subtitle: '${_data?['total_deliveries'] ?? 0} deliveries',
                                bgColor: totalCardBg,
                                textColor: const Color(0xFF1B7A3D),
                              ),
                            ),
                          ],
                        ),
                        const SizedBox(height: 12),
                        _WeeklyPayoutCard(entries: (_data?['entries'] as List?) ?? const []),
                        const SizedBox(height: 12),
                        Container(
                          padding: const EdgeInsets.all(20),
                          decoration: BoxDecoration(
                            color: motivationBg,
                            borderRadius: BorderRadius.circular(18),
                          ),
                          child: Row(
                            children: [
                              Expanded(
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    const Text(
                                      'Keep going!',
                                      style: TextStyle(fontSize: 17, fontWeight: FontWeight.bold, color: primaryPurple),
                                    ),
                                    const SizedBox(height: 6),
                                    const Text(
                                      "You're doing great today.",
                                      style: TextStyle(fontSize: 13, color: Colors.black54),
                                    ),
                                    const SizedBox(height: 14),
                                    ElevatedButton(
                                      onPressed: _load,
                                      style: ElevatedButton.styleFrom(
                                        backgroundColor: primaryPurple,
                                        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(24)),
                                        padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 12),
                                      ),
                                      child: const Text('Refresh', style: TextStyle(color: Colors.white, fontWeight: FontWeight.w600)),
                                    ),
                                  ],
                                ),
                              ),
                              Container(
                                width: 64,
                                height: 64,
                                decoration: const BoxDecoration(
                                  color: primaryPurple,
                                  shape: BoxShape.circle,
                                ),
                                child: const Icon(Icons.currency_rupee, color: Colors.white, size: 30),
                              ),
                            ],
                          ),
                        ),
                        const SizedBox(height: 24),
                        const Text(
                          'Delivery History',
                          style: TextStyle(fontWeight: FontWeight.bold, fontSize: 18, color: Colors.black87),
                        ),
                        const SizedBox(height: 12),
                        if ((_data?['entries'] as List?)?.isEmpty ?? true)
                          const Padding(
                            padding: EdgeInsets.symmetric(vertical: 24),
                            child: Center(child: Text('No deliveries yet')),
                          )
                        else
                          ...List.generate((_data!['entries'] as List).length, (i) {
                            final entry = _data!['entries'][i];
                            return Container(
                              margin: const EdgeInsets.only(bottom: 12),
                              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
                              decoration: BoxDecoration(
                                color: Colors.white,
                                borderRadius: BorderRadius.circular(16),
                                border: Border.all(color: historyCardBg, width: 1.2),
                              ),
                              child: Row(
                                children: [
                                  Container(
                                    width: 32,
                                    height: 32,
                                    decoration: const BoxDecoration(
                                      color: Color(0xFF22C55E),
                                      shape: BoxShape.circle,
                                    ),
                                    child: Icon(entry['type'] == 'return' ? Icons.assignment_return_outlined : Icons.check, color: Colors.white, size: 18),
                                  ),
                                  const SizedBox(width: 12),
                                  Expanded(
                                    child: Column(
                                      crossAxisAlignment: CrossAxisAlignment.start,
                                      children: [
                                        Text(
                                          '${entry['type'] == 'return' ? 'Return' : 'Order'} #${entry['order_id']}',
                                          style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 15, color: Colors.black87),
                                        ),
                                        const SizedBox(height: 2),
                                        Text(
                                          entry['delivered_at'].toString().substring(0, 16).replaceFirst('T', ' '),
                                          style: const TextStyle(fontSize: 12, color: Colors.black54),
                                        ),
                                      ],
                                    ),
                                  ),
                                  Text(
                                    '+₹${entry['amount']}',
                                    style: const TextStyle(fontWeight: FontWeight.bold, color: Color(0xFF22C55E), fontSize: 14),
                                  ),
                                ],
                              ),
                            );
                          }),
                        
                      ],
                    ),
                  ),
      ),
    );
  }
}

class _SummaryCard extends StatelessWidget {
  final String title;
  final num amount;
  final String subtitle;
  final Color bgColor;
  final Color textColor;

  const _SummaryCard({
    required this.title,
    required this.amount,
    required this.subtitle,
    required this.bgColor,
    required this.textColor,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: bgColor,
        borderRadius: BorderRadius.circular(16),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(title, style: TextStyle(color: textColor, fontWeight: FontWeight.w600, fontSize: 13)),
          const SizedBox(height: 10),
          Text('₹$amount', style: TextStyle(color: textColor, fontSize: 28, fontWeight: FontWeight.bold)),
          const SizedBox(height: 6),
          Text(subtitle, style: const TextStyle(color: Colors.black54, fontSize: 12)),
        ],
      ),
    );
  }
}

class _WeeklyPayoutCard extends StatefulWidget {
  final List<dynamic> entries;
  const _WeeklyPayoutCard({required this.entries});

  @override
  State<_WeeklyPayoutCard> createState() => _WeeklyPayoutCardState();
}

class _WeeklyPayoutCardState extends State<_WeeklyPayoutCard> {
  static const Color _brown = Color(0xFF9A5B00);
  static const List<String> _months = [
    'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
    'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec',
  ];

  late DateTime _weekStart = _mondayOf(DateTime.now());

  static DateTime _mondayOf(DateTime d) => DateTime(d.year, d.month, d.day - (d.weekday - 1));
  static DateTime _addDays(DateTime d, int n) => DateTime(d.year, d.month, d.day + n);
  static String _fmt(DateTime d) => '${d.day.toString().padLeft(2, '0')} ${_months[d.month - 1]}';
  static String _rs(num v) => v == v.roundToDouble() ? v.toInt().toString() : v.toStringAsFixed(2);

  String _range(DateTime start) {
    final end = _addDays(start, 6);
    final year = end.year != start.year ? ' ${end.year}' : '';
    return '${_fmt(start)}$year - ${_fmt(end)} ${end.year}';
  }

  List<num> _sum(DateTime start) {
    final end = _addDays(start, 7);
    num amount = 0;
    num count = 0;
    for (final e in widget.entries) {
      if (e is! Map) continue;
      final t = DateTime.tryParse('${e['delivered_at'] ?? ''}')?.toLocal();
      if (t == null || t.isBefore(start) || !t.isBefore(end)) continue;
      amount += (e['amount'] is num) ? e['amount'] as num : 0;
      count++;
    }
    return [amount, count];
  }

  Future<void> _pickDate() async {
    final now = DateTime.now();
    final picked = await showDatePicker(
      context: context,
      initialDate: _weekStart,
      firstDate: DateTime(now.year - 2),
      lastDate: now,
      helpText: 'Select a day to see that week',
    );
    if (picked != null && mounted) {
      setState(() => _weekStart = _mondayOf(picked));
    }
  }

  @override
  Widget build(BuildContext context) {
    final now = DateTime.now();
    final thisWeek = _mondayOf(now);
    final isThisWeek = _weekStart == thisWeek;
    final canNext = !_addDays(_weekStart, 7).isAfter(now);
    final sel = _sum(_weekStart);

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.fromLTRB(16, 12, 8, 12),
      decoration: BoxDecoration(
        color: const Color(0xFFFFF1D6),
        borderRadius: BorderRadius.circular(16),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Expanded(
                child: Text(
                  'Weekly Payout',
                  style: TextStyle(color: _brown, fontWeight: FontWeight.w600, fontSize: 13),
                ),
              ),
              IconButton(
                tooltip: 'Pick a date',
                icon: const Icon(Icons.calendar_month_outlined, color: _brown),
                onPressed: _pickDate,
              ),
            ],
          ),
          Row(
            children: [
              IconButton(
                visualDensity: VisualDensity.compact,
                icon: const Icon(Icons.chevron_left, color: _brown),
                onPressed: () => setState(() => _weekStart = _addDays(_weekStart, -7)),
              ),
              Expanded(
                child: Column(
                  children: [
                    Text(
                      _range(_weekStart),
                      style: const TextStyle(color: Colors.black87, fontWeight: FontWeight.w600, fontSize: 13),
                    ),
                    if (isThisWeek)
                      const Text('This week', style: TextStyle(color: Colors.black54, fontSize: 11)),
                  ],
                ),
              ),
              IconButton(
                visualDensity: VisualDensity.compact,
                icon: Icon(Icons.chevron_right, color: canNext ? _brown : Colors.black26),
                onPressed: canNext ? () => setState(() => _weekStart = _addDays(_weekStart, 7)) : null,
              ),
            ],
          ),
          Padding(
            padding: const EdgeInsets.only(left: 0, top: 4, right: 8),
            child: Center(
              child: Column(
                children: [
                  Text(
                    '\u20B9${_rs(sel[0])}',
                    style: const TextStyle(color: _brown, fontSize: 28, fontWeight: FontWeight.bold),
                  ),
                  const SizedBox(height: 4),
                  Text('${sel[1].toInt()} jobs', style: const TextStyle(color: Colors.black54, fontSize: 12)),
                ],
              ),
            ),
          ),
          const Divider(height: 20),
          const Padding(
            padding: EdgeInsets.only(bottom: 4),
            child: Text('Recent weeks', style: TextStyle(color: _brown, fontWeight: FontWeight.w600, fontSize: 12)),
          ),
          for (var i = 0; i < 4; i++)
            Builder(builder: (_) {
              final ws = _addDays(thisWeek, -7 * i);
              final s = _sum(ws);
              final selected = ws == _weekStart;
              return InkWell(
                borderRadius: BorderRadius.circular(8),
                onTap: () => setState(() => _weekStart = ws),
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(0, 6, 8, 6),
                  child: Row(
                    children: [
                      Expanded(
                        child: Text(
                          '${_fmt(ws)} - ${_fmt(_addDays(ws, 6))}',
                          style: TextStyle(
                            fontSize: 13,
                            fontWeight: selected ? FontWeight.bold : FontWeight.normal,
                            color: Colors.black87,
                          ),
                        ),
                      ),
                      Text('${s[1].toInt()} jobs  ', style: const TextStyle(fontSize: 12, color: Colors.black54)),
                      Text(
                        '\u20B9${_rs(s[0])}',
                        style: TextStyle(
                          fontSize: 13,
                          fontWeight: selected ? FontWeight.bold : FontWeight.w600,
                          color: _brown,
                        ),
                      ),
                    ],
                  ),
                ),
              );
            }),
        ],
      ),
    );
  }
}