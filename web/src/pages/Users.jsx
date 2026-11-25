import { useEffect, useState } from 'react'
import { api } from '../api.js'
import { Card, Button, Input, Select, Field, Modal, Table, Alert, EmptyState, Spinner, Badge, Toggle } from '../components/UI.jsx'
import Icon from '../components/Icon.jsx'
import { t } from '../i18n.js'

const empty = { email:'', password:'', role_id:'', tenant_id:'', active:true, reauth:'' }

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

  const openNew = () => { setEditing(null); setForm({...empty}); setOpen(true) }
  const openEdit = (u) => {
    setEditing(u.id)
    setForm({ email:u.email, password:'', role_id:u.role_id, tenant_id:u.tenant_id||'', active:u.active, reauth:'' })
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
          reauth_password: form.reauth || undefined,
        }
        if (form.password) payload.password = form.password
        await api('/users/'+editing, { method:'PUT', body: JSON.stringify(payload) })
      } else {
        await api('/users', { method:'POST', body: JSON.stringify({
          email: form.email,
          password: form.password,
          role_id: form.role_id,
          tenant_id: form.tenant_id || null,
          active: form.active,
        }) })
      }
      setOpen(false); load()
    } catch(e) { setErr(e.message) }
  }
  const del = async (id) => { if (!confirm(t('User löschen?'))) return; await api('/users/'+id, { method:'DELETE' }); load() }

  if (err && !open) return <Alert variant="error">{err}</Alert>
  if (!items) return <Spinner />

  return (
    <div className="space-y-4 max-w-7xl">
      <div className="flex justify-end"><Button onClick={openNew}><Icon name="plus" className="w-4 h-4" stroke /> {t('Neuer User')}</Button></div>
      <Card className="p-0">
        {items.length === 0 ? <EmptyState icon="users" title={t('Keine User')} /> : (
          <Table columns={[t('E-Mail'),t('Rolle'),t('Tenant'),t('Status'),'']} rows={items} renderRow={u=>(
            <tr key={u.id} className="border-b border-slate-100 hover:bg-slate-50">
              <td className="px-4 py-3 font-medium text-slate-800">{u.email}</td>
              <td className="px-4 py-3 text-sm text-slate-600">{roles.find(r=>r.id===u.role_id)?.name || '—'}</td>
              <td className="px-4 py-3 text-sm text-slate-600">{tenants.find(tn=>tn.id===u.tenant_id)?.name || '—'}</td>
              <td className="px-4 py-3"><Badge variant={u.active?'green':'slate'}>{u.active?t('aktiv'):t('inaktiv')}</Badge>{u.must_change_password && <Badge variant="amber"> {t('Passwort ändern')}</Badge>}</td>
              <td className="px-4 py-3 text-right">
                <button onClick={()=>openEdit(u)} className="p-1.5 text-slate-400 hover:text-indigo-600"><Icon name="edit" className="w-4 h-4" stroke /></button>
                <button onClick={()=>del(u.id)} className="p-1.5 text-slate-400 hover:text-red-600"><Icon name="trash" className="w-4 h-4" stroke /></button>
              </td>
            </tr>
          )} />
        )}
      </Card>

      <Modal open={open} onClose={()=>setOpen(false)} title={editing ? t('User bearbeiten') : t('Neuer User')}
        footer={<><Button variant="secondary" onClick={()=>setOpen(false)}>{t('Abbrechen')}</Button><Button onClick={save}>{editing?t('Speichern'):t('Erstellen')}</Button></>}>
        <div className="space-y-4">
          {err && <Alert variant="error">{err}</Alert>}
          {!editing && <Alert variant="warn">{t('Ein Passwort ist für neue Benutzer erforderlich.')}</Alert>}
          {editing && <Alert variant="info">{t('Sensible Änderungen (Rolle, E-Mail, Status, Passwort) erfordern dein aktuelles Passwort.')}</Alert>}
          <Field label={t('E-Mail')}><Input value={form.email} onChange={e=>setForm({...form,email:e.target.value})} /></Field>
          <Field label={editing ? t('Passwort (leer = unverändert)') : t('Passwort')}>
            <Input type="password" value={form.password} onChange={e=>setForm({...form,password:e.target.value})} />
          </Field>
          {editing && (
            <Field label={t('Dein aktuelles Passwort (Bestätigung)')}>
              <Input type="password" autoComplete="new-password" value={form.reauth} onChange={e=>setForm({...form,reauth:e.target.value})} placeholder={t('Nur bei sensiblen Änderungen erforderlich')} />
            </Field>
          )}
          <Field label={t('Rolle')}><Select value={form.role_id} onChange={e=>setForm({...form,role_id:e.target.value})}><option value="">—</option>{roles.map(r=><option key={r.id} value={r.id}>{r.name}</option>)}</Select></Field>
          <Field label={t('Tenant')}><Select value={form.tenant_id} onChange={e=>setForm({...form,tenant_id:e.target.value})}><option value="">— (System)</option>{tenants.map(tn=><option key={tn.id} value={tn.id}>{tn.name}</option>)}</Select></Field>
          <div className="pt-1"><Toggle checked={form.active} onChange={v=>setForm({...form,active:v})} label={form.active ? t('aktiv') : t('inaktiv')} /></div>
        </div>
      </Modal>
    </div>
  )
}
