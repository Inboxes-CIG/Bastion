import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '../api.js'
import { Button, Input, Select, Field, Card, Toggle, Spinner, Alert, Badge, MultiSelect } from '../components/UI.jsx'
import Icon from '../components/Icon.jsx'

function emptyForm() {
  return {
    domain: '', extra_domains: '', tenant_id: '', template_id: '', cert_id: '', enabled: true,
    upstream_url: null, upstream_host: null, geoip_enabled: null, geoip_mode: null, geoip_countries: null,
    http2_enabled: null, http3_enabled: null, force_https: null, hsts_enabled: null, hsts_max_age: null,
    websocket_enabled: null, compression_enabled: null,
    rate_limit_rps: null, rate_limit_burst: null, max_body_size_mb: null,
    cache_enabled: null, cache_ttl: null, cache_max_object_kb: null, auth_enabled: null,
  }
}

const TABS = [
  { id: 'general', label: 'Allgemein' },
  { id: 'domains', label: 'Domains' },
  { id: 'tls', label: 'TLS & Zertifikat' },
  { id: 'http', label: 'HTTP' },
  { id: 'geoip', label: 'GeoIP' },
  { id: 'cache', label: 'Cache' },
  { id: 'access', label: 'Zugriff' },
  { id: 'advanced', label: 'Erweitert' },
]

function Tri({ label, value, onChange, templateValue }) {
  return (
    <div className="flex items-center justify-between py-2">
      <div>
        <div className="text-sm text-slate-700">{label}</div>
        {value === null && templateValue !== undefined && templateValue !== null && (
          <div className="text-xs text-slate-400">vom Template: {String(templateValue)}</div>
        )}
      </div>
      <select value={value===null?'inherit':value?'on':'off'}
        onChange={e=>{const v=e.target.value; onChange(v==='inherit'?null:v==='on')}}
        className="rounded-lg border border-slate-300 px-2 py-1.5 text-sm bg-white focus:outline-none focus:ring-2 focus:ring-indigo-500/30">
        <option value="inherit">Inherit</option>
        <option value="on">On</option>
        <option value="off">Off</option>
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
      upstream_url: s.upstream_url, upstream_host: s.upstream_host, geoip_enabled: s.geoip_enabled, geoip_mode: s.geoip_mode, geoip_countries: s.geoip_countries,
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
  const loadAccess = () => {
    if (isNew) return
    api('/sites/'+id+'/users').then(d=>setSiteUsers(d.data||[])).catch(()=>{})
    api('/sites/'+id+'/ip-rules').then(d=>setSiteRules(d.data||[])).catch(()=>{})
  }

  useEffect(() => {
    api('/templates').then(d=>setTemplates(d.data||[])).catch(()=>{})
    api('/tenants').then(d=>setTenants(d.data||[])).catch(()=>{})
    api('/geoip/countries').then(d=>setCountries(d.data||[])).catch(()=>{})
    loadCerts()
    loadSite()
    loadAccess()
  }, [id])

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
      if (userEditing) {
        const payload = { username: userForm.username, active: userForm.active }
        if (userForm.password) payload.password = userForm.password
        await api('/sites/'+id+'/users/'+userEditing, { method:'PUT', body: JSON.stringify(payload) })
      } else {
        await api('/sites/'+id+'/users', { method:'POST', body: JSON.stringify(userForm) })
      }
      setUserForm({ username:'', password:'', active:true }); setUserEditing(null); loadAccess()
    } catch(e) { setUserErr(e.message) }
  }
  const editUser = (u) => { setUserEditing(u.id); setUserForm({ username:u.username, password:'', active:u.active }); setUserErr('') }
  const delUser = async (uid) => { if (!confirm('Benutzer löschen?')) return; await api('/sites/'+id+'/users/'+uid, { method:'DELETE' }); loadAccess() }

  const saveRule = async (e) => {
    e.preventDefault(); setRuleErr('')
    try {
      if (ruleEditing) await api('/sites/'+id+'/ip-rules/'+ruleEditing, { method:'PUT', body: JSON.stringify(ruleForm) })
      else await api('/sites/'+id+'/ip-rules', { method:'POST', body: JSON.stringify(ruleForm) })
      setRuleForm({ kind:'allow', value:'', note:'' }); setRuleEditing(null); loadAccess()
    } catch(e) { setRuleErr(e.message) }
  }
  const editRule = (r) => { setRuleEditing(r.id); setRuleForm({ kind:r.kind, value:r.value, note:r.note||'' }); setRuleErr('') }
  const delRule = async (rid) => { if (!confirm('Regel löschen?')) return; await api('/sites/'+id+'/ip-rules/'+rid, { method:'DELETE' }); loadAccess() }

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
            <h2 className="text-xl font-bold text-slate-800">{isNew ? 'Neue Site' : form.domain}</h2>
            {!isNew && <div className="text-sm text-slate-500">Site-Konfiguration</div>}
          </div>
        </div>
        <div className="flex gap-2">
          {!isNew && <Button variant="danger" onClick={del}><Icon name="trash" className="w-4 h-4" stroke /> Löschen</Button>}
          <Button onClick={save} disabled={saving}>{saving?'Speichern...':(isNew?'Erstellen':'Speichern')}</Button>
        </div>
      </div>

      {err && <Alert variant="error">{err}</Alert>}
      {saved && <Alert variant="success">Gespeichert.</Alert>}

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
              <Field label="Primary Domain"><Input value={form.domain} onChange={e=>setForm({...form,domain:e.target.value})} placeholder="example.com" /></Field>
              <Field label="Tenant">
                <Select value={form.tenant_id} onChange={e=>setForm({...form,tenant_id:e.target.value})}>
                  <option value="">— wählen —</option>
                  {tenants.map(t=><option key={t.id} value={t.id}>{t.name}</option>)}
                </Select>
              </Field>
              <Field label="Template">
                <Select value={form.template_id} onChange={e=>setForm({...form,template_id:e.target.value})}>
                  <option value="">Kein Template</option>
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
              <Field label="Primary Domain" hint="Die Haupt-Domain dieser Site."><Input value={form.domain} onChange={e=>setForm({...form,domain:e.target.value})} /></Field>
              <Field label="Alias-Domains" hint="Kommagetrennt, z.B. www.example.com, shop.example.com"><Input value={form.extra_domains} onChange={e=>setForm({...form,extra_domains:e.target.value})} placeholder="www.example.com, api.example.com" /></Field>
              <Alert variant="info">Nach Änderung der Domains speichern und im Tab „TLS &amp; Zertifikat" neu ausstellen, damit die neuen Domains im Zertifikat enthalten sind.</Alert>
            </div>
          )}

          {tab === 'tls' && (
            <div className="space-y-6">
              <div className="rounded-lg border border-slate-200 p-4">
                <div className="flex items-center justify-between mb-2">
                  <h4 className="font-semibold text-slate-800">Let's Encrypt</h4>
                  {currentCert ? <Badge variant="green">Zertifikat aktiv</Badge> : <Badge variant="slate">kein Zertifikat</Badge>}
                </div>
                {currentCert && (
                  <div className="text-sm text-slate-600 mb-3">
                    {currentCert.domain} · gültig bis {currentCert.not_after ? new Date(currentCert.not_after).toLocaleDateString() : '—'} · {currentCert.issuer}
                  </div>
                )}
                <div className="text-sm text-slate-600 mb-1">Domains im Zertifikat (SAN):</div>
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
                  <Alert variant="info">Site zuerst speichern, dann Zertifikat ausstellen.</Alert>
                ) : (
                  <div className="flex items-end gap-2">
                    <div className="flex-1">
                      <label className="text-sm text-slate-600 mb-1 block">E-Mail für ACME</label>
                      <Input value={leEmail} onChange={e=>setLeEmail(e.target.value)} placeholder="admin@example.com" />
                    </div>
                    <Button variant="success" onClick={issueLE} disabled={leBusy || !leEmail}>
                      <Icon name="certs" className="w-4 h-4" /> {leBusy ? 'Stelle aus...' : (currentCert ? 'Neu ausstellen' : 'Ausstellen')}
                    </Button>
                  </div>
                )}
                {leMsg && <div className={`text-sm mt-2 ${leMsg.startsWith('Fehler')?'text-red-600':'text-emerald-600'}`}>{leMsg}</div>}
              </div>

              <Field label="Manuelles Zertifikat">
                <Select value={form.cert_id||''} onChange={e=>setForm({...form,cert_id:e.target.value})}>
                  <option value="">Kein Zertifikat</option>
                  {certs.map(c=><option key={c.id} value={c.id}>{c.domain} ({c.source})</option>)}
                </Select>
              </Field>

              <div className="divide-y divide-slate-100">
                <Tri label="Force HTTPS" value={form.force_https} onChange={v=>setForm({...form,force_https:v})} templateValue={tmpl?.force_https} />
                <Tri label="HSTS" value={form.hsts_enabled} onChange={v=>setForm({...form,hsts_enabled:v})} templateValue={tmpl?.hsts_enabled} />
                <div className="pt-3"><InheritNumber label="HSTS Max Age (s)" value={form.hsts_max_age} onChange={v=>setForm({...form,hsts_max_age:v})} templateValue={tmpl?.hsts_max_age} /></div>
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
              <Tri label="GeoIP Filter" value={form.geoip_enabled} onChange={v=>setForm({...form,geoip_enabled:v})} templateValue={tmpl?.geoip_enabled} />
              <Field label="Modus" hint="Block: ausgewählte Länder sperren · Allow: nur ausgewählte Länder erlauben">
                <Select value={form.geoip_mode||''} onChange={e=>setForm({...form,geoip_mode:e.target.value||null})}>
                  <option value="">Inherit {tmpl?.geoip_mode?('('+tmpl.geoip_mode+')'):''}</option>
                  <option value="block">Block (ausgewählte sperren)</option>
                  <option value="allow">Allow (nur ausgewählte erlauben)</option>
                </Select>
              </Field>
              <div>
                <div className="flex items-center justify-between mb-1">
                  <label className="text-sm font-medium text-slate-700">Länder</label>
                  <label className="flex items-center gap-1 text-xs text-slate-500">
                    <input type="checkbox" checked={form.geoip_countries===null} onChange={e=>setForm({...form, geoip_countries: e.target.checked?null:[]})} /> vom Template erben
                  </label>
                </div>
                {form.geoip_countries===null ? (
                  <div className="text-sm text-slate-400 px-3 py-2 border border-dashed border-slate-200 rounded-lg">
                    {(tmpl?.geoip_countries||[]).length ? (tmpl.geoip_countries||[]).join(', ') : '— keine (Template)'}
                  </div>
                ) : (
                  <MultiSelect options={countries} value={form.geoip_countries} onChange={v=>setForm({...form,geoip_countries:v})} placeholder="Länder auswählen..." />
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
              <Tri label="Cache aktiv" value={form.cache_enabled} onChange={v=>setForm({...form,cache_enabled:v})} templateValue={tmpl?.cache_enabled} />
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
                <InheritNumber label="TTL (Sekunden, Fallback)" value={form.cache_ttl} onChange={v=>setForm({...form,cache_ttl:v})} templateValue={tmpl?.cache_ttl} />
                <InheritNumber label="Max. Objektgröße (KB)" value={form.cache_max_object_kb} onChange={v=>setForm({...form,cache_max_object_kb:v})} templateValue={tmpl?.cache_max_object_kb} />
              </div>
              <div className="rounded-lg border border-slate-200 p-4">
                <div className="text-sm text-slate-600 mb-2">Zwischengespeicherte Antworten dieser Site sofort verwerfen:</div>
                {isNew ? <div className="text-sm text-slate-400">Site zuerst speichern.</div> : (
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
              <Tri label="Authentifizierung (Basic Auth)" value={form.auth_enabled} onChange={v=>setForm({...form,auth_enabled:v})} templateValue={tmpl?.auth_enabled} />
              {isNew ? <Alert variant="info">Site zuerst speichern, dann Benutzer und IP-Regeln pflegen.</Alert> : (
                <>
                  <div className="rounded-lg border border-slate-200 p-4">
                    <h4 className="font-semibold text-slate-800 mb-2">Site-Benutzer</h4>
                    <form onSubmit={saveUser} className="flex flex-col sm:flex-row gap-2 mb-3">
                      <Input value={userForm.username} onChange={e=>setUserForm({...userForm,username:e.target.value})} placeholder="Benutzername" className="flex-1" required />
                      <Input type="password" value={userForm.password} onChange={e=>setUserForm({...userForm,password:e.target.value})} placeholder={userEditing ? 'Neues Passwort (optional)' : 'Passwort'} className="flex-1" required={!userEditing} />
                      <Toggle checked={userForm.active} onChange={v=>setUserForm({...userForm,active:v})} label="aktiv" />
                      <Button type="submit" variant="success">{userEditing ? 'Speichern' : 'Hinzufügen'}</Button>
                      {userEditing && <Button type="button" variant="secondary" onClick={()=>{setUserEditing(null);setUserForm({username:'',password:'',active:true})}}>Abbrechen</Button>}
                    </form>
                    {userErr && <div className="text-sm text-red-600 mb-2">{userErr}</div>}
                    {siteUsers.length === 0 ? <div className="text-sm text-slate-400">Keine Benutzer.</div> : (
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
                    <h4 className="font-semibold text-slate-800 mb-2">IP-Regeln (Allow / Deny)</h4>
                    <form onSubmit={saveRule} className="flex flex-col sm:flex-row gap-2 mb-3">
                      <Select value={ruleForm.kind} onChange={e=>setRuleForm({...ruleForm,kind:e.target.value})}><option value="allow">Allow</option><option value="deny">Deny</option></Select>
                      <Input value={ruleForm.value} onChange={e=>setRuleForm({...ruleForm,value:e.target.value})} placeholder="IP oder CIDR" className="flex-1" required />
                      <Input value={ruleForm.note} onChange={e=>setRuleForm({...ruleForm,note:e.target.value})} placeholder="Notiz" className="sm:w-40" />
                      <Button type="submit" variant="success">{ruleEditing ? 'Speichern' : 'Hinzufügen'}</Button>
                      {ruleEditing && <Button type="button" variant="secondary" onClick={()=>{setRuleEditing(null);setRuleForm({kind:'allow',value:'',note:''})}}>Abbrechen</Button>}
                    </form>
                    {ruleErr && <div className="text-sm text-red-600 mb-2">{ruleErr}</div>}
                    {siteRules.length === 0 ? <div className="text-sm text-slate-400">Keine Regeln.</div> : (
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

                  <Alert variant="info">Ist Auth aktiv und die IP in der Site-Allow-Liste, ist kein Login nötig. Die globale IP-Whitelist hebt den Site-Login NICHT auf. Nach 3 Fehlversuchen wird die IP 5 Minuten gesperrt (429).</Alert>
                </>
              )}
            </div>
          )}

          {tab === 'advanced' && (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
              <InheritText label="Upstream URL" value={form.upstream_url} onChange={v=>setForm({...form,upstream_url:v})} templateValue={tmpl?.upstream_url} placeholder="http://backend:8080" />
              <InheritText label="Upstream Host / Name (Host-Header)" value={form.upstream_host} onChange={v=>setForm({...form,upstream_host:v})} templateValue={tmpl?.upstream_host} placeholder="z.B. backend.internal (leer = Original-Host)" />
              <InheritNumber label="Rate Limit RPS" value={form.rate_limit_rps} onChange={v=>setForm({...form,rate_limit_rps:v})} templateValue={tmpl?.rate_limit_rps} />
              <InheritNumber label="Rate Limit Burst" value={form.rate_limit_burst} onChange={v=>setForm({...form,rate_limit_burst:v})} templateValue={tmpl?.rate_limit_burst} />
              <InheritNumber label="Max Body Size (MB)" value={form.max_body_size_mb} onChange={v=>setForm({...form,max_body_size_mb:v})} templateValue={tmpl?.max_body_size_mb} />
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
