import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '../api.js'
import { Button, Input, Select, Field, Card, Toggle, Spinner, Alert, Badge, MultiSelect, Modal } from '../components/UI.jsx'
import Icon from '../components/Icon.jsx'
import { t } from '../i18n.js'

function emptyForm() {
  return {
    domain: '', extra_domains: '', tenant_id: '', template_id: '', cert_id: '', enabled: true,
    upstream_url: null, upstream_host: null, upstream_insecure_skip_verify: null, geoip_enabled: null, geoip_mode: null, geoip_countries: null,
    http2_enabled: null, http3_enabled: null, force_https: null, hsts_enabled: null, hsts_max_age: null,
    websocket_enabled: null, compression_enabled: null,
    rate_limit_rps: null, rate_limit_burst: null, max_body_size_mb: null,
    cache_enabled: null, cache_ttl: null, cache_max_object_kb: null, auth_enabled: null,
  }
}

function emptyLocation() {
  return {
    id: null, path_prefix: '/', enabled: true,
    upstream_url: null, upstream_host: null, upstream_insecure_skip_verify: null,
    auth_enabled: null, auth_users_mode: 'inherit', ip_rules_mode: 'inherit',
    cache_enabled: null, cache_ttl: null, cache_max_object_kb: null,
    rate_limit_rps: null, rate_limit_burst: null,
    geoip_enabled: null, geoip_mode: null, geoip_countries: null,
    http3_enabled: null,
  }
}

const TABS = [
  { id: 'general', label: t('Allgemein') },
  { id: 'domains', label: t('Domains') },
  { id: 'locations', label: t('Locations') },
  { id: 'tls', label: t('TLS & Zertifikat') },
  { id: 'http', label: t('HTTP') },
  { id: 'geoip', label: t('GeoIP') },
  { id: 'cache', label: t('Cache') },
  { id: 'access', label: t('Zugriff') },
  { id: 'advanced', label: t('Erweitert') },
]

function Tri({ label, value, onChange, templateValue, inheritLabel = 'vom Template' }) {
  return (
    <div className="flex items-center justify-between py-2">
      <div>
        <div className="text-sm text-slate-700">{label}</div>
        {value === null && templateValue !== undefined && templateValue !== null && (
          <div className="text-xs text-slate-400">{inheritLabel}: {String(templateValue)}</div>
        )}
      </div>
      <select value={value===null?'inherit':value?'on':'off'}
        onChange={e=>{const v=e.target.value; onChange(v==='inherit'?null:v==='on')}}
        className="rounded-lg border border-slate-300 px-2 py-1.5 text-sm bg-white focus:outline-none focus:ring-2 focus:ring-indigo-500/30">
        <option value="inherit">{t('Inherit')}</option>
        <option value="on">{t('On')}</option>
        <option value="off">{t('Off')}</option>
      </select>
    </div>
  )
}

function InheritNumber({ label, value, onChange, templateValue }) {
  const inherited = value === null
  return (
    <div>
      <div className="flex items-center justify-between mb-1">
        <label className="text-sm text-slate-700">{label}</label>
        <label className="flex items-center gap-1 text-xs text-slate-500">
          <input type="checkbox" checked={inherited} onChange={e=>onChange(e.target.checked?null:0)} /> inherit
        </label>
      </div>
      {inherited
        ? <div className="text-sm text-slate-400 px-1 py-1.5">{templateValue ?? '—'}</div>
        : <Input type="number" value={value} onChange={e=>onChange(e.target.value===''?0:Number(e.target.value))} />}
    </div>
  )
}

function InheritText({ label, value, onChange, templateValue, placeholder }) {
  const inherited = value === null
  return (
    <div>
      <div className="flex items-center justify-between mb-1">
        <label className="text-sm text-slate-700">{label}</label>
        <label className="flex items-center gap-1 text-xs text-slate-500">
          <input type="checkbox" checked={inherited} onChange={e=>onChange(e.target.checked?null:'')} /> inherit
        </label>
      </div>
      {inherited
        ? <div className="text-sm text-slate-400 px-1 py-1.5">{templateValue ?? '—'}</div>
        : <Input value={value||''} placeholder={placeholder} onChange={e=>onChange(e.target.value)} />}
    </div>
  )
}

function isLocalDomain(d) {
  const x = (d||'').toLowerCase()
  return x.endsWith('.local') || x.endsWith('.test') || x.endsWith('.localhost') || x === 'localhost' || /^[0-9.]+$/.test(x) || x.includes(':')
}

export default function SiteDetail() {
  const { id } = useParams()
  const nav = useNavigate()
  const isNew = id === 'new'
  const [form, setForm] = useState(emptyForm())
  const [tab, setTab] = useState('general')
  const [templates, setTemplates] = useState([])
  const [tenants, setTenants] = useState([])
  const [certs, setCerts] = useState([])
  const [countries, setCountries] = useState([])
  const [loading, setLoading] = useState(!isNew)
  const [err, setErr] = useState('')
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [leEmail, setLeEmail] = useState('')
  const [leMsg, setLeMsg] = useState('')
  const [leBusy, setLeBusy] = useState(false)
  const [cacheBusy, setCacheBusy] = useState(false)
  const [cacheMsg, setCacheMsg] = useState('')
  const [siteUsers, setSiteUsers] = useState([])
  const [siteRules, setSiteRules] = useState([])
  const [accessScope, setAccessScope] = useState('site')
  const [locations, setLocations] = useState([])
  const [locForm, setLocForm] = useState(null)
  const [locErr, setLocErr] = useState('')
  const [locBusy, setLocBusy] = useState(false)
  const [userForm, setUserForm] = useState({ username:'', password:'', active:true })
  const [userEditing, setUserEditing] = useState(null)
  const [userErr, setUserErr] = useState('')
  const [ruleForm, setRuleForm] = useState({ kind:'allow', value:'', note:'' })
  const [ruleEditing, setRuleEditing] = useState(null)
  const [ruleErr, setRuleErr] = useState('')

  const applySite = (s) => {
    setForm({
      domain: s.domain, extra_domains: (s.extra_domains||[]).join(', '), tenant_id: s.tenant_id,
      template_id: s.template_id||'', cert_id: s.cert_id||'', enabled: s.enabled,
      upstream_url: s.upstream_url, upstream_host: s.upstream_host, upstream_insecure_skip_verify: s.upstream_insecure_skip_verify, geoip_enabled: s.geoip_enabled, geoip_mode: s.geoip_mode, geoip_countries: s.geoip_countries,
      http2_enabled: s.http2_enabled, http3_enabled: s.http3_enabled, force_https: s.force_https,
      hsts_enabled: s.hsts_enabled, hsts_max_age: s.hsts_max_age, websocket_enabled: s.websocket_enabled,
      compression_enabled: s.compression_enabled, rate_limit_rps: s.rate_limit_rps, rate_limit_burst: s.rate_limit_burst,
      max_body_size_mb: s.max_body_size_mb,
      cache_enabled: s.cache_enabled, cache_ttl: s.cache_ttl, cache_max_object_kb: s.cache_max_object_kb, auth_enabled: s.auth_enabled,
    })
  }

  const loadSite = () => {
    if (isNew) { setLoading(false); return }
    api('/sites/'+id).then(d => { applySite(d.data); setLoading(false) }).catch(e => { setErr(e.message); setLoading(false) })
  }

  const loadCerts = () => api('/certificates').then(d=>setCerts(d.data||[])).catch(()=>{})
  const accessBase = () => accessScope === 'site' ? '/sites/'+id : '/sites/'+id+'/locations/'+accessScope
  const loadAccess = () => {
    if (isNew) return
    const base = accessBase()
    api(base+'/users').then(d=>setSiteUsers(d.data||[])).catch(()=>{})
    api(base+'/ip-rules').then(d=>setSiteRules(d.data||[])).catch(()=>{})
  }
  const loadLocations = () => {
    if (isNew) return
    api('/sites/'+id+'/locations').then(d=>setLocations(d.data||[])).catch(()=>{})
  }

  useEffect(() => {
    api('/templates').then(d=>setTemplates(d.data||[])).catch(()=>{})
    api('/tenants').then(d=>setTenants(d.data||[])).catch(()=>{})
    api('/geoip/countries').then(d=>setCountries(d.data||[])).catch(()=>{})
    loadCerts()
    loadSite()
    loadLocations()
  }, [id])

  useEffect(() => {
    loadAccess()
  }, [id, accessScope])

  useEffect(() => {
    if (accessScope !== 'site' && !locations.find(l => l.id === accessScope)) {
      setAccessScope('site')
    }
  }, [locations])

  useEffect(() => {
    if (!leEmail && form.cert_id) {
      const c = certs.find(x => x.id === form.cert_id)
      if (c?.email) setLeEmail(c.email)
    }
  }, [form.cert_id, certs])

  const tmpl = templates.find(t => t.id === form.template_id) || null
  const currentCert = form.cert_id ? certs.find(c => c.id === form.cert_id) : null
  const issueDomains = [form.domain, ...(form.extra_domains||'').split(',').map(x=>x.trim())].filter(Boolean)
  const hasLocal = issueDomains.some(isLocalDomain)

  const clearSiteCache = async () => {
    setCacheMsg(''); setCacheBusy(true)
    try {
      await api('/cache/clear', { method: 'POST', body: JSON.stringify({ site_id: id }) })
      setCacheMsg('Cache dieser Site geleert.')
    } catch (e) { setCacheMsg('Fehler: ' + e.message) } finally { setCacheBusy(false) }
  }

  const saveUser = async (e) => {
    e.preventDefault(); setUserErr('')
    try {
      const base = accessBase()
      if (userEditing) {
        const payload = { username: userForm.username, active: userForm.active }
        if (userForm.password) payload.password = userForm.password
        await api(base+'/users/'+userEditing, { method:'PUT', body: JSON.stringify(payload) })
      } else {
        await api(base+'/users', { method:'POST', body: JSON.stringify(userForm) })
      }
      setUserForm({ username:'', password:'', active:true }); setUserEditing(null); loadAccess()
    } catch(e) { setUserErr(e.message) }
  }
  const editUser = (u) => { setUserEditing(u.id); setUserForm({ username:u.username, password:'', active:u.active }); setUserErr('') }
  const delUser = async (uid) => { if (!confirm('Benutzer löschen?')) return; await api(accessBase()+'/users/'+uid, { method:'DELETE' }); loadAccess() }

  const saveRule = async (e) => {
    e.preventDefault(); setRuleErr('')
    try {
      const base = accessBase()
      if (ruleEditing) await api(base+'/ip-rules/'+ruleEditing, { method:'PUT', body: JSON.stringify(ruleForm) })
      else await api(base+'/ip-rules', { method:'POST', body: JSON.stringify(ruleForm) })
      setRuleForm({ kind:'allow', value:'', note:'' }); setRuleEditing(null); loadAccess()
    } catch(e) { setRuleErr(e.message) }
  }
  const editRule = (r) => { setRuleEditing(r.id); setRuleForm({ kind:r.kind, value:r.value, note:r.note||'' }); setRuleErr('') }
  const delRule = async (rid) => { if (!confirm('Regel löschen?')) return; await api(accessBase()+'/ip-rules/'+rid, { method:'DELETE' }); loadAccess() }

  const saveLocation = async () => {
    if (!locForm) return
    setLocErr(''); setLocBusy(true)
    try {
      const payload = {
        path_prefix: locForm.path_prefix,
        enabled: locForm.enabled,
        upstream_url: locForm.upstream_url || null,
        upstream_host: locForm.upstream_host || null,
        upstream_insecure_skip_verify: locForm.upstream_insecure_skip_verify,
        auth_enabled: locForm.auth_enabled,
        auth_users_mode: locForm.auth_users_mode || 'inherit',
        ip_rules_mode: locForm.ip_rules_mode || 'inherit',
        cache_enabled: locForm.cache_enabled,
        cache_ttl: locForm.cache_ttl,
        cache_max_object_kb: locForm.cache_max_object_kb,
        rate_limit_rps: locForm.rate_limit_rps,
        rate_limit_burst: locForm.rate_limit_burst,
        geoip_enabled: locForm.geoip_enabled,
        geoip_mode: locForm.geoip_mode || null,
        geoip_countries: locForm.geoip_countries,
        http3_enabled: locForm.http3_enabled,
      }
      if (locForm.id) await api('/sites/'+id+'/locations/'+locForm.id, { method:'PUT', body: JSON.stringify(payload) })
      else await api('/sites/'+id+'/locations', { method:'POST', body: JSON.stringify(payload) })
      setLocForm(null); loadLocations()
    } catch(e) { setLocErr(e.message) } finally { setLocBusy(false) }
  }
  const delLocation = async (lid) => {
    if (!confirm('Location löschen?')) return
    try { await api('/sites/'+id+'/locations/'+lid, { method:'DELETE' }); loadLocations() }
    catch(e) { setLocErr(e.message) }
  }
  const inherited = (k) => (form[k] !== null && form[k] !== undefined) ? form[k] : (tmpl ? tmpl[k] : null)

  const save = async () => {
    setErr(''); setSaving(true)
    try {
      const payload = {
        ...form,
        extra_domains: (form.extra_domains||'').split(',').map(x=>x.trim()).filter(Boolean),
        template_id: form.template_id || null,
        cert_id: form.cert_id || null,
        tenant_id: form.tenant_id,
      }
      if (!payload.tenant_id) { setErr('Tenant erforderlich'); setSaving(false); return }
      if (isNew) {
        const d = await api('/sites', { method:'POST', body: JSON.stringify(payload) })
        nav('/sites/'+d.data.id)
      } else {
        await api('/sites/'+id, { method:'PUT', body: JSON.stringify(payload) })
        setSaved(true); setTimeout(()=>setSaved(false), 2000)
      }
    } catch(e) { setErr(e.message) } finally { setSaving(false) }
  }

  const issueLE = async () => {
    setLeMsg(''); setLeBusy(true)
    try {
      const r = await api('/certificates/issue', { method:'POST', body: JSON.stringify({ site_id: id, email: leEmail }) })
      setLeMsg('Zertifikat ausgestellt für: ' + (r.data.domains || []).join(', ') + ((r.data.excluded && r.data.excluded.length) ? ' (ausgeschlossen: ' + r.data.excluded.join(', ') + ')' : ''))
      const d = await api('/sites/'+id); applySite(d.data)
      loadCerts()
    } catch(e) { setLeMsg('Fehler: ' + e.message) } finally { setLeBusy(false) }
  }

  const del = async () => {
    if (!confirm('Site wirklich löschen?')) return
    await api('/sites/'+id, { method:'DELETE' })
    nav('/sites')
  }

  if (loading) return <Spinner />

  return (
    <div className="max-w-5xl space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <button onClick={()=>nav('/sites')} className="p-2 rounded-lg hover:bg-slate-200 text-slate-500"><Icon name="chevron" className="w-5 h-5" stroke /></button>
          <div>
            <h2 className="text-xl font-bold text-slate-800">{isNew ? t('Neue Site') : form.domain}</h2>
            {!isNew && <div className="text-sm text-slate-500">{t('Site-Konfiguration')}</div>}
          </div>
        </div>
        <div className="flex gap-2">
          {!isNew && <Button variant="danger" onClick={del}><Icon name="trash" className="w-4 h-4" stroke /> {t('Löschen')}</Button>}
          <Button onClick={save} disabled={saving}>{saving?t('Speichern...'):(isNew?t('Erstellen'):t('Speichern'))}</Button>
        </div>
      </div>

      {err && <Alert variant="error">{err}</Alert>}
      {saved && <Alert variant="success">{t('Gespeichert.')}</Alert>}

      <div className="bg-white rounded-xl border border-slate-200 shadow-sm">
        <div className="flex gap-1 px-2 border-b border-slate-100 overflow-x-auto">
          {TABS.map(t => (
            <button key={t.id} onClick={()=>setTab(t.id)}
              className={`px-4 py-3 text-sm font-medium whitespace-nowrap border-b-2 ${tab===t.id?'border-indigo-600 text-indigo-600':'border-transparent text-slate-500 hover:text-slate-700'}`}>
              {t.label}
            </button>
          ))}
        </div>

        <div className="p-6 space-y-5">
          {tab === 'general' && (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
              <Field label={t('Primary Domain')}><Input value={form.domain} onChange={e=>setForm({...form,domain:e.target.value})} placeholder="example.com" /></Field>
              <Field label="Tenant">
                <Select value={form.tenant_id} onChange={e=>setForm({...form,tenant_id:e.target.value})}>
                  <option value="">{t('— wählen —')}</option>
                  {tenants.map(t=><option key={t.id} value={t.id}>{t.name}</option>)}
                </Select>
              </Field>
              <Field label="Template">
                <Select value={form.template_id} onChange={e=>setForm({...form,template_id:e.target.value})}>
                  <option value="">{t('Kein Template')}</option>
                  {templates.map(t=><option key={t.id} value={t.id}>{t.name}</option>)}
                </Select>
              </Field>
              <Field label="Status">
                <div className="pt-2"><Toggle checked={form.enabled} onChange={v=>setForm({...form,enabled:v})} label={form.enabled?'Aktiv':'Deaktiviert'} /></div>
              </Field>
            </div>
          )}

          {tab === 'domains' && (
            <div className="space-y-5">
              <Field label={t('Primary Domain')} hint={t('Die Haupt-Domain dieser Site.')}><Input value={form.domain} onChange={e=>setForm({...form,domain:e.target.value})} /></Field>
              <Field label={t('Alias-Domains')} hint={t('Kommagetrennt, z.B. www.example.com, shop.example.com')}><Input value={form.extra_domains} onChange={e=>setForm({...form,extra_domains:e.target.value})} placeholder="www.example.com, api.example.com" /></Field>
              <Alert variant="info">Nach Änderung der Domains speichern und im Tab „TLS &amp; Zertifikat" neu ausstellen, damit die neuen Domains im Zertifikat enthalten sind.</Alert>
            </div>
          )}

          {tab === 'locations' && (
            <div className="space-y-4">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="font-semibold text-slate-800">{t('Pfad-Locations')}</div>
                  <div className="text-sm text-slate-500">{t('Eigene Konfiguration für Unterpfade. Längster passender Prefix gewinnt.')}</div>
                </div>
                <Button onClick={()=>{ setLocErr(''); setLocForm(emptyLocation()) }} disabled={isNew}>{t('Location hinzufügen')}</Button>
              </div>

              {isNew ? <Alert variant="info">{t('Site zuerst speichern, dann Locations anlegen.')}</Alert> : locations.length === 0 ? (
                <div className="text-sm text-slate-400 rounded-lg border border-dashed border-slate-200 p-6 text-center">{t('Noch keine Locations. Ohne Location gilt die Site-Konfiguration für alle Pfade.')}</div>
              ) : (
                <div className="divide-y divide-slate-100 rounded-lg border border-slate-200">
                  {locations.map(l => (
                    <div key={l.id} className="p-4 flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="font-mono text-sm font-semibold text-slate-800">{l.path_prefix}</span>
                          {!l.enabled && <Badge variant="slate">{t('deaktiviert')}</Badge>}
                          {l.auth_enabled !== null && <Badge variant={l.auth_enabled?'green':'slate'}>Auth {l.auth_enabled?'an':'aus'}</Badge>}
                          {l.cache_enabled !== null && <Badge variant={l.cache_enabled?'blue':'slate'}>Cache {l.cache_enabled?'an':'aus'}</Badge>}
                          {l.geoip_enabled !== null && <Badge variant={l.geoip_enabled?'amber':'slate'}>GeoIP {l.geoip_enabled?'an':'aus'}</Badge>}
                          {l.rate_limit_rps !== null && <Badge variant="indigo">Limit {l.rate_limit_rps} rps</Badge>}
                          {l.auth_users_mode === 'own' && <Badge variant="blue">{t('eigene Benutzer')}</Badge>}
                          {l.ip_rules_mode === 'own' && <Badge variant="blue">{t('eigene IP-Regeln')}</Badge>}
                        </div>
                        <div className="text-xs text-slate-500 mt-1 truncate">
                          Upstream: {l.upstream_url || 'von der Site'} {l.upstream_host ? '· Host: ' + l.upstream_host : ''}
                        </div>
                      </div>
                      <div className="flex items-center shrink-0">
                        <button onClick={()=>{ setLocErr(''); setLocForm({ ...emptyLocation(), ...l }) }} className="p-1.5 text-slate-400 hover:text-indigo-600"><Icon name="edit" className="w-4 h-4" stroke /></button>
                        <button onClick={()=>delLocation(l.id)} className="p-1.5 text-slate-400 hover:text-red-600"><Icon name="trash" className="w-4 h-4" stroke /></button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
              {locErr && !locForm && <div className="text-sm text-red-600">{locErr}</div>}
              <Alert variant="info">Locations erben nicht gesetzte Werte von der Site bzw. dem Template. Der längste Prefix gewinnt: <code>/api</code> matcht <code>/api</code> und <code>/api/v1</code>, aber nicht <code>/apix</code>.</Alert>
            </div>
          )}

          {tab === 'tls' && (
            <div className="space-y-6">
              <div className="rounded-lg border border-slate-200 p-4">
                <div className="flex items-center justify-between mb-2">
                  <h4 className="font-semibold text-slate-800">Let's Encrypt</h4>
                  {currentCert ? <Badge variant="green">{t('Zertifikat aktiv')}</Badge> : <Badge variant="slate">{t('kein Zertifikat')}</Badge>}
                </div>
                {currentCert && (
                  <div className="text-sm text-slate-600 mb-3">
                    {currentCert.domain} · gültig bis {currentCert.not_after ? new Date(currentCert.not_after).toLocaleDateString() : '—'} · {currentCert.issuer}
                  </div>
                )}
                <div className="text-sm text-slate-600 mb-1">{t('Domains im Zertifikat (SAN):')}</div>
                <div className="flex flex-wrap gap-1 mb-3">
                  {issueDomains.length === 0 && <span className="text-sm text-slate-400">—</span>}
                  {issueDomains.map(d => (
                    <Badge key={d} variant={isLocalDomain(d)?'amber':'indigo'}>{d}</Badge>
                  ))}
                </div>
                {hasLocal && (
                  <div className="mb-3"><Alert variant="warn">Eine der Domains ist lokal/Test (z.B. .local). Let's Encrypt kann solche Domains nicht validieren. Entferne sie, sonst schlägt die Ausstellung fehl.</Alert></div>
                )}
                {isNew ? (
                  <Alert variant="info">{t('Site zuerst speichern, dann Zertifikat ausstellen.')}</Alert>
                ) : (
                  <div className="flex items-end gap-2">
                    <div className="flex-1">
                      <label className="text-sm text-slate-600 mb-1 block">{t('E-Mail für ACME')}</label>
                      <Input value={leEmail} onChange={e=>setLeEmail(e.target.value)} placeholder="admin@example.com" />
                    </div>
                    <Button variant="success" onClick={issueLE} disabled={leBusy || !leEmail}>
                      <Icon name="certs" className="w-4 h-4" /> {leBusy ? 'Stelle aus...' : (currentCert ? 'Neu ausstellen' : 'Ausstellen')}
                    </Button>
                  </div>
                )}
                {leMsg && <div className={`text-sm mt-2 ${leMsg.startsWith('Fehler')?'text-red-600':'text-emerald-600'}`}>{leMsg}</div>}
              </div>

              <Field label={t('Manuelles Zertifikat')}>
                <Select value={form.cert_id||''} onChange={e=>setForm({...form,cert_id:e.target.value})}>
                  <option value="">{t('Kein Zertifikat')}</option>
                  {certs.map(c=><option key={c.id} value={c.id}>{c.domain} ({c.source})</option>)}
                </Select>
              </Field>

              <div className="divide-y divide-slate-100">
                <Tri label="Force HTTPS" value={form.force_https} onChange={v=>setForm({...form,force_https:v})} templateValue={tmpl?.force_https} />
                <Tri label="HSTS" value={form.hsts_enabled} onChange={v=>setForm({...form,hsts_enabled:v})} templateValue={tmpl?.hsts_enabled} />
                <div className="pt-3"><InheritNumber label={t('HSTS Max Age (s)')} value={form.hsts_max_age} onChange={v=>setForm({...form,hsts_max_age:v})} templateValue={tmpl?.hsts_max_age} /></div>
              </div>
            </div>
          )}

          {tab === 'http' && (
            <div className="space-y-2 divide-y divide-slate-100">
              <Tri label="HTTP/2" value={form.http2_enabled} onChange={v=>setForm({...form,http2_enabled:v})} templateValue={tmpl?.http2_enabled} />
              <Tri label="HTTP/3 (QUIC)" value={form.http3_enabled} onChange={v=>setForm({...form,http3_enabled:v})} templateValue={tmpl?.http3_enabled} />
              <Tri label="WebSocket" value={form.websocket_enabled} onChange={v=>setForm({...form,websocket_enabled:v})} templateValue={tmpl?.websocket_enabled} />
              <Tri label="Compression" value={form.compression_enabled} onChange={v=>setForm({...form,compression_enabled:v})} templateValue={tmpl?.compression_enabled} />
            </div>
          )}

          {tab === 'geoip' && (
            <div className="space-y-5">
              <Tri label={t('GeoIP Filter')} value={form.geoip_enabled} onChange={v=>setForm({...form,geoip_enabled:v})} templateValue={tmpl?.geoip_enabled} />
              <Field label={t('Modus')} hint={t('Block: ausgewählte Länder sperren · Allow: nur ausgewählte Länder erlauben')}>
                <Select value={form.geoip_mode||''} onChange={e=>setForm({...form,geoip_mode:e.target.value||null})}>
                  <option value="">Inherit {tmpl?.geoip_mode?('('+tmpl.geoip_mode+')'):''}</option>
                  <option value="block">{t('Block (ausgewählte sperren)')}</option>
                  <option value="allow">{t('Allow (nur ausgewählte erlauben)')}</option>
                </Select>
              </Field>
              <div>
                <div className="flex items-center justify-between mb-1">
                  <label className="text-sm font-medium text-slate-700">Länder</label>
                  <label className="flex items-center gap-1 text-xs text-slate-500">
                    <input type="checkbox" checked={form.geoip_countries===null} onChange={e=>setForm({...form, geoip_countries: e.target.checked?null:[]})} />{t('vom Template erben')}</label>
                </div>
                {form.geoip_countries===null ? (
                  <div className="text-sm text-slate-400 px-3 py-2 border border-dashed border-slate-200 rounded-lg">
                    {(tmpl?.geoip_countries||[]).length ? (tmpl.geoip_countries||[]).join(', ') : '— keine (Template)'}
                  </div>
                ) : (
                  <MultiSelect options={countries} value={form.geoip_countries} onChange={v=>setForm({...form,geoip_countries:v})} placeholder={t('Länder auswählen...')} />
                )}
                {form.geoip_countries!==null && form.geoip_countries.length>0 && (
                  <div className="text-xs text-slate-400 mt-1">{form.geoip_countries.length} Länder ausgewählt</div>
                )}
              </div>
              <Alert variant="info">Bei Modus „Block" werden die ausgewählten Länder gesperrt. Bei „Allow" werden nur die ausgewählten Länder erlaubt.</Alert>
            </div>
          )}

          {tab === 'cache' && (
            <div className="space-y-5">
              <Tri label={t('Cache aktiv')} value={form.cache_enabled} onChange={v=>setForm({...form,cache_enabled:v})} templateValue={tmpl?.cache_enabled} />
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
                <InheritNumber label={t('TTL (Sekunden, Fallback)')} value={form.cache_ttl} onChange={v=>setForm({...form,cache_ttl:v})} templateValue={tmpl?.cache_ttl} />
                <InheritNumber label={t('Max. Objektgröße (KB)')} value={form.cache_max_object_kb} onChange={v=>setForm({...form,cache_max_object_kb:v})} templateValue={tmpl?.cache_max_object_kb} />
              </div>
              <div className="rounded-lg border border-slate-200 p-4">
                <div className="text-sm text-slate-600 mb-2">{t('Zwischengespeicherte Antworten dieser Site sofort verwerfen:')}</div>
                {isNew ? <div className="text-sm text-slate-400">{t('Site zuerst speichern.')}</div> : (
                  <div className="flex items-center gap-3">
                    <Button variant="secondary" onClick={clearSiteCache} disabled={cacheBusy}>{cacheBusy ? '...' : 'Cache dieser Site leeren'}</Button>
                    {cacheMsg && <span className={`text-sm ${cacheMsg.startsWith('Fehler')?'text-red-600':'text-emerald-600'}`}>{cacheMsg}</span>}
                  </div>
                )}
              </div>
              <Alert variant="info">Globaler Schalter und Standardwerte unter Settings → Response-Cache. Hier steuerst du Cache pro Site (Vererbung vom Template möglich). Die TTL ist nur Fallback: Liefert das Backend <code>Cache-Control</code> (s-maxage/max-age) oder <code>Expires</code>, gilt dessen Frischezeit; no-store/no-cache/private verhindern das Cachen.</Alert>
            </div>
          )}

          {tab === 'access' && (
            <div className="space-y-6">
              {!isNew && locations.length > 0 && (
                <Field label={t('Zugriffsebene')} hint={t('Benutzer und IP-Regeln getrennt für die Site oder eine Location verwalten.')}>
                  <Select value={accessScope} onChange={e=>{ setAccessScope(e.target.value); setUserEditing(null); setUserForm({ username:'', password:'', active:true }); setRuleEditing(null); setRuleForm({ kind:'allow', value:'', note:'' }) }}>
                    <option value="site">{t('Site (Standard)')}</option>
                    {locations.map(l => <option key={l.id} value={l.id}>Location {l.path_prefix}{l.enabled ? '' : ' (deaktiviert)'}</option>)}
                  </Select>
                </Field>
              )}
              {accessScope !== 'site' && (() => {
                const l = locations.find(x=>x.id===accessScope)
                if (!l) return null
                return (
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                    <Alert variant={l.auth_users_mode==='own' ? 'info' : 'warn'}>
                      Benutzer: {l.auth_users_mode==='own' ? 'eigene Liste dieser Location' : 'von der Site geerbt'}
                    </Alert>
                    <Alert variant={l.ip_rules_mode==='own' ? 'info' : 'warn'}>
                      IP-Regeln: {l.ip_rules_mode==='own' ? 'eigene Liste dieser Location' : 'von der Site geerbt'}
                    </Alert>
                  </div>
                )
              })()}
              {accessScope === 'site' ? (
                <Tri label={t('Authentifizierung (Basic Auth)')} value={form.auth_enabled} onChange={v=>setForm({...form,auth_enabled:v})} templateValue={tmpl?.auth_enabled} />
              ) : (
                <Alert variant="info">Basic Auth sowie „eigene/geerbte Benutzer &amp; IP-Regeln" stellst du im Tab <b>Locations</b> ein. Hier verwaltest du die Einträge der gewählten Location.</Alert>
              )}
              {isNew ? <Alert variant="info">{t('Site zuerst speichern, dann Benutzer und IP-Regeln pflegen.')}</Alert> : (
                <>
                  <div className="rounded-lg border border-slate-200 p-4">
                    <h4 className="font-semibold text-slate-800 mb-2">{t('Site-Benutzer')}</h4>
                    <form onSubmit={saveUser} className="flex flex-col sm:flex-row gap-2 mb-3">
                      <Input value={userForm.username} onChange={e=>setUserForm({...userForm,username:e.target.value})} placeholder={t('Benutzername')} className="flex-1" required />
                      <Input type="password" value={userForm.password} onChange={e=>setUserForm({...userForm,password:e.target.value})} placeholder={userEditing ? 'Neues Passwort (optional)' : 'Passwort'} className="flex-1" required={!userEditing} />
                      <Toggle checked={userForm.active} onChange={v=>setUserForm({...userForm,active:v})} label={t('aktiv')} />
                      <Button type="submit" variant="success">{userEditing ? 'Speichern' : 'Hinzufügen'}</Button>
                      {userEditing && <Button type="button" variant="secondary" onClick={()=>{setUserEditing(null);setUserForm({username:'',password:'',active:true})}}>{t('Abbrechen')}</Button>}
                    </form>
                    {userErr && <div className="text-sm text-red-600 mb-2">{userErr}</div>}
                    {siteUsers.length === 0 ? <div className="text-sm text-slate-400">{t('Keine Benutzer.')}</div> : (
                      <div className="divide-y divide-slate-100">
                        {siteUsers.map(u => (
                          <div key={u.id} className="flex items-center justify-between py-2">
                            <div>
                              <div className="text-sm font-medium text-slate-800">{u.username}</div>
                              <div className="text-xs text-slate-400">{u.active ? 'aktiv' : 'inaktiv'}</div>
                            </div>
                            <div className="flex items-center">
                              <button onClick={()=>editUser(u)} className="p-1.5 text-slate-400 hover:text-indigo-600"><Icon name="edit" className="w-4 h-4" stroke /></button>
                              <button onClick={()=>delUser(u.id)} className="p-1.5 text-slate-400 hover:text-red-600"><Icon name="trash" className="w-4 h-4" stroke /></button>
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>

                  <div className="rounded-lg border border-slate-200 p-4">
                    <h4 className="font-semibold text-slate-800 mb-2">{t('IP-Regeln (Allow / Deny)')}</h4>
                    <form onSubmit={saveRule} className="flex flex-col sm:flex-row gap-2 mb-3">
                      <Select value={ruleForm.kind} onChange={e=>setRuleForm({...ruleForm,kind:e.target.value})}><option value="allow">Allow</option><option value="deny">Deny</option></Select>
                      <Input value={ruleForm.value} onChange={e=>setRuleForm({...ruleForm,value:e.target.value})} placeholder={t('IP oder CIDR')} className="flex-1" required />
                      <Input value={ruleForm.note} onChange={e=>setRuleForm({...ruleForm,note:e.target.value})} placeholder={t('Notiz')} className="sm:w-40" />
                      <Button type="submit" variant="success">{ruleEditing ? 'Speichern' : 'Hinzufügen'}</Button>
                      {ruleEditing && <Button type="button" variant="secondary" onClick={()=>{setRuleEditing(null);setRuleForm({kind:'allow',value:'',note:''})}}>{t('Abbrechen')}</Button>}
                    </form>
                    {ruleErr && <div className="text-sm text-red-600 mb-2">{ruleErr}</div>}
                    {siteRules.length === 0 ? <div className="text-sm text-slate-400">{t('Keine Regeln.')}</div> : (
                      <div className="divide-y divide-slate-100">
                        {siteRules.map(r => (
                          <div key={r.id} className="flex items-center justify-between py-2">
                            <div className="flex items-center gap-2">
                              <Badge variant={r.kind==='allow' ? 'green' : 'red'}>{r.kind}</Badge>
                              <span className="font-mono text-sm text-slate-800">{r.value}</span>
                              {r.note && <span className="text-xs text-slate-400">{r.note}</span>}
                            </div>
                            <div className="flex items-center">
                              <button onClick={()=>editRule(r)} className="p-1.5 text-slate-400 hover:text-indigo-600"><Icon name="edit" className="w-4 h-4" stroke /></button>
                              <button onClick={()=>delRule(r.id)} className="p-1.5 text-slate-400 hover:text-red-600"><Icon name="trash" className="w-4 h-4" stroke /></button>
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>

                  <Alert variant="info">Ist Auth aktiv und die IP in der Allow-Liste der gewählten Ebene (Site oder Location), ist kein Login nötig. Die globale IP-Whitelist hebt den Login NICHT auf. Nach 3 Fehlversuchen wird die IP 5 Minuten gesperrt (429).</Alert>
                </>
              )}
            </div>
          )}

          {tab === 'advanced' && (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
              <InheritText label="Upstream URL" value={form.upstream_url} onChange={v=>setForm({...form,upstream_url:v})} templateValue={tmpl?.upstream_url} placeholder="http://backend:8080" />
              <InheritText label={t('Upstream Host / Name (Host-Header)')} value={form.upstream_host} onChange={v=>setForm({...form,upstream_host:v})} templateValue={tmpl?.upstream_host} placeholder={t('z.B. backend.internal (leer = Original-Host)')} />
              <Tri label={t('Upstream-TLS-Zertifikat ignorieren')} value={form.upstream_insecure_skip_verify} onChange={v=>setForm({...form,upstream_insecure_skip_verify:v})} templateValue={tmpl?.upstream_insecure_skip_verify} />
              <div className="sm:col-span-2"><Alert variant="warn">Nur für Upstreams mit selbstsigniertem Zertifikat. Deaktiviert die TLS-Zertifikatsprüfung zum Backend.</Alert></div>
              <InheritNumber label={t('Rate Limit RPS')} value={form.rate_limit_rps} onChange={v=>setForm({...form,rate_limit_rps:v})} templateValue={tmpl?.rate_limit_rps} />
              <InheritNumber label={t('Rate Limit Burst')} value={form.rate_limit_burst} onChange={v=>setForm({...form,rate_limit_burst:v})} templateValue={tmpl?.rate_limit_burst} />
              <InheritNumber label={t('Max Body Size (MB)')} value={form.max_body_size_mb} onChange={v=>setForm({...form,max_body_size_mb:v})} templateValue={tmpl?.max_body_size_mb} />
            </div>
          )}
        </div>
      </div>

      <Modal
        open={!!locForm}
        onClose={()=>setLocForm(null)}
        title={locForm?.id ? 'Location bearbeiten' : 'Neue Location'}
        size="lg"
        footer={<>
          <Button variant="secondary" onClick={()=>setLocForm(null)}>{t('Abbrechen')}</Button>
          <Button onClick={saveLocation} disabled={locBusy || !locForm?.path_prefix}>{locBusy ? 'Speichern...' : 'Speichern'}</Button>
        </>}
      >
        {locForm && (
          <form onSubmit={(e)=>{ e.preventDefault(); saveLocation() }} className="space-y-5">
            {locErr && <Alert variant="error">{locErr}</Alert>}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <Field label={t('Pfad-Prefix')} hint={t('Muss mit / beginnen, z. B. /api oder /static')}>
                <Input value={locForm.path_prefix} onChange={e=>setLocForm({...locForm,path_prefix:e.target.value})} placeholder="/api" required />
              </Field>
              <Field label="Status">
                <div className="pt-2"><Toggle checked={!!locForm.enabled} onChange={v=>setLocForm({...locForm,enabled:v})} label={locForm.enabled?'Aktiv':'Deaktiviert'} /></div>
              </Field>
              <InheritText label="Upstream URL" value={locForm.upstream_url} onChange={v=>setLocForm({...locForm,upstream_url:v})} templateValue={inherited('upstream_url')} placeholder="http://backend:8080" />
              <InheritText label={t('Upstream Host / Name')} value={locForm.upstream_host} onChange={v=>setLocForm({...locForm,upstream_host:v})} templateValue={inherited('upstream_host')} placeholder={t('leer = Site-Wert')} />
              <Tri label={t('Upstream-TLS-Zertifikat ignorieren')} value={locForm.upstream_insecure_skip_verify} onChange={v=>setLocForm({...locForm,upstream_insecure_skip_verify:v})} templateValue={inherited('upstream_insecure_skip_verify')} inheritLabel="von der Site" />
            </div>

            <div className="rounded-lg border border-slate-200 p-4 space-y-1">
              <div className="text-sm font-semibold text-slate-800 mb-1">{t('Zugriff & Limit')}</div>
              <Tri label="Basic Auth" value={locForm.auth_enabled} onChange={v=>setLocForm({...locForm,auth_enabled:v})} templateValue={inherited('auth_enabled')} inheritLabel="von der Site" />
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-2">
                <Field label={t('Auth-Benutzer')} hint={t('Eigene Benutzer oder die der Site verwenden')}>
                  <Select value={locForm.auth_users_mode||'inherit'} onChange={e=>setLocForm({...locForm,auth_users_mode:e.target.value})}>
                    <option value="inherit">{t('Von der Site erben')}</option>
                    <option value="own">{t('Eigene Benutzer')}</option>
                  </Select>
                </Field>
                <Field label={t('IP-Regeln')} hint={t('Eigene Allow/Deny-Regeln oder die der Site')}>
                  <Select value={locForm.ip_rules_mode||'inherit'} onChange={e=>setLocForm({...locForm,ip_rules_mode:e.target.value})}>
                    <option value="inherit">{t('Von der Site erben')}</option>
                    <option value="own">{t('Eigene Regeln')}</option>
                  </Select>
                </Field>
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-2">
                <InheritNumber label={t('Rate Limit RPS')} value={locForm.rate_limit_rps} onChange={v=>setLocForm({...locForm,rate_limit_rps:v})} templateValue={inherited('rate_limit_rps')} />
                <InheritNumber label={t('Rate Limit Burst')} value={locForm.rate_limit_burst} onChange={v=>setLocForm({...locForm,rate_limit_burst:v})} templateValue={inherited('rate_limit_burst')} />
              </div>
            </div>

            <div className="rounded-lg border border-slate-200 p-4 space-y-1">
              <div className="text-sm font-semibold text-slate-800 mb-1">Cache</div>
              <Tri label={t('Cache aktiv')} value={locForm.cache_enabled} onChange={v=>setLocForm({...locForm,cache_enabled:v})} templateValue={inherited('cache_enabled')} inheritLabel="von der Site" />
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-2">
                <InheritNumber label={t('TTL (Sekunden, Fallback)')} value={locForm.cache_ttl} onChange={v=>setLocForm({...locForm,cache_ttl:v})} templateValue={inherited('cache_ttl')} />
                <InheritNumber label={t('Max. Objektgröße (KB)')} value={locForm.cache_max_object_kb} onChange={v=>setLocForm({...locForm,cache_max_object_kb:v})} templateValue={inherited('cache_max_object_kb')} />
              </div>
            </div>

            <div className="rounded-lg border border-slate-200 p-4 space-y-3">
              <div className="text-sm font-semibold text-slate-800">{t('GeoIP & HTTP/3')}</div>
              <Tri label={t('GeoIP Filter')} value={locForm.geoip_enabled} onChange={v=>setLocForm({...locForm,geoip_enabled:v})} templateValue={inherited('geoip_enabled')} inheritLabel="von der Site" />
              <Field label={t('Modus')}>
                <Select value={locForm.geoip_mode||''} onChange={e=>setLocForm({...locForm,geoip_mode:e.target.value||null})}>
                  <option value="">{t('Inherit')}</option>
                  <option value="block">{t('Block (ausgewählte Länder sperren)')}</option>
                  <option value="allow">{t('Allow (nur ausgewählte Länder erlauben)')}</option>
                </Select>
              </Field>
              <div>
                <div className="flex items-center justify-between mb-1">
                  <label className="text-sm font-medium text-slate-700">Länder</label>
                  <label className="flex items-center gap-1 text-xs text-slate-500">
                    <input type="checkbox" checked={locForm.geoip_countries===null} onChange={e=>setLocForm({...locForm, geoip_countries: e.target.checked?null:[]})} />{t('von der Site erben')}</label>
                </div>
                {locForm.geoip_countries===null ? (
                  <div className="text-sm text-slate-400 px-3 py-2 border border-dashed border-slate-200 rounded-lg">
                    {(inherited('geoip_countries')||[]).length ? (inherited('geoip_countries')||[]).join(', ') : '— keine (Site)'}
                  </div>
                ) : (
                  <MultiSelect options={countries} value={locForm.geoip_countries} onChange={v=>setLocForm({...locForm,geoip_countries:v})} placeholder={t('Länder auswählen...')} />
                )}
              </div>
              <Tri label="HTTP/3 (QUIC)" value={locForm.http3_enabled} onChange={v=>setLocForm({...locForm,http3_enabled:v})} templateValue={inherited('http3_enabled')} inheritLabel="von der Site" />
            </div>
          </form>
        )}
      </Modal>
    </div>
  )
}
