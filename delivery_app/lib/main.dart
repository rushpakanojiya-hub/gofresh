import 'package:flutter/material.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'services/push_service.dart';
import 'services/api_service.dart';
import 'services/location_service.dart';
import 'screens/login_screen.dart';
import 'screens/home_shell.dart';
import 'screens/onboarding_flow.dart';

/// Background/killed isolate entry point. Must be a top-level function.
@pragma('vm:entry-point')
Future<void> _firebaseMessagingBackgroundHandler(RemoteMessage message) async {
  if (Firebase.apps.isEmpty) await Firebase.initializeApp();
}

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  FirebaseMessaging.onBackgroundMessage(_firebaseMessagingBackgroundHandler);
  await LocationService.initialize();
  runApp(const MyApp());
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});
  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      navigatorKey: PushService.navigatorKey,
      title: 'Delivery Partner',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: Colors.deepPurple),
        useMaterial3: true,
      ),
      home: const SplashDecider(),
    );
  }
}

class SplashDecider extends StatefulWidget {
  const SplashDecider({super.key});
  @override
  State<SplashDecider> createState() => _SplashDeciderState();
}

class _SplashDeciderState extends State<SplashDecider> {
  @override
  void initState() {
    super.initState();
    _checkLogin();
  }

  Future<void> _checkLogin() async {
    final token = await ApiService.getToken();
    await Future.delayed(const Duration(milliseconds: 300));
    if (!mounted) return;
    if (token != null) {
      Map<String, dynamic>? ob;
      try {
        ob = await ApiService.getOnboarding();
      } catch (_) {}
      if (!mounted) return;
      final status = ob?['approval_status']?.toString() ?? 'approved';
      if (ob != null && status != 'approved') {
        final initial = ob;
        Navigator.pushReplacement(
          context,
          MaterialPageRoute(builder: (_) => OnboardingFlow(initial: initial)),
        );
        return;
      }
      LocationService.startTracking();
      Navigator.pushReplacement(
        context,
        MaterialPageRoute(builder: (_) => const HomeShell()),
      );
    } else {
      Navigator.pushReplacement(
        context,
        MaterialPageRoute(builder: (_) => const LoginScreen()),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return const Scaffold(
      body: Center(child: CircularProgressIndicator()),
    );
  }
}
