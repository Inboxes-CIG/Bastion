import { useEffect, useState } from 'react'
import { me, api } from '../api.js'
import { Card, Alert, Spinner, Button, Input, Select, Field, Badge, Toggle } from '../components/UI.jsx'

function Row({ label, value }) {
  return (
    <div className="flex items-center justify-between py-3 border-b border-slate-100 last:border-0">
      <span className="text-sm text-slate-500">{label}</span>
      <span className="text-sm font-medium text-slate-800">{value}</span>
    </div>
  )
}

function fmtBytes(n) {
  if (!n) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB']
  let i = 0
  while (n >= 1024 && i < u.length - 1) { n /= 1024; i++ }
  return n.toFixed(1) + ' ' + u[i]
}

export default function Settings() {
  const [user, setUser] = useState(null)
  const [adminInfo, setAdminInfo] = useState(null)
  const [certs, setCerts] = useState([])
  const [adminDomain, setAdminDomain] = useState('')
  const [email, setEmail] = useState('')
  const [msg, setMsg] = useState('')
  const [busy, setBusy] = useState(false)
  const [cacheData, setCacheData] = useState(null)
  const [cacheCfg, setCacheCfg] = useState(null)
  const [cacheMsg, setCacheMsg] = useState('')

  const loadCerts = () => api('/certificates').then(d=>setCerts(d.data||[])).catch(()=>{})
  const [ver, setVer] = useState(null)

  const loadCache = () => api('/cache').then(d => { setCacheData(d.data); setCacheCfg(d.data.config) }).catch(()=>{})

  useEffect(() => {
    me().then(d=>setUser(d.data)).catch(()=>{})
    api('/admin/info').then(d => {
      setAdminInfo(d.data)
      if (d.data.admin_hosts && d.data.admin_hosts.length) setAdminDomain(d.data.admin_hosts[0])
    }).catch(()=>{})
    loadCerts()
    loadCache()
    api('/version').then(d=>setVer(d.data)).catch(()=>{})
  }, [])

  if (!user) return <Spinner />

  const adminHosts = adminInfo?.admin_hosts || []
  const current = certs.find(c => c.source === 'letsencrypt' && c.domain === adminDomain)

  const issue = async () => {
    setMsg(''); setBusy(true)
    try {
      const r = await api('/certificates/issue', { method:'POST', body: JSON.stringify({ domain: adminDomain, email }) })
      setMsg('Ausgestellt für: ' + (r.data.domains || []).join(', '))
      loadCerts()
    } catch(e) { setMsg('Fehler: ' + e.message) } finally { setBusy(false) }
  }

  const saveCache = async () => {
    setCacheMsg('')
    try {
      const r = await api('/cache/config', { method:'PUT', body: JSON.stringify(cacheCfg) })
      setCacheData(r.data); setCacheCfg(r.data.config); setCacheMsg('Gespeichert.')
    } catch(e) { setCacheMsg('Fehler: ' + e.message) }
  }
  const clearCache = async () => {
    setCacheMsg('')
    try { const r = await api('/cache/clear', { method:'POST' }); setCacheMsg('Cache geleert.'); loadCache() }
    catch(e) { setCacheMsg('Fehler: ' + e.message) }
  }
  const setCfg = (k, v) => setCacheCfg({ ...cacheCfg, [k]: v })

  return (
    <div className="max-w-3xl space-y-6">
      <Card title="Konto">
        <Row label="E-Mail" value={user.email} />
        <Row label="Rolle" value={user.role} />
        <Row label="Tenant" value={user.tenant_id || 'System'} />
        <Row label="Status" value={user.active ? 'aktiv' : 'inaktiv'} />
      </Card>

      <Card title="GUI-Zertifikat (Let's Encrypt)" subtitle="Gültiges TLS-Zertifikat für die Admin-Oberfläche">
        <div className="text-sm text-slate-600 mb-2">Admin-Hosts:</div>
        <div className="flex flex-wrap gap-1 mb-4">
          {adminHosts.length === 0 && <span className="text-sm text-slate-400">—</span>}
          {adminHosts.map(h => (
            <Badge key={h} variant={h === adminDomain ? 'indigo' : 'slate'}>{h}</Badge>
          ))}
        </div>

        {current ? (
          <Alert variant="success">
            Aktiv: {current.domain} · gültig bis {current.not_after ? new Date(current.not_after).toLocaleDateString() : '—'}
          </Alert>
        ) : (
          <Alert variant="warn">Für {adminDomain || 'die Admin-Domain'} ist noch kein Let's-Encrypt-Zertifikat vorhanden.</Alert>
        )}

        <div className="flex flex-col sm:flex-row sm:items-end gap-2 mt-4">
          <Field label="Admin-Domain" className="flex-1">
            <Select value={adminDomain} onChange={e=>setAdminDomain(e.target.value)}>
              {adminHosts.map(h => <option key={h} value={h}>{h}</option>)}
            </Select>
          </Field>
          <Field label="E-Mail für ACME" className="flex-1">
            <Input value={email} onChange={e=>setEmail(e.target.value)} placeholder="admin@example.com" />
          </Field>
          <Button variant="success" onClick={issue} disabled={busy || !email || !adminDomain}>
            {busy ? 'Stelle aus...' : (current ? 'Neu ausstellen' : 'Ausstellen')}
          </Button>
        </div>
        {msg && <div className={`text-sm mt-2 ${msg.startsWith('Fehler')?'text-red-600':'text-emerald-600'}`}>{msg}</div>}
        <div className="mt-4"><Alert variant="info">Die Admin-Domain muss öffentlich auf diesen Server zeigen und Port 80 erreichbar sein (HTTP-01). Zugriff dann über https://&lt;Admin-Domain&gt;/.</Alert></div>
      </Card>

      <Card title="Response-Cache" subtitle="HTTP-Cache für anonyme GET-Anfragen">
        {!cacheCfg ? <Spinner /> : (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-4">
              <div className="rounded-lg bg-slate-50 p-3"><div className="text-xl font-bold text-slate-800">{cacheData?.entries ?? 0}</div><div className="text-xs text-slate-500">Einträge</div></div>
              <div className="rounded-lg bg-slate-50 p-3"><div className="text-xl font-bold text-slate-800">{fmtBytes(cacheData?.size_bytes)}</div><div className="text-xs text-slate-500">Größe</div></div>
              <div className="rounded-lg bg-slate-50 p-3"><div className="text-xl font-bold text-emerald-700">{cacheData?.hits ?? 0}</div><div className="text-xs text-slate-500">Hits</div></div>
              <div className="rounded-lg bg-slate-50 p-3"><div className="text-xl font-bold text-amber-700">{cacheData?.misses ?? 0}</div><div className="text-xs text-slate-500">Misses</div></div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <Field label="Cache aktiv"><div className="pt-2"><Toggle checked={!!cacheCfg.enabled} onChange={v=>setCfg('enabled', v)} label={cacheCfg.enabled ? 'an' : 'aus'} /></div></Field>
              <Field label="Haltezeit (TTL, Sekunden)"><Input type="number" value={cacheCfg.ttl_seconds} onChange={e=>setCfg('ttl_seconds', Number(e.target.value))} /></Field>
              <Field label="Max. Einträge"><Input type="number" value={cacheCfg.max_entries} onChange={e=>setCfg('max_entries', Number(e.target.value))} /></Field>
              <Field label="Max. Objektgröße (KB)"><Input type="number" value={cacheCfg.max_object_kb} onChange={e=>setCfg('max_object_kb', Number(e.target.value))} /></Field>
              <Field label="Max. Gesamtgröße (MB)"><Input type="number" value={cacheCfg.max_total_mb} onChange={e=>setCfg('max_total_mb', Number(e.target.value))} /></Field>
            </div>

            <div className="flex items-center gap-2 mt-4">
              <Button onClick={saveCache}>Speichern</Button>
              <Button variant="secondary" onClick={clearCache}>Cache leeren</Button>
              {cacheMsg && <span className={`text-sm ${cacheMsg.startsWith('Fehler')?'text-red-600':'text-emerald-600'}`}>{cacheMsg}</span>}
            </div>
            <div className="mt-3 text-xs text-slate-400">Gecacht werden GET-Antworten (2xx/3xx/404/410) – auch komprimiert (pro Accept-Encoding) und mit Cookies. Ausgenommen: Cache-Control: no-cache/no-store, Set-Cookie oder Vary: Cookie.</div>
          </>
        )}
      </Card>

      <Card title="Version">
        <Row label="Installiert" value={ver?.current || '—'} />
        <Row label="Neueste" value={ver?.latest || '—'} />
        <Row label="Status" value={ver?.update_available ? 'Update verfügbar' : 'Aktuell'} />
        {ver?.update_available && (
          <div className="pt-2">
            <a href={ver.url} target="_blank" rel="noreferrer" className="text-sm text-indigo-600 hover:underline">Release v{ver.latest} öffnen</a>
          </div>
        )}
      </Card>

      <Card title="Über diese Instanz">
        <Row label="Produkt" value="Bastion" />
        <Row label="Backend" value="Go 1.26 · Gin · PostgreSQL" />
        <Row label="Frontend" value="React · Vite · Tailwind" />
        <Row label="Proxy" value="net/http ReverseProxy · TLS SNI · HTTP/3" />
        <Row label="GeoIP" value="DB-IP Lite (MMDB)" />
        <Row label="Zertifikate" value="certmagic (Let's Encrypt) + manuell" />
      </Card>
    </div>
  )
}
