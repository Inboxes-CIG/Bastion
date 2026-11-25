import { useEffect, useState } from 'react'
import { me, api, mfaSetup, mfaEnable, mfaDisable, changePassword } from '../api.js'
import { Card, Alert, Spinner, Button, Input, Select, Field, Badge, Toggle } from '../components/UI.jsx'
import { t, currentLang, setLang, LANGS } from '../i18n.js'

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
  const [mfaData, setMfaData] = useState(null)
  const [mfaCode, setMfaCode] = useState('')
  const [mfaMsg, setMfaMsg] = useState('')
  const [mfaBusy, setMfaBusy] = useState(false)
  const [mfaPassword, setMfaPassword] = useState('')
  const [pw, setPw] = useState({ cur: '', nw: '', nw2: '' })
  const [pwMsg, setPwMsg] = useState('')

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

  const startMFA = async () => {
    setMfaMsg(''); setMfaBusy(true)
    try {
      const r = await mfaSetup()
      setMfaData(r.data); setMfaCode('')
    } catch(e) { setMfaMsg('Fehler: ' + e.message) } finally { setMfaBusy(false) }
  }
  const enableMFA = async () => {
    setMfaMsg(''); setMfaBusy(true)
    try {
      await mfaEnable(mfaCode)
      const d = await me()
      setUser(d.data); setMfaData(null); setMfaCode('')
      setMfaMsg('MFA wurde aktiviert.')
    } catch(e) { setMfaMsg('Fehler: ' + e.message) } finally { setMfaBusy(false) }
  }
  const disableMFA = async () => {
    setMfaMsg(''); setMfaBusy(true)
    try {
      await mfaDisable(mfaCode, mfaPassword)
      const d = await me()
      setUser(d.data); setMfaCode(''); setMfaPassword('')
      setMfaMsg(t('MFA wurde deaktiviert.'))
    } catch(e) { setMfaMsg('Fehler: ' + e.message) } finally { setMfaBusy(false) }
  }

  const savePassword = async () => {
    setPwMsg('')
    if (pw.nw.length < 8) { setPwMsg('Fehler: ' + t('Das neue Passwort muss mindestens 8 Zeichen haben.')); return }
    if (pw.nw !== pw.nw2) { setPwMsg('Fehler: ' + t('Die Passwörter stimmen nicht überein.')); return }
    try {
      await changePassword(pw.cur, pw.nw)
      setPw({ cur:'', nw:'', nw2:'' })
      setPwMsg(t('Passwort geändert.'))
    } catch(e) { setPwMsg('Fehler: ' + e.message) }
  }

  return (
    <div className="max-w-3xl space-y-6">
      <Card title={t('Konto')}>
        <Row label={t('E-Mail')} value={user.email} />
        <Row label={t('Rolle')} value={user.role} />
        <Row label={t('Tenant')} value={user.tenant_id || 'System'} />
        <Row label={t('Status')} value={user.active ? t('aktiv') : t('inaktiv')} />
      </Card>

      <Card title={t('Sprache & Oberfläche')}>
        <Field label={t('Sprache')} hint={t('Die Auswahl wird im Browser gespeichert.')}>
          <Select value={currentLang()} onChange={e=>setLang(e.target.value)}>
            {LANGS.map(l => <option key={l.value} value={l.value}>{l.label}</option>)}
          </Select>
        </Field>
      </Card>

      <Card title={t('Zwei-Faktor-Authentifizierung (MFA)')} subtitle={t('Optionales TOTP-Login mit gängigen Authenticator-Apps')}>
        <div className="flex items-center justify-between py-1">
          <span className="text-sm text-slate-500">{t('Status')}</span>
          <Badge variant={user.mfa_enabled ? 'green' : 'slate'}>{user.mfa_enabled ? t('aktiv') : t('aus')}</Badge>
        </div>

        {mfaMsg && <div className={`text-sm mt-2 ${mfaMsg.startsWith('Fehler')?'text-red-600':'text-emerald-600'}`}>{mfaMsg}</div>}

        {user.mfa_enabled ? (
          <div className="mt-4 space-y-3">
            <p className="text-sm text-slate-600">{t('MFA ist aktiv. Zum Deaktivieren bitte einen aktuellen Code aus deiner Authenticator-App eingeben.')}</p>
            <div className="flex flex-col sm:flex-row sm:items-end gap-2">
              <Field label={t('Code')} className="flex-1">
                <Input value={mfaCode} onChange={e=>setMfaCode(e.target.value.replace(/\D/g,'').slice(0,6))} placeholder="000000" inputMode="numeric" />
              </Field>
              <Button variant="danger" onClick={disableMFA} disabled={mfaBusy || mfaCode.length < 6}>{t('Deaktivieren')}</Button>
            </div>
          </div>
        ) : mfaData ? (
          <div className="mt-4 space-y-3">
            <p className="text-sm text-slate-600">{t('Scanne den QR-Code mit deiner Authenticator-App (z. B. Google Authenticator, Authy, 1Password) und bestätige anschließend mit dem angezeigten 6-stelligen Code.')}</p>
            <div className="flex flex-col sm:flex-row gap-4 items-start">
              <img src={mfaData.qr_data_url} alt="MFA QR-Code" className="w-40 h-40 rounded-lg border border-slate-200 bg-white p-1" />
              <div className="text-sm text-slate-600">
                <div className="mb-1">{t('Manueller Schlüssel:')}</div>
                <code className="block break-all rounded-lg bg-slate-100 px-3 py-2 font-mono text-xs text-slate-800">{mfaData.secret}</code>
              </div>
            </div>
            <div className="flex flex-col sm:flex-row sm:items-end gap-2">
              <Field label={t('Code aus der App')} className="flex-1">
                <Input value={mfaCode} onChange={e=>setMfaCode(e.target.value.replace(/\D/g,'').slice(0,6))} placeholder="000000" inputMode="numeric" autoFocus />
              </Field>
              <Button variant="success" onClick={enableMFA} disabled={mfaBusy || mfaCode.length < 6}>{t('Aktivieren')}</Button>
              <Button variant="secondary" onClick={()=>{ setMfaData(null); setMfaCode(''); setMfaMsg('') }}>{t('Abbrechen')}</Button>
            </div>
          </div>
        ) : (
          <div className="mt-4">
            <Button onClick={startMFA} disabled={mfaBusy}>{mfaBusy ? t('Vorbereiten...') : t('MFA aktivieren')}</Button>
            <div className="mt-3 text-xs text-slate-400">{t('Unterstützt TOTP (RFC 6238) – kompatibel mit allen gängigen Authenticator-Apps. Der Schlüssel wird verschlüsselt gespeichert.')}</div>
          </div>
        )}
      </Card>

      {user.role === 'superadmin' && (
      <Card title={t("GUI-Zertifikat (Let's Encrypt)")} subtitle={t('Gültiges TLS-Zertifikat für die Admin-Oberfläche')}>
        <div className="text-sm text-slate-600 mb-2">{t('Admin-Hosts:')}</div>
        <div className="flex flex-wrap gap-1 mb-4">
          {adminHosts.length === 0 && <span className="text-sm text-slate-400">—</span>}
          {adminHosts.map(h => (
            <Badge key={h} variant={h === adminDomain ? 'indigo' : 'slate'}>{h}</Badge>
          ))}
        </div>

        {current ? (
          <Alert variant="success">
            {t('Aktiv:')} {current.domain} · {t('gültig bis')} {current.not_after ? new Date(current.not_after).toLocaleDateString() : '—'}
          </Alert>
        ) : (
          <Alert variant="warn">{t("Für {domain} ist noch kein Let's-Encrypt-Zertifikat vorhanden.", { domain: adminDomain || t('die Admin-Domain') })}</Alert>
        )}

        <div className="flex flex-col sm:flex-row sm:items-end gap-2 mt-4">
          <Field label={t('Admin-Domain')} className="flex-1">
            <Select value={adminDomain} onChange={e=>setAdminDomain(e.target.value)}>
              {adminHosts.map(h => <option key={h} value={h}>{h}</option>)}
            </Select>
          </Field>
          <Field label={t('E-Mail für ACME')} className="flex-1">
            <Input value={email} onChange={e=>setEmail(e.target.value)} placeholder="admin@example.com" />
          </Field>
          <Button variant="success" onClick={issue} disabled={busy || !email || !adminDomain}>
            {busy ? t('Stelle aus...') : (current ? t('Neu ausstellen') : t('Ausstellen'))}
          </Button>
        </div>
        {msg && <div className={`text-sm mt-2 ${msg.startsWith('Fehler')?'text-red-600':'text-emerald-600'}`}>{msg}</div>}
        <div className="mt-4"><Alert variant="info">{t('Die Admin-Domain muss öffentlich auf diesen Server zeigen und Port 80 erreichbar sein (HTTP-01). Zugriff dann über https://<Admin-Domain>/.')}</Alert></div>
      </Card>
      )}

      {user.role === 'superadmin' && (
      <Card title={t('Response-Cache')} subtitle={t('HTTP-Cache für anonyme GET-Anfragen')}>
        {!cacheCfg ? <Spinner /> : (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-4">
              <div className="rounded-lg bg-slate-50 p-3"><div className="text-xl font-bold text-slate-800">{cacheData?.entries ?? 0}</div><div className="text-xs text-slate-500">{t('Einträge')}</div></div>
              <div className="rounded-lg bg-slate-50 p-3"><div className="text-xl font-bold text-slate-800">{fmtBytes(cacheData?.size_bytes)}</div><div className="text-xs text-slate-500">{t('Größe')}</div></div>
              <div className="rounded-lg bg-slate-50 p-3"><div className="text-xl font-bold text-emerald-700">{cacheData?.hits ?? 0}</div><div className="text-xs text-slate-500">{t('Hits')}</div></div>
              <div className="rounded-lg bg-slate-50 p-3"><div className="text-xl font-bold text-amber-700">{cacheData?.misses ?? 0}</div><div className="text-xs text-slate-500">{t('Misses')}</div></div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <Field label={t('Cache aktiv')}><div className="pt-2"><Toggle checked={!!cacheCfg.enabled} onChange={v=>setCfg('enabled', v)} label={cacheCfg.enabled ? t('an') : t('aus')} /></div></Field>
              <Field label={t('Haltezeit (TTL, Sekunden)')}><Input type="number" value={cacheCfg.ttl_seconds} onChange={e=>setCfg('ttl_seconds', Number(e.target.value))} /></Field>
              <Field label={t('Max. Einträge')}><Input type="number" value={cacheCfg.max_entries} onChange={e=>setCfg('max_entries', Number(e.target.value))} /></Field>
              <Field label={t('Max. Objektgröße (KB)')}><Input type="number" value={cacheCfg.max_object_kb} onChange={e=>setCfg('max_object_kb', Number(e.target.value))} /></Field>
              <Field label={t('Max. Gesamtgröße (MB)')}><Input type="number" value={cacheCfg.max_total_mb} onChange={e=>setCfg('max_total_mb', Number(e.target.value))} /></Field>
            </div>

            <div className="flex items-center gap-2 mt-4">
              <Button onClick={saveCache}>{t('Speichern')}</Button>
              <Button variant="secondary" onClick={clearCache}>{t('Cache leeren')}</Button>
              {cacheMsg && <span className={`text-sm ${cacheMsg.startsWith('Fehler')?'text-red-600':'text-emerald-600'}`}>{cacheMsg}</span>}
            </div>
            <div className="mt-3 text-xs text-slate-400">{t('Gecacht werden anonyme GET-Antworten (2xx/3xx/404/410, kein 206). Die Frischezeit kommt aus den Backend-Headern Cache-Control: s-maxage/max-age bzw. Expires; fehlen diese, gilt die TTL hier. Nicht gecacht: no-store, no-cache, private, Set-Cookie, Vary mit anderen Feldern als Accept-Encoding. ETag/Last-Modified werden für 304-Antworten genutzt.')}</div>
          </>
        )}
      </Card>
      )}

      <Card title={t('Version')}>
        <Row label={t('Installiert')} value={ver?.current || '—'} />
        <Row label={t('Neueste')} value={ver?.latest || '—'} />
        <Row label={t('Status')} value={ver?.update_available ? t('Update verfügbar') : t('Aktuell')} />
        {ver?.update_available && (
          <div className="pt-2">
            <a href={ver.url} target="_blank" rel="noreferrer" className="text-sm text-indigo-600 hover:underline">{t('Release v{version} öffnen', { version: ver.latest })}</a>
          </div>
        )}
      </Card>

      <Card title={t('Über diese Instanz')}>
        <Row label={t('Produkt')} value="Bastion" />
        <Row label={t('Backend')} value="Go 1.26 · Gin · PostgreSQL" />
        <Row label={t('Frontend')} value="React · Vite · Tailwind" />
        <Row label={t('Proxy')} value="net/http ReverseProxy · TLS SNI · HTTP/3" />
        <Row label={t('GeoIP')} value="DB-IP Lite (MMDB)" />
        <Row label={t('Zertifikate')} value="certmagic (Let's Encrypt) + manual" />
      </Card>
    </div>
  )
}
