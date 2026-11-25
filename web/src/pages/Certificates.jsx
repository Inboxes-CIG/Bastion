import { useEffect, useState } from 'react'
import { api, getUser } from '../api.js'
import { Card, Button, Input, Textarea, Select, Field, Modal, Table, Alert, EmptyState, Spinner, Badge } from '../components/UI.jsx'
import Icon from '../components/Icon.jsx'
import { t } from '../i18n.js'

export default function Certificates() {
  const [items, setItems] = useState(null)
  const [tenants, setTenants] = useState([])
  const [err, setErr] = useState('')
  const [openUpload, setOpenUpload] = useState(false)
  const [form, setForm] = useState({ domain:'', cert_pem:'', key_pem:'', tenant_id:'' })
  const [msg, setMsg] = useState('')

  const cu = getUser()
  const isSuper = cu?.role === 'superadmin'
  const myTenantId = isSuper ? '' : (cu?.tenant_id || '')

  const load = () => api('/certificates').then(d=>setItems(d.data||[])).catch(e=>setErr(e.message))
  useEffect(() => { load(); api('/tenants').then(d=>setTenants(d.data||[])).catch(()=>{}) }, [])

  const upload = async () => {
    try { await api('/certificates', { method:'POST', body: JSON.stringify({ ...form, tenant_id: isSuper ? form.tenant_id : myTenantId }) }); setOpenUpload(false); setForm({domain:'',cert_pem:'',key_pem:'',tenant_id:''}); load() }
    catch(e) { setMsg(e.message) }
  }
  const del = async (id) => { if (!confirm(t('Zertifikat löschen?'))) return; await api('/certificates/'+id, { method:'DELETE' }); load() }
  const fmt = (d) => d ? new Date(d).toLocaleDateString() : '—'

  if (err) return <Alert variant="error">{err}</Alert>
  if (!items) return <Spinner />

  return (
    <div className="space-y-4 max-w-7xl">
      {msg && <Alert variant="error">{msg}</Alert>}
      <Alert variant="info">{t("Let's Encrypt-Zertifikate werden pro Site unter Sites → Site → TLS & Zertifikat ausgestellt (inkl. aller Alias-Domains). Hier kannst du nur manuelle Zertifikate hochladen.")}</Alert>
      <div className="flex gap-2 justify-end">
        <Button variant="secondary" onClick={()=>setOpenUpload(true)}><Icon name="plus" className="w-4 h-4" stroke /> {t('Zertifikat hochladen')}</Button>
      </div>
      <Card className="p-0">
        {items.length === 0 ? (
          <EmptyState icon="certs" title={t('Keine Zertifikate')} description={t("Lade ein Zertifikat hoch oder stelle eines über die Site-Konfiguration via Let's Encrypt aus.")} />
        ) : (
          <Table columns={[t('Domain'),'Issuer',t('Source'),t('Gültig bis'),'Auto-Renew','']} rows={items} renderRow={c=>(
            <tr key={c.id} className="border-b border-slate-100 hover:bg-slate-50">
              <td className="px-4 py-3 font-medium text-slate-800">{c.domain}</td>
              <td className="px-4 py-3 text-sm text-slate-600">{c.issuer || '—'}</td>
              <td className="px-4 py-3"><Badge variant={c.source==='letsencrypt'?'green':'slate'}>{c.source}</Badge></td>
              <td className="px-4 py-3 text-sm text-slate-600">{fmt(c.not_after)}</td>
              <td className="px-4 py-3 text-sm">{c.auto_renew?t('Ja'):'—'}</td>
              <td className="px-4 py-3 text-right"><button onClick={()=>del(c.id)} className="p-1.5 text-slate-400 hover:text-red-600"><Icon name="trash" className="w-4 h-4" stroke /></button></td>
            </tr>
          )} />
        )}
      </Card>

      <Modal open={openUpload} onClose={()=>setOpenUpload(false)} title={t('Zertifikat hochladen')}
        footer={<><Button variant="secondary" onClick={()=>setOpenUpload(false)}>{t('Abbrechen')}</Button><Button onClick={upload}>{t('Hochladen')}</Button></>}>
        <div className="space-y-4">
          <Field label={t('Domain')} hint={t('Primäre Domain (z. B. example.com). Alle SANs aus dem Zertifikat – auch Wildcards wie *.example.com – werden automatisch übernommen.')}><Input value={form.domain} onChange={e=>setForm({...form,domain:e.target.value})} /></Field>
          <Field label="Certificate PEM (inkl. Chain)"><Textarea value={form.cert_pem} onChange={e=>setForm({...form,cert_pem:e.target.value})} /></Field>
          <Field label="Private Key PEM"><Textarea value={form.key_pem} onChange={e=>setForm({...form,key_pem:e.target.value})} /></Field>
          <Field label={t('Tenant')}>
            {isSuper ? (
              <Select value={form.tenant_id} onChange={e=>setForm({...form,tenant_id:e.target.value})}><option value="">—</option>{tenants.map(tn=><option key={tn.id} value={tn.id}>{tn.name}</option>)}</Select>
            ) : (
              <Select value={myTenantId} disabled>{tenants.filter(tn=>tn.id===myTenantId).map(tn=><option key={tn.id} value={tn.id}>{tn.name}</option>)}</Select>
            )}
          </Field>
        </div>
      </Modal>
    </div>
  )
}
