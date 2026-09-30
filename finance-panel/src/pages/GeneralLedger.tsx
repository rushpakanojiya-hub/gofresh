import { useEffect, useState } from 'react'
import {
  getAccounts,
  createManualJournalEntry,
  getLedgerEntries,
  getTrialBalance,
} from '../api/finance'
import type { Account, LedgerEntry, LedgerEntryLine, TrialBalance } from '../types/finance'

function formatCurrency(value: number) {
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    maximumFractionDigits: 2,
  }).format(value)
}

function todayISO() {
  return new Date().toISOString().slice(0, 10)
}

function daysAgoISO(days: number) {
  const d = new Date()
  d.setDate(d.getDate() - days)
  return d.toISOString().slice(0, 10)
}

type TabKey = 'journal' | 'ledger' | 'trial-balance'

const TABS: { key: TabKey; label: string }[] = [
  { key: 'journal', label: 'Manual Journal Entry' },
  { key: 'ledger', label: 'Ledger' },
  { key: 'trial-balance', label: 'Trial Balance' },
]

type DraftLine = LedgerEntryLine & { key: string }

function newDraftLine(): DraftLine {
  return { key: Math.random().toString(36).slice(2), account_id: 0, type: 'debit', amount: 0, description: '' }
}

export default function GeneralLedger() {
  const [tab, setTab] = useState<TabKey>('journal')
  const [accounts, setAccounts] = useState<Account[]>([])

  useEffect(() => {
    getAccounts().then((data) => setAccounts(data.accounts ?? [])).catch(() => setAccounts([]))
  }, [])

  return (
    <div>
      <h1 className="text-2xl font-semibold text-white">General Ledger</h1>
      <p className="mt-1 text-sm text-gray-400">
        Manual journal entries, account ledger, and trial balance. Journals are posted immediately on submission —
        there is no pending-approval step.
      </p>

      <div className="mt-6 flex gap-1 border-b border-gray-800">
        {TABS.map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={`px-4 py-2 text-sm rounded-t-lg transition-colors ${
              tab === t.key
                ? 'bg-gray-900 text-emerald-400 border-b-2 border-emerald-500'
                : 'text-gray-400 hover:text-gray-200'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      <div className="mt-6">
        {tab === 'journal' && <JournalTab accounts={accounts} />}
        {tab === 'ledger' && <LedgerTab accounts={accounts} />}
        {tab === 'trial-balance' && <TrialBalanceTab />}
      </div>
    </div>
  )
}

function JournalTab({ accounts }: { accounts: Account[] }) {
  const [entryDate, setEntryDate] = useState(todayISO())
  const [lines, setLines] = useState<DraftLine[]>([newDraftLine(), newDraftLine()])
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState<string | null>(null)

  const totalDebit = lines.filter((l) => l.type === 'debit').reduce((sum, l) => sum + (Number(l.amount) || 0), 0)
  const totalCredit = lines.filter((l) => l.type === 'credit').reduce((sum, l) => sum + (Number(l.amount) || 0), 0)
  const isBalanced = totalDebit === totalCredit && totalDebit > 0

  function updateLine(key: string, patch: Partial<DraftLine>) {
    setLines((prev) => prev.map((l) => (l.key === key ? { ...l, ...patch } : l)))
  }

  function addLine() {
    setLines((prev) => [...prev, newDraftLine()])
  }

  function removeLine(key: string) {
    setLines((prev) => (prev.length > 2 ? prev.filter((l) => l.key !== key) : prev))
  }

  async function handleSubmit() {
    setError('')
    setSuccess(null)
    if (!isBalanced) {
      setError('Total debit must equal total credit, and must be greater than zero.')
      return
    }
    if (lines.some((l) => !l.account_id || !l.amount)) {
      setError('Every line needs an account and a non-zero amount.')
      return
    }
    setSubmitting(true)
    try {
      const payload = {
        entry_date: entryDate,
        lines: lines.map(({ key, ...rest }) => rest),
      }
      const result = await createManualJournalEntry(payload)
      setSuccess(`Posted successfully. Transaction ref: ${result.transaction_ref}`)
      setLines([newDraftLine(), newDraftLine()])
    } catch (err: any) {
      setError(err?.response?.data?.message ?? 'Failed to post journal entry.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div>
      <div className="flex items-end gap-4">
        <div>
          <label className="mb-1 block text-sm text-gray-400">Entry Date</label>
          <input
            type="date"
            value={entryDate}
            onChange={(e) => setEntryDate(e.target.value)}
            className="rounded border border-gray-700 bg-gray-900 px-3 py-2 text-white"
          />
        </div>
      </div>

      <div className="mt-6 overflow-x-auto rounded border border-gray-800">
        <table className="w-full text-left text-sm">
          <thead className="bg-gray-900 text-gray-400">
            <tr>
              <th className="px-4 py-3 font-medium">Account</th>
              <th className="px-4 py-3 font-medium">Type</th>
              <th className="px-4 py-3 font-medium">Amount</th>
              <th className="px-4 py-3 font-medium">Description</th>
              <th className="px-4 py-3 font-medium"></th>
            </tr>
          </thead>
          <tbody>
            {lines.map((line) => (
              <tr key={line.key} className="border-t border-gray-800">
                <td className="px-4 py-2">
                  <select
                    value={line.account_id || ''}
                    onChange={(e) => updateLine(line.key, { account_id: Number(e.target.value) })}
                    className="w-full rounded border border-gray-700 bg-gray-900 px-2 py-1.5 text-white"
                  >
                    <option value="">Select account</option>
                    {accounts.map((a) => (
                      <option key={a.id} value={a.id}>
                        {a.code} — {a.name}
                      </option>
                    ))}
                  </select>
                </td>
                <td className="px-4 py-2">
                  <select
                    value={line.type}
                    onChange={(e) => updateLine(line.key, { type: e.target.value as 'debit' | 'credit' })}
                    className="w-full rounded border border-gray-700 bg-gray-900 px-2 py-1.5 text-white"
                  >
                    <option value="debit">Debit</option>
                    <option value="credit">Credit</option>
                  </select>
                </td>
                <td className="px-4 py-2">
                  <input
                    type="number"
                    min="0"
                    step="0.01"
                    value={line.amount || ''}
                    onChange={(e) => updateLine(line.key, { amount: Number(e.target.value) })}
                    className="w-28 rounded border border-gray-700 bg-gray-900 px-2 py-1.5 text-white"
                  />
                </td>
                <td className="px-4 py-2">
                  <input
                    type="text"
                    value={line.description ?? ''}
                    onChange={(e) => updateLine(line.key, { description: e.target.value })}
                    placeholder="Narration"
                    className="w-full rounded border border-gray-700 bg-gray-900 px-2 py-1.5 text-white"
                  />
                </td>
                <td className="px-4 py-2">
                  <button
                    onClick={() => removeLine(line.key)}
                    disabled={lines.length <= 2}
                    className="text-xs text-red-400 hover:text-red-300 disabled:opacity-30"
                  >
                    Remove
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <button
        onClick={addLine}
        className="mt-3 rounded border border-gray-700 px-3 py-1.5 text-sm text-gray-300 hover:bg-gray-800"
      >
        + Add Line
      </button>

      <div className="mt-4 flex flex-wrap gap-4">
        <div className="rounded border border-gray-800 px-4 py-3">
          <p className="text-xs text-gray-500">Total Debit</p>
          <p className="text-lg font-semibold text-white">{formatCurrency(totalDebit)}</p>
        </div>
        <div className="rounded border border-gray-800 px-4 py-3">
          <p className="text-xs text-gray-500">Total Credit</p>
          <p className="text-lg font-semibold text-white">{formatCurrency(totalCredit)}</p>
        </div>
        <div className="rounded border border-gray-800 px-4 py-3">
          <p className="text-xs text-gray-500">Status</p>
          <p className={`text-lg font-semibold ${isBalanced ? 'text-emerald-400' : 'text-red-400'}`}>
            {isBalanced ? 'Balanced' : 'Not Balanced'}
          </p>
        </div>
      </div>

      {error && <div className="mt-4 rounded border border-red-800 bg-red-950 px-4 py-2 text-red-300">{error}</div>}
      {success && (
        <div className="mt-4 rounded border border-emerald-800 bg-emerald-950 px-4 py-2 text-emerald-300">
          {success}
        </div>
      )}

      <button
        onClick={handleSubmit}
        disabled={submitting || !isBalanced}
        className="mt-4 rounded bg-emerald-600 px-5 py-2 font-medium text-white hover:bg-emerald-500 disabled:opacity-50"
      >
        {submitting ? 'Posting…' : 'Post Journal Entry'}
      </button>
      <p className="mt-2 text-xs text-gray-500">This will be posted immediately — there is no approval step.</p>
    </div>
  )
}

const PAGE_SIZE = 20

function LedgerTab({ accounts }: { accounts: Account[] }) {
  const [accountId, setAccountId] = useState<string>('')
  const [from, setFrom] = useState(daysAgoISO(29))
  const [to, setTo] = useState(todayISO())
  const [page, setPage] = useState(1)
  const [entries, setEntries] = useState<LedgerEntry[]>([])
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(1)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  async function load(pageNum = page) {
    setLoading(true)
    setError('')
    try {
      const params: any = { from, to, page: pageNum, limit: PAGE_SIZE }
      if (accountId) params.account_id = Number(accountId)
      const data = await getLedgerEntries(params)
      setEntries(data.entries ?? [])
      setTotal(data.total ?? 0)
      setTotalPages(data.total_pages ?? 1)
    } catch (err: any) {
      setError(err?.response?.data?.message ?? 'Failed to load ledger entries.')
      setEntries([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load(1)
    setPage(1)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function handleApply() {
    setPage(1)
    load(1)
  }

  function goToPage(p: number) {
    setPage(p)
    load(p)
  }

  // Running balance computed over the currently loaded page only (oldest to newest as returned).
  let running = 0
  const withRunning = entries.map((e) => {
    running += e.type === 'debit' ? e.amount : -e.amount
    return { ...e, running }
  })

  return (
    <div>
      <div className="flex flex-wrap items-end gap-4">
        <div>
          <label className="mb-1 block text-sm text-gray-400">Account</label>
          <select
            value={accountId}
            onChange={(e) => setAccountId(e.target.value)}
            className="rounded border border-gray-700 bg-gray-900 px-3 py-2 text-white"
          >
            <option value="">All accounts</option>
            {accounts.map((a) => (
              <option key={a.id} value={a.id}>
                {a.code} — {a.name}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="mb-1 block text-sm text-gray-400">From</label>
          <input
            type="date"
            value={from}
            onChange={(e) => setFrom(e.target.value)}
            className="rounded border border-gray-700 bg-gray-900 px-3 py-2 text-white"
          />
        </div>
        <div>
          <label className="mb-1 block text-sm text-gray-400">To</label>
          <input
            type="date"
            value={to}
            onChange={(e) => setTo(e.target.value)}
            className="rounded border border-gray-700 bg-gray-900 px-3 py-2 text-white"
          />
        </div>
        <button
          onClick={handleApply}
          disabled={loading}
          className="rounded bg-emerald-600 px-4 py-2 font-medium text-white hover:bg-emerald-500 disabled:opacity-50"
        >
          {loading ? 'Loading…' : 'Apply'}
        </button>
      </div>

      {error && <div className="mt-4 rounded border border-red-800 bg-red-950 px-4 py-2 text-red-300">{error}</div>}

      <div className="mt-6 overflow-x-auto rounded border border-gray-800">
        <table className="w-full text-left text-sm">
          <thead className="bg-gray-900 text-gray-400">
            <tr>
              <th className="px-4 py-3 font-medium">Date</th>
              <th className="px-4 py-3 font-medium">Ref</th>
              <th className="px-4 py-3 font-medium">Account</th>
              <th className="px-4 py-3 font-medium">Type</th>
              <th className="px-4 py-3 font-medium">Amount</th>
              <th className="px-4 py-3 font-medium">Running Balance</th>
              <th className="px-4 py-3 font-medium">Description</th>
            </tr>
          </thead>
          <tbody>
            {withRunning.length === 0 && !loading && (
              <tr>
                <td colSpan={7} className="px-4 py-6 text-center text-gray-500">
                  No ledger entries for the selected filters.
                </td>
              </tr>
            )}
            {withRunning.map((e) => (
              <tr key={e.id} className="border-t border-gray-800">
                <td className="px-4 py-3 text-gray-300">{e.entry_date}</td>
                <td className="px-4 py-3 text-gray-400">{e.transaction_ref}</td>
                <td className="px-4 py-3 text-white">{e.account ? `${e.account.code} — ${e.account.name}` : e.account_id}</td>
                <td className="px-4 py-3 text-gray-300 capitalize">{e.type}</td>
                <td className={`px-4 py-3 ${e.type === 'debit' ? 'text-emerald-400' : 'text-red-400'}`}>
                  {formatCurrency(e.amount)}
                </td>
                <td className="px-4 py-3 text-gray-300">{formatCurrency(e.running)}</td>
                <td className="px-4 py-3 text-gray-400">{e.description ?? '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {entries.length > 0 && (
        <p className="mt-2 text-xs text-gray-500">
          Running balance is calculated only across the entries currently loaded on this page, not from account
          inception.
        </p>
      )}

      {total > 0 && (
        <div className="mt-4 flex items-center justify-between text-sm text-gray-400">
          <div>
            Page {page} of {totalPages} ({total} total)
          </div>
          <div className="flex gap-2">
            <button
              onClick={() => goToPage(page - 1)}
              disabled={page <= 1 || loading}
              className="rounded border border-gray-700 px-3 py-1 hover:bg-gray-800 disabled:opacity-50"
            >
              Prev
            </button>
            <button
              onClick={() => goToPage(page + 1)}
              disabled={page >= totalPages || loading}
              className="rounded border border-gray-700 px-3 py-1 hover:bg-gray-800 disabled:opacity-50"
            >
              Next
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

function TrialBalanceTab() {
  const [asOf, setAsOf] = useState(todayISO())
  const [data, setData] = useState<TrialBalance | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  async function load(asOfDate = asOf) {
    setLoading(true)
    setError('')
    try {
      const result = await getTrialBalance(asOfDate)
      setData(result)
    } catch (err: any) {
      setError(err?.response?.data?.message ?? 'Failed to load trial balance.')
      setData(null)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <div>
      <div className="flex items-end gap-4">
        <div>
          <label className="mb-1 block text-sm text-gray-400">As of</label>
          <input
            type="date"
            value={asOf}
            onChange={(e) => setAsOf(e.target.value)}
            className="rounded border border-gray-700 bg-gray-900 px-3 py-2 text-white"
          />
        </div>
        <button
          onClick={() => load()}
          disabled={loading}
          className="rounded bg-emerald-600 px-4 py-2 font-medium text-white hover:bg-emerald-500 disabled:opacity-50"
        >
          {loading ? 'Loading…' : 'Apply'}
        </button>
      </div>

      {error && <div className="mt-4 rounded border border-red-800 bg-red-950 px-4 py-2 text-red-300">{error}</div>}

      {data && (
        <>
          <div
            className={`mt-6 inline-block rounded px-3 py-2 text-xs ${
              data.is_balanced ? 'bg-emerald-600/15 text-emerald-400' : 'bg-red-600/15 text-red-400'
            }`}
          >
            {data.is_balanced ? 'Balanced \u2713' : 'Out of balance \u2717'} {'\u2014'} as of {data.as_of}
          </div>

          <div className="mt-4 overflow-x-auto rounded border border-gray-800">
            <table className="w-full text-left text-sm">
              <thead className="bg-gray-900 text-gray-400">
                <tr>
                  <th className="px-4 py-3 font-medium">Code</th>
                  <th className="px-4 py-3 font-medium">Account</th>
                  <th className="px-4 py-3 font-medium">Type</th>
                  <th className="px-4 py-3 font-medium">Total Debit</th>
                  <th className="px-4 py-3 font-medium">Total Credit</th>
                </tr>
              </thead>
              <tbody>
                {data.accounts.length === 0 && (
                  <tr>
                    <td colSpan={5} className="px-4 py-6 text-center text-gray-500">
                      No account activity as of this date.
                    </td>
                  </tr>
                )}
                {data.accounts.map((a) => (
                  <tr key={a.account_id} className="border-t border-gray-800">
                    <td className="px-4 py-3 text-gray-400">{a.account_code}</td>
                    <td className="px-4 py-3 text-white">{a.account_name}</td>
                    <td className="px-4 py-3 text-gray-300 capitalize">{a.account_type}</td>
                    <td className="px-4 py-3 text-gray-300">{formatCurrency(a.total_debit)}</td>
                    <td className="px-4 py-3 text-gray-300">{formatCurrency(a.total_credit)}</td>
                  </tr>
                ))}
              </tbody>
              {data.accounts.length > 0 && (
                <tfoot>
                  <tr className="border-t border-gray-700 bg-gray-900 font-medium">
                    <td className="px-4 py-3 text-white" colSpan={3}>
                      Total
                    </td>
                    <td className="px-4 py-3 text-white">{formatCurrency(data.total_debit)}</td>
                    <td className="px-4 py-3 text-white">{formatCurrency(data.total_credit)}</td>
                  </tr>
                </tfoot>
              )}
            </table>
          </div>
        </>
      )}
    </div>
  )
}
