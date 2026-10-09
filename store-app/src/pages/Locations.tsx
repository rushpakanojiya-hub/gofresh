import { useCallback, useEffect, useState } from 'react'
import { createBin, createRack, createZone, getLocationOccupancy } from '../api/warehouse'
import type { OccupancyBin, OccupancyData } from '../types/warehouse'
import { getErrorMessage } from '../utils/errors'

export default function Locations() {
  const [data, setData] = useState<OccupancyData | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [selected, setSelected] = useState<{ path: string; bin: OccupancyBin } | null>(null)

  const load = useCallback(async () => {
    try {
      setData(await getLocationOccupancy())
      setError(null)
    } catch (err) {
      setError(getErrorMessage(err, 'Failed to load locations.'))
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const [adding, setAdding] = useState<{ kind: 'zone' | 'rack' | 'bin'; parentId?: number } | null>(null)
  const [newName, setNewName] = useState('')
  const [isSaving, setIsSaving] = useState(false)
  const [addError, setAddError] = useState<string | null>(null)

  const openAdd = (kind: 'zone' | 'rack' | 'bin', parentId?: number) => {
    setAdding({ kind, parentId })
    setNewName('')
    setAddError(null)
  }

  const submitAdd = async () => {
    const name = newName.trim()
    if (!adding || !name) return
    setIsSaving(true)
    setAddError(null)
    try {
      if (adding.kind === 'zone') await createZone(name)
      else if (adding.kind === 'rack') await createRack(adding.parentId as number, name)
      else await createBin(adding.parentId as number, name)
      setAdding(null)
      setNewName('')
      await load()
    } catch (err) {
      setAddError(getErrorMessage(err, 'Failed to save.'))
    } finally {
      setIsSaving(false)
    }
  }
  const s = data?.summary

  return (
    <div className="p-6 max-w-6xl">
      <div className="flex items-center justify-between mb-4">
        <div>
          <p className="font-mono text-[10px] tracking-widest text-red-500 uppercase mb-1">Warehouse</p>
          <h1 className="font-display text-2xl font-semibold">Locations</h1>
        </div>
        <button
          onClick={load}
          className="text-xs px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 border border-slate-700 transition-colors"
        >
          Refresh
        </button>
      </div>

      <div className="mb-4">
        <button
          onClick={() => openAdd('zone')}
          className="text-xs px-3 py-1.5 rounded-lg bg-red-600 hover:bg-red-500 transition-colors"
        >
          + Add Zone
        </button>
      </div>
      {error && (
        <div className="border border-rose-900 bg-rose-950/40 text-rose-300 text-sm rounded-lg px-4 py-3 mb-4">
          {error}
        </div>
      )}
      {isLoading && <p className="text-sm text-slate-400">Loading...</p>}

      {s && (
        <div className="grid grid-cols-2 md:grid-cols-5 gap-3 mb-5">
          {[
            { label: 'Zones', value: s.zones },
            { label: 'Racks', value: s.racks },
            { label: 'Bins', value: s.bins },
            { label: 'Occupied Bins', value: `${s.occupied_bins}/${s.bins}` },
            { label: 'Products w/o Bin', value: s.unassigned_products },
          ].map((c) => (
            <div key={c.label} className="border border-slate-800 bg-slate-900 rounded-xl p-3">
              <p className="text-xs text-slate-500">{c.label}</p>
              <p className="text-xl font-semibold mt-1">{c.value}</p>
            </div>
          ))}
        </div>
      )}

      {data && data.zones.length === 0 && (
        <div className="border border-slate-800 rounded-xl bg-slate-900 p-8 text-center text-sm text-slate-500">
          No zones defined yet.
        </div>
      )}

      {data?.zones.map((z) => (
        <div key={z.id} className="mb-6">
          <div className="flex items-center justify-between mb-2"><p className="text-xs uppercase tracking-wide text-slate-500">Zone {z.name}</p><button onClick={() => openAdd('rack', z.id)} className="text-xs px-2 py-1 rounded-lg bg-slate-800 hover:bg-slate-700 border border-slate-700 transition-colors">+ Rack</button></div>
          {z.racks.length === 0 && <p className="text-sm text-slate-600 mb-2">No racks.</p>}
          {z.racks.map((r) => (
            <div key={r.id} className="border border-slate-800 bg-slate-900 rounded-xl p-3 mb-3">
              <div className="flex items-center justify-between mb-2"><p className="text-sm font-medium">Rack {r.name}</p><button onClick={() => openAdd('bin', r.id)} className="text-xs px-2 py-1 rounded-lg bg-slate-800 hover:bg-slate-700 border border-slate-700 transition-colors">+ Bin</button></div>
              {r.bins.length === 0 ? (
                <p className="text-xs text-slate-600">No bins.</p>
              ) : (
                <div className="grid grid-cols-3 md:grid-cols-6 gap-2">
                  {r.bins.map((b) => (
                    <button
                      key={b.id}
                      onClick={() => setSelected({ path: `${z.name} / ${r.name} / ${b.name}`, bin: b })}
                      className={`text-left rounded-lg border px-2 py-2 text-xs transition-colors ${
                        b.product_count > 0
                          ? 'border-emerald-800 bg-emerald-950/40 hover:bg-emerald-950/70'
                          : 'border-slate-800 bg-slate-950 text-slate-500 hover:bg-slate-800/40'
                      }`}
                    >
                      <p className="font-medium">{b.name}</p>
                      <p>{b.product_count > 0 ? `${b.total_qty} units` : 'Empty'}</p>
                    </button>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      ))}

      {adding && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl w-full max-w-sm p-6">
            <h2 className="text-base font-semibold mb-3">Add {adding.kind}</h2>
            <input
              autoFocus
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder={adding.kind === 'zone' ? 'e.g. A' : adding.kind === 'rack' ? 'e.g. A-01' : 'e.g. A-01-01'}
              className="w-full text-sm px-3 py-2 rounded-lg bg-slate-950 border border-slate-700 mb-3"
            />
            {addError && <p className="text-sm text-rose-300 mb-3">{addError}</p>}
            <div className="flex justify-end gap-2">
              <button
                onClick={() => setAdding(null)}
                className="text-sm px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={submitAdd}
                disabled={isSaving || !newName.trim()}
                className="text-sm px-3 py-1.5 rounded-lg bg-red-600 hover:bg-red-500 disabled:opacity-50 transition-colors"
              >
                {isSaving ? 'Saving...' : 'Save'}
              </button>
            </div>
          </div>
        </div>
      )}
      {selected && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl w-full max-w-sm p-6">
            <h2 className="text-base font-semibold mb-1">Bin {selected.bin.name}</h2>
            <p className="text-xs text-slate-500 mb-4">{selected.path}</p>
            {selected.bin.products.length === 0 ? (
              <p className="text-sm text-slate-400 mb-4">Empty.</p>
            ) : (
              <ul className="divide-y divide-slate-800 mb-4">
                {selected.bin.products.map((p) => (
                  <li key={p.product_id} className="flex justify-between py-2 text-sm">
                    <span>{p.name}</span>
                    <span className="text-slate-400">{p.stock}</span>
                  </li>
                ))}
              </ul>
            )}
            <div className="flex justify-end">
              <button
                onClick={() => setSelected(null)}
                className="text-sm px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 transition-colors"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}