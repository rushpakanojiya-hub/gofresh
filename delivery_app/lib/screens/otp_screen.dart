import 'package:flutter/material.dart';
import '../services/api_service.dart';
import '../services/location_service.dart';
import 'home_shell.dart';
import 'onboarding_flow.dart';
import '../services/push_service.dart';

class OtpScreen extends StatefulWidget {
  final String phone;
  final String? testOtp;
  const OtpScreen({super.key, required this.phone, this.testOtp});

  @override
  State<OtpScreen> createState() => _OtpScreenState();
}

class _OtpScreenState extends State<OtpScreen> {
  final _otpController = TextEditingController();
  bool _loading = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    if (widget.testOtp != null && widget.testOtp!.isNotEmpty) {
      _otpController.text = widget.testOtp!;
    }
  }

  Future<void> _verifyOtp() async {
    final otp = _otpController.text.trim();
    if (otp.length < 4) {
      setState(() => _error = 'Enter a valid OTP');
      return;
    }
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final data = await ApiService.verifyOtp(widget.phone, otp);
      setState(() => _loading = false);
      if (data['token'] != null) {
        final partner = data['delivery_partner'];
        final status = (partner is Map ? partner['approval_status'] : null)?.toString() ?? 'approved';
        if (!mounted) return;
        PushService.registerCurrentToken();
        if (status == 'approved') {
          LocationService.startTracking();
          Navigator.pushAndRemoveUntil(
            context,
            MaterialPageRoute(builder: (_) => const HomeShell()),
            (route) => false,
          );
        } else {
          Navigator.pushAndRemoveUntil(
            context,
            MaterialPageRoute(builder: (_) => OnboardingFlow(initial: Map<String, dynamic>.from(partner as Map))),
            (route) => false,
          );
        }
      } else {
        setState(() => _error = data['error'] ?? 'Invalid OTP');
      }
    } catch (e) {
      setState(() {
        _loading = false;
        _error = 'Network error. Please try again.';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Verify OTP')),
      body: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              'Enter the OTP sent to ${widget.phone}',
              textAlign: TextAlign.center,
              style: const TextStyle(fontSize: 16),
            ),
            if (widget.testOtp != null && widget.testOtp!.isNotEmpty) ...[
              const SizedBox(height: 8),
              Text(
                '(Test mode — auto-filled)',
                textAlign: TextAlign.center,
                style: TextStyle(fontSize: 12, color: Colors.grey[600]),
              ),
            ],
            const SizedBox(height: 16),
            TextField(
              controller: _otpController,
              keyboardType: TextInputType.number,
              maxLength: 6,
              decoration: const InputDecoration(
                labelText: 'OTP',
                border: OutlineInputBorder(),
                counterText: '',
              ),
            ),
            if (_error != null) ...[
              const SizedBox(height: 8),
              Text(_error!, style: const TextStyle(color: Colors.red)),
            ],
            const SizedBox(height: 16),
            ElevatedButton(
              onPressed: _loading ? null : _verifyOtp,
              style: ElevatedButton.styleFrom(padding: const EdgeInsets.all(16)),
              child: _loading
                  ? const SizedBox(
                      height: 20,
                      width: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Text('Verify & Login'),
            ),
          ],
        ),
      ),
    );
  }
}

