import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { stats, api, getUser } from '../api.js'
import { StatCard, Card, Badge, Spinner, Alert, Button } from '../components/UI.jsx'
import Icon from '../components/Icon.jsx'
import { t } from '../i18n.js'

const LAYOUT_KEY = 'bastion_dashboard_layout'
const DEFAULT_ORDER = ['stats', 'traffic', 'topsites', 'system', 'countries', 'certs', 'quick']
const META = {
  stats: { label: 'Kennzahlen', span: 3 },
  traffic: { label: 'Traffic', span: 2 },
  system: { label: 'System-Ressourcen', span: 2 },
  topsites: { label: 'Top Sites', span: 1 },
  countries: { label: 'Geblockte Länder', span: 1 },
  certs: { label: 'Ablaufende Zertifikate', span: 2 },
  quick: { label: 'Schnellzugriff', span: 1 },
}
const DEFAULT_SIZES = Object.fromEntries(DEFAULT_ORDER.map(id => [id, META[id].span]))

function loadLayout() {
  const defaults = () => ({ order: DEFAULT_ORDER.slice(), hidden: [], sizes: { ...DEFAULT_SIZES } })
  try {
    const raw = localStorage.getItem(LAYOUT_KEY)
    if (!raw) return defaults()
    const parsed = JSON.parse(raw)
    const known = new Set(DEFAULT_ORDER)
    const order = (parsed.order || []).filter(id => known.has(id))
    for (const id of DEFAULT_ORDER) {
      if (!order.includes(id)) order.push(id)
    }
    const hidden = (parsed.hidden || []).filter(id => known.has(id))
    const sizes = { ...DEFAULT_SIZES }
    for (const id of DEFAULT_ORDER) {
      const size = Number(parsed.sizes?.[id])
      if (size >= 1 && size <= 3) sizes[id] = size
    }
    return { order, hidden, sizes }
  } catch {
    return defaults()
  }
}

function daysLeft(d) {
  if (!d) return null
  return Math.ceil((new Date(d) - Date.now()) / 86400000)
}

function Bar({ label, value, total, color }) {
  const pct = total > 0 ? Math.round((value / total) * 100) : 0
  return (
    <div>
      <div className="flex items-center justify-between text-sm mb-1">
        <span className="text-slate-600">{label}</span>
        <span className="text-slate-500">{value} <span className="text-slate-300">/</span> {total}</span>
      </div>
      <div className="h-2 rounded-full bg-slate-100 overflow-hidden">
        <div className={`h-full ${color}`} style={{ width: pct + '%' }} />
      </div>
    </div>
  )
}

function fmtBytes(n) {
  if (!n) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  while (n >= 1024 && i < u.length - 1) { n /= 1024; i++ }
  return n.toFixed(1) + ' ' + u[i]
}

function SysMetric({ label, pct, detail, color }) {
  const p = Math.max(0, Math.min(100, pct || 0))
  return (
    <div>
      <div className="flex items-center justify-between text-sm mb-1">
        <span className="text-slate-600">{label}</span>
        <span className="text-slate-500">{p.toFixed(0)} %</span>
      </div>
      <div className="h-2 rounded-full bg-slate-100 overflow-hidden">
        <div className={`h-full ${color}`} style={{ width: p + '%' }} />
      </div>
      {detail && <div className="text-xs text-slate-400 mt-1">{detail}</div>}
    </div>
  )
}

export default function Dashboard() {
  const [data, setData] = useState(null)
  const [countries, setCountries] = useState([])
  const [err, setErr] = useState('')
  const [ver, setVer] = useState(null)
  const [layout, setLayout] = useState(loadLayout)
  const [editMode, setEditMode] = useState(false)
  const [dragId, setDragId] = useState(null)
  const [dragOver, setDragOver] = useState(null)

  const load = () => stats().then(d => setData(d.data)).catch(e => setErr(e.message))
  useEffect(() => {
    load()
    api('/geoip/countries').then(d=>setCountries(d.data||[])).catch(()=>{})
    api('/version').then(d=>setVer(d.data)).catch(()=>{})
    const timer = setInterval(load, 10000)
    return () => clearInterval(timer)
  }, [])

  const persist = (next) => {
    setLayout(next)
    try { localStorage.setItem(LAYOUT_KEY, JSON.stringify(next)) } catch {}
  }
  const hide = (id) => persist({ ...layout, hidden: [...new Set([...layout.hidden, id])] })
  const show = (id) => persist({ ...layout, hidden: layout.hidden.filter(x => x !== id) })
  const reset = () => {
    try { localStorage.removeItem(LAYOUT_KEY) } catch {}
    setLayout({ order: DEFAULT_ORDER.slice(), hidden: [], sizes: { ...DEFAULT_SIZES } })
  }
  const setSize = (id, size) => persist({ ...layout, sizes: { ...layout.sizes, [id]: size } })
  const dropOn = (target) => {
    if (!dragId || dragId === target) return
    const order = layout.order.filter(id => id !== dragId)
    const idx = order.indexOf(target)
    if (idx < 0) return
    order.splice(idx, 0, dragId)
    persist({ ...layout, order })
  }

  const visible = useMemo(() => layout.order.filter(id => !layout.hidden.includes(id)), [layout])
  const hidden = useMemo(() => layout.order.filter(id => layout.hidden.includes(id)), [layout])

  if (err) return <Alert variant="error">{err}</Alert>
  if (!data) return <Spinner />

  const m = data.traffic_24h || data.metrics || {}
  const sys = data.system || {}
  const expiring = data.expiring_certs || []
  const total = m.total || 0
  const blocked = m.blocked || 0
  const allowed = m.allowed || 0
  const blockRate = total > 0 ? Math.round((blocked / total) * 100) : 0
  const reasons = m.by_reason || {}
  const statuses = m.by_status || {}
  const topSites = m.top_sites || []
  const blockedCountries = m.blocked_countries || []
  const countryName = (code) => countries.find(c => c.code === code)?.name || code
  const maxCountry = blockedCountries.length > 0 ? blockedCountries[0].count : 1

  const renderWidget = (id) => {
    switch (id) {
      case 'stats':
        return (
          <div className={`grid grid-cols-1 sm:grid-cols-2 ${(layout.sizes?.stats || 3) >= 3 ? 'lg:grid-cols-4' : 'lg:grid-cols-2'} gap-4`}>
            <StatCard label={t('Sites')} value={data.sites} icon="sites" accent="indigo" />
            <StatCard label={t('Verbindungen (24h)')} value={total} icon="bolt" accent="sky" hint={`${(m.rps || 0).toFixed(2)} req/s`} />
            <StatCard label={t('Geblockt (24h)')} value={blocked} icon="shield" accent="rose" hint={t('{n}% Blockrate', { n: blockRate })} />
            <StatCard label={t('Zertifikate')} value={data.certs} icon="certs" accent="emerald" />
          </div>
        )
      case 'traffic':
        return (
          <Card title={t('Traffic')} subtitle={t('Letzte 24 Stunden')}>
            <div className="flex items-center gap-8 mb-2">
              <div>
                <span className="text-2xl font-bold text-emerald-700">{allowed}</span>
                <span className="text-sm text-emerald-600 ml-1.5">{t('Erlaubt')}</span>
              </div>
              <div>
                <span className="text-2xl font-bold text-rose-700">{blocked}</span>
                <span className="text-sm text-rose-600 ml-1.5">{t('Geblockt')}</span>
              </div>
            </div>
            <Bar label={t('Erlaubt')} value={allowed} total={total} color="bg-emerald-500" />
            <div className="mt-4 grid grid-cols-2 gap-x-8 gap-y-3">
              <div>
                <div className="text-xs font-semibold text-slate-400 uppercase mb-2">{t('Status')}</div>
                {Object.keys(statuses).length === 0 && <div className="text-sm text-slate-400">—</div>}
                {Object.entries(statuses).map(([k, v]) => (
                  <div key={k} className="flex items-center justify-between text-sm py-0.5">
                    <span className="text-slate-500">{k}</span><span className="text-slate-700 font-medium">{v}</span>
                  </div>
                ))}
              </div>
              <div>
                <div className="text-xs font-semibold text-slate-400 uppercase mb-2">{t('Block-Gründe')}</div>
                {Object.keys(reasons).length === 0 && <div className="text-sm text-slate-400">—</div>}
                {Object.entries(reasons).map(([k, v]) => (
                  <div key={k} className="flex items-center justify-between text-sm py-0.5">
                    <span className="text-slate-500">{k}</span><span className="text-slate-700 font-medium">{v}</span>
                  </div>
                ))}
              </div>
            </div>
          </Card>
        )
      case 'system':
        return (
          <Card title={t('System-Ressourcen')} subtitle={t('Live-Auslastung')}>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-6">
              <SysMetric label="CPU" pct={sys.cpu_percent || 0}
                detail={`Load ${(sys.load1 || 0).toFixed(2)} / ${(sys.load5 || 0).toFixed(2)} / ${(sys.load15 || 0).toFixed(2)}`}
                color="bg-indigo-500" />
              <SysMetric label="RAM"
                pct={sys.ram_total ? (sys.ram_used / sys.ram_total * 100) : 0}
                detail={`${fmtBytes(sys.ram_used)} / ${fmtBytes(sys.ram_total)}`}
                color="bg-emerald-500" />
              <SysMetric label={t('Disk')}
                pct={sys.disk_total ? (sys.disk_used / sys.disk_total * 100) : 0}
                detail={`${fmtBytes(sys.disk_used)} / ${fmtBytes(sys.disk_total)}`}
                color="bg-amber-500" />
            </div>
          </Card>
        )
      case 'topsites':
        return (
          <Card title={t('Top Sites')}>
            {topSites.length === 0 ? (
              <div className="text-sm text-slate-400 py-6 text-center">{t('Noch kein Traffic.')}</div>
            ) : (
              <div className="divide-y divide-slate-100">
                {topSites.map(s => (
                  <div key={s.domain} className="flex items-center justify-between py-2.5">
                    <div className="min-w-0">
                      <div className="text-sm font-medium text-slate-800 truncate">{s.domain}</div>
                      <div className="text-xs text-slate-400">{s.blocked} {t('geblockt')}</div>
                    </div>
                    <Badge variant="slate">{s.total}</Badge>
                  </div>
                ))}
              </div>
            )}
          </Card>
        )
      case 'countries':
        return (
          <Card title={t('Geblockte Länder')} subtitle={t('Top 10 nach Blockierungen')}>
            {blockedCountries.length === 0 ? (
              <div className="text-sm text-slate-400 py-6 text-center">{t('Keine geblockten Länder.')}</div>
            ) : (
              <div className="space-y-2.5">
                {blockedCountries.map(c => (
                  <div key={c.country}>
                    <div className="flex items-center justify-between text-sm mb-1">
                      <span className="text-slate-700">{countryName(c.country)} <span className="text-xs text-slate-400 font-mono">{c.country}</span></span>
                      <span className="font-medium text-slate-700">{c.count}</span>
                    </div>
                    <div className="h-1.5 rounded-full bg-slate-100 overflow-hidden">
                      <div className="h-full bg-rose-500" style={{ width: Math.round((c.count / maxCountry) * 100) + '%' }} />
                    </div>
                  </div>
                ))}
              </div>
            )}
          </Card>
        )
      case 'certs':
        return (
          <Card title={t('Ablaufende Zertifikate')} subtitle={t('Nächste 30 Tage')}>
            {expiring.length === 0 ? (
              <div className="text-sm text-slate-400 py-6 text-center">{t('Keine Zertifikate laufen demnächst ab.')}</div>
            ) : (
              <div className="divide-y divide-slate-100">
                {expiring.map(c => {
                  const dl = daysLeft(c.not_after)
                  return (
                    <div key={c.id} className="flex items-center justify-between py-3">
                      <div className="flex items-center gap-3">
                        <Icon name="certs" className="w-5 h-5 text-slate-400" />
                        <div>
                          <div className="text-sm font-medium text-slate-800">{c.domain}</div>
                          <div className="text-xs text-slate-400">{c.source}</div>
                        </div>
                      </div>
                      <Badge variant={dl < 7 ? 'red' : 'amber'}>{dl} {t('Tage')}</Badge>
                    </div>
                  )
                })}
              </div>
            )}
          </Card>
        )
      case 'quick':
        return (
          <Card title={t('Schnellzugriff')}>
            <div className="grid grid-cols-2 gap-3">
              <Link to="/sites" className="flex flex-col items-center gap-2 p-4 rounded-lg border border-slate-200 hover:border-indigo-300 hover:bg-indigo-50/40 transition-colors">
                <Icon name="plus" className="w-5 h-5 text-indigo-600" stroke />
                <span className="text-sm text-slate-700">{t('Neue Site')}</span>
              </Link>
              <Link to="/logs" className="flex flex-col items-center gap-2 p-4 rounded-lg border border-slate-200 hover:border-indigo-300 hover:bg-indigo-50/40 transition-colors">
                <Icon name="clock" className="w-5 h-5 text-sky-600" />
                <span className="text-sm text-slate-700">{t('Logs')}</span>
              </Link>
              {getUser()?.role === 'superadmin' && (
              <Link to="/crowdsec" className="flex flex-col items-center gap-2 p-4 rounded-lg border border-slate-200 hover:border-indigo-300 hover:bg-indigo-50/40 transition-colors">
                <Icon name="shield" className="w-5 h-5 text-rose-600" />
                <span className="text-sm text-slate-700">CrowdSec</span>
              </Link>
              )}
              <Link to="/certificates" className="flex flex-col items-center gap-2 p-4 rounded-lg border border-slate-200 hover:border-indigo-300 hover:bg-indigo-50/40 transition-colors">
                <Icon name="certs" className="w-5 h-5 text-emerald-600" />
                <span className="text-sm text-slate-700">{t('Zertifikat')}</span>
              </Link>
            </div>
          </Card>
        )
      default:
        return null
    }
  }

  const spanClass = (id) => {
    const size = layout.sizes?.[id] || META[id]?.span || 1
    return size === 3 ? 'lg:col-span-3' : size === 2 ? 'lg:col-span-2' : 'lg:col-span-1'
  }

  return (
    <div className="space-y-6 max-w-7xl">
      {ver?.update_available && (
        <Alert variant="warn">
          {t('Neue Version verfügbar:')} <a className="underline" href={ver.url} target="_blank" rel="noreferrer">v{ver.latest}</a> – {t('aktuell läuft')} v{ver.current}.
        </Alert>
      )}

      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2">
        <div className="text-xs text-slate-400">{t('Layout wird in diesem Browser gespeichert.')}</div>
        <div className="flex items-center gap-2">
          {editMode && <Button variant="secondary" onClick={reset}>{t('Zurücksetzen')}</Button>}
          <Button variant={editMode ? 'success' : 'secondary'} onClick={()=>setEditMode(v=>!v)}>
            {editMode ? t('Fertig') : t('Dashboard anpassen')}
          </Button>
        </div>
      </div>

      {editMode && (
        <div className="rounded-lg border border-dashed border-indigo-300 bg-indigo-50/40 px-4 py-3 text-sm text-indigo-900">
          {t('Ziehe die Elemente an die gewünschte Position, ändere die Breite oder blende sie aus. Das Layout wird in diesem Browser gespeichert.')}
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 grid-flow-dense">
        {visible.map(id => (
          <div
            key={id}
            className={`min-w-0 ${spanClass(id)} ${dragOver === id ? 'ring-2 ring-indigo-400 rounded-xl' : ''}`}
            draggable={editMode}
            onDragStart={(e) => {
              if (!editMode) return
              setDragId(id)
              e.dataTransfer.effectAllowed = 'move'
              e.dataTransfer.setData('text/plain', id)
            }}
            onDragOver={(e) => {
              if (!editMode) return
              e.preventDefault()
              setDragOver(id)
            }}
            onDragLeave={() => setDragOver(null)}
            onDrop={(e) => {
              if (!editMode) return
              e.preventDefault()
              dropOn(id)
              setDragId(null)
              setDragOver(null)
            }}
            onDragEnd={() => { setDragId(null); setDragOver(null) }}
          >
            {editMode && (
              <div className="flex items-center justify-between gap-2 mb-1 px-1" onMouseDown={(e)=>e.stopPropagation()}>
                <span className="text-xs text-slate-500 cursor-grab select-none">⋮⋮ {t(META[id].label)}</span>
                <div className="flex items-center gap-3">
                  <label className="flex items-center gap-1 text-xs text-slate-500">
                    {t('Breite')}
                    <select
                      value={layout.sizes?.[id] || META[id].span}
                      onChange={(e)=>setSize(id, Number(e.target.value))}
                      className="rounded border border-slate-300 bg-white px-1.5 py-1 text-xs text-slate-700"
                    >
                      <option value={1}>1/3</option>
                      <option value={2}>2/3</option>
                      <option value={3}>{t('100%')}</option>
                    </select>
                  </label>
                  <button onClick={() => hide(id)} className="text-xs text-red-600 hover:underline">{t('Ausblenden')}</button>
                </div>
              </div>
            )}
            {renderWidget(id)}
          </div>
        ))}
      </div>

      {editMode && hidden.length > 0 && (
        <div className="rounded-lg border border-dashed border-slate-300 bg-white p-4">
          <div className="text-sm font-medium text-slate-700 mb-2">{t('Ausgeblendete Elemente')}</div>
          <div className="flex flex-wrap gap-2">
            {hidden.map(id => (
              <button key={id} onClick={() => show(id)} className="inline-flex items-center gap-1 rounded-full bg-slate-100 hover:bg-indigo-50 text-slate-700 px-3 py-1.5 text-sm">
                + {t(META[id].label)} {t('einblenden')}
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
