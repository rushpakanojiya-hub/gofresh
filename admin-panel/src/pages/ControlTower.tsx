import { useEffect, useState } from 'react'
import Layout from '../components/Layout'
import {
  getControlTowerOverview,
  getControlTowerSettings,
  getLiveOperations,
  updatePlatformPause,
  updateCodEnabled,
  updateCityStatus,
  listSupportTickets,
} from '../api/admin'
import type { ControlTowerOverview, LiveOperations, SupportTicket } from '../types/admin'

const ISSUE_TYPE_LABELS: Record<string, string> = {
  missing_item: 'Missing item',
  wrong_item: 'Wrong item',
  damaged_item: 'Damaged item',
  expired_item: 'Expired item',
  delivery_issue: 'Delivery issue',
  payment_issue: 'Payment issue',
  refund_issue: 'Refund issue',
  other: 'Other',
}

type PendingAction =
  | { type: 'platform'; nextPaused: boolean }
  | { type: 'city'; city: string; nextStatus: 'open' | 'paused' }
  | { type: 'cod'; nextEnabled: boolean }

export default function ControlTower() {
  const [overview, setOverview] = useState<ControlTowerOverview | null>(null)
  const [codEnabled, setCodEnabled] = useState<boolean | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [tickets, setTickets] = useState<SupportTicket[]>([])
  const [ticketsLoaded, setTicketsLoaded] = useState(false)

  const [liveOps, setLiveOps] = useState<LiveOperations | null>(null)
  const [liveOpsLoaded, setLiveOpsLoaded] = useState(false)

  const [pendingAction, setPendingAction] = useState<PendingAction | null>(null)
  const [actionReason, setActionReason] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [toast, setToast] = useState<{ kind: 'success' | 'error'; message: string } | null>(null)

  async function load() {
    setIsLoading(true)
    setError(null)
    try {
      const res = await getControlTowerOverview()
      setOverview(res)
      try {
        const settings = await getControlTowerSettings()
        setCodEnabled(settings.cod_enabled)
      } catch {
        // non-fatal - COD card just won't render if this fails
      }
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to load Control Tower data.')
    } finally {
      setIsLoading(false)
    }
  }

  async function loadTickets() {
    try {
      const res = await listSupportTickets()
      setTickets(res ?? [])
    } catch {
      // non-fatal - the issue-type breakdown just won't populate
    } finally {
      setTicketsLoaded(true)
    }
  }

  async function loadLiveOps() {
    try {
      const res = await getLiveOperations()
      setLiveOps(res)
    } catch {
      // non-fatal - panel just shows a load error state
    } finally {
      setLiveOpsLoaded(true)
    }
  }

  useEffect(() => {
    load()
    loadTickets()
    loadLiveOps()
  }, [])

  function showToast(kind: 'success' | 'error', message: string) {
    setToast({ kind, message })
    setTimeout(() => setToast(null), 4000)
  }

  async function confirmAction() {
    if (!pendingAction || !actionReason.trim()) return
    setIsSubmitting(true)
    try {
      if (pendingAction.type === 'platform') {
        await updatePlatformPause(pendingAction.nextPaused, actionReason.trim())
        showToast(
          'success',
          pendingAction.nextPaused ? 'Platform paused. New orders are blocked.' : 'Platform resumed. Orders are live.'
        )
      } else if (pendingAction.type === 'city') {
        await updateCityStatus(pendingAction.city, pendingAction.nextStatus, actionReason.trim())
        showToast(
          'success',
          `${pendingAction.city}: all stores set to ${pendingAction.nextStatus}.`
        )
      } else {
        await updateCodEnabled(pendingAction.nextEnabled, actionReason.trim())
        setCodEnabled(pendingAction.nextEnabled)
        showToast(
          'success',
          pendingAction.nextEnabled ? 'COD re-enabled. Customers can pay on delivery again.' : 'COD disabled. Checkout will reject cod payment method.'
        )
      }
      setPendingAction(null)
      setActionReason('')
      await load()
    } catch (err: any) {
      showToast('error', err.response?.data?.error ?? 'Action failed. Nothing was changed.')
    } finally {
      setIsSubmitting(false)
    }
  }

  const openTickets = tickets.filter((t) => t.status === 'open' || t.status === 'in_progress')
  const inProgressCount = tickets.filter((t) => t.status === 'in_progress').length
  const unassignedCount = openTickets.filter((t) => !t.assigned_to_staff_id).length
  const issueCounts = openTickets.reduce<Record<string, number>>((acc, t) => {
    const key = t.issue_type || 'other'
    acc[key] = (acc[key] ?? 0) + 1
    return acc
  }, {})

  return (
    <Layout>
      <div className="p-8 space-y-8">
        <div>
          <h1 className="text-xl font-semibold">Control Tower</h1>
          <p className="text-sm text-slate-400 mt-1">Platform-wide operations, kill switches, and support load</p>
        </div>

        {isLoading && <p className="text-slate-400">Loading...</p>}
        {error && <p className="text-red-400">{error}</p>}

        {!isLoading && !error && overview && (
          <>
            {/* Kill Switches */}
            <section>
              <h2 className="text-sm font-semibold text-slate-300 mb-3">Kill Switches</h2>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="border border-slate-800 rounded-xl p-5 bg-slate-900">
                  <div className="flex items-center justify-between mb-2">
                    <p className="text-sm font-medium text-slate-200">Platform</p>
                    <span
                      className={`text-xs px-2 py-1 rounded-full font-medium ${
                        overview.platform_orders_paused
                          ? 'bg-red-500/15 text-red-300'
                          : 'bg-emerald-500/15 text-emerald-300'
                      }`}
                    >
                      {overview.platform_orders_paused ? 'Paused' : 'Live'}
                    </span>
                  </div>
                  <p className="text-xs text-slate-500 mb-4">
                    {overview.platform_orders_paused
                      ? 'All new orders are currently blocked, platform-wide.'
                      : 'New orders are being accepted normally.'}
                  </p>
                  <button
                    onClick={() =>
                      setPendingAction({ type: 'platform', nextPaused: !overview.platform_orders_paused })
                    }
                    className={`w-full py-2 rounded-lg text-sm font-medium transition-colors ${
                      overview.platform_orders_paused
                        ? 'bg-emerald-500 hover:bg-emerald-400 text-white'
                        : 'bg-red-500 hover:bg-red-400 text-white'
                    }`}
                  >
                    {overview.platform_orders_paused ? 'Resume platform' : 'Pause platform'}
                  </button>
                </div>

                <div className="border border-slate-800 rounded-xl p-5 bg-slate-900">
                  <div className="flex items-center justify-between mb-2">
                    <p className="text-sm font-medium text-slate-200">Cash on Delivery</p>
                    <span
                      className={`text-xs px-2 py-1 rounded-full font-medium ${
                        codEnabled === false
                          ? 'bg-red-500/15 text-red-300'
                          : 'bg-emerald-500/15 text-emerald-300'
                      }`}
                    >
                      {codEnabled === null ? 'Loading...' : codEnabled ? 'Available' : 'Disabled'}
                    </span>
                  </div>
                  <p className="text-xs text-slate-500 mb-4">
                    {codEnabled === false
                      ? 'Checkout rejects any cod payment method - enforced server-side.'
                      : 'Customers can pay on delivery at checkout.'}
                  </p>
                  <button
                    onClick={() => setPendingAction({ type: 'cod', nextEnabled: !codEnabled })}
                    disabled={codEnabled === null}
                    className={`w-full py-2 rounded-lg text-sm font-medium transition-colors disabled:opacity-40 ${
                      codEnabled === false
                        ? 'bg-emerald-500 hover:bg-emerald-400 text-white'
                        : 'bg-red-500 hover:bg-red-400 text-white'
                    }`}
                  >
                    {codEnabled === false ? 'Enable COD' : 'Disable COD'}
                  </button>
                </div>
              </div>

              <p className="text-xs text-slate-500 mt-4 mb-2">Per-city</p>
              <div className="border border-slate-800 rounded-xl overflow-hidden">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="bg-slate-900 text-slate-400 text-left">
                      <th className="px-4 py-3 font-medium">City</th>
                      <th className="px-4 py-3 font-medium">Stores</th>
                      <th className="px-4 py-3 font-medium">Open</th>
                      <th className="px-4 py-3 font-medium">Paused</th>
                      <th className="px-4 py-3 font-medium">Closed</th>
                      <th className="px-4 py-3 font-medium"></th>
                    </tr>
                  </thead>
                  <tbody>
                    {overview.cities.map((c) => {
                      const cityIsPaused = c.open_stores === 0 && c.paused_stores > 0
                      return (
                        <tr key={c.city} className="border-t border-slate-800">
                          <td className="px-4 py-3 text-slate-200">{c.city}</td>
                          <td className="px-4 py-3 text-slate-400">{c.total_stores}</td>
                          <td className="px-4 py-3 text-emerald-300">{c.open_stores}</td>
                          <td className="px-4 py-3 text-amber-300">{c.paused_stores}</td>
                          <td className="px-4 py-3 text-red-300">{c.closed_stores}</td>
                          <td className="px-4 py-3 text-right space-x-2">
                            <button
                              onClick={() =>
                                setPendingAction({ type: 'city', city: c.city, nextStatus: 'paused' })
                              }
                              disabled={cityIsPaused}
                              className="text-xs px-2 py-1.5 rounded-lg bg-amber-500/15 text-amber-300 hover:bg-amber-500/25 disabled:opacity-30 transition-colors"
                            >
                              Pause all
                            </button>
                            <button
                              onClick={() =>
                                setPendingAction({ type: 'city', city: c.city, nextStatus: 'open' })
                              }
                              className="text-xs px-2 py-1.5 rounded-lg bg-emerald-500/15 text-emerald-300 hover:bg-emerald-500/25 transition-colors"
                            >
                              Open all
                            </button>
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
              <p className="text-xs text-slate-500 mt-2">
                Individual store pause/open lives on the{' '}
                <a href="/warehouses" className="text-indigo-400 hover:text-indigo-300">
                  Stores page
                </a>{' '}
                - this table updates the same status field, not a separate system.
              </p>
            </section>

            {/* Live Operations */}
            <section>
              <h2 className="text-sm font-semibold text-slate-300 mb-3">Live Operations</h2>
              {!liveOpsLoaded && <p className="text-slate-400 text-sm">Loading...</p>}
              {liveOpsLoaded && !liveOps && (
                <p className="text-red-400 text-sm">Failed to load live operations data.</p>
              )}
              {liveOpsLoaded && liveOps && (
                <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                  <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                    <p className="text-xs text-slate-500 mb-1">Active Orders</p>
                    <p className="text-lg font-semibold text-slate-100">{liveOps.active_orders}</p>
                  </div>
                  <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                    <p className="text-xs text-slate-500 mb-1">Picking</p>
                    <p className="text-lg font-semibold text-slate-100">{liveOps.picking}</p>
                  </div>
                  <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                    <p className="text-xs text-slate-500 mb-1">Packed</p>
                    <p className="text-lg font-semibold text-slate-100">{liveOps.packed}</p>
                  </div>
                  <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                    <p className="text-xs text-slate-500 mb-1">Ready for Pickup</p>
                    <p className="text-lg font-semibold text-slate-100">{liveOps.ready_for_pickup}</p>
                  </div>
                  <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                    <p className="text-xs text-slate-500 mb-1">Out for Delivery</p>
                    <p className="text-lg font-semibold text-slate-100">{liveOps.out_for_delivery}</p>
                  </div>
                  <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                    <p className="text-xs text-slate-500 mb-1">Active Riders</p>
                    <p className="text-lg font-semibold text-emerald-300">{liveOps.active_riders}</p>
                  </div>
                  <div className="border border-dashed border-slate-800 rounded-xl p-4 bg-slate-900/50 opacity-60">
                    <p className="text-xs text-slate-500 mb-1">Delayed</p>
                    <p className="text-xs font-medium text-slate-500">Not available yet</p>
                  </div>
                  <div className="border border-dashed border-slate-800 rounded-xl p-4 bg-slate-900/50 opacity-60">
                    <p className="text-xs text-slate-500 mb-1">Failed</p>
                    <p className="text-xs font-medium text-slate-500">Not available yet</p>
                  </div>
                </div>
              )}
            </section>

            {/* Support Overview */}
            <section>
              <h2 className="text-sm font-semibold text-slate-300 mb-3">Support Overview</h2>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Open</p>
                  <p className="text-lg font-semibold text-amber-300">{overview.open_support_tickets}</p>
                </div>
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">In progress</p>
                  <p className="text-lg font-semibold text-indigo-300">
                    {ticketsLoaded ? inProgressCount : '-'}
                  </p>
                </div>
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Unassigned</p>
                  <p className="text-lg font-semibold text-red-300">
                    {ticketsLoaded ? unassignedCount : '-'}
                  </p>
                </div>
                <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                  <p className="text-xs text-slate-500 mb-1">Total stores</p>
                  <p className="text-lg font-semibold text-slate-100">{overview.total_stores}</p>
                </div>
              </div>

              <div className="border border-slate-800 rounded-xl p-4 bg-slate-900">
                <p className="text-xs text-slate-500 mb-3">Open tickets by issue type</p>
                {!ticketsLoaded && <p className="text-sm text-slate-500">Loading...</p>}
                {ticketsLoaded && openTickets.length === 0 && (
                  <p className="text-sm text-slate-500">No open tickets.</p>
                )}
                {ticketsLoaded && openTickets.length > 0 && (
                  <div className="flex flex-wrap gap-2">
                    {Object.entries(issueCounts).map(([type, count]) => (
                      <span
                        key={type}
                        className="text-xs px-2.5 py-1.5 rounded-lg bg-slate-800 text-slate-300"
                      >
                        {ISSUE_TYPE_LABELS[type] ?? type}: <span className="text-slate-100 font-medium">{count}</span>
                      </span>
                    ))}
                  </div>
                )}
                <a href="/support" className="text-xs text-indigo-400 hover:text-indigo-300 mt-3 inline-block">
                  Open Support Tickets page {'->'}
                </a>
              </div>
            </section>
          </>
        )}
      </div>

      {pendingAction && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 w-full max-w-sm">
            <h3 className="text-sm font-semibold text-slate-200 mb-2">Confirm action</h3>
            <p className="text-sm text-slate-400 mb-5">
              {pendingAction.type === 'platform' &&
                (pendingAction.nextPaused
                  ? 'This blocks all new orders across the entire platform, immediately.'
                  : 'This resumes new orders across the entire platform, immediately.')}
              {pendingAction.type === 'city' &&
                `This sets every store in ${pendingAction.city} to "${pendingAction.nextStatus}".`}
            
              {pendingAction.type === 'cod' &&
                (pendingAction.nextEnabled
                  ? 'This re-enables Cash on Delivery platform-wide, immediately.'
                  : 'This disables Cash on Delivery platform-wide, immediately.')}
            </p>
            <label className="text-xs text-slate-400 block mb-1">Reason (required)</label>
            <textarea
              value={actionReason}
              onChange={(e) => setActionReason(e.target.value)}
              placeholder="Why is this action needed?"
              rows={2}
              className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm mb-3"
            />
            <p className="text-xs text-slate-500 mb-5">This action is recorded in the audit log.</p>
            <div className="flex gap-2">
              <button
                onClick={() => { setPendingAction(null); setActionReason('') }}
                disabled={isSubmitting}
                className="flex-1 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-sm transition-colors disabled:opacity-40"
              >
                Cancel
              </button>
              <button
                onClick={confirmAction}
                disabled={isSubmitting || !actionReason.trim()}
                className="flex-1 py-2 rounded-lg bg-red-500 hover:bg-red-400 text-white text-sm font-medium disabled:opacity-40 transition-colors"
              >
                {isSubmitting ? 'Applying...' : 'Confirm'}
              </button>
            </div>
          </div>
        </div>
      )}

      {toast && (
        <div
          className={`fixed bottom-6 right-6 px-4 py-3 rounded-lg text-sm font-medium shadow-lg z-50 ${
            toast.kind === 'success' ? 'bg-emerald-500 text-white' : 'bg-red-500 text-white'
          }`}
        >
          {toast.message}
        </div>
      )}
    </Layout>
  )
}
