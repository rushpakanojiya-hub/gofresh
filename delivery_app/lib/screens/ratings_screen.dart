import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../services/push_service.dart';

class RatingsScreen extends StatefulWidget {
  const RatingsScreen({super.key});

  @override
  State<RatingsScreen> createState() => _RatingsScreenState();
}

class _RatingsScreenState extends State<RatingsScreen> {
  static const Color primaryPurple = Color(0xFF5B2A9E);
  static const Color pageBg = Color(0xFFF7F1FB);

  List<dynamic> _ratings = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    PushService.ratingRefresh.addListener(_load);
    _load();
  }

  @override
  void dispose() {
    PushService.ratingRefresh.removeListener(_load);
    super.dispose();
  }

  Future<void> _load() async {
    if (!mounted) return;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final r = await ApiService.getMyRatings();
      if (!mounted) return;
      setState(() {
        _ratings = r;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = 'Failed to load ratings';
        _loading = false;
      });
    }
  }

  Future<void> _openOrder(dynamic orderId) async {
    final id = '$orderId';
    try {
      dynamic item;
      for (final status in <String?>[null, 'delivered']) {
        final list = await ApiService.getMyDeliveries(status: status);
        for (final o in list) {
          if (o is Map && '${o['order_id'] ?? o['id']}' == id) {
            item = o;
            break;
          }
        }
        if (item != null) break;
      }
      if (!mounted) return;
      if (item == null) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Order details not available')),
        );
        return;
      }
      // Order detail screen now opens only after the handover step.
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Could not open order')),
      );
    }
  }

  String _date(dynamic raw) {
    try {
      final d = DateTime.parse(raw.toString()).toLocal();
      return '${d.day}/${d.month}/${d.year}';
    } catch (_) {
      return '';
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: pageBg,
      appBar: AppBar(
        backgroundColor: pageBg,
        elevation: 0,
        title: const Text('My Ratings', style: TextStyle(color: Colors.black87)),
        iconTheme: const IconThemeData(color: Colors.black87),
      ),
      body: SafeArea(
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : _error != null
                ? Center(child: Text(_error!))
                : _ratings.isEmpty
                    ? const Center(child: Text('No ratings yet', style: TextStyle(color: Colors.black45)))
                    : RefreshIndicator(
                        onRefresh: _load,
                        child: ListView.builder(
                          padding: const EdgeInsets.all(16),
                          itemCount: _ratings.length,
                          itemBuilder: (context, i) {
                            final r = _ratings[i];
                            final stars = (r['rating'] ?? 0) as int;
                            final review = (r['review'] ?? '').toString();
                            return GestureDetector(
                              onTap: () => _openOrder(r['order_id']),
                              child: Container(
                                margin: const EdgeInsets.only(bottom: 10),
                                padding: const EdgeInsets.all(14),
                                decoration: BoxDecoration(
                                  color: Colors.white,
                                  borderRadius: BorderRadius.circular(14),
                                ),
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Row(
                                      children: [
                                        Text('Order #${r['order_id']}',
                                            style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 14)),
                                        const Spacer(),
                                        Text(_date(r['created_at']),
                                            style: const TextStyle(fontSize: 11, color: Colors.black38)),
                                      ],
                                    ),
                                    const SizedBox(height: 6),
                                    Row(
                                      children: List.generate(
                                        5,
                                        (k) => Icon(
                                          k < stars ? Icons.star : Icons.star_border,
                                          color: primaryPurple,
                                          size: 20,
                                        ),
                                      ),
                                    ),
                                    if (review.isNotEmpty) ...[
                                      const SizedBox(height: 6),
                                      Text(review, style: const TextStyle(fontSize: 13, color: Colors.black54)),
                                    ],
                                  ],
                                ),
                              ),
                            );
                          },
                        ),
                      ),
      ),
    );
  }
}