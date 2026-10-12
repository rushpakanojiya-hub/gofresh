// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../theme.dart';

class SlotsScreen extends StatefulWidget {
  const SlotsScreen({super.key});

  @override
  State<SlotsScreen> createState() => _SlotsScreenState();
}

class _SlotsScreenState extends State<SlotsScreen> {
  late final List<DateTime> _days;
  int _sel = 0;
  Map<String, dynamic> _data = {};
  List<Map<String, dynamic>> _bookings = [];
  bool _loading = true;
  bool _booking = false;
  String? _error;
  int? _selSlot;

  @override
  void initState() {
    super.initState();
    final now = DateTime.now();
    _days = List.generate(7, (i) => DateTime(now.year, now.month, now.day + i));
    _refresh();
  }

  void _snack(String m) {
    if (!mounted) return;
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(m)));
  }

  Future<void> _loadSlots() async {
    try {
      final r = await ApiService.getSlots(dateKey(_days[_sel]));
      if (!mounted) return;
      setState(() {
        if (r['error'] != null) {
          _error = r['error'].toString();
        } else {
          _data = r;
          _error = null;
        }
        _selSlot = null;
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

  Future<void> _loadBookings() async {
    try {
      final r = await ApiService.myBookings();
      if (!mounted || r['error'] != null) return;
      setState(() => _bookings = asList(r['bookings']));
    } catch (_) {}
  }

  Future<void> _refresh() async {
    await Future.wait([_loadSlots(), _loadBookings()]);
  }

  Future<void> _pickDay(int i) async {
    if (i == _sel) return;
    setState(() {
      _sel = i;
      _loading = true;
    });
    await _loadSlots();
  }

  Future<void> _book() async {
    final id = _selSlot;
    if (id == null || _booking) return;
    setState(() => _booking = true);
    try {
      final r = await ApiService.bookSlot(id, dateKey(_days[_sel]));
      if (!mounted) return;
      _snack(r['error'] != null ? r['error'].toString() : 'Slot booked');
      await _refresh();
    } catch (_) {
      _snack('Network error. Please try again.');
    }
    if (mounted) setState(() => _booking = false);
  }

  Future<void> _cancel(int id) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('Cancel booking?'),
        content: Text('This slot will be released for other putters.'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: Text('Keep')),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: Text('Cancel booking', style: TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );
    if (ok != true) return;
    try {
      final r = await ApiService.cancelBooking(id);
      if (!mounted) return;
      if (r['error'] != null) {
        _snack(r['error'].toString());
      } else {
        _snack('Booking cancelled');
      }
      await _refresh();
    } catch (_) {
      _snack('Network error. Please try again.');
    }
  }

  Widget _dayChip(int i) {
    final d = _days[i];
    final on = i == _sel;
    final key = dateKey(d);
    final has = _bookings.any((b) => b['date'] == key);
    return GestureDetector(
      onTap: () => _pickDay(i),
      child: Container(
        width: 62,
        margin: EdgeInsets.only(right: 10),
        padding: EdgeInsets.symmetric(vertical: 10),
        decoration: BoxDecoration(
          color: on ? AppColors.blue : Colors.white,
          borderRadius: BorderRadius.circular(16),
          border: Border.all(color: on ? AppColors.blue : AppColors.line),
        ),
        child: Column(
          children: [
            Text(i == 0 ? 'Today' : weekdayShort(d),
                style: TextStyle(fontSize: 11, fontWeight: FontWeight.w600, color: on ? Colors.white70 : AppColors.muted)),
            SizedBox(height: 4),
            Text('${d.day}',
                style: TextStyle(fontSize: 20, fontWeight: FontWeight.w800, color: on ? Colors.white : AppColors.ink)),
            SizedBox(height: 4),
            Container(
              width: 6,
              height: 6,
              decoration: BoxDecoration(
                color: has ? (on ? Colors.white : AppColors.green) : Colors.transparent,
                shape: BoxShape.circle,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _slotCard(Map<String, dynamic> s) {
    final id = asInt(s['id']);
    final cap = asInt(s['capacity']);
    final booked = asInt(s['booked']);
    final mine = s['booked_by_me'] == true;
    final ended = s['ended'] == true;
    final full = cap > 0 && booked >= cap;
    final disabled = mine || full || ended;
    final selected = _selSlot == id;
    String label = 'Open';
    Color color = AppColors.green;
    if (mine) {
      label = 'Booked by you';
      color = AppColors.blue;
    } else if (ended) {
      label = 'Over';
      color = AppColors.muted;
    } else if (full) {
      label = 'Full';
      color = AppColors.red;
    }
    final progress = cap > 0 ? (booked / cap).clamp(0.0, 1.0).toDouble() : 0.0;
    return Padding(
      padding: EdgeInsets.only(bottom: 10),
      child: InkWell(
        borderRadius: BorderRadius.circular(16),
        onTap: disabled ? null : () => setState(() => _selSlot = selected ? null : id),
        child: Opacity(
          opacity: (ended || full) && !mine ? 0.6 : 1,
          child: Container(
            padding: EdgeInsets.all(14),
            decoration: BoxDecoration(
              color: Colors.white,
              borderRadius: BorderRadius.circular(16),
              border: Border.all(
                color: (selected || mine) ? AppColors.blue : AppColors.line,
                width: selected ? 2 : 1,
              ),
            ),
            child: Row(
              children: [
                Container(
                  width: 44,
                  height: 44,
                  decoration: BoxDecoration(color: Color(0xFFEFF5FF), shape: BoxShape.circle),
                  child: Icon(selected ? Icons.check_rounded : Icons.schedule_rounded, color: AppColors.blue),
                ),
                SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('${time12((s['start_time'] ?? '').toString())} - ${time12((s['end_time'] ?? '').toString())}',
                          style: TextStyle(fontSize: 16, fontWeight: FontWeight.w800, color: AppColors.ink)),
                      SizedBox(height: 8),
                      ClipRRect(
                        borderRadius: BorderRadius.circular(4),
                        child: LinearProgressIndicator(
                          value: progress,
                          minHeight: 6,
                          backgroundColor: Color(0xFFE4E7EC),
                          color: full ? AppColors.red : AppColors.green,
                        ),
                      ),
                      SizedBox(height: 4),
                      Text('$booked/$cap booked', style: TextStyle(fontSize: 11, color: AppColors.muted)),
                    ],
                  ),
                ),
                SizedBox(width: 8),
                StatusPill(label, color),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _bookingTile(Map<String, dynamic> b) {
    final id = asInt(b['id']);
    final d = DateTime.tryParse((b['date'] ?? '').toString());
    return Container(
      margin: EdgeInsets.only(bottom: 10),
      padding: EdgeInsets.fromLTRB(14, 12, 6, 12),
      decoration: cardDecoration(),
      child: Row(
        children: [
          Container(
            width: 44,
            height: 44,
            decoration: BoxDecoration(color: Color(0xFFE8F9EE), shape: BoxShape.circle),
            child: Icon(Icons.event_available_rounded, color: AppColors.green),
          ),
          SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(d == null ? (b['date'] ?? '').toString() : longDate(d),
                    style: TextStyle(fontSize: 14, fontWeight: FontWeight.w800, color: AppColors.ink)),
                SizedBox(height: 2),
                Text('${time12((b['start_time'] ?? '').toString())} - ${time12((b['end_time'] ?? '').toString())}  |  ${b['store_name'] ?? ''}',
                    style: TextStyle(fontSize: 12, color: AppColors.muted)),
              ],
            ),
          ),
          TextButton(onPressed: () => _cancel(id), child: Text('Cancel', style: TextStyle(color: Colors.red))),
        ],
      ),
    );
  }

  Widget _sectionTitle(String t) {
    return Padding(
      padding: EdgeInsets.only(top: 6, bottom: 12),
      child: Text(t, style: TextStyle(fontSize: 16, fontWeight: FontWeight.w800, color: AppColors.ink)),
    );
  }

  @override
  Widget build(BuildContext context) {
    final store = asMap(_data['store']);
    final slots = asList(_data['slots']);
    final storeName = (store['name'] ?? '').toString();

    Widget slotArea;
    if (_loading) {
      slotArea = Padding(
        padding: EdgeInsets.symmetric(vertical: 36),
        child: Center(child: CircularProgressIndicator()),
      );
    } else if (_error != null) {
      slotArea = Padding(
        padding: EdgeInsets.symmetric(vertical: 24),
        child: Center(child: Text(_error!, style: TextStyle(color: AppColors.muted))),
      );
    } else if (slots.isEmpty) {
      slotArea = Padding(
        padding: EdgeInsets.symmetric(vertical: 24),
        child: Center(child: Text('No slots available for this day', style: TextStyle(color: AppColors.muted))),
      );
    } else {
      slotArea = Column(children: [for (final s in slots) _slotCard(s)]);
    }

    return Column(
      children: [
        GradientHeader(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('Book your slot', style: TextStyle(color: Colors.white, fontSize: 24, fontWeight: FontWeight.w800)),
              SizedBox(height: 6),
              Row(
                children: [
                  Icon(Icons.storefront_outlined, size: 16, color: Colors.white70),
                  SizedBox(width: 6),
                  Expanded(
                    child: Text(storeName.isEmpty ? 'Your store' : storeName,
                        style: TextStyle(color: Colors.white70, fontSize: 13, fontWeight: FontWeight.w600)),
                  ),
                ],
              ),
            ],
          ),
        ),
        Expanded(
          child: RefreshIndicator(
            onRefresh: _refresh,
            child: ListView(
              physics: AlwaysScrollableScrollPhysics(),
              padding: EdgeInsets.fromLTRB(0, 14, 0, 24),
              children: [
                SizedBox(
                  height: 94,
                  child: ListView.builder(
                    scrollDirection: Axis.horizontal,
                    padding: EdgeInsets.symmetric(horizontal: 16),
                    itemCount: _days.length,
                    itemBuilder: (c, i) => _dayChip(i),
                  ),
                ),
                Padding(
                  padding: EdgeInsets.fromLTRB(16, 8, 16, 0),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      _sectionTitle('Available slots - ${longDate(_days[_sel])}'),
                      slotArea,
                      SizedBox(height: 12),
                      _sectionTitle('My bookings'),
                      if (_bookings.isEmpty)
                        Padding(
                          padding: EdgeInsets.symmetric(vertical: 12),
                          child: Text('No upcoming bookings', style: TextStyle(color: AppColors.muted)),
                        ),
                      for (final b in _bookings) _bookingTile(b),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
        if (_selSlot != null)
          Container(
            padding: EdgeInsets.fromLTRB(16, 12, 16, 12),
            decoration: BoxDecoration(
              color: Colors.white,
              boxShadow: [BoxShadow(color: Color(0x14101828), blurRadius: 14, offset: Offset(0, -4))],
            ),
            child: PrimaryButton(
              label: 'Book slot',
              icon: Icons.check_circle_outline,
              color: AppColors.blue,
              busy: _booking,
              onPressed: _book,
            ),
          ),
      ],
    );
  }
}