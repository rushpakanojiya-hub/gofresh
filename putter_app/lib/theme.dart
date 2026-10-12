// ignore_for_file: prefer_const_constructors, prefer_const_literals_to_create_immutables, unnecessary_const
import 'package:flutter/material.dart';

class AppColors {
  static const Color navy = Color(0xFF0A2A66);
  static const Color blue = Color(0xFF0D47A1);
  static const Color sky = Color(0xFF1E6FD9);
  static const Color green = Color(0xFF16A34A);
  static const Color orange = Color(0xFFEA580C);
  static const Color red = Color(0xFFDC2626);
  static const Color bg = Color(0xFFF4F6FA);
  static const Color line = Color(0xFFE4E7EC);
  static const Color ink = Color(0xFF101828);
  static const Color muted = Color(0xFF667085);
}

const String kOrigin = 'https://gofresh-evl7.onrender.com';

BoxDecoration cardDecoration({Color color = Colors.white, double radius = 16, Color? border}) {
  return BoxDecoration(
    color: color,
    borderRadius: BorderRadius.circular(radius),
    border: Border.all(color: border ?? AppColors.line),
    boxShadow: const [
      BoxShadow(color: Color(0x0D101828), blurRadius: 14, offset: Offset(0, 5)),
    ],
  );
}

class GradientHeader extends StatelessWidget {
  final Widget child;
  const GradientHeader({super.key, required this.child});

  @override
  Widget build(BuildContext context) {
    final top = MediaQuery.of(context).padding.top;
    return Container(
      width: double.infinity,
      padding: EdgeInsets.fromLTRB(20, top + 18, 20, 26),
      decoration: const BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [AppColors.navy, AppColors.sky],
        ),
        borderRadius: BorderRadius.vertical(bottom: Radius.circular(28)),
      ),
      child: child,
    );
  }
}

class StatusPill extends StatelessWidget {
  final String text;
  final Color color;
  const StatusPill(this.text, this.color, {super.key});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(color: color.withAlpha(28), borderRadius: BorderRadius.circular(20)),
      child: Text(text, style: TextStyle(fontSize: 11, fontWeight: FontWeight.w700, color: color)),
    );
  }
}

class PrimaryButton extends StatelessWidget {
  final String label;
  final VoidCallback? onPressed;
  final bool busy;
  final Color color;
  final IconData? icon;
  const PrimaryButton({
    super.key,
    required this.label,
    required this.onPressed,
    this.busy = false,
    this.color = Colors.black,
    this.icon,
  });

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: double.infinity,
      child: ElevatedButton(
        onPressed: busy ? null : onPressed,
        style: ElevatedButton.styleFrom(
          backgroundColor: color,
          foregroundColor: Colors.white,
          disabledBackgroundColor: Color(0xFFD0D5DD),
          disabledForegroundColor: Colors.white,
          padding: EdgeInsets.symmetric(vertical: 15),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
        ),
        child: busy
            ? SizedBox(
                height: 20,
                width: 20,
                child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
              )
            : Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (icon != null) ...[Icon(icon, size: 18), SizedBox(width: 8)],
                  Text(label, style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700)),
                ],
              ),
      ),
    );
  }
}

Map<String, dynamic> asMap(dynamic v) => v is Map ? Map<String, dynamic>.from(v) : <String, dynamic>{};

List<Map<String, dynamic>> asList(dynamic v) =>
    v is List ? v.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList() : <Map<String, dynamic>>[];

int asInt(dynamic v) => (v as num?)?.toInt() ?? 0;

String two(int n) => n.toString().padLeft(2, '0');

String hm(int secs) => '${secs ~/ 3600}h ${two((secs % 3600) ~/ 60)}m';

String clock12(DateTime d) {
  final h = d.hour % 12 == 0 ? 12 : d.hour % 12;
  return '$h:${two(d.minute)} ${d.hour >= 12 ? 'PM' : 'AM'}';
}

String time12(String raw) {
  final p = raw.split(':');
  if (p.length < 2) return raw;
  final h = int.tryParse(p[0]) ?? 0;
  final m = int.tryParse(p[1]) ?? 0;
  final h12 = h % 12 == 0 ? 12 : h % 12;
  return '$h12:${two(m)} ${(h >= 12 && h < 24) ? 'PM' : 'AM'}';
}

String dateKey(DateTime d) => '${d.year}-${two(d.month)}-${two(d.day)}';

const List<String> _wd = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
const List<String> _mo = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

String weekdayShort(DateTime d) => _wd[d.weekday - 1];

String longDate(DateTime d) => '${weekdayShort(d)}, ${d.day} ${_mo[d.month - 1]}';