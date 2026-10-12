import 'dart:async';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'api_service.dart';

class PushService {
  static bool _ready = false;
  static StreamSubscription<String>? _refreshSub;

  static Future<void> init() async {
    try {
      await Firebase.initializeApp();
      const ch = AndroidNotificationChannel('putter_channel', 'Putter alerts', importance: Importance.high);
      await FlutterLocalNotificationsPlugin().resolvePlatformSpecificImplementation<AndroidFlutterLocalNotificationsPlugin>()?.createNotificationChannel(ch);
      _ready = true;
    } catch (_) {
      _ready = false;
    }
  }

  static Future<void> register() async {
    if (!_ready) return;
    try {
      final m = FirebaseMessaging.instance;
      await m.requestPermission();
      final t = await m.getToken();
      if (t == null) return;
      await ApiService.registerDeviceToken(t);
      _refreshSub?.cancel();
      _refreshSub = m.onTokenRefresh.listen((n) {
        ApiService.registerDeviceToken(n);
      });
    } catch (_) {}
  }

  static Future<void> unregister() async {
    if (!_ready) return;
    try {
      _refreshSub?.cancel();
      _refreshSub = null;
      final t = await FirebaseMessaging.instance.getToken();
      if (t != null) await ApiService.unregisterDeviceToken(t);
    } catch (_) {}
  }
}