import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';

class ApiService {
  static const String baseUrl = 'https://gofresh-evl7.onrender.com/api/v1';
  static const String _tokenKey = 'putter_token';
  static const String _staffKey = 'putter_staff';

  static Map<String, dynamic> _decode(http.Response res) {
    try {
      final d = jsonDecode(res.body);
      if (d is Map<String, dynamic>) {
        d['_http'] = res.statusCode;
        return d;
      }
    } catch (_) {}
    return {'error': 'Unexpected response (${res.statusCode})', '_http': res.statusCode};
  }

  static Future<Map<String, dynamic>> sendOtp(String phone) async {
    final res = await http
        .post(
          Uri.parse('$baseUrl/warehouse/send-otp'),
          headers: {'Content-Type': 'application/json'},
          body: jsonEncode({'phone': phone}),
        )
        .timeout(const Duration(seconds: 20));
    return _decode(res);
  }

  static Future<Map<String, dynamic>> verifyOtp(String phone, String otp) async {
    final res = await http
        .post(
          Uri.parse('$baseUrl/warehouse/verify-otp'),
          headers: {'Content-Type': 'application/json'},
          body: jsonEncode({'phone': phone, 'otp': otp}),
        )
        .timeout(const Duration(seconds: 20));
    final data = _decode(res);
    if (data['token'] != null) {
      final staff = data['staff'] ?? data['warehouse_staff'];
      final role = staff is Map ? (staff['role'] ?? '').toString() : '';
      if (role != 'putter') {
        data.remove('token');
        data['error'] = 'This app is only for putters. Please use the app for your role.';
        return data;
      }
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString(_tokenKey, data['token'].toString());
      await prefs.setString(_staffKey, jsonEncode(staff));
    }
    return data;
  }

  static Future<String?> getToken() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_tokenKey);
  }

  static Future<void> logout() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_tokenKey);
    await prefs.remove(_staffKey);
  }

  static Future<Map<String, String>> _headers() async {
    final t = await getToken();
    return {
      'Content-Type': 'application/json',
      if (t != null) 'Authorization': 'Bearer $t',
    };
  }

  static Future<Map<String, dynamic>> _get(String path) async {
    final res = await http
        .get(Uri.parse('$baseUrl$path'), headers: await _headers())
        .timeout(const Duration(seconds: 25));
    return _decode(res);
  }

  static Future<Map<String, dynamic>> _post(String path, Map<String, dynamic> body) async {
    final res = await http
        .post(Uri.parse('$baseUrl$path'), headers: await _headers(), body: jsonEncode(body))
        .timeout(const Duration(seconds: 25));
    return _decode(res);
  }

  static Future<Map<String, dynamic>> _delete(String path) async {
    final res = await http
        .delete(Uri.parse('$baseUrl$path'), headers: await _headers())
        .timeout(const Duration(seconds: 25));
    return _decode(res);
  }

  // ---------- putter ----------
  static Future<Map<String, dynamic>> getDashboard() => _get('/warehouse/putter/dashboard');
  static Future<Map<String, dynamic>> startShift() => _post('/warehouse/putter/shift/start', {});
  static Future<Map<String, dynamic>> endShift() => _post('/warehouse/putter/shift/end', {});

  // ---------- slots ----------
  static Future<Map<String, dynamic>> getSlots(String date) => _get('/warehouse/putter/slots?date=$date');
  static Future<Map<String, dynamic>> bookSlot(int slotId, String date) =>
      _post('/warehouse/putter/slots/$slotId/book', {'date': date});
  static Future<Map<String, dynamic>> myBookings() => _get('/warehouse/putter/bookings/me');
  static Future<Map<String, dynamic>> cancelBooking(int id) => _delete('/warehouse/putter/bookings/$id');
}