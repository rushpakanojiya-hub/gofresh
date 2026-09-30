import { useEffect, useState } from 'react'
import Layout from '../components/Layout'
import { getActiveDeliveries, getRiderWorkload, getFailedDeliveries } from '../api/admin'
import type { ActiveDelivery, RiderWorkload, FailedDelivery } from '../types/admin'

type Tab = 'active' | 'workload' | 'failed'

export default function DeliveryManagement() {
  const [tab, setTab] = useState<Tab>('active')

  const [activeDeliveries, setActiveDeliveries] = useState<ActiveDelivery[]>([])
  const [activeCount, setActiveCount] = useState(0)
  const [isLoadingActive, setIsLoadingActive] = useState(true)
  const [activeError, setActiveError] = useState<string | null>(null)

  const [riders, setRiders] = useState<RiderWorkload[]>([])
  const [isLoadingRiders, setIsLoadingRiders] = useState(true)
  const [ridersError, setRidersError] = useState<string | null>(null)

  const [failedDeliveries, setFailedDeliveries] = useState<FailedDelivery[]>([])
  const [failedCount, setFailedCount] = useState(0)
  const [isLoadingFailed, setIsLoadingFailed] = useState(true)
  const [failedError, setFailedError] = useState<string | null>(null)

  async function loadActive() {
    setIsLoadingActive(true)
    setActiveError(null)
    try {
      const res = await getActiveDeliveries()
      setActiveDeliveries(res.deliveries ?? [])
      setActiveCount(res.active_count ?? 0)
    } catch (err: any) {
      setActiveError(err.response?.data?.error ?? 'Failed to load active deliveries.')
    } finally {
      setIsLoadingActive(false)
    }
  }

  async function loadRiders() {
    setIsLoadingRiders(true)
    setRidersError(null)
    try {
      const res = await getRiderWorkload()
      setRiders(res.riders ?? [])
    } catch (err: any) {
      setRidersError(err.response?.data?.error ?? 'Failed to load rider workload.')
    } finally {
      setIsLoadingRiders(false)
    }
  }

  async function loadFailed() {
    setIsLoadingFailed(true)
    setFailedError(null)
    try {
      const res = await getFailedDeliveries()
      setFailedDeliveries(res.deliveries ?? [])
      setFailedCount(res.failed_count ?? 0)
    } catch (err: any) {
      setFailedError(err.response?.data?.error ?? 'Failed to load failed deliveries.')
    } finally {
      setIsLoadingFailed(false)
    }
  }

  useEffect(() => {
    loadActive()
    loadRiders()
    loadFailed()
  }, [])

  function formatDate(value: string) {
    return new Date(value).toLocaleString()
  }

  const TABS: { key: Tab; label: string }[] = [
    { key: 'active', label: `Active Deliveries (${activeCount})` },
    { key: 'workload', label: 'Rider Workload' },
    { key: 'failed', label: `Failed/Returned (${failedCount})` },
  ]

  return (
    <Layout>
      <div className="p-8">
        <div className="mb-6">
          <h1 className="text-xl font-semibold">Delivery Management</h1>
          <p className="text-sm text-slate-400 mt-1">
            Live in-flight deliveries, rider workload, and failed/returned orders.
          </p>
        </div>

        <div className="flex gap-2 mb-6 border-b border-slate-800">
          {TABS.map((t) => (
            <button
              key={t.key}
              onClick={() => setTab(t.key)}
              className={
                'px-4 py-2 text-sm font-medium border-b-2 transition-colors ' +
                (tab === t.key
                  ? 'border-indigo-500 text-indigo-300'
                  : 'border-transparent text-slate-500 hover:text-slate-300')
              }
            >
              {t.label}
            </button>
          ))}
        </div>

        {tab === 'active' && (
          <>
            {isLoadingActive && <p className="text-slate-400">Loading...</p>}
            {activeError && <p className="text-red-400">{activeError}</p>}
            {!isLoadingActive && !activeError && activeDeliveries.length === 0 && (
              <div className="border border-dashed border-slate-800 rounded-xl p-10 text-center text-slate-500">
                No active deliveries right now.
              </div>
            )}
            {!isLoadingActive && activeDeliveries.length > 0 && (
              <div className="border border-slate-800 rounded-xl overflow-hidden">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="bg-slate-900 text-slate-400 text-left">
                      <th className="px-4 py-3 font-medium">Order</th>
                      <th className="px-4 py-3 font-medium">Order Status</th>
                      <th className="px-4 py-3 font-medium">Delivery Status</th>
                      <th className="px-4 py-3 font-medium">Rider</th>
                      <th className="px-4 py-3 font-medium">Area</th>
                      <th className="px-4 py-3 font-medium">Updated</th>
                    </tr>
                  </thead>
                  <tbody>
                    {activeDeliveries.map((d) => (
                      <tr key={d.order_id} className="border-t border-slate-800">
                        <td className="px-4 py-3 text-slate-200">#{d.order_id}</td>
                        <td className="px-4 py-3 text-slate-400">{d.order_status.replace('_', ' ')}</td>
                        <td className="px-4 py-3">
                          <span className="px-2 py-0.5 rounded-md text-xs font-medium bg-indigo-500/15 text-indigo-300">
                            {d.delivery_status.replace('_', ' ')}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-slate-300">
                          {d.partner_name} <span className="text-slate-500 text-xs">({d.partner_phone})</span>
                        </td>
                        <td className="px-4 py-3 text-slate-400">{d.customer_area}</td>
                        <td className="px-4 py-3 text-slate-500 text-xs">{formatDate(d.assigned_at)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </>
        )}

        {tab === 'workload' && (
          <>
            {isLoadingRiders && <p className="text-slate-400">Loading...</p>}
            {ridersError && <p className="text-red-400">{ridersError}</p>}
            {!isLoadingRiders && riders.length > 0 && (
              <div className="border border-slate-800 rounded-xl overflow-hidden">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="bg-slate-900 text-slate-400 text-left">
                      <th className="px-4 py-3 font-medium">Rider</th>
                      <th className="px-4 py-3 font-medium">Status</th>
                      <th className="px-4 py-3 font-medium">Active Orders</th>
                      <th className="px-4 py-3 font-medium">Capacity</th>
                      <th className="px-4 py-3 font-medium">Load</th>
                    </tr>
                  </thead>
                  <tbody>
                    {riders.map((r) => {
                      const loadPct = r.max_active_orders > 0 ? Math.round((r.active_orders / r.max_active_orders) * 100) : 0
                      const isFull = r.active_orders >= r.max_active_orders && r.max_active_orders > 0
                      return (
                        <tr key={r.partner_id} className="border-t border-slate-800">
                          <td className="px-4 py-3 text-slate-200">
                            {r.name} <span className="text-slate-500 text-xs">({r.phone})</span>
                          </td>
                          <td className="px-4 py-3">
                            <span
                              className={
                                'px-2 py-0.5 rounded-md text-xs font-medium mr-1 ' +
                                (r.is_online ? 'bg-emerald-500/15 text-emerald-300' : 'bg-slate-700/50 text-slate-400')
                              }
                            >
                              {r.is_online ? 'Online' : 'Offline'}
                            </span>
                            {!r.is_active && (
                              <span className="px-2 py-0.5 rounded-md text-xs font-medium bg-red-500/15 text-red-300">
                                Inactive
                              </span>
                            )}
                          </td>
                          <td className="px-4 py-3 text-slate-300">{r.active_orders}</td>
                          <td className="px-4 py-3 text-slate-400">{r.max_active_orders}</td>
                          <td className="px-4 py-3">
                            <span
                              className={
                                'px-2 py-0.5 rounded-md text-xs font-medium ' +
                                (isFull ? 'bg-red-500/15 text-red-300' : 'bg-slate-800 text-slate-400')
                              }
                            >
                              {loadPct}%
                            </span>
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </>
        )}

        {tab === 'failed' && (
          <>
            {isLoadingFailed && <p className="text-slate-400">Loading...</p>}
            {failedError && <p className="text-red-400">{failedError}</p>}
            {!isLoadingFailed && !failedError && failedDeliveries.length === 0 && (
              <div className="border border-dashed border-slate-800 rounded-xl p-10 text-center text-slate-500">
                No failed/returned deliveries.
              </div>
            )}
            {!isLoadingFailed && failedDeliveries.length > 0 && (
              <div className="border border-slate-800 rounded-xl overflow-hidden">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="bg-slate-900 text-slate-400 text-left">
                      <th className="px-4 py-3 font-medium">Order</th>
                      <th className="px-4 py-3 font-medium">Rider</th>
                      <th className="px-4 py-3 font-medium">Area</th>
                      <th className="px-4 py-3 font-medium">Returned At</th>
                    </tr>
                  </thead>
                  <tbody>
                    {failedDeliveries.map((f) => (
                      <tr key={f.order_id} className="border-t border-slate-800">
                        <td className="px-4 py-3 text-slate-200">#{f.order_id}</td>
                        <td className="px-4 py-3 text-slate-400">{f.partner_name || '-'}</td>
                        <td className="px-4 py-3 text-slate-400">{f.customer_area}</td>
                        <td className="px-4 py-3 text-slate-500 text-xs">{formatDate(f.returned_at)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </>
        )}
      </div>
    </Layout>
  )
}
