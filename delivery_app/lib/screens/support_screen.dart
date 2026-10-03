import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

class SupportScreen extends StatelessWidget {
  const SupportScreen({super.key});

  static const Color primaryPurple = Color(0xFF5B2A9E);
  static const Color pageBg = Color(0xFFF7F1FB);

  static const List<Map<String, dynamic>> _categories = [
    {'icon': Icons.person_outline, 'label': 'Customer Issue'},
    {'icon': Icons.storefront_outlined, 'label': 'Store Issue'},
    {'icon': Icons.payment_outlined, 'label': 'Payment Issue'},
    {'icon': Icons.currency_rupee, 'label': 'COD Issue'},
    {'icon': Icons.local_shipping_outlined, 'label': 'Delivery Issue'},
    {'icon': Icons.phone_android_outlined, 'label': 'App Issue'},
  ];

  Future<void> _contactSupport(BuildContext context, String category) async {
    final uri = Uri(
      scheme: 'tel',
      path: '+911800000000',
    );
    try {
      await launchUrl(uri);
    } catch (_) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Could not open dialer. Please call support directly.')),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: pageBg,
      appBar: AppBar(
        backgroundColor: pageBg,
        elevation: 0,
        title: const Text('Help & Support', style: TextStyle(color: Colors.black87)),
        iconTheme: const IconThemeData(color: Colors.black87),
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            const Text(
              'Order Related',
              style: TextStyle(fontSize: 13, fontWeight: FontWeight.bold, color: Colors.black54, letterSpacing: 0.5),
            ),
            const SizedBox(height: 10),
            ..._categories.map((cat) {
              return Container(
                margin: const EdgeInsets.only(bottom: 10),
                decoration: BoxDecoration(color: Colors.white, borderRadius: BorderRadius.circular(14)),
                child: ListTile(
                  leading: Icon(cat['icon'] as IconData, color: primaryPurple),
                  title: Text(cat['label'] as String),
                  trailing: const Icon(Icons.chevron_right, size: 18),
                  onTap: () => _contactSupport(context, cat['label'] as String),
                ),
              );
            }),
            const SizedBox(height: 20),
            SizedBox(
              width: double.infinity,
              child: ElevatedButton.icon(
                onPressed: () => _contactSupport(context, 'General'),
                icon: const Icon(Icons.support_agent),
                label: const Text('Contact Support'),
                style: ElevatedButton.styleFrom(
                  backgroundColor: primaryPurple,
                  foregroundColor: Colors.white,
                  padding: const EdgeInsets.symmetric(vertical: 14),
                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
