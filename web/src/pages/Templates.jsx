import { useEffect, useState } from 'react'
import { api } from '../api.js'
import { Card, Button, Input, Select, Field, Modal, Table, Toggle, Alert, EmptyState, Spinner, MultiSelect } from '../components/UI.jsx'
import Icon from '../components/Icon.jsx'

const empty = { name:'', description:'', tenant_id:'', upstream_url:'', upstream_host:'', geoip_enabled:false, geoip_mode:'allow', geoip_countries:[], http2_enabled:true, http3_enabled:false, force_https:true, hsts_enabled:true, hsts_max_age:31536000, websocket_enabled:false, compression_enabled:true, rate_limit_rps:0, rate_limit_burst:0, max_body_size_mb:0, cache_enabled:true, cache_ttl:60, cache_max_object_kb:256, auth_enabled:false }

export default function Templates() {
  const [items, setItems] = useState(null)
  const [tenants, setTenants] = useState([])
  const [err, setErr] = useState('')
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState(null)
  const [form, setForm] = useState(empty)
  const [countries, setCountries] = useState([])

  const load = () => api('/templates').then(d=>setItems(d.data||[])).catch(e=>setErr(e.message))
  useEffect(() => { load(); api('/tenants').then(d=>setTenants(d.data||[])).catch(()=>{}); api('/geoip/countries').then(d=>setCountries(d.data||[])).catch(()=>{}) }, [])

  const openNew = () => { setEditing(null); setForm(empty); setOpen(true) }
  const openEdit = (t) => {
    setEditing(t.id)
    setForm({
      name:t.name, description:t.description||'', tenant_id:t.tenant_id, upstream_url:t.upstream_url||'', upstream_host:t.upstream_host||'',
      geoip_enabled:t.geoip_enabled??false, geoip_mode:t.geoip_mode||'allow', geoip_countries:t.geoip_countries||[], http2_enabled:t.http2_enabled??true,
      http3_enabled:t.http3_enabled??false, force_https:t.force_https??true, hsts_enabled:t.hsts_enabled??true,
      hsts_max_age:t.hsts_max_age??31536000, websocket_enabled:t.websocket_enabled??false,
      compression_enabled:t.compression_enabled??true, rate_limit_rps:t.rate_limit_rps??0,
      rate_limit_burst:t.rate_limit_burst??0, max_body_size_mb:t.max_body_size_mb??0, cache_enabled:t.cache_enabled, cache_ttl:t.cache_ttl, cache_max_object_kb:t.cache_max_object_kb, auth_enabled:t.auth_enabled??false,
    })
    setOpen(true)
  }

  const save = async () => {
    try {
      const payload = { ...form, upstream_url: form.upstream_url || null, upstream_host: form.upstream_host || null }
      if (editing) await api('/templates/'+editing, { method:'PUT', body: JSON.stringify(payload) })
      else await api('/templates', { method:'POST', body: JSON.stringify(payload) })
      setOpen(false); load()
    } catch(e) { setErr(e.message) }
  }

  const del = async (id) => {
    if (!confirm('Template löschen? Sites werden abgekoppelt.')) return
    await api('/templates/'+id, { method:'DELETE' }); load()
  }

  if (err) return <Alert variant="error">{err}</Alert>
  if (!items) return <Spinner />

  return (
    <div className="space-y-4 max-w-7xl">
      <div className="flex justify-end"><Button onClick={openNew}><Icon name="plus" className="w-4 h-4" stroke /> Neues Template</Button></div>
      <Card className="p-0">
        {items.length === 0 ? (
          <EmptyState icon="templates" title="Keine Templates" description="Templates definieren Standard-Einstellungen für Sites." action={<Button onClick={openNew}>Neues Template</Button>} />
        ) : (
          <Table columns={['Name','Tenant','Upstream','HTTP/2','GeoIP','']} rows={items} renderRow={t=>(
            <tr key={t.id} className="border-b border-slate-100 hover:bg-slate-50">
              <td className="px-4 py-3"><div className="font-medium text-slate-800">{t.name}</div>{t.description && <div className="text-xs text-slate-400">{t.description}</div>}</td>
              <td className="px-4 py-3 text-sm text-slate-600">{tenants.find(x=>x.id===t.tenant_id)?.name || '—'}</td>
              <td className="px-4 py-3 text-xs text-slate-500">{t.upstream_url || '—'}</td>
              <td className="px-4 py-3 text-xs">{t.http2_enabled===null?'—':t.http2_enabled?'on':'off'}</td>
              <td className="px-4 py-3 text-xs">{t.geoip_enabled?'on':'off'}</td>
              <td className="px-4 py-3 text-right">
                <button onClick={()=>openEdit(t)} className="p-1.5 text-slate-400 hover:text-indigo-600"><Icon name="edit" className="w-4 h-4" stroke /></button>
                <button onClick={()=>del(t.id)} className="p-1.5 text-slate-400 hover:text-red-600"><Icon name="trash" className="w-4 h-4" stroke /></button>
              </td>
            </tr>
          )} />
        )}
      </Card>

      <Modal open={open} onClose={()=>setOpen(false)} title={editing?'Template bearbeiten':'Neues Template'} size="lg"
        footer={<><Button variant="secondary" onClick={()=>setOpen(false)}>Abbrechen</Button><Button onClick={save}>Speichern</Button></>}>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Field label="Name"><Input value={form.name} onChange={e=>setForm({...form,name:e.target.value})} /></Field>
          <Field label="Tenant"><Select value={form.tenant_id} onChange={e=>setForm({...form,tenant_id:e.target.value})}><option value="">— wählen —</option>{tenants.map(t=><option key={t.id} value={t.id}>{t.name}</option>)}</Select></Field>
          <Field label="Beschreibung" className="sm:col-span-2"><Input value={form.description} onChange={e=>setForm({...form,description:e.target.value})} /></Field>
          <Field label="Upstream URL" className="sm:col-span-2"><Input value={form.upstream_url} onChange={e=>setForm({...form,upstream_url:e.target.value})} placeholder="http://backend:8080" /></Field>
          <Field label="Upstream Host / Name (Host-Header)" className="sm:col-span-2"><Input value={form.upstream_host} onChange={e=>setForm({...form,upstream_host:e.target.value})} placeholder="z.B. backend.internal (leer = Original-Host)" /></Field>
          <Field label="GeoIP Modus"><Select value={form.geoip_mode} onChange={e=>setForm({...form,geoip_mode:e.target.value})}><option value="block">Block (ausgewählte sperren)</option><option value="allow">Allow (nur ausgewählte erlauben)</option></Select></Field>
          <Field label="Länder" className="sm:col-span-2">
            <MultiSelect options={countries} value={form.geoip_countries} onChange={v=>setForm({...form,geoip_countries:v})} placeholder="Länder auswählen..." />
          </Field>
          <Field label="HSTS Max Age"><Input type="number" value={form.hsts_max_age} onChange={e=>setForm({...form,hsts_max_age:Number(e.target.value)})} /></Field>
          <Field label="Rate Limit RPS"><Input type="number" value={form.rate_limit_rps} onChange={e=>setForm({...form,rate_limit_rps:Number(e.target.value)})} /></Field>
          <Field label="Max Body Size (MB)"><Input type="number" value={form.max_body_size_mb} onChange={e=>setForm({...form,max_body_size_mb:Number(e.target.value)})} /></Field>
          <Field label="Cache aktiv"><div className="pt-2"><Toggle checked={form.cache_enabled===true} onChange={v=>setForm({...form,cache_enabled:v})} label={form.cache_enabled?'an':'aus'} /></div></Field>
          <Field label="Cache TTL (s)"><Input type="number" value={form.cache_ttl??''} onChange={e=>setForm({...form,cache_ttl:e.target.value===''?null:Number(e.target.value)})} /></Field>
          <Field label="Cache max. Objekt (KB)"><Input type="number" value={form.cache_max_object_kb??''} onChange={e=>setForm({...form,cache_max_object_kb:e.target.value===''?null:Number(e.target.value)})} /></Field>
        </div>
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-3 mt-5 pt-4 border-t border-slate-100">
          <Toggle checked={form.geoip_enabled} onChange={v=>setForm({...form,geoip_enabled:v})} label="GeoIP" />
          <Toggle checked={form.http2_enabled} onChange={v=>setForm({...form,http2_enabled:v})} label="HTTP/2" />
          <Toggle checked={form.http3_enabled} onChange={v=>setForm({...form,http3_enabled:v})} label="HTTP/3" />
          <Toggle checked={form.force_https} onChange={v=>setForm({...form,force_https:v})} label="Force HTTPS" />
          <Toggle checked={form.hsts_enabled} onChange={v=>setForm({...form,hsts_enabled:v})} label="HSTS" />
          <Toggle checked={form.websocket_enabled} onChange={v=>setForm({...form,websocket_enabled:v})} label="WebSocket" />
          <Toggle checked={form.compression_enabled} onChange={v=>setForm({...form,compression_enabled:v})} label="Compression" />
          <Toggle checked={form.auth_enabled===true} onChange={v=>setForm({...form,auth_enabled:v})} label="Auth (Basic)" />
        </div>
      </Modal>
    </div>
  )
}
