import 'dart:async';

import 'package:flutter_background_service/flutter_background_service.dart';
import 'package:geolocator/geolocator.dart';

import 'api_service.dart';

/// Handles continuous GPS reporting for the delivery partner, even while
/// the app is backgrounded or the screen is off. Location is pushed via a
/// long-running foreground service (Android) rather than a plain
/// Timer.periodic in the UI isolate, since a UI-isolate timer is paused by
/// the OS as soon as the app leaves the foreground - which was silently
/// starving most partners of fresh location data and making them
/// ineligible for auto-assignment.
class LocationService {
  static Future<bool> requestPermission() async {
    bool serviceEnabled = await Geolocator.isLocationServiceEnabled();
    if (!serviceEnabled) return false;

    LocationPermission permission = await Geolocator.checkPermission();
    if (permission == LocationPermission.denied) {
      permission = await Geolocator.requestPermission();
      if (permission == LocationPermission.denied) return false;
    }
    if (permission == LocationPermission.deniedForever) return false;

    // Background location is a separate grant on Android 10+ - fine/coarse
    // location alone is not enough to keep receiving updates once the app
    // is no longer in the foreground. Requesting it is safe to call even
    // if it's already granted, or if the platform doesn't distinguish it.
    if (permission != LocationPermission.always) {
      await Geolocator.requestPermission();
    }

    return true;
  }

  /// One-time setup - must be called before startTracking(), typically
  /// once from main().
  static Future<void> initialize() async {
    final service = FlutterBackgroundService();
    await service.configure(
      androidConfiguration: AndroidConfiguration(
        onStart: onServiceStart,
        autoStart: false,
        isForegroundMode: true,
        initialNotificationTitle: 'Delivery Partner',
        initialNotificationContent: 'Tracking your location while online',
        foregroundServiceNotificationId: 9001,
        foregroundServiceTypes: [AndroidForegroundType.location],
      ),
      iosConfiguration: IosConfiguration(
        autoStart: false,
        onForeground: onServiceStart,
        onBackground: onIosBackground,
      ),
    );
  }

  static Future<void> startTracking() async {
    final hasPermission = await requestPermission();
    if (!hasPermission) return;
    final service = FlutterBackgroundService();
    final running = await service.isRunning();
    if (!running) {
      await service.startService();
    }
  }

  static void stopTracking() {
    FlutterBackgroundService().invoke('stopService');
  }
}

// These two callbacks MUST be top-level (not class members) functions for
// flutter_background_service's native callback-handle registration to find
// them reliably - a static class method compiles to a different kind of
// tear-off that the plugin's isolate-spawn lookup cannot always resolve,
// which is what caused the "must be annotated" native crash.
@pragma('vm:entry-point')
void onServiceStart(ServiceInstance service) async {
  if (service is AndroidServiceInstance) {
    service.setAsForegroundService();
  }

  service.on('stopService').listen((event) {
    service.stopSelf();
  });

  Timer.periodic(const Duration(seconds: 20), (timer) async {
    try {
      final pos = await Geolocator.getCurrentPosition(
        locationSettings: const LocationSettings(accuracy: LocationAccuracy.high),
      );
      await ApiService.updateLocation(pos.latitude, pos.longitude);
    } catch (_) {
      // Silently skip a failed location update; next timer tick will retry.
    }
  });
}

@pragma('vm:entry-point')
bool onIosBackground(ServiceInstance service) {
  return true;
}
