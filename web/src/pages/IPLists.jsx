import { useEffect, useState } from 'react'
import { api } from '../api.js'
import { Card, Button, Input, Select, Alert, Spinner } from '../components/UI.jsx'
import Icon from '../components/Icon.jsx'
import { t } from '../i18n.js'

const TTLS = [
  { v: '0', label: 'unbegrenzt' },
  { v: '600', label: '10 Minuten' },
  { v: '3600', label: '1 Stunde' },
  { v: '86400', label: '24 Stunden' },
  { v: '604800', label: '7 Tage' },
]

function ListCard({ title, subtitle, kind, entries, onAdd, onUpdate, onDelete }) {
  const [value, setValue] = useState('')
  const [note, setNote] = useState('')
  const [ttl, setTtl] = useState('0')
  const [err, setErr] = useState('')
  const [editingId, setEditingId] = useState(null)

  const startEdit = (e) => { setEditingId(e.id); setValue(e.value); setNote(e.note || ''); setTtl('keep'); setErr('') }
  const cancel = () => { setEditingId(null); setValue(''); setNote(''); setTtl('0'); setErr('') }

  const submit = async (ev) => {
    ev.preventDefault(); setErr('')
    try {
      if (editingId) {
        await onUpdate(kind, editingId, value, note, ttl)
        cancel()
      } else {
        await onAdd(kind, value, note, ttl)
        setValue(''); setNote(''); setTtl('0')
      }
    } catch (e) { setErr(e.message) }
  }

  return (
    <Card title={title} subtitle={subtitle}>
      <form onSubmit={submit} className="flex flex-col sm:flex-row gap-2 mb-3">
        <Input value={value} onChange={e=>setValue(e.target.value)} placeholder={t('IP oder CIDR (z.B. 203.0.113.5)')} className="flex-1" required />
        <Input value={note} onChange={e=>setNote(e.target.value)} placeholder={t('Notiz')} className="sm:w-36" />
        <Select value={ttl} onChange={e=>setTtl(e.target.value)} className="sm:w-36">
          {editingId && <option value="keep">{t('unverändert')}</option>}
          {TTLS.map(o => <option key={o.v} value={o.v}>{t(o.label)}</option>)}
        </Select>
        <Button type="submit" variant={kind==='allow'?'success':'danger'}>{editingId ? t('Speichern') : t('Hinzufügen')}</Button>
        {editingId && <Button type="button" variant="secondary" onClick={cancel}>{t('Abbrechen')}</Button>}
      </form>
      {err && <div className="text-sm text-red-600 mb-2">{err}</div>}
      {entries.length === 0 ? (
        <div className="text-sm text-slate-400 py-6 text-center">{t('Noch keine Einträge.')}</div>
      ) : (
        <div className="divide-y divide-slate-100">
          {entries.map(e => (
            <div key={e.id} className={`flex items-center justify-between py-2 ${editingId===e.id?'bg-indigo-50/50 -mx-2 px-2 rounded':''}`}>
              <div>
                <div className="font-mono text-sm text-slate-800">
                  {e.value}
                  {e.source === 'auto_ban' && <span className="ml-2 text-[10px] uppercase text-rose-500">{t('auto')}</span>}
                </div>
                {(e.note || e.expires_at) && <div className="text-xs text-slate-400">{e.note}{e.expires_at && <span className="ml-1">· {t('läuft ab')} {new Date(e.expires_at).toLocaleString()}</span>}</div>}
              </div>
              <div className="flex items-center">
                <button onClick={()=>startEdit(e)} className="p-1.5 text-slate-400 hover:text-indigo-600"><Icon name="edit" className="w-4 h-4" stroke /></button>
                <button onClick={()=>onDelete(e.id)} className="p-1.5 text-slate-400 hover:text-red-600"><Icon name="trash" className="w-4 h-4" stroke /></button>
              </div>
            </div>
          ))}
        </div>
      )}
    </Card>
  )
}

export default function IPLists() {
  const [entries, setEntries] = useState(null)
  const [err, setErr] = useState('')

  const load = () => api('/iplist').then(d=>setEntries(d.data||[])).catch(e=>setErr(e.message))
  useEffect(() => { load() }, [])

  const add = async (kind, value, note, ttl) => {
    const body = { kind, value, note }
    if (ttl !== 'keep') body.ttl_seconds = Number(ttl) || 0
    await api('/iplist', { method:'POST', body: JSON.stringify(body) })
    load()
  }
  const update = async (kind, id, value, note, ttl) => {
    const body = { kind, value, note }
    if (ttl !== 'keep') body.ttl_seconds = Number(ttl) || 0
    await api('/iplist/'+id, { method:'PUT', body: JSON.stringify(body) })
    load()
  }
  const del = async (id) => {
    if (!confirm(t('Eintrag löschen?'))) return
    await api('/iplist/'+id, { method:'DELETE' })
    load()
  }

  if (err) return <Alert variant="error">{err}</Alert>
  if (!entries) return <Spinner />

  const allow = entries.filter(e => e.kind === 'allow')
  const deny = entries.filter(e => e.kind === 'deny')

  return (
    <div className="space-y-4 max-w-6xl">
      <Alert variant="info">
        <b>{t('Allowlist')}</b> {t('überspringt CrowdSec- und GeoIP-Sperren (z.B. für dein Büro/VPN), aber NICHT den per-Site Login.')}
        <b> {t('Blocklist')}</b> {t('sperrt IPs sofort und ohne Rückmeldung – hat Vorrang vor der Allowlist.')}
        <div className="mt-1 text-xs text-slate-500">{t('Einträge können mit Ablauf hinterlegt werden; abgelaufene Sperren werden automatisch entfernt. Automatische 404-Sperren eskalieren: zuerst kurz, dann 24h, danach dauerhaft.')}</div>
      </Alert>
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <ListCard title={t('Allowlist (Whitelist)')} subtitle={t('Immer erlaubte IPs / CIDRs')} kind="allow" entries={allow} onAdd={add} onUpdate={update} onDelete={del} />
        <ListCard title={t('Blocklist (Blacklist)')} subtitle={t('Immer gesperrte IPs / CIDRs')} kind="deny" entries={deny} onAdd={add} onUpdate={update} onDelete={del} />
      </div>
    </div>
  )
}
