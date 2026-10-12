import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import '../services/api_service.dart';
import '../services/push_service.dart';
import 'shell_screen.dart';

class OtpScreen extends StatefulWidget {
  final String phone;
  final String? testOtp;
  const OtpScreen({super.key, required this.phone, this.testOtp});

  @override
  State<OtpScreen> createState() => _OtpScreenState();
}

class _OtpScreenState extends State<OtpScreen> {
  final _otp = TextEditingController();
  bool _loading = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    final t = widget.testOtp;
    if (t != null && t.isNotEmpty) _otp.text = t;
  }

  @override
  void dispose() {
    _otp.dispose();
    super.dispose();
  }

  Future<void> _verify() async {
    final code = _otp.text.trim();
    if (code.length < 4) {
      setState(() => _error = 'Enter a valid OTP');
      return;
    }
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final data = await ApiService.verifyOtp(widget.phone, code);
      if (!mounted) return;
      setState(() => _loading = false);
      if (data['token'] != null) {
        PushService.register();
        Navigator.pushAndRemoveUntil(
          context,
          MaterialPageRoute(builder: (_) => const ShellScreen()),
          (route) => false,
        );
      } else {
        setState(() => _error = data['error']?.toString() ?? 'Invalid OTP');
      }
    } catch (_) {
      if (!mounted) return;
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
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            const SizedBox(height: 40),
            Text('Enter the OTP sent to +91 ${widget.phone}', style: const TextStyle(fontSize: 16)),
            const SizedBox(height: 20),
            TextField(
              controller: _otp,
              keyboardType: TextInputType.number,
              maxLength: 6,
              autofocus: true,
              textAlign: TextAlign.center,
              inputFormatters: [FilteringTextInputFormatter.digitsOnly],
              style: const TextStyle(fontSize: 28, fontWeight: FontWeight.w700, letterSpacing: 8),
              decoration: InputDecoration(
                counterText: '',
                border: OutlineInputBorder(borderRadius: BorderRadius.circular(12)),
              ),
              onChanged: (_) {
                if (_error != null) setState(() => _error = null);
              },
            ),
            if (_error != null)
              Padding(
                padding: const EdgeInsets.only(top: 10),
                child: Text(_error!, style: const TextStyle(color: Colors.red)),
              ),
            const SizedBox(height: 20),
            ElevatedButton(
              onPressed: _loading ? null : _verify,
              style: ElevatedButton.styleFrom(
                backgroundColor: Colors.black,
                foregroundColor: Colors.white,
                padding: const EdgeInsets.symmetric(vertical: 16),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
              ),
              child: _loading
                  ? const SizedBox(
                      height: 20,
                      width: 20,
                      child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                    )
                  : const Text('Verify', style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700)),
            ),
          ],
        ),
      ),
    );
  }
}