import { useEffect, useState } from 'react'
import { api } from '../api.js'
import { Card, Button, Input, Field, Modal, Table, Alert, EmptyState, Spinner } from '../components/UI.jsx'
import Icon from '../components/Icon.jsx'
import { t } from '../i18n.js'

const empty = { name:'', slug:'', allowed_upstream_hosts:'' }

export default function Tenants() {
  const [items, setItems] = useState(null)
  const [err, setErr] = useState('')
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState(null)
  const [form, setForm] = useState(empty)

  const load = () => api('/tenants').then(d=>setItems(d.data||[])).catch(e=>setErr(e.message))
  useEffect(() => { load() }, [])

  const openNew = () => { setEditing(null); setForm(empty); setOpen(true) }
  const openEdit = (tn) => { setEditing(tn.id); setForm({ name:tn.name, slug:tn.slug, allowed_upstream_hosts:(tn.allowed_upstream_hosts||[]).join(', ') }); setOpen(true) }
  const save = async () => {
    setErr('')
    const payload = {
      name: form.name,
      slug: form.slug,
      allowed_upstream_hosts: form.allowed_upstream_hosts.split(',').map(s=>s.trim()).filter(Boolean),
    }
    try {
      if (editing) await api('/tenants/'+editing, { method:'PUT', body: JSON.stringify(payload) })
      else await api('/tenants', { method:'POST', body: JSON.stringify(payload) })
      setOpen(false); load()
    } catch(e) { setErr(e.message) }
  }
  const del = async (id) => { if (!confirm(t('Tenant löschen?'))) return; await api('/tenants/'+id, { method:'DELETE' }); load() }

  if (err && !open) return <Alert variant="error">{err}</Alert>
  if (!items) return <Spinner />

  return (
    <div className="space-y-4 max-w-7xl">
      <div className="flex justify-end"><Button onClick={openNew}><Icon name="plus" className="w-4 h-4" stroke /> {t('Neuer Tenant')}</Button></div>
      <Card className="p-0">
        {items.length === 0 ? <EmptyState icon="tenants" title={t('Keine Tenants')} /> : (
          <Table columns={[t('Name'),'Slug',t('Upstream-Allowliste'),t('Erstellt'),'']} rows={items} renderRow={tn=>(
            <tr key={tn.id} className="border-b border-slate-100 hover:bg-slate-50">
              <td className="px-4 py-3 font-medium text-slate-800">{tn.name}</td>
              <td className="px-4 py-3 text-sm text-slate-600 font-mono">{tn.slug}</td>
              <td className="px-4 py-3 text-xs font-mono text-slate-500">{(tn.allowed_upstream_hosts||[]).join(', ') || '—'}</td>
              <td className="px-4 py-3 text-sm text-slate-500">{new Date(tn.created_at).toLocaleDateString()}</td>
              <td className="px-4 py-3 text-right">
                <button onClick={()=>openEdit(tn)} className="p-1.5 text-slate-400 hover:text-indigo-600"><Icon name="edit" className="w-4 h-4" stroke /></button>
                <button onClick={()=>del(tn.id)} className="p-1.5 text-slate-400 hover:text-red-600"><Icon name="trash" className="w-4 h-4" stroke /></button>
              </td>
            </tr>
          )} />
        )}
      </Card>

      <Modal open={open} onClose={()=>setOpen(false)} title={editing ? t('Tenant bearbeiten') : t('Neuer Tenant')}
        footer={<><Button variant="secondary" onClick={()=>setOpen(false)}>{t('Abbrechen')}</Button><Button onClick={save}>{editing?t('Speichern'):t('Erstellen')}</Button></>}>
        <div className="space-y-4">
          {err && <Alert variant="error">{err}</Alert>}
          <Field label={t('Name')}><Input value={form.name} onChange={e=>setForm({...form,name:e.target.value})} /></Field>
          <Field label="Slug"><Input value={form.slug} onChange={e=>setForm({...form,slug:e.target.value})} placeholder="my-tenant" /></Field>
          <Field label={t('Upstream-Allowliste')} hint={t('Kommagetrennte Hosts, Domains oder CIDRs, die private Upstreams für diesen Tenant dürfen (z.B. 10.0.0.0/8, internal.example.com).')}>
            <Input value={form.allowed_upstream_hosts} onChange={e=>setForm({...form,allowed_upstream_hosts:e.target.value})} placeholder="10.0.0.0/8, internal.example.com" />
          </Field>
        </div>
      </Modal>
    </div>
  )
}
