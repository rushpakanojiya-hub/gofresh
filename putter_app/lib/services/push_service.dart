import 'dart:async';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:flutter/material.dart';
import '../screens/putaway_task_screen.dart';
import '../theme.dart';
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
      FirebaseMessaging.onMessageOpenedApp.listen(_open);
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
  static final GlobalKey<NavigatorState> navKey = GlobalKey<NavigatorState>();

  static Future<void> _open(RemoteMessage m) async {
    try {
      if (m.data['type'] != 'putaway_assigned') return;
      final id = int.tryParse('${m.data['task_id'] ?? ''}');
      if (id == null) return;
      if (await ApiService.getToken() == null) return;
      bool on = false;
      try {
        final d = await ApiService.getDashboard();
        on = asMap(d['shift'])['active'] == true;
      } catch (_) {}
      navKey.currentState?.push(
        MaterialPageRoute(builder: (_) => PutawayTaskScreen(taskId: id, shiftActive: on)),
      );
    } catch (_) {}
  }

  static Future<void> handleInitial() async {
    if (!_ready) return;
    try {
      final m = await FirebaseMessaging.instance.getInitialMessage();
      if (m != null) {
        await Future.delayed(const Duration(milliseconds: 600));
        await _open(m);
      }
    } catch (_) {}
  }
}