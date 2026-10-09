import { useCallback, useEffect, useState } from 'react'
import { createReceiving, listReceivings, markReceived, qcReceiving } from '../api/warehouse'
import type { Receiving as ReceivingRow, ReceivingStatus } from '../types/warehouse'
import { getErrorMessage } from '../utils/errors'

type Tab = 'all' | ReceivingStatus

const TABS: { label: string; value: Tab }[] = [
  { label: 'All', value: 'all' },
  { label: 'Pending', value: 'pending' },
  { label: 'Received', value: 'received' },
  { label: 'Accepted', value: 'accepted' },
  { label: 'Put Away', value: 'put_away' },
  { label: 'Rejected', value: 'rejected' },
]

const STATUS_TONE: Record<string, string> = {
  pending: 'text-slate-300',
  received: 'text-sky-300',
  accepted: 'text-amber-300',
  put_away: 'text-emerald-300',
  rejected: 'text-rose-300',
}

const LIMIT = 20

type Modal =
  | { kind: 'create' }
  | { kind: 'receive'; row: ReceivingRow }
  | { kind: 'qc'; row: ReceivingRow }
  | null

const inputCls = 'w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm mb-3'

export default function Receiving() {
  const [tab, setTab] = useState<Tab>('all')
  const [page, setPage] = useState(1)
  const [rows, setRows] = useState<ReceivingRow[]>([])
  const [totalPages, setTotalPages] = useState(1)
  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  const [modal, setModal] = useState<Modal>(null)
  const [saving, setSaving] = useState(false)
  const [modalError, setModalError] = useState<string | null>(null)

  // form fields
  const [supplier, setSupplier] = useState('')
  const [reference, setReference] = useState('')
  const [productId, setProductId] = useState('')
  const [expected, setExpected] = useState('')
  const [receivedQty, setReceivedQty] = useState('')
  const [damagedQty, setDamagedQty] = useState('0')
  const [notes, setNotes] = useState('')
  const [qcAction, setQcAction] = useState<'accept' | 'reject'>('accept')
  const [acceptedQty, setAcceptedQty] = useState('')
  const [reason, setReason] = useState('')

  const load = useCallback(async () => {
    try {
      const res = await listReceivings({
        status: tab === 'all' ? undefined : tab,
        page,
        limit: LIMIT,
      })
      setRows(res.receivings)
      setTotalPages(res.total_pages || 1)
      setError(null)
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to load receivings.'))
    } finally {
      setIsLoading(false)
    }
  }, [tab, page])

  useEffect(() => {
    setIsLoading(true)
    load()
  }, [load])

  function open(m: Modal) {
    setModal(m)
    setModalError(null)
    setSupplier('')
    setReference('')
    setProductId('')
    setExpected('')
    setNotes('')
    setQcAction('accept')
    setReason('')
    if (m && m.kind === 'receive') {
      setReceivedQty(String(m.row.expected_quantity))
      setDamagedQty('0')
    }
    if (m && m.kind === 'qc') {
      setAcceptedQty(String(Math.max(m.row.received_quantity - m.row.damaged_quantity, 0)))
    }
  }

  async function submit() {
    if (!modal) return
    setSaving(true)
    setModalError(null)
    try {
      if (modal.kind === 'create') {
        const pid = Number(productId)
        const exp = Number(expected)
        if (!supplier.trim() || !pid || !exp || exp <= 0) {
          throw new Error('Supplier, product ID and expected quantity are required.')
        }
        await createReceiving({
          supplier_name: supplier.trim(),
          reference_number: reference.trim() || undefined,
          product_id: pid,
          expected_quantity: exp,
        })
      } else if (modal.kind === 'receive') {
        const rq = Number(receivedQty)
        const dq = Number(damagedQty)
        if (isNaN(rq) || rq < 0 || isNaN(dq) || dq < 0 || dq > rq) {
          throw new Error('Check quantities: damaged cannot exceed received.')
        }
        await markReceived(modal.row.id, {
          received_quantity: rq,
          damaged_quantity: dq,
          notes: notes.trim() || undefined,
        })
      } else {
        if (qcAction === 'accept') {
          const aq = Number(acceptedQty)
          if (isNaN(aq) || aq <= 0) throw new Error('Accepted quantity must be more than 0.')
          await qcReceiving(modal.row.id, { action: 'accept', accepted_quantity: aq })
        } else {
          if (!reason.trim()) throw new Error('Rejection reason is required.')
          await qcReceiving(modal.row.id, { action: 'reject', rejection_reason: reason.trim() })
        }
      }
      setModal(null)
      await load()
    } catch (err) {
      setModalError(getErrorMessage(err, 'Action failed.'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="p-6 max-w-6xl">
      <div className="flex items-center justify-between mb-4">
        <div>
          <p className="font-mono text-[10px] tracking-widest text-red-500 uppercase mb-1">Inbound</p>
          <h1 className="font-display text-2xl font-semibold">Receiving</h1>
        </div>
        <div className="flex gap-2">
          <button
            onClick={load}
            className="text-xs px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 border border-slate-700 transition-colors"
          >
            Refresh
          </button>
          <button
            onClick={() => open({ kind: 'create' })}
            className="text-xs px-3 py-1.5 rounded-lg bg-red-600 hover:bg-red-500 transition-colors"
          >
            New Receiving
          </button>
        </div>
      </div>

      <div className="flex gap-1 border-b border-slate-800 mb-4 overflow-x-auto">
        {TABS.map((t) => (
          <button
            key={t.value}
            onClick={() => {
              setTab(t.value)
              setPage(1)
            }}
            className={`px-3 py-2 text-sm border-b-2 whitespace-nowrap transition-colors ${
              tab === t.value
                ? 'border-red-400 text-red-300 font-medium'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {error && (
        <div className="border border-rose-900 bg-rose-950/40 text-rose-300 text-sm rounded-lg px-4 py-3 mb-4">
          {error}
        </div>
      )}
      {isLoading && <p className="text-sm text-slate-400">Loading...</p>}

      {!isLoading && rows.length === 0 && (
        <div className="border border-slate-800 rounded-xl bg-slate-900 p-8 text-center text-sm text-slate-500">
          No receivings here.
        </div>
      )}

      {rows.length > 0 && (
        <div className="border border-slate-800 rounded-xl bg-slate-900 overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-slate-800/50 text-slate-400 text-xs uppercase">
              <tr>
                <th className="text-left px-4 py-2.5">#</th>
                <th className="text-left px-4 py-2.5">Product</th>
                <th className="text-left px-4 py-2.5">Supplier</th>
                <th className="text-right px-4 py-2.5">Expected</th>
                <th className="text-right px-4 py-2.5">Received</th>
                <th className="text-right px-4 py-2.5">Damaged</th>
                <th className="text-right px-4 py-2.5">Accepted</th>
                <th className="text-left px-4 py-2.5">Status</th>
                <th className="text-right px-4 py-2.5">Action</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800">
              {rows.map((r) => (
                <tr key={r.id} className="hover:bg-slate-800/30">
                  <td className="px-4 py-3 font-medium">#{r.id}</td>
                  <td className="px-4 py-3">{r.product?.name ?? `Product #${r.product_id}`}</td>
                  <td className="px-4 py-3 text-slate-400">
                    {r.supplier_name}
                    {r.reference_number && (
                      <span className="block text-xs text-slate-500">{r.reference_number}</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right">{r.expected_quantity}</td>
                  <td className="px-4 py-3 text-right">{r.received_quantity}</td>
                  <td className="px-4 py-3 text-right text-slate-400">{r.damaged_quantity}</td>
                  <td className="px-4 py-3 text-right">{r.accepted_quantity}</td>
                  <td className="px-4 py-3">
                    <span className={`text-xs uppercase ${STATUS_TONE[r.status] ?? ''}`}>
                      {r.status.replace('_', ' ')}
                    </span>
                    {r.status === 'rejected' && r.rejection_reason && (
                      <span className="block text-xs text-slate-500">{r.rejection_reason}</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right">
                    {r.status === 'pending' && (
                      <button
                        onClick={() => open({ kind: 'receive', row: r })}
                        className="text-xs px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 border border-slate-700 transition-colors"
                      >
                        Mark Received
                      </button>
                    )}
                    {r.status === 'received' && (
                      <button
                        onClick={() => open({ kind: 'qc', row: r })}
                        className="text-xs px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 border border-slate-700 transition-colors"
                      >
                        QC
                      </button>
                    )}
                    {r.status === 'accepted' && (
                      <span className="text-xs text-slate-500">See Putaway</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {totalPages > 1 && (
        <div className="flex items-center justify-end gap-2 mt-4 text-sm">
          <button
            disabled={page <= 1}
            onClick={() => setPage((p) => p - 1)}
            className="px-3 py-1.5 rounded-lg bg-slate-800 disabled:opacity-40"
          >
            Prev
          </button>
          <span className="text-slate-400">
            {page} / {totalPages}
          </span>
          <button
            disabled={page >= totalPages}
            onClick={() => setPage((p) => p + 1)}
            className="px-3 py-1.5 rounded-lg bg-slate-800 disabled:opacity-40"
          >
            Next
          </button>
        </div>
      )}

      {modal && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl w-full max-w-sm p-6">
            {modal.kind === 'create' && (
              <>
                <h2 className="text-base font-semibold mb-4">New Receiving</h2>
                <input className={inputCls} placeholder="Supplier name" value={supplier} onChange={(e) => setSupplier(e.target.value)} />
                <input className={inputCls} placeholder="Reference / invoice no. (optional)" value={reference} onChange={(e) => setReference(e.target.value)} />
                <input className={inputCls} placeholder="Product ID" inputMode="numeric" value={productId} onChange={(e) => setProductId(e.target.value)} />
                <input className={inputCls} placeholder="Expected quantity" inputMode="numeric" value={expected} onChange={(e) => setExpected(e.target.value)} />
              </>
            )}
            {modal.kind === 'receive' && (
              <>
                <h2 className="text-base font-semibold mb-1">Receive #{modal.row.id}</h2>
                <p className="text-xs text-slate-500 mb-4">
                  {modal.row.product?.name ?? `Product #${modal.row.product_id}`} &middot; expected {modal.row.expected_quantity}
                </p>
                <label className="text-xs text-slate-500">Received quantity</label>
                <input className={inputCls} inputMode="numeric" value={receivedQty} onChange={(e) => setReceivedQty(e.target.value)} />
                <label className="text-xs text-slate-500">Damaged quantity</label>
                <input className={inputCls} inputMode="numeric" value={damagedQty} onChange={(e) => setDamagedQty(e.target.value)} />
                <input className={inputCls} placeholder="Notes (optional)" value={notes} onChange={(e) => setNotes(e.target.value)} />
              </>
            )}
            {modal.kind === 'qc' && (
              <>
                <h2 className="text-base font-semibold mb-1">QC #{modal.row.id}</h2>
                <p className="text-xs text-slate-500 mb-4">
                  Received {modal.row.received_quantity}, damaged {modal.row.damaged_quantity}
                </p>
                <div className="flex gap-2 mb-3">
                  {(['accept', 'reject'] as const).map((a) => (
                    <button
                      key={a}
                      onClick={() => setQcAction(a)}
                      className={`flex-1 text-sm py-1.5 rounded-lg border transition-colors ${
                        qcAction === a
                          ? 'border-red-400 text-red-300 bg-red-950/30'
                          : 'border-slate-700 text-slate-400'
                      }`}
                    >
                      {a === 'accept' ? 'Accept' : 'Reject'}
                    </button>
                  ))}
                </div>
                {qcAction === 'accept' ? (
                  <>
                    <label className="text-xs text-slate-500">Accepted quantity</label>
                    <input className={inputCls} inputMode="numeric" value={acceptedQty} onChange={(e) => setAcceptedQty(e.target.value)} />
                    <p className="text-xs text-slate-500 mb-3">A putaway task is created automatically.</p>
                  </>
                ) : (
                  <input className={inputCls} placeholder="Rejection reason" value={reason} onChange={(e) => setReason(e.target.value)} />
                )}
              </>
            )}

            {modalError && (
              <div className="border border-rose-900 bg-rose-950/40 text-rose-300 text-sm rounded-lg px-3 py-2 mb-3">
                {modalError}
              </div>
            )}
            <div className="flex justify-end gap-2">
              <button onClick={() => setModal(null)} className="text-sm px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 transition-colors">
                Cancel
              </button>
              <button onClick={submit} disabled={saving} className="text-sm px-3 py-1.5 rounded-lg bg-red-600 hover:bg-red-500 disabled:opacity-40 transition-colors">
                {saving ? 'Saving...' : 'Submit'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}