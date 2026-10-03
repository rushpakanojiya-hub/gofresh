import 'package:camera/camera.dart';
import 'package:flutter/material.dart';

/// Always opens the FRONT camera and returns the captured photo path.
class SelfieCameraScreen extends StatefulWidget {
  const SelfieCameraScreen({super.key});

  @override
  State<SelfieCameraScreen> createState() => _SelfieCameraScreenState();
}

class _SelfieCameraScreenState extends State<SelfieCameraScreen> {
  CameraController? _c;
  String? _error;
  bool _taking = false;

  @override
  void initState() {
    super.initState();
    _init();
  }

  Future<void> _init() async {
    try {
      final cams = await availableCameras();
      final front = cams.where((c) => c.lensDirection == CameraLensDirection.front);
      if (front.isEmpty) {
        setState(() => _error = 'No front camera found on this device');
        return;
      }
      final c = CameraController(front.first, ResolutionPreset.high, enableAudio: false);
      await c.initialize();
      if (!mounted) {
        await c.dispose();
        return;
      }
      setState(() => _c = c);
    } catch (e) {
      if (mounted) setState(() => _error = 'Could not open camera. Please allow camera permission.');
    }
  }

  @override
  void dispose() {
    _c?.dispose();
    super.dispose();
  }

  Future<void> _snap() async {
    final c = _c;
    if (c == null || _taking) return;
    setState(() => _taking = true);
    try {
      final x = await c.takePicture();
      if (!mounted) return;
      Navigator.pop(context, x.path);
    } catch (_) {
      if (mounted) setState(() => _taking = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = _c;
    return Scaffold(
      backgroundColor: Colors.black,
      body: SafeArea(
        child: _error != null
            ? Center(
                child: Padding(
                  padding: const EdgeInsets.all(24),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(_error!,
                          textAlign: TextAlign.center,
                          style: const TextStyle(color: Colors.white)),
                      const SizedBox(height: 16),
                      ElevatedButton(
                        onPressed: () => Navigator.pop(context),
                        child: const Text('Back'),
                      ),
                    ],
                  ),
                ),
              )
            : (c == null || !c.value.isInitialized)
                ? const Center(child: CircularProgressIndicator())
                : Column(
                    children: [
                      Expanded(child: Center(child: CameraPreview(c))),
                      Padding(
                        padding: const EdgeInsets.symmetric(vertical: 20, horizontal: 24),
                        child: Row(
                          mainAxisAlignment: MainAxisAlignment.spaceBetween,
                          children: [
                            TextButton(
                              onPressed: () => Navigator.pop(context),
                              child: const Text('Cancel', style: TextStyle(color: Colors.white)),
                            ),
                            GestureDetector(
                              onTap: _snap,
                              child: Container(
                                width: 72,
                                height: 72,
                                decoration: BoxDecoration(
                                  shape: BoxShape.circle,
                                  color: _taking ? Colors.white54 : Colors.white,
                                  border: Border.all(color: Colors.white70, width: 4),
                                ),
                              ),
                            ),
                            const SizedBox(width: 64),
                          ],
                        ),
                      ),
                    ],
                  ),
      ),
    );
  }
}