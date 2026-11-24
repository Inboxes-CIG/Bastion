import { useEffect, useState } from 'react'
import { api } from '../api.js'
import { Card, Input, Select, Badge, Spinner, Alert, EmptyState, Table } from '../components/UI.jsx'

export default function Audit() {
  const [items, setItems] = useState(null)
  const [err, setErr] = useState('')
  const [action, setAction] = useState('')
  const [q, setQ] = useState('')

  const load = () => {
    const p = new URLSearchParams()
    if (action) p.set('action', action)
    if (q) p.set('q', q)
    api('/audit?'+p.toString()).then(d=>setItems(d.data||[])).catch(e=>setErr(e.message))
  }
  useEffect(load, [action, q])

  if (err) return <Alert variant="error">{err}</Alert>
  if (!items) return <Spinner />

  const variant = (a) => a==='login'?'green':a==='login_failed'?'red':a==='create'?'indigo':a==='update'?'amber':a==='delete'?'red':'slate'
  const label = (a) => ({ login:'Login', login_failed:'Login fehlgeschlagen', create:'Erstellen', update:'Ändern', delete:'Löschen' }[a] || a)

  return (
    <div className="space-y-4 max-w-7xl">
      <div className="flex flex-col sm:flex-row gap-3">
        <Select value={action} onChange={e=>setAction(e.target.value)}>
          <option value="">Alle Aktionen</option>
          <option value="login">Login</option>
          <option value="login_failed">Login fehlgeschlagen</option>
          <option value="create">Erstellen</option>
          <option value="update">Ändern</option>
          <option value="delete">Löschen</option>
        </Select>
        <Input value={q} onChange={e=>setQ(e.target.value)} placeholder="User, Pfad, Ressource..." />
      </div>
      <Card className="p-0">
        {items.length === 0 ? <EmptyState icon="clock" title="Keine Audit-Einträge" /> : (
          <Table columns={['Zeit','User','Aktion','Ressource','Request','Status','IP']} rows={items} renderRow={e=>(
            <tr key={e.id} className="border-b border-slate-100 hover:bg-slate-50">
              <td className="px-4 py-2 text-xs text-slate-500 whitespace-nowrap">{new Date(e.created_at).toLocaleString()}</td>
              <td className="px-4 py-2 text-sm text-slate-800">{e.user_email || '—'}</td>
              <td className="px-4 py-2"><Badge variant={variant(e.action)}>{label(e.action)}</Badge></td>
              <td className="px-4 py-2 text-sm text-slate-600">{e.resource || '—'}</td>
              <td className="px-4 py-2 text-xs font-mono text-slate-500">{e.method} {e.path}</td>
              <td className="px-4 py-2"><Badge variant={e.status < 400 ? 'green' : 'red'}>{e.status}</Badge></td>
              <td className="px-4 py-2 text-xs font-mono text-slate-500">{e.ip}</td>
            </tr>
          )} />
        )}
      </Card>
    </div>
  )
}
