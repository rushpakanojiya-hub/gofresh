import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import '../services/api_service.dart';
import 'home_screen.dart';

class OtpScreen extends StatefulWidget {
  final String phone;
  final String? testOtp;
  const OtpScreen({super.key, required this.phone, this.testOtp});

  @override
  State<OtpScreen> createState() => _OtpScreenState();
}

class _OtpScreenState extends State<OtpScreen> {
  static const int _otpLength = 6;
  final _otpController = TextEditingController();
  final _focusNode = FocusNode();
  bool _loading = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    if (widget.testOtp != null && widget.testOtp!.isNotEmpty) {
      _otpController.text = widget.testOtp!;
    }
    _otpController.addListener(() {
      if (mounted) setState(() {});
    });
    _focusNode.addListener(() {
      if (mounted) setState(() {});
    });
  }

  @override
  void dispose() {
    _otpController.dispose();
    _focusNode.dispose();
    super.dispose();
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
        if (!mounted) return;
        Navigator.pushAndRemoveUntil(
          context,
          MaterialPageRoute(builder: (_) => const HomeScreen()),
          (route) => false,
        );
      } else {
        setState(() => _error = data['error']?.toString() ?? 'Invalid OTP');
      }
    } catch (e) {
      setState(() {
        _loading = false;
        _error = 'Network error. Please try again.';
      });
    }
  }

  Widget _buildOtpBoxes() {
    final text = _otpController.text;
    final activeIndex = text.length < _otpLength ? text.length : _otpLength - 1;
    final primary = Theme.of(context).colorScheme.primary;
    return SizedBox(
      height: 56,
      child: Stack(
        children: [
          IgnorePointer(
            child: Row(
              children: List.generate(_otpLength, (i) {
                final ch = i < text.length ? text[i] : '';
                final focused = _focusNode.hasFocus && i == activeIndex;
                final borderColor = _error != null
                    ? Colors.red
                    : (focused ? primary : Colors.grey.shade400);
                return Expanded(
                  child: Container(
                    margin: const EdgeInsets.symmetric(horizontal: 4),
                    alignment: Alignment.center,
                    decoration: BoxDecoration(
                      borderRadius: BorderRadius.circular(10),
                      border: Border.all(color: borderColor, width: focused || _error != null ? 2 : 1),
                    ),
                    child: Text(
                      ch,
                      style: TextStyle(
                        fontSize: 22,
                        fontWeight: FontWeight.w600,
                        color: _error != null ? Colors.red : null,
                      ),
                    ),
                  ),
                );
              }),
            ),
          ),
          Positioned.fill(
            child: TextField(
              controller: _otpController,
              focusNode: _focusNode,
              autofocus: widget.testOtp == null || widget.testOtp!.isEmpty,
              keyboardType: TextInputType.number,
              maxLength: _otpLength,
              showCursor: false,
              enableInteractiveSelection: false,
              autofillHints: const [AutofillHints.oneTimeCode],
              inputFormatters: [FilteringTextInputFormatter.digitsOnly],
              style: const TextStyle(color: Colors.transparent),
              cursorColor: Colors.transparent,
              decoration: const InputDecoration(
                border: InputBorder.none,
                counterText: '',
                fillColor: Colors.transparent,
              ),
              onChanged: (_) {
                if (_error != null) setState(() => _error = null);
              },
            ),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Verify OTP')),
      body: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.start,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            const SizedBox(height: 80),
            Text(
              'Enter the OTP sent to ${widget.phone}',
              textAlign: TextAlign.center,
              style: const TextStyle(fontSize: 16),
            ),
            if (widget.testOtp != null && widget.testOtp!.isNotEmpty) ...[
              const SizedBox(height: 8),
              Text(
                '(Test mode - auto-filled)',
                textAlign: TextAlign.center,
                style: TextStyle(fontSize: 12, color: Colors.grey[600]),
              ),
            ],
            const SizedBox(height: 16),
            _buildOtpBoxes(),
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