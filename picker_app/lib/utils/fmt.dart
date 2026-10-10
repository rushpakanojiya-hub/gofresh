String fmtDate(DateTime d) =>
    '${d.year.toString().padLeft(4, '0')}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';

String fmtTime(String hhmm) {
  final p = hhmm.split(':');
  if (p.length < 2) return hhmm;
  var h = int.tryParse(p[0]) ?? 0;
  final m = p[1];
  final ap = h >= 12 ? 'PM' : 'AM';
  h = h % 12;
  if (h == 0) h = 12;
  return '${h.toString().padLeft(2, '0')}:$m $ap';
}

const _dayNames = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
const _monthNames = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December',
];

String fmtDay(DateTime d) => _dayNames[d.weekday - 1];
String fmtMonth(DateTime d) => _monthNames[d.month - 1];
String fmtDateLong(DateTime d) => '${fmtDay(d)}, ${d.day} ${fmtMonth(d)}';

String fmtHm(int seconds) {
  if (seconds < 0) seconds = 0;
  final h = seconds ~/ 3600;
  final m = (seconds % 3600) ~/ 60;
  return '${h.toString().padLeft(2, '0')}h ${m.toString().padLeft(2, '0')}m';
}
