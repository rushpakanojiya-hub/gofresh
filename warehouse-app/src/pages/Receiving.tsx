import { useEffect, useState, useCallback } from 'react'
import {
  listReceivings,
  createReceiving,
  markReceived,
  qcReceiving,
  putAwayReceiving,
  listZones,
  listRacks,
  listBins,
} from '../api/warehouse'
import type { Receiving, WarehouseZone, WarehouseRack, WarehouseBin } from '../types/warehouse'
import StatusBadge from '../components/StatusBadge'
import { getErrorMessage } from '../utils/errors'

const STATUS_FILTERS: { value: string; label: string }[] = [
  { value: '', label: 'All' },
  { value: 'pending', label: 'Pending' },
  { value: 'received', label: 'Received' },
  { value: 'accepted', label: 'Accepted' },
  { value: 'rejected', label: 'Rejected' },
  { value: 'put_away', label: 'Put Away' },
]

export default function ReceivingPage() {
  const [receivings, setReceivings] = useState<Receiving[]>([])
  const [statusFilter, setStatusFilter] = useState('')
  const [page, setPage] = useState(1)
  const [totalPages, setTotalPages] = useState(1)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [showCreate, setShowCreate] = useState(false)
  const [supplierName, setSupplierName] = useState('')
  const [referenceNumber, setReferenceNumber] = useState('')
  const [productId, setProductId] = useState('')
  const [expectedQty, setExpectedQty] = useState('')
  const [createNotes, setCreateNotes] = useState('')
  const [isCreating, setIsCreating] = useState(false)

  const [selected, setSelected] = useState<Receiving | null>(null)
  const [isActing, setIsActing] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

  // Receive form state
  const [receivedQty, setReceivedQty] = useState('')
  const [damagedQty, setDamagedQty] = useState('')
  const [receiveNotes, setReceiveNotes] = useState('')

  // QC form state
  const [acceptedQty, setAcceptedQty] = useState('')
  const [rejectionReason, setRejectionReason] = useState('')

  // Put-away form state
  const [putAwayBinId, setPutAwayBinId] = useState('')
  const [putAwayZones, setPutAwayZones] = useState<WarehouseZone[]>([])
  const [putAwayRacks, setPutAwayRacks] = useState<WarehouseRack[]>([])
  const [putAwayBins, setPutAwayBins] = useState<WarehouseBin[]>([])
  const [putAwayZoneId, setPutAwayZoneId] = useState('')
  const [putAwayRackId, setPutAwayRackId] = useState('')
  const [isLoadingLocations, setIsLoadingLocations] = useState(false)

  const load = useCallback(async () => {
    setError(null)
    setIsLoading(true)
    try {
      const data = await listReceivings({ status: statusFilter || undefined, page, limit: 20 })
      setReceivings(data.receivings)
      setTotalPages(data.total_pages || 1)
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to load receiving records.'))
    } finally {
      setIsLoading(false)
    }
  }, [statusFilter, page])

  useEffect(() => {
    load()
  }, [load])

  useEffect(() => {
    setPage(1)
  }, [statusFilter])

  const openDetail = (rec: Receiving) => {
    setSelected(rec)
    setActionError(null)
    setReceivedQty(String(rec.expected_quantity))
    setDamagedQty('0')
    setReceiveNotes('')
    setAcceptedQty(String(rec.received_quantity))
    setRejectionReason('')
    setPutAwayBinId('')
    setPutAwayZoneId('')
    setPutAwayRackId('')
    setPutAwayRacks([])
    setPutAwayBins([])
    setIsLoadingLocations(true)
    listZones()
      .then((data) => setPutAwayZones(data.zones))
      .catch(() => setPutAwayZones([]))
      .finally(() => setIsLoadingLocations(false))
  }

  const submitCreate = async () => {
    if (!supplierName.trim() || !productId || !expectedQty) return
    setIsCreating(true)
    setError(null)
    try {
      await createReceiving({
        supplier_name: supplierName.trim(),
        reference_number: referenceNumber.trim() || undefined,
        product_id: parseInt(productId, 10),
        expected_quantity: parseInt(expectedQty, 10),
        notes: createNotes.trim() || undefined,
      })
      setSupplierName('')
      setReferenceNumber('')
      setProductId('')
      setExpectedQty('')
      setCreateNotes('')
      setShowCreate(false)
      await load()
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to create receiving record.'))
    } finally {
      setIsCreating(false)
    }
  }

  const submitReceive = async () => {
    if (!selected) return
    const rq = parseInt(receivedQty, 10)
    const dq = parseInt(damagedQty, 10) || 0
    if (isNaN(rq) || rq < 0) {
      setActionError('Enter a valid received quantity.')
      return
    }
    setIsActing(true)
    setActionError(null)
    try {
      const updated = await markReceived(selected.id, { received_quantity: rq, damaged_quantity: dq, notes: receiveNotes.trim() || undefined })
      setSelected(updated)
      setAcceptedQty(String(updated.received_quantity))
      await load()
    } catch (err) {
      setActionError(getErrorMessage(err, 'Failed to mark received.'))
    } finally {
      setIsActing(false)
    }
  }

  const submitQC = async (action: 'accept' | 'reject') => {
    if (!selected) return
    if (action === 'accept') {
      const aq = parseInt(acceptedQty, 10)
      if (isNaN(aq) || aq <= 0 || aq > selected.received_quantity) {
        setActionError(`Accepted quantity must be between 1 and ${selected.received_quantity}.`)
        return
      }
    } else if (!rejectionReason.trim()) {
      setActionError('Rejection reason is required.')
      return
    }
    setIsActing(true)
    setActionError(null)
    try {
      const updated = await qcReceiving(selected.id, {
        action,
        accepted_quantity: action === 'accept' ? parseInt(acceptedQty, 10) : undefined,
        rejection_reason: action === 'reject' ? rejectionReason.trim() : undefined,
      })
      setSelected(updated)
      await load()
    } catch (err) {
      setActionError(getErrorMessage(err, 'Failed to record QC decision.'))
    } finally {
      setIsActing(false)
    }
  }

  const submitPutAway = async () => {
    if (!selected) return
    setIsActing(true)
    setActionError(null)
    try {
      const binId = putAwayBinId ? parseInt(putAwayBinId, 10) : null
      const updated = await putAwayReceiving(selected.id, binId)
      setSelected(updated)
      await load()
    } catch (err) {
      setActionError(getErrorMessage(err, 'Failed to put away.'))
    } finally {
      setIsActing(false)
    }
  }

  const selectPutAwayZone = (zoneId: string) => {
    setPutAwayZoneId(zoneId)
    setPutAwayRackId('')
    setPutAwayBinId('')
    setPutAwayBins([])
    setPutAwayRacks([])
    if (!zoneId) return
    setIsLoadingLocations(true)
    listRacks(parseInt(zoneId, 10))
      .then((data) => setPutAwayRacks(data.racks))
      .catch(() => setPutAwayRacks([]))
      .finally(() => setIsLoadingLocations(false))
  }

  const selectPutAwayRack = (rackId: string) => {
    setPutAwayRackId(rackId)
    setPutAwayBinId('')
    setPutAwayBins([])
    if (!rackId) return
    setIsLoadingLocations(true)
    listBins(parseInt(rackId, 10))
      .then((data) => setPutAwayBins(data.bins))
      .catch(() => setPutAwayBins([]))
      .finally(() => setIsLoadingLocations(false))
  }

  return (
    <div className="p-6 max-w-6xl">
      <div className="flex items-center justify-between mb-6">
        <h1 className="font-display text-2xl font-semibold">Receiving / Inbound</h1>
        <div className="flex gap-2">
          <button onClick={load} className="text-xs px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 transition-colors">
            Refresh
          </button>
          <button
            onClick={() => setShowCreate(true)}
            className="text-xs px-3 py-1.5 rounded-lg bg-indigo-500/20 text-indigo-300 hover:bg-indigo-500/30 transition-colors"
          >
            + New Receiving
          </button>
        </div>
      </div>

      <div className="flex gap-1 mb-4">
        {STATUS_FILTERS.map((f) => (
          <button
            key={f.value}
            onClick={() => setStatusFilter(f.value)}
            className={`text-xs px-3 py-1.5 rounded-lg transition-colors ${
              statusFilter === f.value ? 'bg-indigo-500/20 text-indigo-300' : 'bg-slate-800 text-slate-400 hover:bg-slate-700'
            }`}
          >
            {f.label}
          </button>
        ))}
      </div>

      {error && (
        <div className="border border-rose-900 bg-rose-950/40 text-rose-300 text-sm rounded-lg px-4 py-3 mb-4">{error}</div>
      )}

      {isLoading && <p className="text-sm text-slate-400">Loading receiving records...</p>}

      {!isLoading && receivings.length === 0 && (
        <div className="border border-slate-800 rounded-xl bg-slate-900 p-8 text-center text-sm text-slate-500">
          No receiving records found for this filter.
        </div>
      )}

      {!isLoading && receivings.length > 0 && (
        <div className="border border-slate-800 rounded-xl bg-slate-900 overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-slate-800/50 text-slate-400 text-xs uppercase">
              <tr>
                <th className="text-left px-4 py-2.5">#</th>
                <th className="text-left px-4 py-2.5">Supplier</th>
                <th className="text-left px-4 py-2.5">Reference</th>
                <th className="text-left px-4 py-2.5">Product</th>
                <th className="text-right px-4 py-2.5">Expected</th>
                <th className="text-right px-4 py-2.5">Received</th>
                <th className="text-right px-4 py-2.5">Accepted</th>
                <th className="text-left px-4 py-2.5">Status</th>
                <th className="text-left px-4 py-2.5">Created</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800">
              {receivings.map((rec) => (
                <tr key={rec.id} onClick={() => openDetail(rec)} className="hover:bg-slate-800/30 cursor-pointer">
                  <td className="px-4 py-3">#{rec.id}</td>
                  <td className="px-4 py-3">{rec.supplier_name}</td>
                  <td className="px-4 py-3 text-slate-400">{rec.reference_number || '-'}</td>
                  <td className="px-4 py-3">{rec.product?.name ?? `#${rec.product_id}`}</td>
                  <td className="px-4 py-3 text-right">{rec.expected_quantity}</td>
                  <td className="px-4 py-3 text-right text-slate-400">{rec.received_quantity || '-'}</td>
                  <td className="px-4 py-3 text-right text-slate-400">{rec.accepted_quantity || '-'}</td>
                  <td className="px-4 py-3">
                    <StatusBadge status={rec.status} />
                  </td>
                  <td className="px-4 py-3 text-slate-500 text-xs">{new Date(rec.created_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {!isLoading && totalPages > 1 && (
        <div className="flex items-center justify-between mt-4">
          <button
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page <= 1}
            className="text-xs px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 disabled:opacity-40"
          >
            Previous
          </button>
          <span className="text-xs text-slate-500">
            Page {page} of {totalPages}
          </span>
          <button
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            disabled={page >= totalPages}
            className="text-xs px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 disabled:opacity-40"
          >
            Next
          </button>
        </div>
      )}

      {/* Create modal */}
      {showCreate && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center p-4 z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 w-full max-w-md">
            <h2 className="text-sm font-semibold mb-4">New Receiving</h2>
            <div className="space-y-3">
              <div>
                <label className="text-xs text-slate-500 block mb-1">Supplier name</label>
                <input
                  value={supplierName}
                  onChange={(e) => setSupplierName(e.target.value)}
                  className="w-full text-sm bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-slate-200 focus:outline-none focus:border-indigo-500"
                />
              </div>
              <div>
                <label className="text-xs text-slate-500 block mb-1">Reference number (optional)</label>
                <input
                  value={referenceNumber}
                  onChange={(e) => setReferenceNumber(e.target.value)}
                  placeholder="PO-2026-001"
                  className="w-full text-sm bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-slate-200 focus:outline-none focus:border-indigo-500"
                />
              </div>
              <div className="grid grid-cols-2 gap-2">
                <div>
                  <label className="text-xs text-slate-500 block mb-1">Product ID</label>
                  <input
                    value={productId}
                    onChange={(e) => setProductId(e.target.value)}
                    className="w-full text-sm bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-slate-200 focus:outline-none focus:border-indigo-500"
                  />
                </div>
                <div>
                  <label className="text-xs text-slate-500 block mb-1">Expected quantity</label>
                  <input
                    type="number"
                    min={1}
                    value={expectedQty}
                    onChange={(e) => setExpectedQty(e.target.value)}
                    className="w-full text-sm bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-slate-200 focus:outline-none focus:border-indigo-500"
                  />
                </div>
              </div>
              <div>
                <label className="text-xs text-slate-500 block mb-1">Notes (optional)</label>
                <textarea
                  value={createNotes}
                  onChange={(e) => setCreateNotes(e.target.value)}
                  rows={2}
                  className="w-full text-sm bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-slate-200 focus:outline-none focus:border-indigo-500"
                />
              </div>
            </div>
            <div className="flex gap-2 justify-end mt-4">
              <button onClick={() => setShowCreate(false)} disabled={isCreating} className="text-xs px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700">
                Cancel
              </button>
              <button
                onClick={submitCreate}
                disabled={isCreating || !supplierName.trim() || !productId || !expectedQty}
                className="text-xs px-3 py-1.5 rounded-lg bg-indigo-500/20 text-indigo-300 hover:bg-indigo-500/30 disabled:opacity-50"
              >
                {isCreating ? 'Creating...' : 'Create'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Detail / action modal */}
      {selected && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center p-4 z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 w-full max-w-lg max-h-[90vh] overflow-y-auto">
            <div className="flex items-center justify-between mb-1">
              <h2 className="text-sm font-semibold">Receiving #{selected.id}</h2>
              <StatusBadge status={selected.status} />
            </div>
            <p className="text-xs text-slate-500 mb-4">
              {selected.supplier_name} {selected.reference_number ? `ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â· ${selected.reference_number}` : ''} ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â·{' '}
              {selected.product?.name ?? `Product #${selected.product_id}`}
            </p>

            {actionError && (
              <div className="border border-rose-900 bg-rose-950/40 text-rose-300 text-xs rounded-lg px-3 py-2 mb-3">{actionError}</div>
            )}

            <div className="text-xs text-slate-400 space-y-1 mb-4 bg-slate-950/50 border border-slate-800 rounded-lg p-3">
              <p>Expected: <span className="text-slate-200">{selected.expected_quantity}</span></p>
              {selected.received_quantity > 0 && (
                <p>
                  Received: <span className="text-slate-200">{selected.received_quantity}</span> &middot; Damaged:{' '}
                  <span className="text-slate-200">{selected.damaged_quantity}</span>
                </p>
              )}
              {selected.accepted_quantity > 0 && <p>Accepted: <span className="text-slate-200">{selected.accepted_quantity}</span></p>}
              {selected.rejection_reason && <p className="text-rose-400">Rejected: {selected.rejection_reason}</p>}
              {selected.bin?.name && <p>Bin: <span className="text-slate-200">{selected.bin.name}</span></p>}
            </div>

            {selected.status === 'pending' && (
              <div className="space-y-3">
                <h3 className="text-xs font-medium text-slate-300">Mark Received</h3>
                <div className="grid grid-cols-2 gap-2">
                  <div>
                    <label className="text-xs text-slate-500 block mb-1">Received qty</label>
                    <input
                      type="number"
                      min={0}
                      value={receivedQty}
                      onChange={(e) => setReceivedQty(e.target.value)}
                      className="w-full text-sm bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-slate-200"
                    />
                  </div>
                  <div>
                    <label className="text-xs text-slate-500 block mb-1">Damaged qty</label>
                    <input
                      type="number"
                      min={0}
                      value={damagedQty}
                      onChange={(e) => setDamagedQty(e.target.value)}
                      className="w-full text-sm bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-slate-200"
                    />
                  </div>
                </div>
                <textarea
                  value={receiveNotes}
                  onChange={(e) => setReceiveNotes(e.target.value)}
                  placeholder="Notes (optional)"
                  rows={2}
                  className="w-full text-sm bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-slate-200"
                />
                <button
                  onClick={submitReceive}
                  disabled={isActing}
                  className="w-full text-xs px-3 py-2 rounded-lg bg-indigo-500/20 text-indigo-300 hover:bg-indigo-500/30 disabled:opacity-50"
                >
                  {isActing ? 'Saving...' : 'Confirm Received'}
                </button>
              </div>
            )}

            {selected.status === 'received' && (
              <div className="space-y-3">
                <h3 className="text-xs font-medium text-slate-300">Quality Check</h3>
                <div>
                  <label className="text-xs text-slate-500 block mb-1">Accepted qty (out of {selected.received_quantity})</label>
                  <input
                    type="number"
                    min={1}
                    max={selected.received_quantity}
                    value={acceptedQty}
                    onChange={(e) => setAcceptedQty(e.target.value)}
                    className="w-full text-sm bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-slate-200"
                  />
                </div>
                <button
                  onClick={() => submitQC('accept')}
                  disabled={isActing}
                  className="w-full text-xs px-3 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white disabled:opacity-50"
                >
                  {isActing ? 'Saving...' : 'Accept'}
                </button>
                <div className="border-t border-slate-800 pt-3">
                  <input
                    value={rejectionReason}
                    onChange={(e) => setRejectionReason(e.target.value)}
                    placeholder="Rejection reason"
                    className="w-full text-sm bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-slate-200 mb-2"
                  />
                  <button
                    onClick={() => submitQC('reject')}
                    disabled={isActing}
                    className="w-full text-xs px-3 py-2 rounded-lg bg-rose-600/80 hover:bg-rose-600 text-white disabled:opacity-50"
                  >
                    Reject
                  </button>
                </div>
              </div>
            )}

            {selected.status === 'accepted' && (
              <div className="space-y-3">
                <h3 className="text-xs font-medium text-slate-300">Put Away</h3>
                <p className="text-xs text-slate-500 -mt-2">Select a location, or skip to put away without one.</p>
                <div className="grid grid-cols-3 gap-2">
                  <div>
                    <label className="text-xs text-slate-500 block mb-1">Zone</label>
                    <select
                      value={putAwayZoneId}
                      onChange={(e) => selectPutAwayZone(e.target.value)}
                      className="w-full text-sm bg-slate-800 border border-slate-700 rounded-lg px-2 py-2 text-slate-200"
                    >
                      <option value="">Select zone...</option>
                      {putAwayZones.map((z) => (
                        <option key={z.id} value={z.id}>{z.name}</option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label className="text-xs text-slate-500 block mb-1">Rack</label>
                    <select
                      value={putAwayRackId}
                      onChange={(e) => selectPutAwayRack(e.target.value)}
                      disabled={!putAwayZoneId}
                      className="w-full text-sm bg-slate-800 border border-slate-700 rounded-lg px-2 py-2 text-slate-200 disabled:opacity-40"
                    >
                      <option value="">Select rack...</option>
                      {putAwayRacks.map((r) => (
                        <option key={r.id} value={r.id}>{r.name}</option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label className="text-xs text-slate-500 block mb-1">Bin</label>
                    <select
                      value={putAwayBinId}
                      onChange={(e) => setPutAwayBinId(e.target.value)}
                      disabled={!putAwayRackId}
                      className="w-full text-sm bg-slate-800 border border-slate-700 rounded-lg px-2 py-2 text-slate-200 disabled:opacity-40"
                    >
                      <option value="">Select bin...</option>
                      {putAwayBins.map((b) => (
                        <option key={b.id} value={b.id}>{b.name}</option>
                      ))}
                    </select>
                  </div>
                </div>
                {isLoadingLocations && <p className="text-xs text-slate-500">Loading...</p>}
                {putAwayZones.length === 0 && !isLoadingLocations && (
                  <p className="text-xs text-amber-400">No zones set up yet - you can still put away without a bin.</p>
                )}
                <button
                  onClick={submitPutAway}
                  disabled={isActing}
                  className="w-full text-xs px-3 py-2 rounded-lg bg-indigo-500/20 text-indigo-300 hover:bg-indigo-500/30 disabled:opacity-50"
                >
                  {isActing ? 'Saving...' : `Put Away (adds ${selected.accepted_quantity} to inventory)`}
                </button>
              </div>
            )}

            {(selected.status === 'put_away' || selected.status === 'rejected') && (
              <p className="text-xs text-slate-500">This record is finalized ÃƒÆ’Ã†â€™Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€šÃ‚Â no further action available.</p>
            )}

            <button onClick={() => setSelected(null)} className="w-full text-xs px-3 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 mt-3">
              Close
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
