import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';

class ApiService {
  static const String baseUrl = 'https://gofresh-evl7.onrender.com/api/v1';
  static const String _tokenKey = 'picker_token';
  static const String _staffKey = 'picker_staff';

  static Map<String, dynamic> _decode(http.Response res) {
    try {
      final d = jsonDecode(res.body);
      if (d is Map<String, dynamic>) return d;
    } catch (_) {}
    return {'error': 'Unexpected response (${res.statusCode})'};
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
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString(_tokenKey, data['token'].toString());
      final staff = data['staff'] ?? data['warehouse_staff'];
      if (staff != null) await prefs.setString(_staffKey, jsonEncode(staff));
    }
    return data;
  }

  static Future<String?> getToken() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_tokenKey);
  }

  static Future<Map<String, dynamic>?> getSavedStaff() async {
    final prefs = await SharedPreferences.getInstance();
    final s = prefs.getString(_staffKey);
    if (s == null) return null;
    try {
      final d = jsonDecode(s);
      if (d is Map<String, dynamic>) return d;
    } catch (_) {}
    return null;
  }

  static Future<void> logout() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_tokenKey);
    await prefs.remove(_staffKey);
  }

  // ---------- authed helpers ----------
  static Future<Map<String, String>> _headers() async {
    final t = await getToken();
    return {
      'Content-Type': 'application/json',
      if (t != null) 'Authorization': 'Bearer $t',
    };
  }

  static Future<Map<String, dynamic>> _get(String path, [Map<String, String>? query]) async {
    final uri = Uri.parse('$baseUrl$path').replace(queryParameters: query);
    final res = await http.get(uri, headers: await _headers()).timeout(const Duration(seconds: 25));
    return _decode(res);
  }

  static Future<Map<String, dynamic>> _post(String path, Map<String, dynamic> body) async {
    final res = await http
        .post(Uri.parse('$baseUrl$path'), headers: await _headers(), body: jsonEncode(body))
        .timeout(const Duration(seconds: 25));
    return _decode(res);
  }

  static Future<Map<String, dynamic>> _put(String path, Map<String, dynamic> body) async {
    final res = await http
        .put(Uri.parse('$baseUrl$path'), headers: await _headers(), body: jsonEncode(body))
        .timeout(const Duration(seconds: 25));
    return _decode(res);
  }

  static Future<Map<String, dynamic>> _delete(String path) async {
    final res = await http
        .delete(Uri.parse('$baseUrl$path'), headers: await _headers())
        .timeout(const Duration(seconds: 25));
    return _decode(res);
  }

  // ---------- picker: stores, slots, presence ----------
  static Future<Map<String, dynamic>> getPickerStores({double? lat, double? lng, String? date}) {
    final q = <String, String>{
      if (lat != null && lng != null) 'lat': lat.toString(),
      if (lat != null && lng != null) 'lng': lng.toString(),
      'date': ?date,
    };
    return _get('/warehouse/picker/stores', q.isEmpty ? null : q);
  }

  static Future<Map<String, dynamic>> bookSlot(int slotId, String date) =>
      _post('/warehouse/picker/slots/$slotId/book', {'date': date});

  static Future<Map<String, dynamic>> getMyBookings() => _get('/warehouse/picker/bookings/me');

  static Future<Map<String, dynamic>> cancelBooking(int id) => _delete('/warehouse/picker/bookings/$id');

  static Future<Map<String, dynamic>> getPresence() => _get('/warehouse/picker/presence');

  static Future<Map<String, dynamic>> setPresence(bool online, String workflow) =>
      _put('/warehouse/picker/presence', {'online': online, 'workflow': workflow});

  // ---------- picker: tasks ----------
  static Future<Map<String, dynamic>> getPickerTask() => _get('/warehouse/picker/task');

  static Future<Map<String, dynamic>> getPickerSummary() => _get('/warehouse/picker/summary');
  static Future<Map<String, dynamic>> getPickerHistory() => _get('/warehouse/picker/history');
  static Future<Map<String, dynamic>> getPickerToday() => _get('/warehouse/picker/today');

  static Future<Map<String, dynamic>> getPickerPayout(String range, String date) => _get('/warehouse/picker/payout?range=$range&date=$date');

  static Future<Map<String, dynamic>> getPickingTask(int orderId) => _get('/warehouse/picking/$orderId');

  static Future<Map<String, dynamic>> startPicking(int orderId) =>
      _put('/warehouse/picking/$orderId/start', {});

  static Future<Map<String, dynamic>> markPickItem(int itemId, String status, {int? quantityPicked, String? reason}) =>
      _put('/warehouse/picking/items/$itemId', {
        'status': status,
        'quantity_picked': ?quantityPicked,
        'reason': ?reason,
      });

  static Future<Map<String, dynamic>> completePicking(int orderId) =>
      _put('/warehouse/picking/$orderId/complete', {});

  static Future<Map<String, dynamic>> pickerHandover(int orderId, {int packageCount = 1}) =>
      _post('/warehouse/picker/orders/$orderId/handover', {'package_count': packageCount});
}