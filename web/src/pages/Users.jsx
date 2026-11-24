import { useEffect, useState } from 'react'
import { api } from '../api.js'
import { Card, Button, Input, Select, Field, Modal, Table, Alert, EmptyState, Spinner, Badge, Toggle } from '../components/UI.jsx'
import Icon from '../components/Icon.jsx'

const empty = { email:'', password:'', role_id:'', tenant_id:'', active:true }

export default function Users() {
  const [items, setItems] = useState(null)
  const [roles, setRoles] = useState([])
  const [tenants, setTenants] = useState([])
  const [err, setErr] = useState('')
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState(null)
  const [form, setForm] = useState(empty)

  const load = () => api('/users').then(d=>setItems(d.data||[])).catch(e=>setErr(e.message))
  useEffect(() => {
    load()
    api('/roles').then(d=>setRoles(d.data||[])).catch(()=>{})
    api('/tenants').then(d=>setTenants(d.data||[])).catch(()=>{})
  }, [])

  const openNew = () => { setEditing(null); setForm(empty); setOpen(true) }
  const openEdit = (u) => {
    setEditing(u.id)
    setForm({ email:u.email, password:'', role_id:u.role_id, tenant_id:u.tenant_id||'', active:u.active })
    setOpen(true)
  }
  const save = async () => {
    setErr('')
    try {
      if (editing) {
        const payload = {
          email: form.email,
          role_id: form.role_id,
          tenant_id: form.tenant_id || null,
          clear_tenant: !form.tenant_id,
          active: form.active,
        }
        if (form.password) payload.password = form.password
        await api('/users/'+editing, { method:'PUT', body: JSON.stringify(payload) })
      } else {
        await api('/users', { method:'POST', body: JSON.stringify({ ...form, tenant_id: form.tenant_id || null }) })
      }
      setOpen(false); load()
    } catch(e) { setErr(e.message) }
  }
  const del = async (id) => { if (!confirm('User löschen?')) return; await api('/users/'+id, { method:'DELETE' }); load() }

  if (err && !open) return <Alert variant="error">{err}</Alert>
  if (!items) return <Spinner />

  return (
    <div className="space-y-4 max-w-7xl">
      <div className="flex justify-end"><Button onClick={openNew}><Icon name="plus" className="w-4 h-4" stroke /> Neuer User</Button></div>
      <Card className="p-0">
        {items.length === 0 ? <EmptyState icon="users" title="Keine User" /> : (
          <Table columns={['E-Mail','Rolle','Tenant','Status','']} rows={items} renderRow={u=>(
            <tr key={u.id} className="border-b border-slate-100 hover:bg-slate-50">
              <td className="px-4 py-3 font-medium text-slate-800">{u.email}</td>
              <td className="px-4 py-3 text-sm text-slate-600">{roles.find(r=>r.id===u.role_id)?.name || '—'}</td>
              <td className="px-4 py-3 text-sm text-slate-600">{tenants.find(t=>t.id===u.tenant_id)?.name || '—'}</td>
              <td className="px-4 py-3"><Badge variant={u.active?'green':'slate'}>{u.active?'aktiv':'inaktiv'}</Badge></td>
              <td className="px-4 py-3 text-right">
                <button onClick={()=>openEdit(u)} className="p-1.5 text-slate-400 hover:text-indigo-600"><Icon name="edit" className="w-4 h-4" stroke /></button>
                <button onClick={()=>del(u.id)} className="p-1.5 text-slate-400 hover:text-red-600"><Icon name="trash" className="w-4 h-4" stroke /></button>
              </td>
            </tr>
          )} />
        )}
      </Card>

      <Modal open={open} onClose={()=>setOpen(false)} title={editing ? 'User bearbeiten' : 'Neuer User'}
        footer={<><Button variant="secondary" onClick={()=>setOpen(false)}>Abbrechen</Button><Button onClick={save}>{editing?'Speichern':'Erstellen'}</Button></>}>
        <div className="space-y-4">
          {err && <Alert variant="error">{err}</Alert>}
          <Field label="E-Mail"><Input value={form.email} onChange={e=>setForm({...form,email:e.target.value})} /></Field>
          <Field label={editing ? 'Passwort (leer = unverändert)' : 'Passwort'}>
            <Input type="password" value={form.password} onChange={e=>setForm({...form,password:e.target.value})} />
          </Field>
          <Field label="Rolle"><Select value={form.role_id} onChange={e=>setForm({...form,role_id:e.target.value})}><option value="">—</option>{roles.map(r=><option key={r.id} value={r.id}>{r.name}</option>)}</Select></Field>
          <Field label="Tenant"><Select value={form.tenant_id} onChange={e=>setForm({...form,tenant_id:e.target.value})}><option value="">— (System)</option>{tenants.map(t=><option key={t.id} value={t.id}>{t.name}</option>)}</Select></Field>
          <div className="pt-1"><Toggle checked={form.active} onChange={v=>setForm({...form,active:v})} label={form.active ? 'aktiv' : 'inaktiv'} /></div>
        </div>
      </Modal>
    </div>
  )
}
