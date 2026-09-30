import { useEffect, useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { listAddresses } from '../api/addresses'
import { checkout, createPaymentOrder, verifyPayment, getCheckoutEstimate, type CheckoutEstimate } from '../api/orders'
import { validateCoupon, getWallet } from '../api/misc'
import { useCart } from '../context/CartContext'
import type { Address, Coupon } from '../types'

declare global {
  interface Window {
    Razorpay: any
  }
}

export default function Checkout() {
  const { cart, refreshCart } = useCart()
  const navigate = useNavigate()

  const [addresses, setAddresses] = useState<Address[]>([])
  const [addressId, setAddressId] = useState<number | null>(null)
  const [paymentMethod, setPaymentMethod] = useState<'cod' | 'online'>('cod')
  const [couponCode, setCouponCode] = useState('')
  const [appliedCoupon, setAppliedCoupon] = useState<{ coupon: Coupon; discount_amount: number } | null>(null)
  const [couponError, setCouponError] = useState<string | null>(null)
  const [useWallet, setUseWallet] = useState(false)
  const [walletBalance, setWalletBalance] = useState(0)
  const [isPlacing, setIsPlacing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [estimate, setEstimate] = useState<CheckoutEstimate | null>(null)

  useEffect(() => {
    listAddresses().then((addrs) => {
      setAddresses(addrs)
      const def = addrs.find((a) => a.is_default) ?? addrs[0]
      if (def) setAddressId(def.id)
    })
    getWallet().then((w) => setWalletBalance(w.balance))
  }, [])

  useEffect(() => {
    // Load Razorpay checkout script once, for online payments
    if (document.getElementById('razorpay-script')) return
    const script = document.createElement('script')
    script.id = 'razorpay-script'
    script.src = 'https://checkout.razorpay.com/v1/checkout.js'
    document.body.appendChild(script)
  }, [])

  useEffect(() => {
    // Fetch the real delivery charge / platform fee for the selected
    // address so the displayed total matches what checkout will actually
    // charge, instead of only summing item subtotals.
    if (!addressId) {
      setEstimate(null)
      return
    }
    let cancelled = false
    getCheckoutEstimate(addressId)
      .then((est) => {
        if (!cancelled) setEstimate(est)
      })
      .catch(() => {
        if (!cancelled) setEstimate(null)
      })
    return () => {
      cancelled = true
    }
  }, [addressId])

  const itemsAmount = cart?.total_amount ?? 0
  const discount = appliedCoupon?.discount_amount ?? 0
  const deliveryCharge = estimate?.delivery_charge ?? 0
  const platformFee = estimate?.platform_fee ?? 0
  const totalBeforeWallet = Math.max(0, itemsAmount + deliveryCharge + platformFee - discount)
  const walletUsable = Math.min(walletBalance, totalBeforeWallet)
  const walletApplied = useWallet ? walletUsable : 0
  const estimatedTotal = Math.max(0, totalBeforeWallet - walletApplied)

  async function handleApplyCoupon() {
    setCouponError(null)
    if (!couponCode.trim()) return
    try {
      const res = await validateCoupon(couponCode.trim(), itemsAmount)
      setAppliedCoupon(res)
    } catch (err: any) {
      setAppliedCoupon(null)
      setCouponError(err.response?.data?.error ?? 'Invalid coupon.')
    }
  }

  async function handlePlaceOrder() {
    if (!addressId) {
      setError('Select a delivery address.')
      return
    }
    setError(null)
    setIsPlacing(true)
    try {
      const order = await checkout({
        address_id: addressId,
        payment_method: paymentMethod,
        coupon_code: appliedCoupon ? couponCode.trim() : undefined,
        use_wallet: useWallet,
      })

      if (paymentMethod === 'online' && order.payment_status !== 'paid') {
        const payRes = await createPaymentOrder(order.id)
        const rzp = new window.Razorpay({
          key: payRes.key_id,
          amount: payRes.amount,
          currency: payRes.currency,
          order_id: payRes.razorpay_order_id,
          name: 'Bazaar',
          description: `Order #${order.id}`,
          handler: async (response: any) => {
            try {
              await verifyPayment(order.id, {
                razorpay_order_id: response.razorpay_order_id,
                razorpay_payment_id: response.razorpay_payment_id,
                razorpay_signature: response.razorpay_signature,
              })
              await refreshCart()
              navigate(`/orders/${order.id}`)
            } catch {
              setError('Payment verification failed. Check your orders page for status.')
            }
          },
          modal: {
            ondismiss: () => setIsPlacing(false),
          },
        })
        rzp.open()
        return
      }

      await refreshCart()
      navigate(`/orders/${order.id}`)
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to place order.')
      setIsPlacing(false)
    }
  }

  if (!cart || cart.items.length === 0) {
    return (
      <div className="max-w-xl mx-auto px-6 py-20 text-center">
        <p className="text-ink/60 mb-6">Your cart is empty.</p>
        <Link to="/products" className="text-marigold font-medium hover:underline">
          Browse products →
        </Link>
      </div>
    )
  }

  return (
    <div className="max-w-3xl mx-auto px-6 py-10">
      <h1 className="font-display text-3xl font-600 mb-8">Checkout</h1>

      <section className="mb-8">
        <div className="flex items-center justify-between mb-3">
          <h2 className="font-medium">Delivery address</h2>
          <Link to="/addresses" className="text-sm text-marigold hover:underline">
            Manage addresses
          </Link>
        </div>
        {addresses.length === 0 ? (
          <p className="text-sm text-ink/60">
            No saved addresses.{' '}
            <Link to="/addresses" className="text-marigold hover:underline">
              Add one
            </Link>{' '}
            to continue.
          </p>
        ) : (
          <div className="space-y-2">
            {addresses.map((a) => (
              <label
                key={a.id}
                className={`flex items-start gap-3 border rounded-xl p-4 cursor-pointer transition-colors ${
                  addressId === a.id ? 'border-ink' : 'border-line'
                }`}
              >
                <input
                  type="radio"
                  checked={addressId === a.id}
                  onChange={() => setAddressId(a.id)}
                  className="mt-1"
                />
                <div className="text-sm">
                  <p className="font-medium">{a.label || 'Address'}</p>
                  <p className="text-ink/60">
                    {a.full_name} · {a.phone}
                  </p>
                  <p className="text-ink/60">
                    {a.line1}, {a.city}, {a.state} - {a.pincode}
                  </p>
                </div>
              </label>
            ))}
          </div>
        )}
      </section>

      <section className="mb-8">
        <h2 className="font-medium mb-3">Coupon</h2>
        <div className="flex gap-2">
          <input
            value={couponCode}
            onChange={(e) => setCouponCode(e.target.value.toUpperCase())}
            placeholder="Enter code"
            className="flex-1 border border-line rounded-lg px-3 py-2 outline-none focus:border-ink font-mono"
          />
          <button
            onClick={handleApplyCoupon}
            className="px-4 py-2 rounded-lg border border-line hover:border-ink text-sm font-medium"
          >
            Apply
          </button>
        </div>
        {couponError && <p className="text-clay text-sm mt-2">{couponError}</p>}
        {appliedCoupon && (
          <p className="text-leaf text-sm mt-2">
            Coupon applied — you save ₹{appliedCoupon.discount_amount.toFixed(2)}
          </p>
        )}
      </section>

      {walletBalance > 0 && (
        <section className="mb-8">
          <label className="flex items-center gap-3 border border-line rounded-xl p-4 cursor-pointer">
            <input type="checkbox" checked={useWallet} onChange={(e) => setUseWallet(e.target.checked)} />
            <span className="text-sm">
              Use wallet balance{' '}
              <span className="font-mono text-leaf">(₹{walletBalance.toFixed(2)} available)</span>
            </span>
          </label>
        </section>
      )}

      <section className="mb-8">
        <h2 className="font-medium mb-3">Payment method</h2>
        <div className="grid grid-cols-2 gap-3">
          <label
            className={`border rounded-xl p-4 cursor-pointer text-center transition-colors ${
              paymentMethod === 'cod' ? 'border-ink' : 'border-line'
            }`}
          >
            <input
              type="radio"
              className="sr-only"
              checked={paymentMethod === 'cod'}
              onChange={() => setPaymentMethod('cod')}
            />
            <p className="font-medium text-sm">Cash on Delivery</p>
          </label>
          <label
            className={`border rounded-xl p-4 cursor-pointer text-center transition-colors ${
              paymentMethod === 'online' ? 'border-ink' : 'border-line'
            }`}
          >
            <input
              type="radio"
              className="sr-only"
              checked={paymentMethod === 'online'}
              onChange={() => setPaymentMethod('online')}
            />
            <p className="font-medium text-sm">Pay Online</p>
          </label>
        </div>
      </section>

      <section className="border-t border-line pt-6 mb-8 space-y-1.5 text-sm">
        <div className="flex justify-between">
          <span className="text-ink/60">Items total</span>
          <span className="font-mono">₹{itemsAmount.toFixed(2)}</span>
        </div>
        {discount > 0 && (
          <div className="flex justify-between text-leaf">
            <span>Coupon discount</span>
            <span className="font-mono">−₹{discount.toFixed(2)}</span>
          </div>
        )}
        {walletApplied > 0 && (
          <div className="flex justify-between text-leaf">
            <span>Wallet applied</span>
            <span className="font-mono">−₹{walletApplied.toFixed(2)}</span>
          </div>
        )}
        <div className="flex justify-between font-semibold text-base pt-2 border-t border-line">
          <span>Estimated total</span>
          <span className="font-mono">₹{estimatedTotal.toFixed(2)}</span>
        </div>
        <p className="text-xs text-ink/40">Delivery charges, if any, are calculated at order confirmation.</p>
      </section>

      {error && <p className="text-clay text-sm mb-4">{error}</p>}

      <button
        onClick={handlePlaceOrder}
        disabled={isPlacing || !addressId}
        className="w-full bg-ink text-paper font-medium py-3.5 rounded-lg hover:bg-marigold transition-colors disabled:opacity-50"
      >
        {isPlacing ? 'Placing order...' : 'Place order'}
      </button>
    </div>
  )
}
