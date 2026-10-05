import { useCallback, useEffect, useState } from 'react'
import { QRCodeSVG } from 'qrcode.react'
import { getCheckinQR } from '../api/warehouse'
import { useAuth } from '../context/AuthContext'

const REFRESH_MS = 30000

export default function StoreQR() {
  const { staff } = useAuth()
  const [token, setToken] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [secondsLeft, setSecondsLeft] = useState(0)

  const load = useCallback(async () => {
    try {
      const data = await getCheckinQR()
      setToken(data.token)
      setSecondsLeft(
        Math.max(0, Math.round((new Date(data.expires_at).getTime() - Date.now()) / 1000)),
      )
      setError(null)
    } catch {
      setError('QR load nahi hua. Internet check karo.')
    }
  }, [])

  useEffect(() => {
    load()
    const refresh = setInterval(load, REFRESH_MS)
    const tick = setInterval(() => setSecondsLeft((s) => (s > 0 ? s - 1 : 0)), 1000)
    return () => {
      clearInterval(refresh)
      clearInterval(tick)
    }
  }, [load])

  return (
    <div className="p-8 max-w-xl">
      <p className="font-mono text-[10px] tracking-widest text-red-500 uppercase mb-1">
        Delivery partner check-in
      </p>
      <h1 className="font-display text-2xl mb-2">Store QR</h1>
      <p className="text-sm text-slate-400 mb-6">
        Delivery partner apni app me scan button se ye QR scan karega. Scan ke baad hi use
        order auto-assign honge. Ye QR har 30 second me badalta hai, photo kaam nahi karegi.
      </p>

      <div className="inline-block bg-white p-5">
        {token ? (
          <QRCodeSVG value={token} size={280} level="M" />
        ) : (
          <div className="w-[280px] h-[280px] flex items-center justify-center text-slate-500 text-sm">
            {error ?? 'Loading...'}
          </div>
        )}
      </div>

      <p className="mt-3 text-xs text-slate-400">
        {staff?.warehouse?.name ?? 'Store #' + (staff?.warehouse_id ?? '-')}
        {token ? ' - badalne me ' + secondsLeft + 's' : ''}
      </p>
      {error && token && <p className="mt-2 text-xs text-red-400">{error}</p>}
    </div>
  )
}