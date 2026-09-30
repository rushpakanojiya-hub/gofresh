import { useEffect, useState } from 'react'
import apiClient from '../api/client'

interface MismatchItem {
  check_type: string
  severity: 'low' | 'medium' | 'high'
  entity_type: string
  entity_id: number
  order_id?: number
  description: string
  detected_amount?: number
  detected_at: string
  is_dismissed: boolean
}

interface MismatchSummary {
  total: number
  by_type: Record<string, number>
}

function formatCurrency(value: number) {
  return new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR', maximumFractionDigits: 0 }).format(value)
}

async function getMismatches(params: { check_type?: string; dismissed?: string }): Promise<{ mismatches: MismatchItem[]; summary: MismatchSummary }> {
  const { data } = await apiClient.get('/admin/finance/mismatch-center', { params })
  return data
}

async function dismissMismatch(payload: { check_type: string; entity_id: number; reason: string }) {
  const { data } = await apiClient.post('/admin/finance/mismatch-center/dismiss', payload)
  return data
}

async function undismissMismatch(payload: { check_type: string; entity_id: number }) {
  const { data } = await apiClient.post('/admin/finance/mismatch-center/undismiss', payload)
  return data
}

const SEVERITY_STYLE: Record<string, string> = {
  high: 'bg-red-600/15 text-red-400',
  medium: 'bg-amber-600/15 text-amber-400',
  low: 'bg-slate-700/30 text-slate-400',
}

const CHECK_TYPE_LABEL: Record<string, string> = {
  order_payment_amount: 'Order/Payment Amount',
  sale_not_posted: 'Sale Not Posted',
  refund_ledger_mismatch: 'Refund/Ledger Mismatch',
  bank_unmatched: 'Bank Unmatched',
  duplicate_payment_ref: 'Duplicate Payment Ref',
  refund_exceeds_payment: 'Refund Exceeds Payment',
}

export default function MismatchCenter() {
  const [items, setItems] = useState<MismatchItem[]>([])
  const [summary, setSummary] = useState<MismatchSummary | null>(null)
  const [checkTypeFilter, setCheckTypeFilter] = useState('')
  const [showDismissed, setShowDismissed] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [dismissingKey, setDismissingKey] = useState<string | null>(null)
  const [dismissReason, setDismissReason] = useState('')
  const [actionError, setActionError] = useState<string | null>(null)

  function load() {
    setIsLoading(true)
    setError(null)
    getMismatches({
      check_type: checkTypeFilter || undefined,
      dismissed: showDismissed ? undefined : 'false',
    })
      .then((res) => {
        setItems(res.mismatches ?? [])
        setSummary(res.summary ?? null)
      })
      .catch((err) => setError(err.response?.data?.error ?? 'Could not load mismatches.'))
      .finally(() => setIsLoading(false))
  }

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [checkTypeFilter, showDismissed])

  function keyFor(item: MismatchItem) {
    return `${item.check_type}-${item.entity_id}`
  }

  async function handleDismiss(item: MismatchItem) {
    if (!dismissReason.trim()) {
      setActionError('A reason is required to dismiss a mismatch.')
      return
    }
    setActionError(null)
    try {
      await dismissMismatch({ check_type: item.check_type, entity_id: item.entity_id, reason: dismissReason })
      setDismissingKey(null)
      setDismissReason('')
      load()
    } catch (err: any) {
      setActionError(err.response?.data?.error ?? 'Could not dismiss mismatch.')
    }
  }

  async function handleUndismiss(item: MismatchItem) {
    setActionError(null)
    try {
      await undismissMismatch({ check_type: item.check_type, entity_id: item.entity_id })
      load()
    } catch (err: any) {
      setActionError(err.response?.data?.error ?? 'Could not undismiss mismatch.')
    }
  }

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-lg font-semibold">Mismatch Center</h1>
          <p className="text-sm text-slate-500">
            Live-detected inconsistencies across orders, payments, ledger entries, and bank transactions.
          </p>
        </div>
        {summary && (
          <div className="text-right">
            <p className="text-2xl font-semibold">{summary.total}</p>
            <p className="text-xs text-slate-500">open mismatch{summary.total !== 1 ? 'es' : ''}</p>
          </div>
        )}
      </div>

      {summary && Object.keys(summary.by_type).length > 0 && (
        <div className="flex flex-wrap gap-2 mb-6">
          {Object.entries(summary.by_type).map(([type, count]) => (
            <button
              key={type}
              onClick={() => setCheckTypeFilter(checkTypeFilter === type ? '' : type)}
              className={`text-xs px-3 py-1.5 rounded-lg border transition-colors ${
                checkTypeFilter === type
                  ? 'border-emerald-600 bg-emerald-600/15 text-emerald-400'
                  : 'border-slate-800 text-slate-400 hover:text-slate-200'
              }`}
            >
              {CHECK_TYPE_LABEL[type] ?? type} ({count})
            </button>
          ))}
        </div>
      )}

      <div className="flex items-center gap-4 mb-4 text-sm">
        <label className="flex items-center gap-2 text-slate-400">
          <input type="checkbox" checked={showDismissed} onChange={(e) => setShowDismissed(e.target.checked)} />
          Show dismissed
        </label>
        {checkTypeFilter && (
          <button onClick={() => setCheckTypeFilter('')} className="text-emerald-400 hover:text-emerald-300">
            Clear filter
          </button>
        )}
      </div>

      {actionError && <p className="text-sm text-red-400 mb-3">{actionError}</p>}

      {isLoading && <p className="text-sm text-slate-500">Loading mismatches...</p>}
      {!isLoading && error && (
        <div className="border border-slate-800 rounded-xl p-6 text-sm text-slate-500">{error}</div>
      )}

      {!isLoading && !error && (
        <div className="border border-slate-800 rounded-xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-slate-900 text-slate-400 text-left">
                <th className="px-4 py-2 font-medium">Severity</th>
                <th className="px-4 py-2 font-medium">Check</th>
                <th className="px-4 py-2 font-medium">Description</th>
                <th className="px-4 py-2 font-medium text-right">Amount</th>
                <th className="px-4 py-2 font-medium text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {items.length === 0 && (
                <tr>
                  <td colSpan={5} className="px-4 py-6 text-center text-slate-500">
                    {showDismissed ? 'No mismatches found.' : 'No open mismatches. Everything reconciles.'}
                  </td>
                </tr>
              )}
              {items.map((item) => {
                const key = keyFor(item)
                return (
                  <tr key={key} className="border-t border-slate-800">
                    <td className="px-4 py-2">
                      <span className={`text-xs px-2 py-0.5 rounded-full ${SEVERITY_STYLE[item.severity] ?? SEVERITY_STYLE.low}`}>
                        {item.severity}
                      </span>
                    </td>
                    <td className="px-4 py-2 text-slate-400">{CHECK_TYPE_LABEL[item.check_type] ?? item.check_type}</td>
                    <td className="px-4 py-2">
                      {item.description}
                      {item.is_dismissed && <span className="ml-2 text-xs text-slate-500">(dismissed)</span>}
                    </td>
                    <td className="px-4 py-2 text-right">
                      {item.detected_amount ? formatCurrency(item.detected_amount) : '—'}
                    </td>
                    <td className="px-4 py-2 text-right">
                      {item.is_dismissed ? (
                        <button onClick={() => handleUndismiss(item)} className="text-xs text-emerald-400 hover:text-emerald-300">
                          Restore
                        </button>
                      ) : dismissingKey === key ? (
                        <div className="flex items-center justify-end gap-2">
                          <input
                            value={dismissReason}
                            onChange={(e) => setDismissReason(e.target.value)}
                            placeholder="Reason"
                            className="bg-slate-800 border border-slate-700 rounded-lg px-2 py-1 text-xs w-40"
                          />
                          <button onClick={() => handleDismiss(item)} className="text-xs text-emerald-400 hover:text-emerald-300">
                            Confirm
                          </button>
                          <button
                            onClick={() => { setDismissingKey(null); setDismissReason('') }}
                            className="text-xs text-slate-500 hover:text-slate-300"
                          >
                            Cancel
                          </button>
                        </div>
                      ) : (
                        <button
                          onClick={() => setDismissingKey(key)}
                          className="text-xs text-slate-500 hover:text-red-400"
                        >
                          Dismiss
                        </button>
                      )}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
