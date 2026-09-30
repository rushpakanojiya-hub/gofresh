import { useEffect, useState } from 'react'
import Layout from '../components/Layout'
import {
  listSupportTickets,
  getSupportTicketMessages,
  replyToSupportTicket,
  updateSupportTicketStatus,
  updateSupportTicket,
  listAdminStaff,
  creditWallet,
} from '../api/admin'
import type { SupportTicket, SupportMessage } from '../types/admin'

const STATUS_OPTIONS = ['open', 'in_progress', 'resolved', 'closed']

const STATUS_STYLES: Record<string, string> = {
  open: 'bg-amber-500/15 text-amber-300',
  in_progress: 'bg-indigo-500/15 text-indigo-300',
  resolved: 'bg-emerald-500/15 text-emerald-300',
  closed: 'bg-slate-700/50 text-slate-400',
}

const ISSUE_TYPE_OPTIONS = [
  { value: 'other', label: 'Other' },
  { value: 'missing_item', label: 'Missing item' },
  { value: 'wrong_item', label: 'Wrong item' },
  { value: 'damaged_item', label: 'Damaged item' },
  { value: 'expired_item', label: 'Expired item' },
  { value: 'delivery_issue', label: 'Delivery issue' },
  { value: 'payment_issue', label: 'Payment issue' },
  { value: 'refund_issue', label: 'Refund issue' },
]

const PRIORITY_OPTIONS = [
  { value: 'low', label: 'Low' },
  { value: 'normal', label: 'Normal' },
  { value: 'high', label: 'High' },
  { value: 'urgent', label: 'Urgent' },
]

export default function SupportTickets() {
  const [tickets, setTickets] = useState<SupportTicket[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [statusFilter, setStatusFilter] = useState('')

  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [messages, setMessages] = useState<SupportMessage[]>([])
  const [isLoadingThread, setIsLoadingThread] = useState(false)
  const [replyText, setReplyText] = useState('')
  const [isSending, setIsSending] = useState(false)

  const [staff, setStaff] = useState<any[]>([])

  const [showCreditModal, setShowCreditModal] = useState(false)
  const [creditAmount, setCreditAmount] = useState('')
  const [creditNote, setCreditNote] = useState('')
  const [isCrediting, setIsCrediting] = useState(false)
  const [creditError, setCreditError] = useState<string | null>(null)

  async function load() {
    setIsLoading(true)
    setError(null)
    try {
      const res = await listSupportTickets(statusFilter || undefined)
      setTickets(res ?? [])
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to load tickets.')
    } finally {
      setIsLoading(false)
    }
  }

  async function loadStaff() {
    try {
      const res = await listAdminStaff()
      setStaff(res.staff ?? res ?? [])
    } catch {
      // non-fatal - assignment dropdown just won't populate
    }
  }

  useEffect(() => {
    load()
    loadStaff()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [statusFilter])

  async function openTicket(id: number) {
    setSelectedId(id)
    setIsLoadingThread(true)
    try {
      const res = await getSupportTicketMessages(id)
      setMessages(res.messages ?? [])
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to load ticket thread.')
    } finally {
      setIsLoadingThread(false)
    }
  }

  async function handleReply() {
    if (!selectedId || !replyText.trim()) return
    setIsSending(true)
    try {
      const msg = await replyToSupportTicket(selectedId, replyText.trim())
      setMessages((prev) => [...prev, msg])
      setReplyText('')
      setTickets((prev) =>
        prev.map((t) => (t.id === selectedId && t.status === 'open' ? { ...t, status: 'in_progress' } : t))
      )
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to send reply.')
    } finally {
      setIsSending(false)
    }
  }

  async function handleStatusChange(id: number, status: string) {
    try {
      await updateSupportTicketStatus(id, status)
      setTickets((prev) => prev.map((t) => (t.id === id ? { ...t, status: status as any } : t)))
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to update ticket status.')
    }
  }

  async function handleIssueTypeChange(id: number, issueType: string) {
    try {
      const updated = await updateSupportTicket(id, { issue_type: issueType })
      setTickets((prev) => prev.map((t) => (t.id === id ? { ...t, issue_type: updated.issue_type } : t)))
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to update issue type.')
    }
  }

  async function handlePriorityChange(id: number, priority: string) {
    try {
      const updated = await updateSupportTicket(id, { priority })
      setTickets((prev) => prev.map((t) => (t.id === id ? { ...t, priority: updated.priority } : t)))
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to update priority.')
    }
  }

  async function handleAssignChange(id: number, staffId: string) {
    try {
      const body = staffId ? { assigned_to_staff_id: Number(staffId) } : { assigned_to_staff_id: null }
      const updated = await updateSupportTicket(id, body as any)
      setTickets((prev) =>
        prev.map((t) =>
          t.id === id
            ? { ...t, assigned_to_staff_id: updated.assigned_to_staff_id, assigned_to_staff: updated.assigned_to_staff }
            : t
        )
      )
    } catch (err: any) {
      alert(err.response?.data?.error ?? 'Failed to update assignment.')
    }
  }

  function openCreditModal() {
    setCreditAmount('')
    setCreditNote('')
    setCreditError(null)
    setShowCreditModal(true)
  }

  async function handleCreditWallet() {
    if (!selectedTicket) return
    const amount = parseFloat(creditAmount)
    if (!amount || amount <= 0) {
      setCreditError('Enter a valid amount greater than 0.')
      return
    }
    setIsCrediting(true)
    setCreditError(null)
    try {
      await creditWallet(selectedTicket.user_id, amount, creditNote.trim() || undefined)
      setShowCreditModal(false)
    } catch (err: any) {
      setCreditError(err.response?.data?.error ?? 'Failed to credit wallet.')
    } finally {
      setIsCrediting(false)
    }
  }

  const selectedTicket = tickets.find((t) => t.id === selectedId)

  return (
    <Layout>
      <div className="p-8">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-xl font-semibold">Customer Support</h1>
            <p className="text-sm text-slate-400 mt-1">Customer tickets and support threads</p>
          </div>
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
          >
            <option value="">All statuses</option>
            {STATUS_OPTIONS.map((s) => (
              <option key={s} value={s}>
                {s.replace('_', ' ')}
              </option>
            ))}
          </select>
        </div>

        <div className="grid grid-cols-5 gap-6">
          <div className="col-span-2">
            {isLoading && <p className="text-slate-400">Loading...</p>}
            {error && <p className="text-red-400">{error}</p>}

            {!isLoading && !error && tickets.length === 0 && (
              <div className="border border-dashed border-slate-800 rounded-xl p-10 text-center text-slate-500">
                No tickets found.
              </div>
            )}

            {!isLoading && tickets.length > 0 && (
              <div className="border border-slate-800 rounded-xl overflow-hidden divide-y divide-slate-800">
                {tickets.map((t) => (
                  <button
                    key={t.id}
                    onClick={() => openTicket(t.id)}
                    className={
                      'w-full text-left px-4 py-3 hover:bg-slate-900 transition-colors ' +
                      (selectedId === t.id ? 'bg-slate-900' : '')
                    }
                  >
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-sm text-slate-200 font-medium truncate">{t.subject}</span>
                      <span
                        className={
                          'px-2 py-0.5 rounded-md text-xs font-medium shrink-0 ml-2 ' +
                          (STATUS_STYLES[t.status] ?? '')
                        }
                      >
                        {t.status.replace('_', ' ')}
                      </span>
                    </div>
                    <div className="text-xs text-slate-500">
                      Ticket #{t.id} &middot; {new Date(t.created_at).toLocaleDateString()}
                      {t.order_id ? ` \u00b7 Order #${t.order_id}` : ''}
                    </div>
                    {t.issue_type && t.issue_type !== 'other' && (
                      <div className="mt-1">
                        <span className="text-xs px-1.5 py-0.5 rounded bg-slate-800 text-slate-400">
                          {ISSUE_TYPE_OPTIONS.find((o) => o.value === t.issue_type)?.label ?? t.issue_type}
                        </span>
                      </div>
                    )}
                  </button>
                ))}
              </div>
            )}
          </div>

          <div className="col-span-3">
            {!selectedId && (
              <div className="border border-dashed border-slate-800 rounded-xl p-10 text-center text-slate-500 h-full flex items-center justify-center">
                Select a ticket to view the conversation
              </div>
            )}

            {selectedId && (
              <div className="border border-slate-800 rounded-xl overflow-hidden flex flex-col h-full">
                <div className="px-4 py-3 bg-slate-900 border-b border-slate-800">
                  <div className="flex items-center justify-between mb-3">
                    <div>
                      <p className="text-sm font-medium text-slate-200">{selectedTicket?.subject}</p>
                      <p className="text-xs text-slate-500">Ticket #{selectedId}</p>
                    </div>
                    <select
                      value={selectedTicket?.status ?? ''}
                      onChange={(e) => handleStatusChange(selectedId, e.target.value)}
                      className="bg-slate-800 border border-slate-700 rounded-lg px-2 py-1.5 text-xs"
                    >
                      {STATUS_OPTIONS.map((s) => (
                        <option key={s} value={s}>
                          {s.replace('_', ' ')}
                        </option>
                      ))}
                    </select>
                  </div>

                  <div className="flex items-center gap-2 flex-wrap">
                    <select
                      value={selectedTicket?.issue_type ?? 'other'}
                      onChange={(e) => handleIssueTypeChange(selectedId, e.target.value)}
                      className="bg-slate-800 border border-slate-700 rounded-lg px-2 py-1.5 text-xs"
                    >
                      {ISSUE_TYPE_OPTIONS.map((o) => (
                        <option key={o.value} value={o.value}>
                          {o.label}
                        </option>
                      ))}
                    </select>

                    <select
                      value={selectedTicket?.priority ?? 'normal'}
                      onChange={(e) => handlePriorityChange(selectedId, e.target.value)}
                      className="bg-slate-800 border border-slate-700 rounded-lg px-2 py-1.5 text-xs"
                    >
                      {PRIORITY_OPTIONS.map((o) => (
                        <option key={o.value} value={o.value}>
                          {o.label}
                        </option>
                      ))}
                    </select>

                    <select
                      value={selectedTicket?.assigned_to_staff_id ?? ''}
                      onChange={(e) => handleAssignChange(selectedId, e.target.value)}
                      className="bg-slate-800 border border-slate-700 rounded-lg px-2 py-1.5 text-xs"
                    >
                      <option value="">Unassigned</option>
                      {staff.map((s: any) => (
                        <option key={s.id} value={s.id}>
                          {s.name || s.phone}
                        </option>
                      ))}
                    </select>

                    <button
                      onClick={openCreditModal}
                      className="text-xs px-2 py-1.5 rounded-lg bg-emerald-500/15 text-emerald-300 hover:bg-emerald-500/25 transition-colors"
                    >
                      Credit Wallet
                    </button>

                    {selectedTicket?.order_id && (
                      
                      <a
                        href="/returns"
                        className="text-xs px-2 py-1.5 rounded-lg bg-slate-800 text-slate-300 hover:bg-slate-700 transition-colors"
                      >
                        View Order #{selectedTicket.order_id} Returns
                      </a>
                    )}
                  </div>
                </div>

                <div className="p-4 space-y-3 max-h-96 overflow-y-auto">
                  {isLoadingThread && <p className="text-slate-400 text-sm">Loading thread...</p>}
                  {!isLoadingThread &&
                    messages.map((m) => (
                      <div
                        key={m.id}
                        className={
                          'max-w-[80%] rounded-lg px-3 py-2 text-sm ' +
                          (m.sender_type === 'admin'
                            ? 'ml-auto bg-indigo-500/15 text-indigo-100'
                            : 'bg-slate-800 text-slate-200')
                        }
                      >
                        <p>{m.message}</p>
                        <p className="text-xs text-slate-500 mt-1">
                          {m.sender_type === 'admin' ? 'You' : 'Customer'} &middot;{' '}
                          {new Date(m.created_at).toLocaleString()}
                        </p>
                      </div>
                    ))}
                </div>

                <div className="p-4 border-t border-slate-800 flex gap-2">
                  <input
                    type="text"
                    value={replyText}
                    onChange={(e) => setReplyText(e.target.value)}
                    onKeyDown={(e) => e.key === 'Enter' && handleReply()}
                    placeholder="Type a reply..."
                    className="flex-1 bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-slate-500"
                  />
                  <button
                    onClick={handleReply}
                    disabled={isSending || !replyText.trim()}
                    className="px-4 py-2 rounded-lg bg-indigo-500 hover:bg-indigo-400 text-white text-sm font-medium disabled:opacity-40 transition-colors"
                  >
                    Send
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {showCreditModal && selectedTicket && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 w-full max-w-sm">
            <h3 className="text-sm font-semibold text-slate-200 mb-1">Credit Wallet</h3>
            <p className="text-xs text-slate-500 mb-4">
              Credits customer #{selectedTicket.user_id}'s wallet directly.
            </p>
            <div className="space-y-3">
              <div>
                <label className="text-xs text-slate-400 block mb-1">Amount</label>
                <input
                  type="number"
                  step="any"
                  value={creditAmount}
                  onChange={(e) => setCreditAmount(e.target.value)}
                  placeholder="e.g. 100"
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                />
              </div>
              <div>
                <label className="text-xs text-slate-400 block mb-1">Note (optional)</label>
                <input
                  type="text"
                  value={creditNote}
                  onChange={(e) => setCreditNote(e.target.value)}
                  placeholder={`Goodwill credit for ticket #${selectedTicket.id}`}
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm"
                />
              </div>
              {creditError && <p className="text-red-400 text-xs">{creditError}</p>}
              <div className="flex gap-2 pt-2">
                <button
                  onClick={() => setShowCreditModal(false)}
                  className="flex-1 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-sm transition-colors"
                >
                  Cancel
                </button>
                <button
                  onClick={handleCreditWallet}
                  disabled={isCrediting}
                  className="flex-1 py-2 rounded-lg bg-emerald-500 hover:bg-emerald-400 text-white text-sm font-medium disabled:opacity-40 transition-colors"
                >
                  {isCrediting ? 'Crediting...' : 'Credit'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </Layout>
  )
}
