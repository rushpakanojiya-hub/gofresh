import { useCallback, useEffect, useMemo, useState } from 'react'
import { getLocationOccupancy, getWarehouseInventory } from '../api/warehouse'
import type { OccupancyBin, OccupancyData } from '../types/warehouse'
import { getErrorMessage } from '../utils/errors'

type Tone = 'ok' | 'low' | 'out' | 'empty'
type Info = { image?: string; stock: number; reserved: number; available: number; status: string }
type FlatBin = { zone: string; rack: string; bin: OccupancyBin }

const RACK_COLORS = ['#2563eb', '#16a34a', '#ea580c', '#7c3aed', '#0d9488', '#db2777', '#4f46e5', '#b45309', '#8b5cf6']

const TONE_CLASS: Record<Tone, string> = {
  ok: 'border-emerald-700 bg-emerald-900/40 text-emerald-100 hover:bg-emerald-900/70',
  low: 'border-amber-600 bg-amber-900/40 text-amber-100 hover:bg-amber-900/70',
  out: 'border-rose-700 bg-rose-900/40 text-rose-100 hover:bg-rose-900/70',
  empty: 'border-slate-800 bg-slate-950 text-slate-500 hover:bg-slate-800/50',
}

const TONE_LABEL: Record<Tone, string> = { ok: 'In Stock', low: 'Low Stock', out: 'Out of Stock', empty: 'Empty' }

const TONE_PILL: Record<Tone, string> = {
  ok: 'bg-emerald-500/15 text-emerald-300',
  low: 'bg-amber-500/15 text-amber-300',
  out: 'bg-rose-500/15 text-rose-300',
  empty: 'bg-slate-700/40 text-slate-400',
}

function statusToTone(status: string | undefined, stock: number): Tone {
  if (status === 'out' || stock <= 0) return 'out'
  if (status === 'low') return 'low'
  return 'ok'
}

function Thumb({ src, name, size }: { src?: string; name: string; size: number }) {
  const style = { width: size, height: size }
  if (src) {
    return <img src={src} alt={name} style={style} className="rounded-lg object-cover bg-slate-800 shrink-0" />
  }
  return (
    <div style={style} className="rounded-lg bg-slate-800 text-slate-400 flex items-center justify-center text-sm font-semibold shrink-0">
      {name.charAt(0).toUpperCase()}
    </div>
  )
}

export default function RacksBins() {
  const [data, setData] = useState<OccupancyData | null>(null)
  const [info, setInfo] = useState<Record<number, Info>>({})
  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [view, setView] = useState<'map' | 'list'>('map')
  const [query, setQuery] = useState('')
  const [selectedId, setSelectedId] = useState<number | null>(null)

  const load = useCallback(async () => {
    setError(null)
    setIsLoading(true)
    try {
      const occ = await getLocationOccupancy()
      setData(occ)
      const map: Record<number, Info> = {}
      try {
        let page = 1
        let totalPages = 1
        do {
          const res = await getWarehouseInventory({ page, limit: 100 })
          totalPages = res.total_pages || 1
          res.rows.forEach((r) => {
            const raw = r as unknown as Record<string, unknown>
            const img = (raw.image_url ?? raw.image ?? raw.product_image ?? raw.thumbnail) as string | undefined
            map[r.product_id] = {
              image: img || undefined,
              stock: r.stock,
              reserved: r.reserved,
              available: r.available,
              status: r.stock_status,
            }
          })
          page += 1
        } while (page <= totalPages && page <= 10)
      } catch {
        // product details optional; bins still show
      }
      setInfo(map)
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to load racks and bins.'))
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const binTone = useCallback(
    (b: OccupancyBin): Tone => {
      if (b.product_count === 0 || b.products.length === 0) return 'empty'
      const tones = b.products.map((p) => statusToTone(info[p.product_id]?.status, info[p.product_id]?.stock ?? p.stock))
      if (tones.includes('out')) return 'out'
      if (tones.includes('low')) return 'low'
      return 'ok'
    },
    [info]
  )

  const flat = useMemo<FlatBin[]>(() => {
    const out: FlatBin[] = []
    data?.zones.forEach((z) => z.racks.forEach((r) => r.bins.forEach((b) => out.push({ zone: z.name, rack: r.name, bin: b }))))
    return out
  }, [data])

  const selected = flat.find((f) => f.bin.id === selectedId) ?? null
  const q = query.trim().toLowerCase()

  const matches = (zone: string, rack: string, b: OccupancyBin) => {
    if (!q) return true
    return (
      rack.toLowerCase().includes(q) ||
      zone.toLowerCase().includes(q) ||
      b.name.toLowerCase().includes(q) ||
      b.products.some((p) => p.name.toLowerCase().includes(q))
    )
  }

  return (
    <div className="p-6 max-w-7xl">
      <div className="flex items-center justify-between mb-4">
        <div>
          <p className="font-mono text-[10px] tracking-widest text-red-500 uppercase mb-1">Warehouse</p>
          <h1 className="font-display text-2xl font-semibold">Racks &amp; Bins</h1>
          <p className="text-xs text-slate-500 mt-1">View rack locations and product mapping</p>
        </div>
        <button
          onClick={load}
          className="text-xs px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 border border-slate-700 transition-colors"
        >
          Refresh
        </button>
      </div>

      <div className="flex flex-wrap items-center gap-3 mb-4">
        <div className="flex rounded-lg overflow-hidden border border-slate-700">
          {(['map', 'list'] as const).map((v) => (
            <button
              key={v}
              onClick={() => setView(v)}
              className={`text-xs px-3 py-1.5 transition-colors ${
                view === v ? 'bg-red-600 text-white' : 'bg-slate-800 text-slate-400 hover:bg-slate-700'
              }`}
            >
              {v === 'map' ? 'View Map' : 'List View'}
            </button>
          ))}
        </div>
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search rack, bin or product..."
          className="text-xs bg-slate-800 border border-slate-700 rounded-lg px-3 py-1.5 text-slate-300 w-64 focus:outline-none focus:border-red-500"
        />
        <div className="ml-auto flex items-center gap-4 text-xs text-slate-400">
          <span className="flex items-center gap-1.5"><span className="w-2.5 h-2.5 rounded-full bg-emerald-500" />In Stock</span>
          <span className="flex items-center gap-1.5"><span className="w-2.5 h-2.5 rounded-full bg-amber-400" />Low Stock</span>
          <span className="flex items-center gap-1.5"><span className="w-2.5 h-2.5 rounded-full bg-rose-500" />Out of Stock</span>
          <span className="flex items-center gap-1.5"><span className="w-2.5 h-2.5 rounded-full bg-slate-600" />Empty</span>
        </div>
      </div>

      {error && (
        <div className="border border-rose-900 bg-rose-950/40 text-rose-300 text-sm rounded-lg px-4 py-3 mb-4">{error}</div>
      )}
      {isLoading && <p className="text-sm text-slate-400">Loading...</p>}

      {data && data.zones.length === 0 && (
        <div className="border border-slate-800 rounded-xl bg-slate-900 p-8 text-center text-sm text-slate-500">
          No zones, racks or bins yet. Create them in Locations first.
        </div>
      )}

      {data && data.zones.length > 0 && (
        <div className="grid grid-cols-1 lg:grid-cols-[1fr_340px] gap-4 items-start">
          <div className="border border-slate-800 bg-slate-900 rounded-xl p-4">
            {view === 'map' ? (
              data.zones.map((z) => (
                <div key={z.id} className="mb-5 last:mb-0">
                  <p className="text-xs uppercase tracking-wide text-slate-500 mb-2">Zone {z.name}</p>
                  {z.racks.length === 0 && <p className="text-sm text-slate-600">No racks.</p>}
                  <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3">
                    {z.racks.map((r, idx) => (
                      <div key={r.id} className="rounded-xl border border-slate-800 bg-slate-950 overflow-hidden">
                        <div
                          className="text-center text-xs font-semibold py-1.5 text-white"
                          style={{ backgroundColor: RACK_COLORS[idx % RACK_COLORS.length] }}
                        >
                          Rack {r.name}
                        </div>
                        <div className="p-2">
                          {r.bins.length === 0 ? (
                            <p className="text-xs text-slate-600 text-center py-2">No bins.</p>
                          ) : (
                            <div className="grid grid-cols-3 gap-1.5">
                              {r.bins.map((b) => {
                                const tone = binTone(b)
                                return (
                                  <button
                                    key={b.id}
                                    onClick={() => setSelectedId(b.id)}
                                    className={`rounded-md border px-1 py-1.5 text-center text-[11px] transition-all ${TONE_CLASS[tone]} ${
                                      selectedId === b.id ? 'ring-2 ring-red-500' : ''
                                    } ${matches(z.name, r.name, b) ? '' : 'opacity-25'}`}
                                  >
                                    <p className="font-semibold truncate">{b.name}</p>
                                    <p className="opacity-80">{b.product_count > 0 ? `${b.total_qty}` : '-'}</p>
                                  </button>
                                )
                              })}
                            </div>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              ))
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead className="text-slate-400 text-xs uppercase">
                    <tr>
                      <th className="text-left px-3 py-2">Location</th>
                      <th className="text-left px-3 py-2">Product</th>
                      <th className="text-right px-3 py-2">Stock</th>
                      <th className="text-right px-3 py-2">Reserved</th>
                      <th className="text-right px-3 py-2">Available</th>
                      <th className="text-left px-3 py-2">Status</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800">
                    {flat
                      .filter((f) => matches(f.zone, f.rack, f.bin))
                      .flatMap((f) => {
                        const path = `${f.zone} / ${f.rack} / ${f.bin.name}`
                        if (f.bin.products.length === 0) {
                          return [
                            <tr key={`e${f.bin.id}`} className="text-slate-500">
                              <td className="px-3 py-2 text-xs">{path}</td>
                              <td className="px-3 py-2" colSpan={4}>Empty</td>
                              <td className="px-3 py-2"><span className={`text-xs px-2 py-1 rounded-full ${TONE_PILL.empty}`}>Empty</span></td>
                            </tr>,
                          ]
                        }
                        return f.bin.products.map((p) => {
                          const i = info[p.product_id]
                          const tone = statusToTone(i?.status, i?.stock ?? p.stock)
                          return (
                            <tr key={`${f.bin.id}-${p.product_id}`} className="hover:bg-slate-800/30 cursor-pointer" onClick={() => setSelectedId(f.bin.id)}>
                              <td className="px-3 py-2 text-xs text-slate-400">{path}</td>
                              <td className="px-3 py-2">
                                <div className="flex items-center gap-2">
                                  <Thumb src={i?.image} name={p.name} size={28} />
                                  <span>{p.name}</span>
                                </div>
                              </td>
                              <td className="px-3 py-2 text-right">{i?.stock ?? p.stock}</td>
                              <td className="px-3 py-2 text-right text-slate-400">{i?.reserved ?? '-'}</td>
                              <td className="px-3 py-2 text-right font-medium">{i?.available ?? '-'}</td>
                              <td className="px-3 py-2"><span className={`text-xs px-2 py-1 rounded-full ${TONE_PILL[tone]}`}>{TONE_LABEL[tone]}</span></td>
                            </tr>
                          )
                        })
                      })}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          <div className="border border-slate-800 bg-slate-900 rounded-xl p-4 lg:sticky lg:top-4">
            {!selected ? (
              <div className="text-sm text-slate-500 py-10 text-center">
                Tip: kisi bin par click karo, uska product, location aur stock yahan dikhega.
              </div>
            ) : (
              (() => {
                const tone = binTone(selected.bin)
                const first = selected.bin.products[0]
                const fi = first ? info[first.product_id] : undefined
                return (
                  <div>
                    <div className="flex items-start justify-between mb-2">
                      <h2 className="text-base font-semibold">Rack {selected.rack} - Bin {selected.bin.name}</h2>
                      <button onClick={() => setSelectedId(null)} className="text-slate-500 hover:text-slate-300 text-lg leading-none">&times;</button>
                    </div>
                    <span className={`text-xs px-2 py-1 rounded-full ${TONE_PILL[tone]}`}>{TONE_LABEL[tone]}</span>
                    <p className="text-xs text-slate-500 mt-2 mb-3">Zone {selected.zone} / {selected.rack} / {selected.bin.name}</p>

                    {first && (
                      <div className="flex items-center gap-3 mb-3">
                        <Thumb src={fi?.image} name={first.name} size={64} />
                        <div>
                          <p className="text-sm font-semibold">{first.name}</p>
                          <p className="text-xs text-slate-500">Bin total: {selected.bin.total_qty} units</p>
                        </div>
                      </div>
                    )}

                    <div className="grid grid-cols-3 gap-2 mb-4">
                      {[
                        { l: 'Rack', v: selected.rack },
                        { l: 'Bin', v: selected.bin.name },
                        { l: 'Total Stock', v: `${selected.bin.total_qty}` },
                      ].map((c) => (
                        <div key={c.l} className="border border-slate-800 rounded-lg p-2">
                          <p className="text-[10px] text-slate-500">{c.l}</p>
                          <p className="text-sm font-semibold truncate">{c.v}</p>
                        </div>
                      ))}
                    </div>

                    <p className="text-sm font-semibold mb-2">Items in this Bin</p>
                    {selected.bin.products.length === 0 ? (
                      <p className="text-sm text-slate-500">Empty.</p>
                    ) : (
                      <ul className="divide-y divide-slate-800">
                        {selected.bin.products.map((p) => {
                          const i = info[p.product_id]
                          const t = statusToTone(i?.status, i?.stock ?? p.stock)
                          return (
                            <li key={p.product_id} className="flex items-center gap-3 py-2.5">
                              <Thumb src={i?.image} name={p.name} size={40} />
                              <div className="flex-1 min-w-0">
                                <p className="text-sm font-medium truncate">{p.name}</p>
                                <p className="text-xs text-slate-500">
                                  Stock {i?.stock ?? p.stock}
                                  {i ? ` · Reserved ${i.reserved} · Available ${i.available}` : ''}
                                </p>
                              </div>
                              <span className={`text-xs px-2 py-1 rounded-full whitespace-nowrap ${TONE_PILL[t]}`}>
                                {i ? `${i.available} left` : `${p.stock} pcs`}
                              </span>
                            </li>
                          )
                        })}
                      </ul>
                    )}
                  </div>
                )
              })()
            )}
          </div>
        </div>
      )}
    </div>
  )
}