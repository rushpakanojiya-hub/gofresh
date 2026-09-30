import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { getTicketMessages, replyToTicket } from '../api/support'
import type { SupportTicket, SupportMessage } from '../types'

const STATUS_COLORS: Record<string, string> = {
  open: 'bg-amber-100 text-amber-700',
  in_progress: 'bg-blue-100 text-blue-700',
  resolved: 'bg-leaf/15 text-leaf',
  closed: 'bg-line text-ink/60',
}

export default function SupportDetail() {
  const { id } = useParams<{ id: string }>()
  const [ticket, setTicket] = useState<SupportTicket | null>(null)
  const [messages, setMessages] = useState<SupportMessage[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [reply, setReply] = useState('')
  const [isSending, setIsSending] = useState(false)
  const [error, setError] = useState<string | null>(null)

  function load() {
    if (!id) return
    setIsLoading(true)
    getTicketMessages(Number(id))
      .then((res) => {
        setTicket(res.ticket)
        setMessages(res.messages ?? [])
      })
      .finally(() => setIsLoading(false))
  }

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

  async function handleSend(e: React.FormEvent) {
    e.preventDefault()
    if (!id || !reply.trim()) return
    setError(null)
    setIsSending(true)
    try {
      await replyToTicket(Number(id), { message: reply })
      setReply('')
      load()
    } catch (err: any) {
      setError(err.response?.data?.error ?? 'Failed to send reply.')
    } finally {
      setIsSending(false)
    }
  }

  if (isLoading) {
    return (
      <div className="max-w-3xl mx-auto px-6 py-10">
        <p className="text-ink/50">Loading...</p>
      </div>
    )
  }

  if (!ticket) {
    return (
      <div className="max-w-3xl mx-auto px-6 py-10">
        <p className="text-ink/60">Ticket not found.</p>
        <Link to="/support" className="text-marigold font-medium hover:underline">
          ← Back to support
        </Link>
      </div>
    )
  }

  return (
    <div className="max-w-3xl mx-auto px-6 py-10">
      <Link to="/support" className="text-sm text-ink/50 hover:text-marigold transition-colors mb-6 inline-block">
        ← Back to support
      </Link>

      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="font-display text-2xl font-600">{ticket.subject}</h1>
          <p className="text-xs text-ink/50 mt-1">Ticket #{ticket.id}</p>
        </div>
        <span
          className={`text-xs font-medium px-2.5 py-1 rounded-full capitalize ${
            STATUS_COLORS[ticket.status] ?? 'bg-line text-ink/60'
          }`}
        >
          {ticket.status.replace('_', ' ')}
        </span>
      </div>

      <div className="space-y-3 mb-6">
        {messages.map((m) => (
          <div
            key={m.id}
            className={`rounded-xl p-4 max-w-[85%] ${
              m.sender_type === 'customer'
                ? 'bg-marigold/10 ml-auto'
                : 'bg-line/50'
            }`}
          >
            <p className="text-sm">{m.message}</p>
            <p className="text-xs text-ink/40 mt-1.5">
              {m.sender_type === 'customer' ? 'You' : 'Support team'} ·{' '}
              {new Date(m.created_at).toLocaleString('en-IN', {
                day: 'numeric',
                month: 'short',
                hour: '2-digit',
                minute: '2-digit',
              })}
            </p>
          </div>
        ))}
      </div>

      {ticket.status !== 'closed' && (
        <form onSubmit={handleSend} className="flex items-end gap-3">
          <div className="flex-1">
            {error && <p className="text-sm text-clay mb-2">{error}</p>}
            <textarea
              value={reply}
              onChange={(e) => setReply(e.target.value)}
              rows={2}
              placeholder="Type a reply..."
              className="w-full border border-line rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-marigold"
            />
          </div>
          <button
            type="submit"
            disabled={isSending || !reply.trim()}
            className="px-4 py-2 rounded-full bg-marigold text-white text-sm font-medium hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {isSending ? 'Sending...' : 'Send'}
          </button>
        </form>
      )}
    </div>
  )
}