// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../theme.dart';
import 'putaway_common.dart';

class PutawayTaskScreen extends StatefulWidget {
  final int taskId;
  final bool shiftActive;
  const PutawayTaskScreen({super.key, required this.taskId, required this.shiftActive});

  @override
  State<PutawayTaskScreen> createState() => _PutawayTaskScreenState();
}

class _PutawayTaskScreenState extends State<PutawayTaskScreen> {
  static const _titles = ['GRN details', 'Scan item', 'Item details', 'Suggested bin', 'Scan rack / bin', 'Confirm quantity', 'Completed'];

  Map<String, dynamic> _r = {};
  bool _loading = true;
  bool _busy = false;
  String? _error;
  String? _stepError;
  int _step = 0;
  int? _binId;
  String _binName = '';
  bool _binSuggested = true;
  int _qty = 0;

  Map<String, dynamic> get _task => asMap(_r['task']);
  Map<String, dynamic> get _grn => asMap(_r['grn']);
  Map<String, dynamic> get _product => asMap(_task['product']);
  int get _total => asInt(_task['quantity']);
  String get _name => (_product['name'] ?? 'Product #${_task['product_id'] ?? ''}').toString();

  @override
  void initState() {
    super.initState();
    _load();
  }

  void _snack(String m) {
    if (!mounted) return;
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(m)));
  }

  Future<void> _load() async {
    try {
      final r = await ApiService.putawayTask(widget.taskId);
      if (!mounted) return;
      setState(() {
        if (r['error'] != null) {
          _error = r['error'].toString();
        } else {
          _r = r;
          _error = null;
          if (_task['status'] == 'completed') _step = 6;
          if (_qty == 0) _qty = asInt(_task['quantity']);
        }
        _loading = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _error = 'Network error. Please try again.';
        _loading = false;
      });
    }
  }

  void _back() {
    if (_step > 0 && _step < 6) {
      setState(() {
        _step--;
        _stepError = null;
      });
    } else {
      Navigator.pop(context);
    }
  }

  // ---------- actions ----------
  Future<void> _begin() async {
    if (!widget.shiftActive) {
      _snack('Start your shift from Home first');
      return;
    }
    setState(() => _busy = true);
    try {
      if (_task['status'] == 'pending') {
        final r = await ApiService.startTask(widget.taskId);
        if (r['error'] != null) {
          _snack(r['error'].toString());
          if (mounted) setState(() => _busy = false);
          return;
        }
      }
      if (mounted) {
        setState(() {
          _step = 1;
          _stepError = null;
        });
      }
    } catch (_) {
      _snack('Network error. Please try again.');
    }
    if (mounted) setState(() => _busy = false);
  }

  Future<void> _scanItem() async {
    final code = await scanCode(context, 'Scan item barcode', 'Enter item barcode');
    if (code == null || !mounted) return;
    setState(() => _busy = true);
    try {
      final r = await ApiService.verifyItem(widget.taskId, code);
      if (!mounted) return;
      setState(() {
        if (r['error'] != null) {
          _stepError = r['error'].toString();
        } else if (r['match'] == true) {
          _stepError = null;
          _step = 2;
        } else {
          _stepError = 'Wrong item scanned. Expected: $_name';
        }
      });
    } catch (_) {
      if (mounted) setState(() => _stepError = 'Network error. Please try again.');
    }
    if (mounted) setState(() => _busy = false);
  }

  Future<void> _scanBin() async {
    final code = await scanCode(context, 'Scan rack / bin', 'Enter bin code');
    if (code == null || !mounted) return;
    setState(() => _busy = true);
    try {
      final r = await ApiService.scanBin(widget.taskId, code);
      if (!mounted) return;
      if (r['error'] != null && r['found'] != true) {
        setState(() => _stepError = r['error'].toString());
      } else if (r['found'] == true) {
        final bin = asMap(r['bin']);
        final suggested = r['is_suggested'] == true;
        var use = true;
        if (!suggested) {
          use = await showDialog<bool>(
                context: context,
                builder: (ctx) => AlertDialog(
                  title: Text('Different bin'),
                  content: Text('${bin['label']} is not the suggested bin. Use it anyway?'),
                  actions: [
                    TextButton(onPressed: () => Navigator.pop(ctx, false), child: Text('Rescan')),
                    TextButton(onPressed: () => Navigator.pop(ctx, true), child: Text('Use this bin')),
                  ],
                ),
              ) ??
              false;
        }
        if (use && mounted) {
          setState(() {
            _binId = asInt(bin['id']);
            _binName = (bin['label'] ?? '').toString();
            _binSuggested = suggested;
            _stepError = null;
            _step = 5;
          });
        }
      } else {
        setState(() => _stepError = 'Bin not found in your warehouse');
      }
    } catch (_) {
      if (mounted) setState(() => _stepError = 'Network error. Please try again.');
    }
    if (mounted) setState(() => _busy = false);
  }

  Future<void> _complete() async {
    final bin = _binId;
    if (bin == null) return;
    setState(() => _busy = true);
    try {
      final r = await ApiService.completeTask(widget.taskId, bin);
      if (!mounted) return;
      if (r['error'] != null) {
        _snack(r['error'].toString());
      } else {
        setState(() {
          _step = 6;
          _stepError = null;
        });
      }
    } catch (_) {
      _snack('Network error. Please try again.');
    }
    if (mounted) setState(() => _busy = false);
  }

  Future<void> _openReport({String type = 'damaged', int? qty}) async {
    final qc = TextEditingController(text: '${qty ?? 1}');
    final rc = TextEditingController();
    String t = type;
    final ok = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.white,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(24))),
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setS) => Padding(
          padding: EdgeInsets.fromLTRB(20, 20, 20, MediaQuery.of(ctx).viewInsets.bottom + 20),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('Report damaged / short stock', style: TextStyle(fontSize: 18, fontWeight: FontWeight.w800)),
              SizedBox(height: 14),
              Wrap(
                spacing: 8,
                children: [
                  ChoiceChip(label: Text('Damaged'), selected: t == 'damaged', onSelected: (_) => setS(() => t = 'damaged')),
                  ChoiceChip(label: Text('Short stock'), selected: t == 'short', onSelected: (_) => setS(() => t = 'short')),
                ],
              ),
              SizedBox(height: 14),
              TextField(
                controller: qc,
                keyboardType: TextInputType.number,
                decoration: InputDecoration(labelText: 'Quantity affected', border: OutlineInputBorder()),
              ),
              SizedBox(height: 12),
              TextField(
                controller: rc,
                maxLines: 2,
                decoration: InputDecoration(labelText: 'Reason (optional)', border: OutlineInputBorder()),
              ),
              SizedBox(height: 16),
              PrimaryButton(label: 'Submit report', icon: Icons.report_gmailerrorred, color: AppColors.red, onPressed: () => Navigator.pop(ctx, true)),
            ],
          ),
        ),
      ),
    );
    final q = int.tryParse(qc.text.trim()) ?? 0;
    final reason = rc.text.trim();
    if (ok != true || !mounted) return;
    try {
      final r = await ApiService.reportIssue(widget.taskId, t, q, reason);
      _snack(r['error'] != null ? r['error'].toString() : 'Report submitted');
      await _load();
    } catch (_) {
      _snack('Network error. Please try again.');
    }
  }

  // ---------- ui helpers ----------
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

  Widget _errText() => _stepError == null
      ? SizedBox.shrink()
      : Padding(
          padding: EdgeInsets.only(bottom: 12),
          child: Text(_stepError!, textAlign: TextAlign.center, style: TextStyle(color: AppColors.red, fontWeight: FontWeight.w600)),
        );

  Widget _productHead() {
    final img = imgUrl(_product['image_url']);
    Widget box(IconData ic) => Container(width: 64, height: 64, color: Color(0xFFF2F4F7), child: Icon(ic, color: Colors.grey));
    return Row(
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(14),
          child: img.isEmpty
              ? box(Icons.inventory_2_outlined)
              : Image.network(img, width: 64, height: 64, fit: BoxFit.cover, errorBuilder: (c, e, s) => box(Icons.broken_image_outlined)),
        ),
        SizedBox(width: 14),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(_name, maxLines: 2, overflow: TextOverflow.ellipsis, style: TextStyle(fontSize: 16, fontWeight: FontWeight.w800, color: AppColors.ink)),
              SizedBox(height: 4),
              Text('Put-away quantity: $_total', style: TextStyle(fontSize: 13, color: AppColors.muted)),
            ],
          ),
        ),
      ],
    );
  }

  // ---------- steps ----------
  Widget _stepGrn() {
    final reports = asList(_r['reports']);
    final ref = (_grn['reference_number'] ?? '').toString();
    return Column(
      children: [
        _card(_productHead()),
        _card(Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('GRN / Received stock', style: TextStyle(fontSize: 15, fontWeight: FontWeight.w800)),
            SizedBox(height: 8),
            _row('GRN no.', ref.isEmpty ? 'GRN-${_grn['id'] ?? ''}' : ref),
            _row('Supplier', (_grn['supplier_name'] ?? '').toString()),
            _row('Received on', fmtDateTime(_grn['received_at'])),
            _row('QC done on', fmtDateTime(_grn['qc_at'])),
            _row('Expected', '${asInt(_grn['expected_quantity'])}'),
            _row('Received', '${asInt(_grn['received_quantity'])}'),
            _row('Damaged (QC)', '${asInt(_grn['damaged_quantity'])}'),
            _row('Accepted', '${asInt(_grn['accepted_quantity'])}'),
            _row('Status', (_grn['status'] ?? '').toString()),
          ],
        )),
        if (reports.isNotEmpty)
          _card(Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('Your reports', style: TextStyle(fontSize: 15, fontWeight: FontWeight.w800)),
              SizedBox(height: 6),
              for (final x in reports)
                _row('${x['type'] == 'short' ? 'Short' : 'Damaged'} x${asInt(x['quantity'])}', (x['reason'] ?? '').toString()),
            ],
          )),
        if (!widget.shiftActive && _task['status'] != 'in_progress')
          Padding(
            padding: EdgeInsets.only(bottom: 10),
            child: Text('Start your shift from Home to begin this put-away.', textAlign: TextAlign.center, style: TextStyle(color: AppColors.orange, fontWeight: FontWeight.w600)),
          ),
        PrimaryButton(
          label: _task['status'] == 'in_progress' ? 'Continue put-away' : 'Start put-away',
          icon: Icons.play_arrow_rounded,
          color: AppColors.blue,
          busy: _busy,
          onPressed: _begin,
        ),
      ],
    );
  }

  Widget _stepScanItem() {
    final hasBarcode = _r['has_barcode'] == true;
    return Column(
      children: [
        _card(Column(children: [
          _hero(Icons.qr_code_scanner, 'Scan item barcode', hasBarcode ? _name : '$_name\nNo barcode set for this item, any code is accepted'),
          _errText(),
          PrimaryButton(label: 'Scan barcode', icon: Icons.qr_code_scanner, color: AppColors.blue, busy: _busy, onPressed: _scanItem),
        ])),
      ],
    );
  }

  Widget _stepDetails() {
    final batches = asList(_r['batches']);
    final code = (_product['barcode'] ?? '').toString();
    return Column(
      children: [
        _card(Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _productHead(),
            SizedBox(height: 10),
            Divider(color: AppColors.line),
            _row('Item code', code.isEmpty ? 'ID ${_task['product_id'] ?? ''}' : code),
            _row('Quantity', '$_total'),
          ],
        )),
        _card(Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Batch & expiry', style: TextStyle(fontSize: 15, fontWeight: FontWeight.w800)),
            SizedBox(height: 8),
            if (batches.isEmpty) Text('No batch info recorded for this receiving', style: TextStyle(color: AppColors.muted)),
            for (final b in batches) _batchTile(b),
          ],
        )),
        PrimaryButton(label: 'Continue', icon: Icons.arrow_forward_rounded, color: AppColors.blue, onPressed: () => setState(() => _step = 3)),
      ],
    );
  }

  Widget _batchTile(Map<String, dynamic> b) {
    final days = asInt(b['days_left']);
    Color c = AppColors.green;
    String label = '$days days left';
    if (days < 0) {
      c = AppColors.red;
      label = 'Expired';
    } else if (days <= 7) {
      c = AppColors.red;
      label = 'Expires in ${days}d';
    } else if (days <= 30) {
      c = AppColors.orange;
      label = 'Expires in ${days}d';
    }
    return Container(
      margin: EdgeInsets.only(top: 8),
      padding: EdgeInsets.all(12),
      decoration: BoxDecoration(color: Color(0xFFF8FAFC), borderRadius: BorderRadius.circular(12), border: Border.all(color: AppColors.line)),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('Batch ${b['batch_number'] ?? '-'}', style: TextStyle(fontWeight: FontWeight.w800)),
                SizedBox(height: 3),
                Text('Expiry: ${fmtDay(b['expiry_date'])}  |  Qty ${asInt(b['quantity'])}', style: TextStyle(fontSize: 12, color: AppColors.muted)),
              ],
            ),
          ),
          StatusPill(label, c),
        ],
      ),
    );
  }

  Widget _stepSuggested() {
    final label = (_r['suggested_bin_label'] ?? '').toString();
    return _card(Column(
      children: [
        _hero(Icons.place_rounded, 'Go to this rack / bin', label.isEmpty ? 'No bin was suggested. Scan any bin in your warehouse.' : 'Suggested location'),
        if (label.isNotEmpty)
          Container(
            width: double.infinity,
            padding: EdgeInsets.symmetric(vertical: 16),
            margin: EdgeInsets.only(bottom: 18),
            decoration: BoxDecoration(color: Color(0xFFEFF5FF), borderRadius: BorderRadius.circular(14)),
            child: Center(child: Text(label, style: TextStyle(fontSize: 22, fontWeight: FontWeight.w800, color: AppColors.blue))),
          ),
        PrimaryButton(label: 'I am at the bin', icon: Icons.arrow_forward_rounded, color: AppColors.blue, onPressed: () => setState(() => _step = 4)),
      ],
    ));
  }

  Widget _stepScanBin() {
    final label = (_r['suggested_bin_label'] ?? '').toString();
    return _card(Column(
      children: [
        _hero(Icons.qr_code_2_rounded, 'Scan rack / bin', label.isEmpty ? 'Scan the bin code' : 'Suggested: $label'),
        _errText(),
        PrimaryButton(label: 'Scan bin', icon: Icons.qr_code_scanner, color: AppColors.blue, busy: _busy, onPressed: _scanBin),
      ],
    ));
  }

  Widget _stepQty() {
    final short = _qty < _total;
    return Column(
      children: [
        _card(Column(
          children: [
            _productHead(),
            SizedBox(height: 10),
            Divider(color: AppColors.line),
            _row('Placing in', _binName),
            if (!_binSuggested)
              Padding(padding: EdgeInsets.only(top: 4), child: Align(alignment: Alignment.centerRight, child: StatusPill('Different bin', AppColors.orange))),
          ],
        )),
        _card(Column(
          children: [
            Text('Quantity placed', style: TextStyle(fontSize: 15, fontWeight: FontWeight.w800)),
            SizedBox(height: 10),
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                IconButton.filledTonal(onPressed: _qty > 0 ? () => setState(() => _qty--) : null, icon: Icon(Icons.remove)),
                SizedBox(width: 22),
                Text('$_qty', style: TextStyle(fontSize: 38, fontWeight: FontWeight.w800, color: short ? AppColors.orange : AppColors.green)),
                SizedBox(width: 22),
                IconButton.filledTonal(onPressed: _qty < _total ? () => setState(() => _qty++) : null, icon: Icon(Icons.add)),
              ],
            ),
            SizedBox(height: 4),
            Text('of $_total expected', style: TextStyle(color: AppColors.muted)),
          ],
        )),
        if (short) ...[
          Container(
            width: double.infinity,
            padding: EdgeInsets.all(12),
            margin: EdgeInsets.only(bottom: 12),
            decoration: BoxDecoration(color: Color(0xFFFFF4E5), borderRadius: BorderRadius.circular(12)),
            child: Text(
              'You placed less than expected. Report the short stock. The task stays open until the full quantity is placed.',
              style: TextStyle(fontSize: 12, color: AppColors.orange, fontWeight: FontWeight.w600),
            ),
          ),
          PrimaryButton(
            label: 'Report short stock',
            icon: Icons.report_gmailerrorred,
            color: AppColors.orange,
            onPressed: () => _openReport(type: 'short', qty: _total - _qty),
          ),
        ] else
          PrimaryButton(label: 'Complete put-away', icon: Icons.check_circle_outline, color: AppColors.green, busy: _busy, onPressed: _complete),
      ],
    );
  }

  Widget _stepDone() {
    final binLabel = _binName.isNotEmpty ? _binName : binText(_task['actual_bin']);
    return _card(Column(
      children: [
        _hero(Icons.check_circle_rounded, 'Put-away completed', 'Stock is now available in your store', color: AppColors.green),
        _row('Item', _name),
        _row('Quantity', '$_total'),
        _row('Bin', binLabel),
        SizedBox(height: 16),
        PrimaryButton(label: 'Back to tasks', icon: Icons.assignment_outlined, color: AppColors.blue, onPressed: () => Navigator.pop(context)),
      ],
    ));
  }

  Widget _stepBody() {
    switch (_step) {
      case 1:
        return _stepScanItem();
      case 2:
        return _stepDetails();
      case 3:
        return _stepSuggested();
      case 4:
        return _stepScanBin();
      case 5:
        return _stepQty();
      case 6:
        return _stepDone();
      default:
        return _stepGrn();
    }
  }

  @override
  Widget build(BuildContext context) {
    Widget body;
    if (_loading) {
      body = Center(child: CircularProgressIndicator());
    } else if (_error != null && _r.isEmpty) {
      body = ListView(
        padding: EdgeInsets.fromLTRB(24, 100, 24, 24),
        children: [
          Center(child: Text(_error!, style: TextStyle(color: AppColors.muted))),
          SizedBox(height: 16),
          Center(child: OutlinedButton(onPressed: _load, child: Text('Retry'))),
        ],
      );
    } else {
      body = ListView(padding: EdgeInsets.fromLTRB(16, 16, 16, 28), children: [_stepBody()]);
    }
    return Scaffold(
      backgroundColor: AppColors.bg,
      body: Column(
        children: [
          GradientHeader(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(children: [
                  InkWell(onTap: _back, child: Icon(Icons.arrow_back, color: Colors.white)),
                  SizedBox(width: 12),
                  Expanded(child: Text(_titles[_step], style: TextStyle(color: Colors.white, fontSize: 22, fontWeight: FontWeight.w800))),
                  if (_step < 6 && !_loading)
                    IconButton(onPressed: () => _openReport(), tooltip: 'Report damaged / short', icon: Icon(Icons.report_gmailerrorred, color: Colors.white)),
                ]),
                if (_step < 6) ...[
                  SizedBox(height: 10),
                  ClipRRect(
                    borderRadius: BorderRadius.circular(4),
                    child: LinearProgressIndicator(value: (_step + 1) / 6, minHeight: 5, backgroundColor: Colors.white24, color: Colors.white),
                  ),
                  SizedBox(height: 6),
                  Text('Step ${_step + 1} of 6', style: TextStyle(color: Colors.white70, fontSize: 12, fontWeight: FontWeight.w600)),
                ],
              ],
            ),
          ),
          Expanded(child: body),
        ],
      ),
    );
  }
}