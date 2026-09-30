import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { listMyTickets, createTicket } from '../api/support'
import type { SupportTicket } from '../types'

const STATUS_COLORS: Record<string, string> = {
  open: 'bg-amber-100 text-amber-700',
  in_progress: 'bg-blue-100 text-blue-700',
  resolved: 'bg-leaf/15 text-leaf',
  closed: 'bg-line text-ink/60',
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

export default function Support() {
  const [tickets, setTickets] = useState<SupportTicket[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [showForm, setShowForm] = useState(false)
  const [subject, setSubject] = useState('')
  const [message, setMessage] = useState('')
  const [issueType, setIssueType] = useState('other')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  function load() {
    setIsLoading(true)
    listMyTickets()
      .then((res) => setTickets(res ?? []))
      .finally(() => setIsLoading(false))
  }

  useEffect(() => {
    load()
  }, [])

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setIsSubmitting(true)
    try {
      await createTicket({ subject, message, issue_type: issueType })
      setSubject('')
      setMessage('')
      setIssueType('other')
      setShowForm(false)
      load()
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to create ticket.')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="max-w-3xl mx-auto px-6 py-10">
      <div className="flex items-center justify-between mb-8">
        <h1 className="font-display text-3xl font-600">Support</h1>
        <button
          onClick={() => setShowForm((v) => !v)}
          className="px-4 py-2 rounded-full bg-ink text-paper text-sm font-medium hover:bg-marigold transition-colors"
        >
          {showForm ? 'Cancel' : 'New ticket'}
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleSubmit} className="border border-line rounded-xl p-5 mb-8 space-y-4">
          {error && <p className="text-sm text-clay">{error}</p>}
          <div>
            <label className="block text-sm font-medium mb-1.5">Subject</label>
            <input
              type="text"
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
              required
              className="w-full border border-line rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-marigold"
              placeholder="Briefly describe your issue"
            />
          </div>
          <div>
            <label className="block text-sm font-medium mb-1.5">Issue type</label>
            <select
              value={issueType}
              onChange={(e) => setIssueType(e.target.value)}
              className="w-full border border-line rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-marigold"
            >
              {ISSUE_TYPE_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium mb-1.5">Message</label>
            <textarea
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              required
              rows={4}
              className="w-full border border-line rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-marigold"
              placeholder="Tell us more about the issue"
            />
          </div>
          <button
            type="submit"
            disabled={isSubmitting}
            className="px-4 py-2 rounded-full bg-marigold text-white text-sm font-medium hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {isSubmitting ? 'Submitting...' : 'Submit ticket'}
          </button>
        </form>
      )}

      {isLoading ? (
        <p className="text-ink/50">Loading...</p>
      ) : tickets.length === 0 ? (
        <div className="text-center py-16">
          <p className="text-ink/60">No support tickets yet.</p>
        </div>
      ) : (
        <div className="space-y-4">
          {tickets.map((t) => (
            <Link
              key={t.id}
              to={`/support/${t.id}`}
              className="block border border-line rounded-xl p-5 hover:border-marigold transition-colors"
            >
              <div className="flex items-center justify-between mb-2">
                <p className="font-medium">{t.subject}</p>
                <span
                  className={`text-xs font-medium px-2.5 py-1 rounded-full capitalize ${
                    STATUS_COLORS[t.status] ?? 'bg-line text-ink/60'
                  }`}
                >
                  {t.status.replace('_', ' ')}
                </span>
              </div>
              <p className="text-xs text-ink/50">
                Ticket #{t.id} ·{' '}
                {new Date(t.created_at).toLocaleDateString('en-IN', {
                  day: 'numeric',
                  month: 'short',
                  year: 'numeric',
                })}
              </p>
            </Link>
          ))}
        </div>
      )}
    </div>
  )
}