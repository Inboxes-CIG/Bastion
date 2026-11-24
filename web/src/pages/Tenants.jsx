import { useEffect, useState } from 'react'
import { api } from '../api.js'
import { Card, Button, Input, Field, Modal, Table, Alert, EmptyState, Spinner } from '../components/UI.jsx'
import Icon from '../components/Icon.jsx'

const empty = { name:'', slug:'' }

export default function Tenants() {
  const [items, setItems] = useState(null)
  const [err, setErr] = useState('')
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState(null)
  const [form, setForm] = useState(empty)

  const load = () => api('/tenants').then(d=>setItems(d.data||[])).catch(e=>setErr(e.message))
  useEffect(load, [])

  const openNew = () => { setEditing(null); setForm(empty); setOpen(true) }
  const openEdit = (t) => { setEditing(t.id); setForm({ name:t.name, slug:t.slug }); setOpen(true) }
  const save = async () => {
    setErr('')
    try {
      if (editing) await api('/tenants/'+editing, { method:'PUT', body: JSON.stringify(form) })
      else await api('/tenants', { method:'POST', body: JSON.stringify(form) })
      setOpen(false); load()
    } catch(e) { setErr(e.message) }
  }
  const del = async (id) => { if (!confirm('Tenant löschen?')) return; await api('/tenants/'+id, { method:'DELETE' }); load() }

  if (err && !open) return <Alert variant="error">{err}</Alert>
  if (!items) return <Spinner />

  return (
    <div className="space-y-4 max-w-7xl">
      <div className="flex justify-end"><Button onClick={openNew}><Icon name="plus" className="w-4 h-4" stroke /> Neuer Tenant</Button></div>
      <Card className="p-0">
        {items.length === 0 ? <EmptyState icon="tenants" title="Keine Tenants" /> : (
          <Table columns={['Name','Slug','Erstellt','']} rows={items} renderRow={t=>(
            <tr key={t.id} className="border-b border-slate-100 hover:bg-slate-50">
              <td className="px-4 py-3 font-medium text-slate-800">{t.name}</td>
              <td className="px-4 py-3 text-sm text-slate-600 font-mono">{t.slug}</td>
              <td className="px-4 py-3 text-sm text-slate-500">{new Date(t.created_at).toLocaleDateString()}</td>
              <td className="px-4 py-3 text-right">
                <button onClick={()=>openEdit(t)} className="p-1.5 text-slate-400 hover:text-indigo-600"><Icon name="edit" className="w-4 h-4" stroke /></button>
                <button onClick={()=>del(t.id)} className="p-1.5 text-slate-400 hover:text-red-600"><Icon name="trash" className="w-4 h-4" stroke /></button>
              </td>
            </tr>
          )} />
        )}
      </Card>

      <Modal open={open} onClose={()=>setOpen(false)} title={editing ? 'Tenant bearbeiten' : 'Neuer Tenant'}
        footer={<><Button variant="secondary" onClick={()=>setOpen(false)}>Abbrechen</Button><Button onClick={save}>{editing?'Speichern':'Erstellen'}</Button></>}>
        <div className="space-y-4">
          {err && <Alert variant="error">{err}</Alert>}
          <Field label="Name"><Input value={form.name} onChange={e=>setForm({...form,name:e.target.value})} /></Field>
          <Field label="Slug"><Input value={form.slug} onChange={e=>setForm({...form,slug:e.target.value})} placeholder="my-tenant" /></Field>
        </div>
      </Modal>
    </div>
  )
}
