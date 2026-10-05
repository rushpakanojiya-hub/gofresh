import 'package:flutter/material.dart';
import '../services/api_service.dart';

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
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
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
                                    child: const Icon(Icons.check, color: Colors.white, size: 18),
                                  ),
                                  const SizedBox(width: 12),
                                  Expanded(
                                    child: Column(
                                      crossAxisAlignment: CrossAxisAlignment.start,
                                      children: [
                                        Text(
                                          'Order #${entry['order_id']}',
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
                        const SizedBox(height: 8),
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
