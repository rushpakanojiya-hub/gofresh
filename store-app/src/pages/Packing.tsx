import { useCallback, useEffect, useState } from 'react'
import { STORE_READ_ONLY } from '../readOnly'
import { useNavigate, useParams } from 'react-router-dom'
import { completePacking, getPackingTask, startPacking } from '../api/warehouse'
import type { PackingTaskResponse } from '../types/warehouse'
import StatusBadge from '../components/StatusBadge'
import { getErrorMessage } from '../utils/errors'

export default function Packing() {
  const { orderId } = useParams()
  const navigate = useNavigate()
  const [data, setData] = useState<PackingTaskResponse | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isStarting, setIsStarting] = useState(false)
  const [isCompleting, setIsCompleting] = useState(false)

  const load = useCallback(async () => {
    if (!orderId) return
    setError(null)
    try {
      const res = await getPackingTask(Number(orderId))
      setData(res)
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to load packing task.'))
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
      await startPacking(Number(orderId))
      await load()
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to start packing.'))
    } finally {
      setIsStarting(false)
    }
  }

  async function handleComplete() {
    if (!orderId) return
    setIsCompleting(true)
    setError(null)
    try {
      await completePacking(Number(orderId))
      await load()
    } catch (err) {
      // Double-pack prevention surfaces here as a 409/400 from the backend.
      setError(getErrorMessage(err, 'Failed to complete packing.'))
    } finally {
      setIsCompleting(false)
    }
  }

  if (isLoading) return <div className="p-6 text-sm text-slate-400">Loading packing task...</div>

  if (!data) {
    return (
      <div className="p-6">
        <div className="border border-rose-900 bg-rose-950/40 text-rose-300 text-sm rounded-lg px-4 py-3">
          {error ?? 'Packing task not found.'}
        </div>
      </div>
    )
  }

  const { packing_task: task, picked_items: items } = data
  const exceptions = items.filter((i) => i.status === 'unavailable' || i.status === 'short')

  return (
    <div className="p-6 max-w-3xl">
      <button onClick={() => navigate('/orders')} className="text-xs text-slate-400 hover:text-slate-200 mb-3">
        &larr; Back to orders
      </button>
      <div className="flex items-center justify-between mb-1">
        <h1 className="font-display text-2xl font-semibold">Packing &mdash; Order #{task.order_id}</h1>
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

      {exceptions.length > 0 && (
        <div className="border border-red-900 bg-red-950/30 text-red-300 text-sm rounded-lg px-4 py-3 mb-4">
          {exceptions.length} item(s) had picking exceptions &mdash; double-check before packing.
        </div>
      )}

      <p className="text-xs uppercase tracking-wide text-slate-500 mb-2">Picked items</p>
      <div className="space-y-2 mb-6">
        {items.map((item) => (
          <div
            key={item.id}
            className="border border-slate-800 rounded-xl bg-slate-900 p-4 flex items-center justify-between"
          >
            <div>
              <p className="font-medium text-sm">{item.product?.name ?? `Product #${item.product_id}`}</p>
              <p className="text-xs text-slate-400 mt-1">
                Needed: {item.quantity_needed} &middot; Picked: {item.quantity_picked}
              </p>
              {item.reason && <p className="text-xs text-red-400 mt-1">Reason: {item.reason}</p>}
            </div>
            <StatusBadge status={item.status} />
          </div>
        ))}
      </div>

      {task.status === 'pending' && (
        STORE_READ_ONLY ? (<p className="text-xs text-slate-400">Not started yet. Packing is done in the picker app.</p>) : (<button
          onClick={handleStart}
          disabled={isStarting}
          className="px-4 py-2 rounded-lg bg-red-500 hover:bg-red-400 text-white text-sm font-medium transition-colors disabled:opacity-50"
        >
          {isStarting ? 'Starting...' : 'Start Packing'}
        </button>)
      )}

      {task.status === 'in_progress' && (
        STORE_READ_ONLY ? (<p className="text-xs text-slate-400">Packing in progress in the picker app.</p>) : (<button
          onClick={handleComplete}
          disabled={isCompleting}
          className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-sm font-medium transition-colors disabled:opacity-50"
        >
          {isCompleting ? 'Completing...' : 'Complete Packing'}
        </button>)
      )}

      {task.status === 'completed' && (
        <div className="flex items-center gap-3">
          <span className="text-sm text-emerald-300">Order is ready for dispatch.</span>
          <button
            onClick={() => navigate('/orders?status=ready_for_dispatch')}
            className="px-4 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-sm font-medium transition-colors"
          >
            Back to Orders
          </button>
        </div>
      )}
    </div>
  )
}
