import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';

class ApiService {
  static const String baseUrl = 'https://gofresh-evl7.onrender.com/api/v1';

  static Future<String?> getToken() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString('token');
  }

  static Future<void> saveToken(String token) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('token', token);
  }

  static Future<void> clearToken() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove('token');
  }

  static Future<Map<String, String>> _headers({bool auth = true}) async {
    final headers = {'Content-Type': 'application/json'};
    if (auth) {
      final token = await getToken();
      if (token != null) headers['Authorization'] = 'Bearer $token';
    }
    return headers;
  }

  static Future<Map<String, dynamic>> sendOtp(String phone) async {
    final res = await http.post(
      Uri.parse('$baseUrl/delivery/send-otp'),
      headers: await _headers(auth: false),
      body: jsonEncode({'phone': phone}),
    );
    return jsonDecode(res.body);
  }

  static Future<Map<String, dynamic>> verifyOtp(String phone, String otp) async {
    final res = await http.post(
      Uri.parse('$baseUrl/delivery/verify-otp'),
      headers: await _headers(auth: false),
      body: jsonEncode({'phone': phone, 'otp': otp}),
    );
    final data = jsonDecode(res.body);
    if (res.statusCode == 200 && data['token'] != null) {
      await saveToken(data['token']);
    }
    return data;
  }

  static Future<List<dynamic>> getMyDeliveries({String? status}) async {
    var url = '$baseUrl/delivery/orders';
    if (status != null) url += '?status=$status';
    final res = await http.get(Uri.parse(url), headers: await _headers());
    final data = jsonDecode(res.body);
    if (res.statusCode == 200) return data['orders'] ?? [];
    throw Exception(data['error'] ?? 'Failed to load orders');
  }

  static Future<Map<String, dynamic>> markShipped(int orderId) async {
    final res = await http.put(
      Uri.parse('$baseUrl/delivery/orders/$orderId/status'),
      headers: await _headers(),
      body: jsonEncode({'status': 'shipped'}),
    );
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to update status');
    return data;
  }

  static Future<Map<String, dynamic>> updateDeliveryStatus(
    int orderId,
    String status, {
    String? otp,
  }) async {
    final body = <String, dynamic>{'status': status};
    if (otp != null) body['otp'] = otp;
    final res = await http.put(
      Uri.parse('$baseUrl/delivery/orders/$orderId/delivery-status'),
      headers: await _headers(),
      body: jsonEncode(body),
    );
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) {
      throw Exception(data['error'] ?? 'Failed to update delivery status');
    }
    return data;
  }

  static Future<Map<String, dynamic>> confirmDelivery(int orderId) async {
    final res = await http.put(
      Uri.parse('$baseUrl/delivery/orders/$orderId/deliver'),
      headers: await _headers(),
    );
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to confirm delivery');
    return data;
  }

  static Future<Map<String, dynamic>> acceptAssignment(int orderId) async {
    final res = await http.put(
      Uri.parse('$baseUrl/delivery/orders/$orderId/accept'),
      headers: await _headers(),
    );
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to accept delivery');
    return data;
  }

  static Future<Map<String, dynamic>> rejectAssignment(int orderId, {String? reason}) async {
    final res = await http.put(
      Uri.parse('$baseUrl/delivery/orders/$orderId/reject'),
      headers: await _headers(),
      body: jsonEncode(
        (reason != null && reason.trim().isNotEmpty) ? {'reason': reason.trim()} : {},
      ),
    );
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to reject delivery');
    return data;
  }

  static Future<void> updateLocation(double lat, double lng) async {
    await http.put(
      Uri.parse('$baseUrl/delivery/location'),
      headers: await _headers(),
      body: jsonEncode({'lat': lat, 'lng': lng}),
    );
  }

  static Future<Map<String, dynamic>> getAvailability() async {
    final res = await http.get(Uri.parse('$baseUrl/delivery/availability'), headers: await _headers());
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to load availability');
    return data;
  }

  static Future<Map<String, dynamic>> updateAvailability(bool isOnline) async {
    final res = await http.put(
      Uri.parse('$baseUrl/delivery/availability'),
      headers: await _headers(),
      body: jsonEncode({'status': isOnline ? 'online' : 'offline'}),
    );
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to update availability');
    return data;
  }

  static Future<Map<String, dynamic>> getEarnings() async {
    final res = await http.get(Uri.parse('$baseUrl/delivery/earnings'), headers: await _headers());
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to load earnings');
    return data;
  }

  static Future<Map<String, dynamic>> getCODSummary() async {
    final res = await http.get(Uri.parse('$baseUrl/delivery/cod-summary'), headers: await _headers());
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to load COD summary');
    return data;
  }

  static Future<List<dynamic>> getCODSettlements() async {
    final res = await http.get(Uri.parse('$baseUrl/delivery/cod-settlements'), headers: await _headers());
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to load settlement history');
    return data['settlements'] ?? [];
  }

  static Future<Map<String, dynamic>> getNotifications({bool unreadOnly = false}) async {
    var url = '$baseUrl/delivery/notifications';
    if (unreadOnly) url += '?unread_only=true';
    final res = await http.get(Uri.parse(url), headers: await _headers());
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to load notifications');
    return data;
  }

  static Future<void> markNotificationRead(int notificationId) async {
    await http.put(
      Uri.parse('$baseUrl/delivery/notifications/$notificationId/read'),
      headers: await _headers(),
    );
  }

  static Future<void> markAllNotificationsRead() async {
    await http.put(
      Uri.parse('$baseUrl/delivery/notifications/read-all'),
      headers: await _headers(),
    );
  }

  static Future<Map<String, dynamic>> uploadDeliveryProof(int orderId, String imagePath) async {
    final uri = Uri.parse('$baseUrl/delivery/orders/$orderId/delivery-proof');
    final request = http.MultipartRequest('PUT', uri);
    final token = await getToken();
    if (token != null) request.headers['Authorization'] = 'Bearer $token';
    request.files.add(await http.MultipartFile.fromPath('image', imagePath));
    final streamedRes = await request.send();
    final res = await http.Response.fromStream(streamedRes);
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) {
      throw Exception(data['error'] ?? 'Failed to upload delivery proof');
    }
    return data;
  }

  static Future<Map<String, dynamic>> resolveFailedDelivery(
    int orderId,
    String action,
    String reason,
  ) async {
    final res = await http.put(
      Uri.parse('$baseUrl/delivery/orders/$orderId/resolve-failed'),
      headers: await _headers(),
      body: jsonEncode({'action': action, 'reason': reason}),
    );
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to resolve delivery');
    return data;
  }



  static Future<Map<String, dynamic>> getProfile() async {
    final res = await http.get(Uri.parse('$baseUrl/delivery/profile'), headers: await _headers());
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to load profile');
    return data;
  }

  static Future<Map<String, dynamic>> getOrderRating(int orderId) async {
    final res = await http.get(Uri.parse('$baseUrl/delivery/orders/$orderId/rating'), headers: await _headers());
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to load rating');
    return data;
  }

  static Future<void> registerDeviceToken(String fcmToken) async {
    try {
      await http.post(
        Uri.parse('$baseUrl/device-token'),
        headers: await _headers(),
        body: jsonEncode({'token': fcmToken, 'platform': 'android'}),
      );
    } catch (_) {}
  }

  static Future<List<dynamic>> getMyRatings() async {
    final res = await http.get(Uri.parse('$baseUrl/delivery/ratings'), headers: await _headers());
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to load ratings');
    return (data['ratings'] as List?) ?? [];
  }

  static Future<Map<String, dynamic>> getOnboarding() async {
    final res = await http.get(Uri.parse('$baseUrl/delivery/onboarding'), headers: await _headers());
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to load onboarding');
    return Map<String, dynamic>.from(data as Map);
  }

  static Future<List<dynamic>> getStores() async {
    final res = await http.get(Uri.parse('$baseUrl/delivery/stores'), headers: await _headers());
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to load stores');
    return (data['stores'] as List?) ?? [];
  }

  static Future<Map<String, dynamic>> saveOnboarding(String path, Map<String, dynamic> body) async {
    final res = await http.put(
      Uri.parse('$baseUrl/delivery/onboarding/$path'),
      headers: await _headers(),
      body: jsonEncode(body),
    );
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to save');
    return Map<String, dynamic>.from(data as Map);
  }

  static Future<String> uploadOnboardingImage(String imagePath) async {
    final token = await getToken();
    final req = http.MultipartRequest('POST', Uri.parse('$baseUrl/delivery/onboarding/upload'));
    if (token != null) req.headers['Authorization'] = 'Bearer $token';
    req.files.add(await http.MultipartFile.fromPath('image', imagePath));
    final streamed = await req.send();
    final res = await http.Response.fromStream(streamed);
    final data = jsonDecode(res.body);
    if (res.statusCode != 200 && res.statusCode != 201) throw Exception(data['error'] ?? 'Upload failed');
    return data['image_url'].toString();
  }

  static Future<Map<String, dynamic>> getMyRating() async {
    final res = await http.get(Uri.parse('$baseUrl/delivery/rating'), headers: await _headers());
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to load rating');
    return data;
  }

  static Future<Map<String, dynamic>> updateProfile(Map<String, dynamic> fields) async {
    final res = await http.put(
      Uri.parse('$baseUrl/delivery/profile'),
      headers: await _headers(),
      body: jsonEncode(fields),
    );
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to update profile');
    return data;
  }

  static Future<List<dynamic>> getMyReturnPickups({String? pickupStatus}) async {
    var url = '$baseUrl/delivery/returns';
    if (pickupStatus != null) url += '?pickup_status=$pickupStatus';
    final res = await http.get(Uri.parse(url), headers: await _headers());
    final data = jsonDecode(res.body);
    if (res.statusCode == 200) return data['returns'] ?? [];
    throw Exception(data['error'] ?? 'Failed to load return pickups');
  }

  static Future<Map<String, dynamic>> acceptReturnPickup(int returnRequestId) async {
    final res = await http.put(
      Uri.parse('$baseUrl/delivery/returns/$returnRequestId/accept'),
      headers: await _headers(),
    );
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to accept pickup');
    return data;
  }

  static Future<Map<String, dynamic>> rejectReturnPickup(int returnRequestId) async {
    final res = await http.put(
      Uri.parse('$baseUrl/delivery/returns/$returnRequestId/reject'),
      headers: await _headers(),
    );
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to reject pickup');
    return data;
  }

  static Future<Map<String, dynamic>> markReturnPickupEnRoute(int returnRequestId) async {
    final res = await http.put(
      Uri.parse('$baseUrl/delivery/returns/$returnRequestId/en-route'),
      headers: await _headers(),
    );
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to update status');
    return data;
  }

  static Future<Map<String, dynamic>> markReturnPickupArrived(int returnRequestId) async {
    final res = await http.put(
      Uri.parse('$baseUrl/delivery/returns/$returnRequestId/arrived'),
      headers: await _headers(),
    );
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to update status');
    return data;
  }

  static Future<Map<String, dynamic>> confirmReturnPickedUp(
    int returnRequestId,
    String conditionPhotoUrl, {
    String? conditionNotes,
  }) async {
    final res = await http.put(
      Uri.parse('$baseUrl/delivery/returns/$returnRequestId/picked-up'),
      headers: await _headers(),
      body: jsonEncode({
        'condition_photo_url': conditionPhotoUrl,
        'condition_notes': ?conditionNotes,
      }),
    );
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to confirm pickup');
    return data;
  }

  static Future<Map<String, dynamic>> handoverReturnToWarehouse(int returnRequestId) async {
    final res = await http.put(
      Uri.parse('$baseUrl/delivery/returns/$returnRequestId/handover'),
      headers: await _headers(),
    );
    final data = jsonDecode(res.body);
    if (res.statusCode != 200) throw Exception(data['error'] ?? 'Failed to confirm handover');
    return data;
  }

  static Future<String> uploadReturnPickupPhoto(String imagePath) async {
    final token = await getToken();
    final request = http.MultipartRequest('POST', Uri.parse('$baseUrl/delivery/upload'));
    if (token != null) request.headers['Authorization'] = 'Bearer $token';
    request.files.add(await http.MultipartFile.fromPath('image', imagePath));
    final streamedRes = await request.send();
    final res = await http.Response.fromStream(streamedRes);
    final data = jsonDecode(res.body);
    if (res.statusCode != 200 && res.statusCode != 201) {
      throw Exception(data['error'] ?? 'Failed to upload photo');
    }
    return data['image_url'] as String;
  }
}
