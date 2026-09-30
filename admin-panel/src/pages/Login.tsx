import { useState, type FormEvent } from 'react'
import loginHero from '../assets/login-hero.png'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'

type Step = 'phone' | 'otp'

export default function Login() {
  const navigate = useNavigate()
  const { sendOtp, verifyOtp } = useAuth()

  const [step, setStep] = useState<Step>('phone')
  const [phone, setPhone] = useState('')
  const [otp, setOtp] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)

  const phoneIsValid = /^\d{10}$/.test(phone)
  const otpIsValid = /^\d{4,6}$/.test(otp)

  async function handleSendOtp(e: FormEvent) {
    e.preventDefault()
    setError(null)

    if (!phoneIsValid) {
      setError('Enter a valid 10-digit phone number.')
      return
    }

    setIsSubmitting(true)
    try {
      await sendOtp(phone)
      setStep('otp')
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to send OTP. Please try again.')
    } finally {
      setIsSubmitting(false)
    }
  }

  async function handleVerifyOtp(e: FormEvent) {
    e.preventDefault()
    setError(null)

    if (!otpIsValid) {
      setError('Enter the OTP you received.')
      return
    }

    setIsSubmitting(true)
    try {
      await verifyOtp(phone, otp)
      navigate('/dashboard')
    } catch (err: any) {
      setError(
        err.response?.data?.error ?? err.message ?? 'Failed to verify OTP. Please try again.'
      )
    } finally {
      setIsSubmitting(false)
    }
  }

  function useDifferentNumber() {
    setStep('phone')
    setOtp('')
    setError(null)
  }

  return (
    <div className="min-h-screen flex bg-white">
      {/* Left branding panel */}
      <div className="hidden lg:flex lg:w-1/2 relative overflow-hidden bg-emerald-900 items-center justify-center">
        <img src={loginHero} alt="GoFresh Admin Panel" className="w-full h-full object-contain" />
      </div>

      {/* Right form panel */}
      <div className="flex-1 flex items-center justify-center p-6 sm:p-10 bg-slate-50">
        <div className="w-full max-w-sm">
          <div className="lg:hidden flex items-center gap-3 mb-8">
            <div className="w-10 h-10 rounded-xl bg-emerald-700 flex items-center justify-center text-white font-bold">
              GF
            </div>
            <div>
              <p className="font-semibold text-slate-900 leading-tight">GoFresh</p>
              <p className="text-xs text-slate-500 leading-tight">Admin Panel</p>
            </div>
          </div>

          <div className="mx-auto mb-6 w-14 h-14 rounded-2xl bg-emerald-50 flex items-center justify-center">
            <svg width="26" height="26" viewBox="0 0 24 24" fill="none" className="text-emerald-600">
              <path
                d="M12 2L4 5v6c0 5.25 3.4 9.74 8 11 4.6-1.26 8-5.75 8-11V5l-8-3z"
                stroke="currentColor"
                strokeWidth="1.7"
                strokeLinejoin="round"
              />
              <circle cx="12" cy="10.5" r="2.3" stroke="currentColor" strokeWidth="1.7" />
              <path d="M9 15.5c0-1.4 1.34-2.5 3-2.5s3 1.1 3 2.5" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
            </svg>
          </div>

          <h2 className="text-2xl font-semibold text-slate-900 text-center mb-1">Admin login</h2>
          <p className="text-sm text-slate-500 text-center mb-8">
            {step === 'phone'
              ? 'Enter your registered phone number to continue'
              : `Enter the code sent to ${phone}`}
          </p>

          {step === 'phone' && (
            <form onSubmit={handleSendOtp} className="space-y-4">
              <div>
                <label htmlFor="phone" className="text-sm font-medium text-slate-700 block mb-1.5">
                  Phone number
                </label>
                <div className="flex items-center border border-slate-300 rounded-xl overflow-hidden focus-within:ring-2 focus-within:ring-emerald-500 focus-within:border-emerald-500 bg-white">
                  <span className="px-4 py-3 text-slate-500 text-sm border-r border-slate-200 bg-slate-50">+91</span>
                  <input
                    id="phone"
                    type="tel"
                    inputMode="numeric"
                    maxLength={10}
                    placeholder="Enter phone number"
                    value={phone}
                    onChange={(e) => setPhone(e.target.value.replace(/\D/g, ''))}
                    className="flex-1 px-4 py-3 text-sm outline-none"
                    autoFocus
                  />
                </div>
              </div>

              {error && <p className="text-sm text-red-600">{error}</p>}

              <button
                type="submit"
                disabled={isSubmitting}
                className="w-full py-3 rounded-xl bg-emerald-700 hover:bg-emerald-800 text-white font-medium transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
              >
                {isSubmitting ? 'Sending...' : 'Send OTP'}
                {!isSubmitting && <span aria-hidden>&rarr;</span>}
              </button>

              <div className="flex items-center gap-3 py-1">
                <div className="flex-1 h-px bg-slate-200" />
                <span className="text-xs text-slate-400">or</span>
                <div className="flex-1 h-px bg-slate-200" />
              </div>

              <div className="border border-emerald-100 bg-emerald-50/60 rounded-xl px-4 py-3 flex items-center gap-2 justify-center">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" className="text-emerald-600 shrink-0">
                  <path d="M12 2l7 3v6c0 4.5-2.9 8.4-7 9.9-4.1-1.5-7-5.4-7-9.9V5l7-3z" stroke="currentColor" strokeWidth="1.8" strokeLinejoin="round" />
                  <path d="M9 12l2 2 4-4" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
                <p className="text-xs text-emerald-800">We never share your number with anyone</p>
              </div>
            </form>
          )}

          {step === 'otp' && (
            <form onSubmit={handleVerifyOtp} className="space-y-4">
              <div>
                <label htmlFor="otp" className="text-sm font-medium text-slate-700 block mb-1.5">
                  Enter OTP
                </label>
                <input
                  id="otp"
                  type="text"
                  inputMode="numeric"
                  maxLength={6}
                  placeholder="123456"
                  value={otp}
                  onChange={(e) => setOtp(e.target.value.replace(/\D/g, ''))}
                  className="w-full px-4 py-3 rounded-xl border border-slate-300 text-sm tracking-widest text-center outline-none focus:ring-2 focus:ring-emerald-500 focus:border-emerald-500"
                  autoFocus
                />
              </div>

              {error && <p className="text-sm text-red-600">{error}</p>}

              <button
                type="submit"
                disabled={isSubmitting}
                className="w-full py-3 rounded-xl bg-emerald-700 hover:bg-emerald-800 text-white font-medium transition-colors disabled:opacity-50"
              >
                {isSubmitting ? 'Verifying...' : 'Verify & login'}
              </button>

              <button
                type="button"
                onClick={useDifferentNumber}
                className="w-full text-center text-sm text-slate-500 hover:text-slate-700 transition-colors"
              >
                Use a different number
              </button>
            </form>
          )}

          <p className="text-xs text-slate-400 text-center mt-10">
            &copy; {new Date().getFullYear()} GoFresh Admin Panel. All rights reserved.
          </p>
        </div>
      </div>
    </div>
  )
}
