import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { stats, api } from '../api.js'
import { StatCard, Card, Badge, Spinner, Alert } from '../components/UI.jsx'
import Icon from '../components/Icon.jsx'

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

  const load = () => stats().then(d => setData(d.data)).catch(e => setErr(e.message))
  useEffect(() => {
    load()
    api('/geoip/countries').then(d=>setCountries(d.data||[])).catch(()=>{})
    api('/version').then(d=>setVer(d.data)).catch(()=>{})
    const t = setInterval(load, 10000)
    return () => clearInterval(t)
  }, [])

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

  return (
    <div className="space-y-6 max-w-7xl">
      {ver?.update_available && (
        <Alert variant="warn">
          Neue Version verfügbar: <a className="underline" href={ver.url} target="_blank" rel="noreferrer">v{ver.latest}</a> – aktuell läuft v{ver.current}.
        </Alert>
      )}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Sites" value={data.sites} icon="sites" accent="indigo" />
        <StatCard label="Verbindungen (24h)" value={total} icon="bolt" accent="sky" hint={`${(m.rps || 0).toFixed(2)} req/s`} />
        <StatCard label="Geblockt (24h)" value={blocked} icon="shield" accent="rose" hint={`${blockRate}% Blockrate`} />
        <StatCard label="Zertifikate" value={data.certs} icon="certs" accent="emerald" />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card title="Traffic" subtitle="Letzte 24 Stunden">
          <div className="grid grid-cols-2 gap-4 mb-5">
            <div className="rounded-lg bg-emerald-50 p-4">
              <div className="text-2xl font-bold text-emerald-700">{allowed}</div>
              <div className="text-sm text-emerald-600">Erlaubt</div>
            </div>
            <div className="rounded-lg bg-rose-50 p-4">
              <div className="text-2xl font-bold text-rose-700">{blocked}</div>
              <div className="text-sm text-rose-600">Geblockt</div>
            </div>
          </div>
          <Bar label="Erlaubt" value={allowed} total={total} color="bg-emerald-500" />
          <div className="mt-4 grid grid-cols-2 gap-x-8 gap-y-3">
            <div>
              <div className="text-xs font-semibold text-slate-400 uppercase mb-2">Status</div>
              {Object.keys(statuses).length === 0 && <div className="text-sm text-slate-400">—</div>}
              {Object.entries(statuses).map(([k, v]) => (
                <div key={k} className="flex items-center justify-between text-sm py-0.5">
                  <span className="text-slate-500">{k}</span><span className="text-slate-700 font-medium">{v}</span>
                </div>
              ))}
            </div>
            <div>
              <div className="text-xs font-semibold text-slate-400 uppercase mb-2">Block-Gründe</div>
              {Object.keys(reasons).length === 0 && <div className="text-sm text-slate-400">—</div>}
              {Object.entries(reasons).map(([k, v]) => (
                <div key={k} className="flex items-center justify-between text-sm py-0.5">
                  <span className="text-slate-500">{k}</span><span className="text-slate-700 font-medium">{v}</span>
                </div>
              ))}
            </div>
          </div>
        </Card>

        <div className="space-y-6">
          <Card title="Top Sites">
            {topSites.length === 0 ? (
              <div className="text-sm text-slate-400 py-6 text-center">Noch kein Traffic.</div>
            ) : (
              <div className="divide-y divide-slate-100">
                {topSites.map(s => (
                  <div key={s.domain} className="flex items-center justify-between py-2.5">
                    <div className="min-w-0">
                      <div className="text-sm font-medium text-slate-800 truncate">{s.domain}</div>
                      <div className="text-xs text-slate-400">{s.blocked} geblockt</div>
                    </div>
                    <Badge variant="slate">{s.total}</Badge>
                  </div>
                ))}
              </div>
            )}
          </Card>

          <Card title="Geblockte Länder" subtitle="Top 10 nach Blockierungen">
            {blockedCountries.length === 0 ? (
              <div className="text-sm text-slate-400 py-6 text-center">Keine geblockten Länder.</div>
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
        </div>
      </div>

      <Card title="System-Ressourcen" subtitle="Live-Auslastung">
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-6">
          <SysMetric label="CPU" pct={sys.cpu_percent || 0}
            detail={`Load ${(sys.load1 || 0).toFixed(2)} / ${(sys.load5 || 0).toFixed(2)} / ${(sys.load15 || 0).toFixed(2)}`}
            color="bg-indigo-500" />
          <SysMetric label="RAM"
            pct={sys.ram_total ? (sys.ram_used / sys.ram_total * 100) : 0}
            detail={`${fmtBytes(sys.ram_used)} / ${fmtBytes(sys.ram_total)}`}
            color="bg-emerald-500" />
          <SysMetric label="Disk"
            pct={sys.disk_total ? (sys.disk_used / sys.disk_total * 100) : 0}
            detail={`${fmtBytes(sys.disk_used)} / ${fmtBytes(sys.disk_total)}`}
            color="bg-amber-500" />
        </div>
      </Card>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <Card title="Ablaufende Zertifikate" subtitle="Nächste 30 Tage" className="lg:col-span-2">
          {expiring.length === 0 ? (
            <div className="text-sm text-slate-400 py-6 text-center">Keine Zertifikate laufen demnächst ab.</div>
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
                    <Badge variant={dl < 7 ? 'red' : 'amber'}>{dl} Tage</Badge>
                  </div>
                )
              })}
            </div>
          )}
        </Card>

        <Card title="Schnellzugriff">
          <div className="grid grid-cols-2 gap-3">
            <Link to="/sites" className="flex flex-col items-center gap-2 p-4 rounded-lg border border-slate-200 hover:border-indigo-300 hover:bg-indigo-50/40 transition-colors">
              <Icon name="plus" className="w-5 h-5 text-indigo-600" stroke />
              <span className="text-sm text-slate-700">Neue Site</span>
            </Link>
            <Link to="/logs" className="flex flex-col items-center gap-2 p-4 rounded-lg border border-slate-200 hover:border-indigo-300 hover:bg-indigo-50/40 transition-colors">
              <Icon name="clock" className="w-5 h-5 text-sky-600" />
              <span className="text-sm text-slate-700">Logs</span>
            </Link>
            <Link to="/crowdsec" className="flex flex-col items-center gap-2 p-4 rounded-lg border border-slate-200 hover:border-indigo-300 hover:bg-indigo-50/40 transition-colors">
              <Icon name="shield" className="w-5 h-5 text-rose-600" />
              <span className="text-sm text-slate-700">CrowdSec</span>
            </Link>
            <Link to="/certificates" className="flex flex-col items-center gap-2 p-4 rounded-lg border border-slate-200 hover:border-indigo-300 hover:bg-indigo-50/40 transition-colors">
              <Icon name="certs" className="w-5 h-5 text-emerald-600" />
              <span className="text-sm text-slate-700">Zertifikat</span>
            </Link>
          </div>
        </Card>
      </div>
    </div>
  )
}
