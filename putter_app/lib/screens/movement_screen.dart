// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../theme.dart';
import 'putaway_common.dart';
import 'movement_history_screen.dart';

class MovementScreen extends StatefulWidget {
  final bool shiftActive;
  const MovementScreen({super.key, required this.shiftActive});

  @override
  State<MovementScreen> createState() => _MovementScreenState();
}

class _MovementScreenState extends State<MovementScreen> {
  static const _titles = ['Scan source bin', 'Select item', 'Scan destination bin', 'Confirm move', 'Moved'];

  int _step = 0;
  bool _busy = false;
  String? _err;
  Map<String, dynamic> _from = {};
  List<Map<String, dynamic>> _items = [];
  Map<String, dynamic> _item = {};
  Map<String, dynamic> _to = {};

  void _snack(String m) {
    if (!mounted) return;
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(m)));
  }

  void _back() {
    if (_step > 0 && _step < 4) {
      setState(() {
        _step--;
        _err = null;
      });
    } else {
      Navigator.pop(context);
    }
  }

  void _reset() {
    setState(() {
      _step = 0;
      _err = null;
      _from = {};
      _items = [];
      _item = {};
      _to = {};
    });
  }

  Future<void> _scanSource() async {
    if (!widget.shiftActive) {
      _snack('Start your shift from Home first');
      return;
    }
    final code = await scanCode(context, 'Scan source bin', 'Enter bin code');
    if (code == null || !mounted) return;
    setState(() => _busy = true);
    try {
      final r = await ApiService.movementLookup(code);
      if (!mounted) return;
      setState(() {
        if (r['found'] != true) {
          _err = (r['error'] ?? 'Bin not found').toString();
        } else if (asList(r['items']).isEmpty) {
          _err = 'No stock in this bin';
        } else {
          _from = asMap(r['bin']);
          _items = asList(r['items']);
          _err = null;
          _step = 1;
        }
      });
    } catch (_) {
      if (mounted) setState(() => _err = 'Network error. Please try again.');
    }
    if (mounted) setState(() => _busy = false);
  }

  Future<void> _scanDest() async {
    final code = await scanCode(context, 'Scan destination bin', 'Enter bin code');
    if (code == null || !mounted) return;
    setState(() => _busy = true);
    try {
      final r = await ApiService.movementLookup(code);
      if (!mounted) return;
      setState(() {
        if (r['found'] != true) {
          _err = (r['error'] ?? 'Bin not found').toString();
        } else if (asInt(asMap(r['bin'])['id']) == asInt(_from['id'])) {
          _err = 'Destination is the same as the source bin';
        } else {
          _to = asMap(r['bin']);
          _err = null;
          _step = 3;
        }
      });
    } catch (_) {
      if (mounted) setState(() => _err = 'Network error. Please try again.');
    }
    if (mounted) setState(() => _busy = false);
  }

  Future<void> _confirm() async {
    setState(() => _busy = true);
    try {
      final r = await ApiService.moveStock(asInt(_item['product_id']), asInt(_from['id']), asInt(_to['id']));
      if (!mounted) return;
      if (r['error'] != null) {
        _snack(r['error'].toString());
      } else {
        setState(() {
          _step = 4;
          _err = null;
        });
      }
    } catch (_) {
      _snack('Network error. Please try again.');
    }
    if (mounted) setState(() => _busy = false);
  }

  Widget _card(Widget child) => Container(
        width: double.infinity,
        margin: EdgeInsets.only(bottom: 14),
        padding: EdgeInsets.all(16),
        decoration: cardDecoration(radius: 20),
        child: child,
      );

  Widget _row(String label, String value) => Padding(
        padding: EdgeInsets.symmetric(vertical: 6),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(child: Text(label, style: TextStyle(fontSize: 13, color: AppColors.muted))),
            SizedBox(width: 12),
            Flexible(
              child: Text(value.isEmpty ? '-' : value,
                  textAlign: TextAlign.right, style: TextStyle(fontSize: 14, fontWeight: FontWeight.w700, color: AppColors.ink)),
            ),
          ],
        ),
      );

  Widget _hero(IconData icon, String title, String sub, {Color color = AppColors.blue}) {
    return Column(
      children: [
        SizedBox(height: 8),
        Container(
          width: 84,
          height: 84,
          decoration: BoxDecoration(color: color.withAlpha(28), shape: BoxShape.circle),
          child: Icon(icon, size: 42, color: color),
        ),
        SizedBox(height: 14),
        Text(title, textAlign: TextAlign.center, style: TextStyle(fontSize: 20, fontWeight: FontWeight.w800, color: AppColors.ink)),
        SizedBox(height: 6),
        Text(sub, textAlign: TextAlign.center, style: TextStyle(fontSize: 13, color: AppColors.muted)),
        SizedBox(height: 18),
      ],
    );
  }

  Widget _errText() => _err == null
      ? SizedBox.shrink()
      : Padding(
          padding: EdgeInsets.only(bottom: 12),
          child: Text(_err!, textAlign: TextAlign.center, style: TextStyle(color: AppColors.red, fontWeight: FontWeight.w600)),
        );

  Widget _stepSource() {
    return Column(
      children: [
        if (!widget.shiftActive)
          Container(
            width: double.infinity,
            padding: EdgeInsets.all(12),
            margin: EdgeInsets.only(bottom: 12),
            decoration: BoxDecoration(color: Color(0xFFFFF4E5), borderRadius: BorderRadius.circular(12)),
            child: Text('Start your shift from Home to move stock.',
                style: TextStyle(fontSize: 12, color: AppColors.orange, fontWeight: FontWeight.w600)),
          ),
        _card(Column(children: [
          _hero(Icons.qr_code_2_rounded, 'Scan source bin', 'Scan the bin you are taking stock from'),
          _errText(),
          PrimaryButton(label: 'Scan bin', icon: Icons.qr_code_scanner, color: AppColors.blue, busy: _busy, onPressed: _scanSource),
        ])),
      ],
    );
  }

  Widget _stepItems() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: EdgeInsets.only(bottom: 10),
          child: Text('Items in ${_from['label'] ?? ''}', style: TextStyle(fontSize: 14, fontWeight: FontWeight.w800, color: AppColors.ink)),
        ),
        for (final it in _items)
          Padding(
            padding: EdgeInsets.only(bottom: 10),
            child: InkWell(
              borderRadius: BorderRadius.circular(16),
              onTap: () => setState(() {
                _item = it;
                _err = null;
                _step = 2;
              }),
              child: Container(
                padding: EdgeInsets.all(12),
                decoration: cardDecoration(),
                child: Row(
                  children: [
                    ClipRRect(
                      borderRadius: BorderRadius.circular(12),
                      child: imgUrl(it['image_url']).isEmpty
                          ? Container(width: 54, height: 54, color: Color(0xFFF2F4F7), child: Icon(Icons.inventory_2_outlined, color: Colors.grey))
                          : Image.network(imgUrl(it['image_url']), width: 54, height: 54, fit: BoxFit.cover,
                              errorBuilder: (c, e, s) => Container(width: 54, height: 54, color: Color(0xFFF2F4F7), child: Icon(Icons.broken_image_outlined, color: Colors.grey))),
                    ),
                    SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text('${it['name'] ?? ''}', maxLines: 2, overflow: TextOverflow.ellipsis, style: TextStyle(fontSize: 14, fontWeight: FontWeight.w800, color: AppColors.ink)),
                          SizedBox(height: 4),
                          Text('Stock ${asInt(it['stock'])}', style: TextStyle(fontSize: 12, color: AppColors.muted)),
                        ],
                      ),
                    ),
                    Icon(Icons.chevron_right, color: AppColors.muted),
                  ],
                ),
              ),
            ),
          ),
      ],
    );
  }

  Widget _stepDest() {
    return _card(Column(children: [
      _hero(Icons.qr_code_2_rounded, 'Scan destination bin', 'Moving ${_item['name'] ?? ''} from ${_from['label'] ?? ''}'),
      _errText(),
      PrimaryButton(label: 'Scan bin', icon: Icons.qr_code_scanner, color: AppColors.blue, busy: _busy, onPressed: _scanDest),
    ]));
  }

  Widget _stepConfirm() {
    return Column(
      children: [
        _card(Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Move summary', style: TextStyle(fontSize: 15, fontWeight: FontWeight.w800)),
            SizedBox(height: 8),
            _row('Item', '${_item['name'] ?? ''}'),
            _row('Stock moving', '${asInt(_item['stock'])}'),
            _row('From', '${_from['label'] ?? ''}'),
            _row('To', '${_to['label'] ?? ''}'),
          ],
        )),
        Container(
          width: double.infinity,
          padding: EdgeInsets.all(12),
          margin: EdgeInsets.only(bottom: 12),
          decoration: BoxDecoration(color: Color(0xFFEFF5FF), borderRadius: BorderRadius.circular(12)),
          child: Text('The full stock of this item moves to the new bin.',
              style: TextStyle(fontSize: 12, color: AppColors.blue, fontWeight: FontWeight.w600)),
        ),
        PrimaryButton(label: 'Confirm move', icon: Icons.check_circle_outline, color: AppColors.green, busy: _busy, onPressed: _confirm),
      ],
    );
  }

  Widget _stepDone() {
    return _card(Column(
      children: [
        _hero(Icons.check_circle_rounded, 'Stock moved', 'Bin location updated', color: AppColors.green),
        _row('Item', '${_item['name'] ?? ''}'),
        _row('From', '${_from['label'] ?? ''}'),
        _row('To', '${_to['label'] ?? ''}'),
        SizedBox(height: 16),
        PrimaryButton(label: 'Move another item', icon: Icons.alt_route_rounded, color: AppColors.blue, onPressed: _reset),
      ],
    ));
  }

  Widget _body() {
    switch (_step) {
      case 1:
        return _stepItems();
      case 2:
        return _stepDest();
      case 3:
        return _stepConfirm();
      case 4:
        return _stepDone();
      default:
        return _stepSource();
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.bg,
      body: Column(
        children: [
          GradientHeader(
            child: Row(children: [
              InkWell(onTap: _back, child: Icon(Icons.arrow_back, color: Colors.white)),
              SizedBox(width: 12),
              Expanded(child: Text(_titles[_step], style: TextStyle(color: Colors.white, fontSize: 22, fontWeight: FontWeight.w800))),
              IconButton(
                tooltip: 'Movement history',
                onPressed: () => Navigator.push(context, MaterialPageRoute(builder: (_) => MovementHistoryScreen())),
                icon: Icon(Icons.history, color: Colors.white),
              ),
            ]),
          ),
          Expanded(child: ListView(padding: EdgeInsets.fromLTRB(16, 16, 16, 28), children: [_body()])),
        ],
      ),
    );
  }
}