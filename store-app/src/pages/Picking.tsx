import { useCallback, useEffect, useState } from 'react'
import { STORE_READ_ONLY } from '../readOnly'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { completePicking, getPickingTask, markPickItem, scanPickItem, startPicking } from '../api/warehouse'
import type { PickingTask, PickingTaskItem } from '../types/warehouse'
import StatusBadge from '../components/StatusBadge'
import CameraScanner from '../components/CameraScanner'
import { getErrorMessage } from '../utils/errors'

function ItemRow({
  item,
  orderId,
  onMark,
  isBusy,
}: {
  item: PickingTaskItem
  orderId: number
  onMark: (status: 'picked' | 'unavailable' | 'short', quantityPicked?: number, reason?: string) => void
  isBusy: boolean
}) {
  const [showShortInput, setShowShortInput] = useState(false)
  const [shortQty, setShortQty] = useState('')
  const [showUnavailableReason, setShowUnavailableReason] = useState(false)
  const [reason, setReason] = useState('')
  const [barcodeInput, setBarcodeInput] = useState('')
  const [isScanning, setIsScanning] = useState(false)
  const [scanState, setScanState] = useState<'idle' | 'match' | 'mismatch'>('idle')
  const [showCamera, setShowCamera] = useState(false)

  const isDone = STORE_READ_ONLY || item.status !== 'pending'

  const submitScan = async (code: string) => {
    if (!code.trim()) return
    setIsScanning(true)
    setScanState('idle')
    try {
      const result = await scanPickItem(item.id, code.trim())
      if (result.match) {
        setScanState('match')
        onMark('picked')
      } else {
        setScanState('mismatch')
      }
    } catch {
      setScanState('mismatch')
    } finally {
      setIsScanning(false)
      setBarcodeInput('')
    }
  }

  const handleScan = () => submitScan(barcodeInput)

  const handleCameraDetected = (code: string) => {
    setShowCamera(false)
    submitScan(code)
  }

  return (
    <div className="border border-slate-800 rounded-xl bg-slate-900 p-4">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="font-medium text-sm">{item.product?.name ?? `Product #${item.product_id}`}</p>
          <p className="text-xs text-slate-400 mt-1">
            Qty needed: <span className="text-slate-200">{item.quantity_needed}</span>
            {item.status !== 'pending' && (
              <>
                {' '}
                &middot; Picked: <span className="text-slate-200">{item.quantity_picked}</span>
              </>
            )}
          </p>
          {item.reason && <p className="text-xs text-red-400 mt-1">Reason: {item.reason}</p>}
        </div>
        <StatusBadge status={item.status} />
      </div>

      {(item.status === 'unavailable' || item.status === 'short') && (
        <Link
          to={`/substitutions?order_id=${orderId}&item_id=${item.id}&product_id=${item.product_id}`}
          className="mt-3 inline-block px-3 py-1.5 rounded-lg bg-red-500/20 text-red-300 hover:bg-red-500/30 text-xs font-medium"
        >
          Request Substitution
        </Link>
      )}

      {!isDone && (
        <div className="mt-3 flex items-center gap-2">
          <input
            type="text"
            autoFocus
            placeholder="Scan barcode or SKU..."
            value={barcodeInput}
            onChange={(e) => setBarcodeInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') handleScan()
            }}
            disabled={isBusy || isScanning}
            className="flex-1 bg-slate-800 border border-slate-700 rounded-lg px-3 py-1.5 text-xs focus:outline-none focus:border-red-500"
          />
          <button
            disabled={isBusy || isScanning}
            onClick={() => setShowCamera(true)}
            title="Scan with camera"
            className="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs font-medium disabled:opacity-40 flex items-center gap-1.5"
          >
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="w-3.5 h-3.5">
              <path d="M3 7V5a2 2 0 0 1 2-2h2" />
              <path d="M17 3h2a2 2 0 0 1 2 2v2" />
              <path d="M21 17v2a2 2 0 0 1-2 2h-2" />
              <path d="M7 21H5a2 2 0 0 1-2-2v-2" />
              <path d="M7 12h10" />
            </svg>
            Camera
          </button>
          <button
            disabled={isBusy || isScanning || !barcodeInput.trim()}
            onClick={handleScan}
            className="px-3 py-1.5 rounded-lg bg-red-500/20 text-red-300 hover:bg-red-500/30 text-xs font-medium disabled:opacity-40"
          >
            {isScanning ? 'Scanning...' : 'Scan'}
          </button>
        </div>
      )}

      {showCamera && (
        <CameraScanner onDetected={handleCameraDetected} onClose={() => setShowCamera(false)} />
      )}

      {scanState === 'mismatch' && !isDone && (
        <div className="mt-2 text-xs text-rose-400">
          Product mismatch â€” verify SKU.
        </div>
      )}

      {!isDone && (
        <div className="mt-3 flex flex-wrap gap-2">
          <button
            disabled={isBusy}
            onClick={() => onMark('picked')}
            className="px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-medium transition-colors disabled:opacity-50"
          >
            Mark Picked (manual)
          </button>
          <button
            disabled={isBusy}
            onClick={() => setShowShortInput((s) => !s)}
            className="px-3 py-1.5 rounded-lg bg-red-600/80 hover:bg-red-600 text-white text-xs font-medium transition-colors disabled:opacity-50"
          >
            Partial / Short
          </button>
          <button
            disabled={isBusy}
            onClick={() => setShowUnavailableReason((s) => !s)}
            className="px-3 py-1.5 rounded-lg bg-rose-600/80 hover:bg-rose-600 text-white text-xs font-medium transition-colors disabled:opacity-50"
          >
            Unavailable
          </button>
        </div>
      )}

      {showShortInput && !isDone && (
        <div className="mt-3 flex items-center gap-2">
          <input
            type="number"
            min={1}
            max={item.quantity_needed - 1}
            placeholder={`1 - ${item.quantity_needed - 1}`}
            value={shortQty}
            onChange={(e) => setShortQty(e.target.value)}
            className="w-28 bg-slate-800 border border-slate-700 rounded-lg px-3 py-1.5 text-xs"
          />
          <button
            disabled={isBusy || !shortQty}
            onClick={() => {
              onMark('short', Number(shortQty))
              setShowShortInput(false)
            }}
            className="px-3 py-1.5 rounded-lg bg-red-600 hover:bg-red-500 text-white text-xs font-medium disabled:opacity-50"
          >
            Confirm short pick
          </button>
        </div>
      )}

      {showUnavailableReason && !isDone && (
        <div className="mt-3 flex items-center gap-2">
          <input
            type="text"
            placeholder="Reason (optional)"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            className="flex-1 bg-slate-800 border border-slate-700 rounded-lg px-3 py-1.5 text-xs"
          />
          <button
            disabled={isBusy}
            onClick={() => {
              onMark('unavailable', undefined, reason)
              setShowUnavailableReason(false)
            }}
            className="px-3 py-1.5 rounded-lg bg-rose-600 hover:bg-rose-500 text-white text-xs font-medium disabled:opacity-50"
          >
            Confirm unavailable
          </button>
        </div>
      )}
    </div>
  )
}

export default function Picking() {
  const { orderId } = useParams()
  const navigate = useNavigate()
  const [task, setTask] = useState<PickingTask | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busyItemId, setBusyItemId] = useState<number | null>(null)
  const [isStarting, setIsStarting] = useState(false)
  const [isCompleting, setIsCompleting] = useState(false)

  const load = useCallback(async () => {
    if (!orderId) return
    setError(null)
    try {
      const data = await getPickingTask(Number(orderId))
      setTask(data)
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to load picking task.'))
    } finally {
      setIsLoading(false)
    }
  }, [orderId])

  useEffect(() => {
    load()
    const t = setInterval(load, 5000)
    return () => clearInterval(t)
  }, [load])

  async function handleStart() {
    if (!orderId) return
    setIsStarting(true)
    setError(null)
    try {
      await startPicking(Number(orderId))
      await load()
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to start picking.'))
    } finally {
      setIsStarting(false)
    }
  }

  async function handleMark(
    item: PickingTaskItem,
    status: 'picked' | 'unavailable' | 'short',
    quantityPicked?: number,
    reason?: string
  ) {
    setBusyItemId(item.id)
    setError(null)
    try {
      await markPickItem(item.id, { status, quantity_picked: quantityPicked, reason })
      await load()
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to update item.'))
    } finally {
      setBusyItemId(null)
    }
  }

  async function handleComplete() {
    if (!orderId) return
    setIsCompleting(true)
    setError(null)
    try {
      await completePicking(Number(orderId))
      navigate(`/packing/${orderId}`)
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to complete picking.'))
    } finally {
      setIsCompleting(false)
    }
  }

  if (isLoading) return <div className="p-6 text-sm text-slate-400">Loading picking task...</div>

  if (!task) {
    return (
      <div className="p-6">
        <div className="border border-rose-900 bg-rose-950/40 text-rose-300 text-sm rounded-lg px-4 py-3">
          {error ?? 'Picking task not found.'}
        </div>
      </div>
    )
  }

  const items = task.items ?? []
  const allMarked = items.length > 0 && items.every((i) => i.status !== 'pending')

  return (
    <div className="p-6 max-w-3xl">
      <button onClick={() => navigate('/orders')} className="text-xs text-slate-400 hover:text-slate-200 mb-3">
        &larr; Back to orders
      </button>
      <div className="flex items-center justify-between mb-1">
        <h1 className="font-display text-2xl font-semibold">Picking &mdash; Order #{task.order_id}</h1>
        <StatusBadge status={task.status} />
      </div>
      <p className="text-xs text-slate-500 mb-6">
        {task.started_at ? `Started ${new Date(task.started_at).toLocaleTimeString()}` : 'Not started yet'}
      </p>

      {error && (
        <div className="border border-rose-900 bg-rose-950/40 text-rose-300 text-sm rounded-lg px-4 py-3 mb-4">
          {error}
        </div>
      )}

      {task.status === 'pending' && (
        STORE_READ_ONLY ? (<p className="mb-6 text-xs text-slate-400">Not started yet. Picking is done in the picker app.</p>) : (<button
          onClick={handleStart}
          disabled={isStarting}
          className="mb-6 px-4 py-2 rounded-lg bg-red-500 hover:bg-red-400 text-white text-sm font-medium transition-colors disabled:opacity-50"
        >
          {isStarting ? 'Starting...' : 'Start Picking'}
        </button>)
      )}

      {task.status !== 'pending' && (
        <div className="space-y-3 mb-6">
          {items.map((item) => (
            <ItemRow
              key={item.id}
              item={item}
              orderId={task.order_id}
              isBusy={busyItemId === item.id}
              onMark={(status, qty, reason) => handleMark(item, status, qty, reason)}
            />
          ))}
        </div>
      )}

      {task.status === 'in_progress' && (
        STORE_READ_ONLY ? (<p className="text-xs text-slate-400">Picking in progress in the picker app.</p>) : (<button
          onClick={handleComplete}
          disabled={!allMarked || isCompleting}
          className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-sm font-medium transition-colors disabled:opacity-50"
        >
          {isCompleting ? 'Completing...' : allMarked ? 'Complete Picking' : 'Mark all items to continue'}
        </button>)
      )}

      {task.status === 'completed' && (
        <button
          onClick={() => navigate(`/packing/${task.order_id}`)}
          className="px-4 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-sm font-medium transition-colors"
        >
          Go to Packing &rarr;
        </button>
      )}
    </div>
  )
}
