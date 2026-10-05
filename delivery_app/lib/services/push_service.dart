import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/material.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import '../screens/new_order_sheet.dart';
import '../screens/order_pickup_screen.dart';
import '../screens/delivery_map_screen.dart';
import '../screens/delivery_handover_screen.dart';
import '../screens/delivery_complete_screen.dart';
import 'api_service.dart';

/// FCM setup for the delivery partner app: permission, token registration,
/// foreground display, and tap routing (foreground / background / killed).
class PushService {
  /// HomeShell listens to this. 1 = Orders tab, 3 = Profile tab.
  static final ValueNotifier<int?> tabRequest = ValueNotifier<int?>(null);

  /// Passed to MaterialApp so a notification tap can open screens.
  static final GlobalKey<NavigatorState> navigatorKey = GlobalKey<NavigatorState>();

  /// ProfileScreen listens to this and reloads (new rating arrived).
  static final ValueNotifier<int> ratingRefresh = ValueNotifier<int>(0);

  static final FlutterLocalNotificationsPlugin _local =
      FlutterLocalNotificationsPlugin();
  static bool _started = false;

  static const AndroidNotificationChannel _channel = AndroidNotificationChannel(
    'delivery_channel',
    'Delivery updates',
    description: 'New orders and customer ratings',
    importance: Importance.high,
  );

  /// Local-notification payload format: "type|order_id".
  static void _onTap(String? payload) {
    final parts = (payload ?? '').split('|');
    _handle(parts.isNotEmpty ? parts[0] : null, parts.length > 1 ? parts[1] : null);
  }

  static void _handle(String? type, String? orderId) {
    if (type == 'rating') {
      ratingRefresh.value++;
      openOrder(orderId, fallbackTab: 3);
      return;
    }
    if (type == 'new_assignment') {
      openOrder(orderId, fallbackTab: 1);
      return;
    }
    _route(type);
  }

  static bool sheetOpen = false;
  static final Set<int> popupShown = {};

  /// Foreground "new_assignment": show the half-screen accept popup.
  static Future<void> _showNewOrder(String? orderId) async {
    final id = int.tryParse(orderId ?? '');
    final ctx = navigatorKey.currentContext;
    if (id == null || ctx == null || sheetOpen || popupShown.contains(id)) return;
    try {
      dynamic item;
      for (var attempt = 0; attempt < 4 && item == null; attempt++) {
        if (attempt > 0) await Future.delayed(const Duration(milliseconds: 1500));
        final list = await ApiService.getMyDeliveries();
        item = list.firstWhere(
          (o) => o is Map && '${o['order_id'] ?? o['id']}' == '$id',
          orElse: () => null,
        );
      }
      final ctx2 = navigatorKey.currentContext;
      if (item == null || ctx2 == null || !ctx2.mounted) return;
      sheetOpen = true;
      popupShown.add(id);
      await NewOrderSheet.show(ctx2, Map<String, dynamic>.from(item as Map));
    } catch (e) {
      debugPrint('new order popup failed: $e');
    } finally {
      sheetOpen = false;
    }
  }
  static Future<void> openOrder(String? orderId, {int fallbackTab = 1}) async {
    final id = int.tryParse(orderId ?? '');
    final nav = navigatorKey.currentState;
    if (id == null || nav == null) {
      tabRequest.value = fallbackTab;
      return;
    }
    try {
      final list = await ApiService.getMyDeliveries();
      final item = list.firstWhere(
        (o) => o is Map && '${o['order_id'] ?? o['id']}' == '$id',
        orElse: () => null,
      );
      Widget? next;
      if (item is Map) {
        final m = Map<String, dynamic>.from(item);
        final ds = m['delivery_status']?.toString();
        if (ds == 'accepted' || ds == 'going_to_store' || ds == 'arrived_at_store') {
          next = OrderPickupScreen(order: m);
        } else if (ds == 'picked_up' || ds == 'out_for_delivery') {
          next = DeliveryMapScreen(order: m);
        } else if (ds == 'arrived' || ds == 'arrived_at_customer') {
          next = DeliveryHandoverScreen(order: m);
        } else if (m['status']?.toString() == 'delivered') {
          next = DeliveryCompleteScreen(order: m);
        } else if (ds == 'delivered') {
          next = DeliveryHandoverScreen(order: m);
        }
      }
      if (next == null) {
        nav.popUntil((r) => r.isFirst);
        tabRequest.value = fallbackTab;
        return;
      }
      final screen = next;
      nav.push(MaterialPageRoute(builder: (_) => screen));
    } catch (e) {
      debugPrint('openOrder failed: $e');
      tabRequest.value = fallbackTab;
    }
  }

  static void _route(String? type) {
    if (type == 'new_assignment') {
      tabRequest.value = 1;
    } else if (type == 'rating') {
      tabRequest.value = 3;
    }
  }

  /// Call this right after login succeeds (token is saved). Re-sends the
  /// current FCM token with a valid JWT, in case start() already ran
  /// before login (e.g. app reopened straight into HomeShell).
  static Future<void> registerCurrentToken() async {
    try {
      if (Firebase.apps.isEmpty) await Firebase.initializeApp();
      final token = await FirebaseMessaging.instance.getToken();
      if (token != null) await ApiService.registerDeviceToken(token);
    } catch (e) {
      debugPrint('registerCurrentToken failed: $e');
    }
  }

  static Future<void> start() async {
    if (_started) return;
    _started = true;
    try {
      if (Firebase.apps.isEmpty) await Firebase.initializeApp();

      await _local.initialize(
        const InitializationSettings(
          android: AndroidInitializationSettings('@mipmap/ic_launcher'),
        ),
        onDidReceiveNotificationResponse: (r) => _onTap(r.payload),
      );
      final android = _local.resolvePlatformSpecificImplementation<AndroidFlutterLocalNotificationsPlugin>();
      await android?.createNotificationChannel(_channel);
      await android?.requestNotificationsPermission();

      final messaging = FirebaseMessaging.instance;
      await messaging.requestPermission();

      final token = await messaging.getToken();
      if (token != null) await ApiService.registerDeviceToken(token);
      messaging.onTokenRefresh.listen(ApiService.registerDeviceToken);

      FirebaseMessaging.onMessage.listen(_onForeground);
      FirebaseMessaging.onMessageOpenedApp
          .listen((m) => _handle(m.data['type']?.toString(), m.data['order_id']?.toString()));

      final initial = await messaging.getInitialMessage();
      if (initial != null) {
        final t = initial.data['type']?.toString();
        final o = initial.data['order_id']?.toString();
        Future.delayed(const Duration(milliseconds: 800), () => _handle(t, o));
      }
    } catch (e) {
      debugPrint('Push init failed: $e');
    }
  }

  static void _onForeground(RemoteMessage msg) {
    if (msg.data['type'] == 'rating') ratingRefresh.value++;
    if (msg.data['type'] == 'new_assignment') {
      _showNewOrder(msg.data['order_id']?.toString());
      return;
    }
    final n = msg.notification;
    if (n == null) return;
    _local.show(
      msg.hashCode,
      n.title,
      n.body,
      const NotificationDetails(
        android: AndroidNotificationDetails(
          'delivery_channel',
          'Delivery updates',
          importance: Importance.high,
          priority: Priority.high,
        ),
      ),
      payload: '${msg.data['type'] ?? ''}|${msg.data['order_id'] ?? ''}',
    );
  }
}