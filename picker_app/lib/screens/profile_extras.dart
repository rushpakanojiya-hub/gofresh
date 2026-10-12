import 'package:flutter/material.dart';
import 'package:package_info_plus/package_info_plus.dart';
import 'package:url_launcher/url_launcher.dart';
import '../services/api_service.dart';

class ProfileExtras extends StatefulWidget {
  const ProfileExtras({super.key});

  @override
  State<ProfileExtras> createState() => _ProfileExtrasState();
}

class _ProfileExtrasState extends State<ProfileExtras> {
  Map<String, dynamic> _p = {};
  bool _loading = true;
  String? _error;
  String _version = '';

  @override
  void initState() {
    super.initState();
    _load();
    _loadVersion();
  }

  Future<void> _loadVersion() async {
    try {
      final i = await PackageInfo.fromPlatform();
      if (mounted) setState(() => _version = '${i.version} (${i.buildNumber})');
    } catch (_) {}
  }

  Future<void> _load() async {
    try {
      final r = await ApiService.getPickerProfile();
      if (!mounted) return;
      setState(() {
        if (r['error'] != null) {
          _error = r['error'].toString();
        } else {
          _p = r;
          _error = null;
        }
        _loading = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _error = 'Network error';
        _loading = false;
      });
    }
  }

  Map<String, dynamic> _m(String k) {
    final v = _p[k];
    return v is Map ? Map<String, dynamic>.from(v) : <String, dynamic>{};
  }

  String _s(Map<String, dynamic> m, String k) => (m[k] ?? '').toString();

  String _date(String raw) {
    final d = DateTime.tryParse(raw)?.toLocal();
    if (d == null) return '';
    const m = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
    return '${d.day} ${m[d.month - 1]} ${d.year}';
  }

  Future<void> _call(String phone) async {
    bool ok = false;
    try {
      ok = await launchUrl(Uri(scheme: 'tel', path: phone));
    } catch (_) {}
    if (!ok && mounted) {
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Could not open the dialer')));
    }
  }

  Future<void> _editPayout() async {
    final pay = _m('payout');
    final upi = TextEditingController(text: _s(pay, 'upi_id'));
    final holder = TextEditingController(text: _s(pay, 'account_holder'));
    final acc = TextEditingController();
    final ifsc = TextEditingController(text: _s(pay, 'ifsc'));
    final last4 = _s(pay, 'account_last4');
    bool busy = false;
    String? err;
    final saved = await showDialog<bool>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setLocal) {
          return AlertDialog(
            title: const Text('Payout details'),
            content: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  TextField(controller: upi, decoration: const InputDecoration(labelText: 'UPI ID', hintText: 'name@bank')),
                  TextField(controller: holder, decoration: const InputDecoration(labelText: 'Account holder name')),
                  TextField(
                    controller: acc,
                    keyboardType: TextInputType.number,
                    decoration: InputDecoration(
                      labelText: 'Bank account number',
                      hintText: last4.isEmpty ? '' : 'Saved: ending $last4 (blank = keep)',
                    ),
                  ),
                  TextField(
                    controller: ifsc,
                    textCapitalization: TextCapitalization.characters,
                    decoration: const InputDecoration(labelText: 'IFSC'),
                  ),
                  if (err != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 8),
                      child: Text(err!, style: const TextStyle(color: Colors.red, fontSize: 12)),
                    ),
                ],
              ),
            ),
            actions: [
              TextButton(onPressed: busy ? null : () => Navigator.pop(ctx, false), child: const Text('Cancel')),
              ElevatedButton(
                onPressed: busy
                    ? null
                    : () async {
                        setLocal(() {
                          busy = true;
                          err = null;
                        });
                        try {
                          final r = await ApiService.savePayoutDetails({
                            'upi_id': upi.text.trim(),
                            'account_holder': holder.text.trim(),
                            'account_number': acc.text.trim(),
                            'ifsc': ifsc.text.trim().toUpperCase(),
                          });
                          if (r['error'] != null) {
                            setLocal(() {
                              busy = false;
                              err = r['error'].toString();
                            });
                            return;
                          }
                          if (ctx.mounted) Navigator.pop(ctx, true);
                        } catch (_) {
                          setLocal(() {
                            busy = false;
                            err = 'Network error';
                          });
                        }
                      },
                child: Text(busy ? 'Saving...' : 'Save'),
              ),
            ],
          );
        },
      ),
    );
    if (saved == true) {
      await _load();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Payout details saved')));
    }
  }

  Widget _card(String title, List<Widget> children, {Widget? trailing}) {
    return Container(
      width: double.infinity,
      margin: const EdgeInsets.only(top: 14),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        border: Border.all(color: const Color(0xFFD0D5DD)),
        borderRadius: BorderRadius.circular(14),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(child: Text(title, style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w800))),
              if (trailing != null) trailing,
            ],
          ),
          const SizedBox(height: 10),
          ...children,
        ],
      ),
    );
  }

  Widget _kv(String k, String v) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(width: 110, child: Text(k, style: TextStyle(color: Colors.grey.shade600, fontSize: 13))),
          Expanded(child: Text(v.isEmpty ? '-' : v, style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 13))),
        ],
      ),
    );
  }

  Widget _stat(String value, String label) {
    return Expanded(
      child: Column(
        children: [
          Text(value, style: const TextStyle(fontSize: 20, fontWeight: FontWeight.w800)),
          const SizedBox(height: 2),
          Text(label, style: TextStyle(fontSize: 12, color: Colors.grey.shade600)),
        ],
      ),
    );
  }

  Widget _range(String title, Map<String, dynamic> m) {
    return _card(title, [
      Row(
        children: [
          _stat('${(m['orders'] as num?)?.toInt() ?? 0}', 'Orders'),
          _stat('${(m['items_picked'] as num?)?.toInt() ?? 0}', 'Items'),
          _stat('${(m['on_time_pct'] as num?)?.toInt() ?? 0}%', 'On time'),
        ],
      ),
    ]);
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Padding(
        padding: EdgeInsets.only(top: 24),
        child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
      );
    }
    if (_error != null) {
      return Padding(
        padding: const EdgeInsets.only(top: 16),
        child: Center(child: Text(_error!, style: TextStyle(color: Colors.grey.shade600))),
      );
    }
    final store = _m('store');
    final sup = _m('support');
    final pay = _m('payout');
    final mgrPhone = _s(sup, 'manager_phone');
    final last4 = _s(pay, 'account_last4');
    final hasBank = last4.isNotEmpty;
    final hasAny = hasBank || _s(pay, 'upi_id').isNotEmpty;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _card('Store', [
          _kv('Store', _s(store, 'name')),
          _kv('City', _s(store, 'city')),
          _kv('Address', _s(store, 'address')),
          _kv('Member since', _date(_s(_p, 'member_since'))),
        ]),
        _range('Last 7 days', _m('week')),
        _range('This month', _m('month')),
        _card(
          'Payout details',
          [
            _kv('UPI ID', _s(pay, 'upi_id')),
            _kv('Account holder', _s(pay, 'account_holder')),
            _kv('Bank account', hasBank ? 'XXXX$last4' : ''),
            _kv('IFSC', _s(pay, 'ifsc')),
          ],
          trailing: TextButton(onPressed: _editPayout, child: Text(hasAny ? 'Edit' : 'Add')),
        ),
        _card('Support', [
          _kv('Store manager', _s(sup, 'manager_name')),
          const SizedBox(height: 8),
          SizedBox(
            width: double.infinity,
            child: OutlinedButton.icon(
              onPressed: mgrPhone.isEmpty ? null : () => _call(mgrPhone),
              icon: const Icon(Icons.call_outlined),
              label: Text(mgrPhone.isEmpty ? 'Manager number not available' : 'Call store manager'),
            ),
          ),
        ]),
        const SizedBox(height: 18),
        Center(
          child: Text(
            'App version ${_version.isEmpty ? '-' : _version}',
            style: TextStyle(fontSize: 12, color: Colors.grey.shade600),
          ),
        ),
      ],
    );
  }
}