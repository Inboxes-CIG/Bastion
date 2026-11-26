import { useEffect, useState } from 'react'
import { api } from '../api.js'
import { Card, Button, Input, Select, Field, Modal, Table, Alert, EmptyState, Spinner, Badge, StatCard } from '../components/UI.jsx'
import Icon from '../components/Icon.jsx'
import { t } from '../i18n.js'

const TABS = [
  { id: 'decisions', label: 'Entscheidungen' },
  { id: 'alerts', label: 'Alerts' },
  { id: 'actors', label: 'Good Actors' },
]

export default function CrowdSec() {
  const [tab, setTab] = useState('decisions')
  const [status, setStatus] = useState(null)
  const [decisions, setDecisions] = useState([])
  const [alerts, setAlerts] = useState([])
  const [actors, setActors] = useState([])
  const [err, setErr] = useState('')
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState({ ip:'', duration:'1h', reason:'manual' })
  const [busy, setBusy] = useState(false)
  const [actorForm, setActorForm] = useState({ value:'', note:'' })
  const [actorEditing, setActorEditing] = useState(null)
  const [actorErr, setActorErr] = useState('')

  const load = () => {
    api('/crowdsec/status').then(d=>setStatus(d.data)).catch(e=>setErr(e.message))
    api('/crowdsec/decisions').then(d=>setDecisions(d.data||[])).catch(e=>setErr(e.message))
    api('/crowdsec/alerts?limit=50').then(d=>setAlerts(d.data||[])).catch(e=>setErr(e.message))
    api('/crowdsec/good-actors').then(d=>setActors(d.data||[])).catch(e=>setErr(e.message))
  }
  useEffect(() => { load() }, [])

  const ban = async () => {
    setErr(''); setBusy(true)
    try {
      await api('/crowdsec/decisions', { method:'POST', body: JSON.stringify(form) })
      setOpen(false); setForm({ ip:'', duration:'1h', reason:'manual' }); load()
    } catch(e) { setErr(e.message) } finally { setBusy(false) }
  }
  const unban = async (id) => {
    if (!confirm(t('Entscheidung wirklich löschen (Unban)?'))) return
    await api('/crowdsec/decisions/'+id, { method:'DELETE' }); load()
  }

  const saveActor = async (e) => {
    e.preventDefault(); setActorErr('')
    try {
      if (actorEditing) await api('/crowdsec/good-actors/'+actorEditing, { method:'PUT', body: JSON.stringify(actorForm) })
      else await api('/crowdsec/good-actors', { method:'POST', body: JSON.stringify(actorForm) })
      setActorForm({ value:'', note:'' }); setActorEditing(null); load()
    } catch(e) { setActorErr(e.message) }
  }
  const editActor = (a) => { setActorEditing(a.id); setActorForm({ value:a.value, note:a.note||'' }); setActorErr('') }
  const delActor = async (id) => {
    if (!confirm(t('Good Actor löschen?'))) return
    await api('/crowdsec/good-actors/'+id, { method:'DELETE' }); load()
  }

  if (!status) return <Spinner />
  if (status.enabled === false) {
    return <div className="max-w-3xl"><Alert variant="warn">{t('CrowdSec ist nicht aktiviert. Setze crowdsec.enabled: true in /etc/bastion/config.yaml.')}</Alert></div>
  }

  return (
    <div className="space-y-5 max-w-7xl">
      {err && <Alert variant="error">{err}</Alert>}

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <StatCard label={t('Verbindung')} value={status.connected ? t('Verbunden') : t('Fehler')} icon="shield" accent={status.connected ? 'emerald' : 'rose'} />
        <StatCard label={t('Aktive Entscheidungen')} value={status.count} icon="bolt" accent="indigo" />
        <div className="bg-white rounded-xl border border-slate-200 shadow-sm p-5">
          <div className="text-sm text-slate-500 mb-1">{t('Zuletzt synchronisiert')}</div>
          <div className="text-sm font-medium text-slate-800">{status.last_sync ? new Date(status.last_sync).toLocaleString() : '—'}</div>
          {status.error && <div className="text-xs text-red-500 mt-1 break-all">{status.error}</div>}
        </div>
      </div>

      <div className="bg-white rounded-xl border border-slate-200 shadow-sm">
        <div className="flex items-center justify-between px-2 border-b border-slate-100">
          <div className="flex gap-1">
            {TABS.map(tb => (
              <button key={tb.id} onClick={()=>setTab(tb.id)}
                className={`px-4 py-3 text-sm font-medium border-b-2 ${tab===tb.id?'border-indigo-600 text-indigo-600':'border-transparent text-slate-500 hover:text-slate-700'}`}>
                {t(tb.label)}
              </button>
            ))}
          </div>
          <button onClick={load} title={t('Neu laden')}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 mr-2 text-sm text-slate-600 hover:text-indigo-600 hover:bg-slate-50 rounded-lg">
            <Icon name="refresh" className="w-4 h-4" stroke /> {t('Neu laden')}
          </button>
        </div>

        <div className="p-5">
          {tab === 'decisions' && (
            <>
              <div className="flex justify-end mb-3"><Button onClick={()=>setOpen(true)}><Icon name="plus" className="w-4 h-4" stroke /> {t('IP sperren')}</Button></div>
              {decisions.length === 0 ? <EmptyState icon="shield" title={t('Keine aktiven Bans')} /> : (
                <Table columns={['IP / Range',t('Dauer'),t('Szenario'),t('Herkunft'),t('Typ'),'']} rows={decisions} renderRow={d=>(
                  <tr key={d.id} className="border-b border-slate-100 hover:bg-slate-50">
                    <td className="px-4 py-3 font-mono text-sm text-slate-800">{d.value}</td>
                    <td className="px-4 py-3 text-sm text-slate-600">{d.duration}</td>
                    <td className="px-4 py-3 text-sm text-slate-600">{d.scenario}</td>
                    <td className="px-4 py-3"><Badge variant={d.origin==='cscli'?'indigo':'slate'}>{d.origin}</Badge></td>
                    <td className="px-4 py-3"><Badge variant="red">{d.type}</Badge></td>
                    <td className="px-4 py-3 text-right"><button onClick={()=>unban(d.id)} className="text-sm text-red-600 hover:underline">Unban</button></td>
                  </tr>
                )} />
              )}
            </>
          )}

          {tab === 'alerts' && (
            alerts.length === 0 ? <EmptyState icon="alert" title={t('Keine Alerts')} description={t('CrowdSec hat noch keine Alerts erzeugt.')} /> : (
              <Table columns={[t('Zeit'),t('Quelle'),t('Szenario'),t('Meldung'),t('Entscheidungen')]} rows={alerts} renderRow={a=>(
                <tr key={a.id} className="border-b border-slate-100 hover:bg-slate-50">
                  <td className="px-4 py-2 text-xs text-slate-500 whitespace-nowrap">{new Date(a.created_at).toLocaleString()}</td>
                  <td className="px-4 py-2 font-mono text-sm text-slate-800">{a.source?.value || a.source?.ip || '—'}</td>
                  <td className="px-4 py-2 text-sm text-slate-700">{a.scenario}</td>
                  <td className="px-4 py-2 text-xs text-slate-500 max-w-xs truncate" title={a.message}>{a.message || '—'}</td>
                  <td className="px-4 py-2 text-sm">{(a.decisions && a.decisions.length) || 0}</td>
                </tr>
              )} />
            )
          )}

          {tab === 'actors' && (
            <>
              <Alert variant="info">{t('Good Actors werden in die CrowdSec-Whitelist geschrieben – für diese IPs/CIDRs erzeugt CrowdSec keine Alerts/Bans (z.B. dein Büro/VPN).')}</Alert>
              <form onSubmit={saveActor} className="flex flex-col sm:flex-row gap-2 my-3">
                <Input value={actorForm.value} onChange={e=>setActorForm({...actorForm,value:e.target.value})} placeholder={t('IP oder CIDR')} className="flex-1" required />
                <Input value={actorForm.note} onChange={e=>setActorForm({...actorForm,note:e.target.value})} placeholder={t('Notiz')} className="sm:w-44" />
                <Button type="submit" variant="success">{actorEditing ? t('Speichern') : t('Hinzufügen')}</Button>
                {actorEditing && <Button type="button" variant="secondary" onClick={()=>{setActorEditing(null);setActorForm({value:'',note:''})}}>{t('Abbrechen')}</Button>}
              </form>
              {actorErr && <div className="text-sm text-red-600 mb-2">{actorErr}</div>}
              {actors.length === 0 ? <EmptyState icon="users" title={t('Keine Good Actors')} /> : (
                <div className="divide-y divide-slate-100">
                  {actors.map(a => (
                    <div key={a.id} className="flex items-center justify-between py-2">
                      <div>
                        <div className="font-mono text-sm text-slate-800">{a.value}</div>
                        {a.note && <div className="text-xs text-slate-400">{a.note}</div>}
                      </div>
                      <div className="flex items-center">
                        <button onClick={()=>editActor(a)} className="p-1.5 text-slate-400 hover:text-indigo-600"><Icon name="edit" className="w-4 h-4" stroke /></button>
                        <button onClick={()=>delActor(a.id)} className="p-1.5 text-slate-400 hover:text-red-600"><Icon name="trash" className="w-4 h-4" stroke /></button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </>
          )}
        </div>
      </div>

      <Modal open={open} onClose={()=>setOpen(false)} title={t('IP sperren (CrowdSec)')}
        footer={<><Button variant="secondary" onClick={()=>setOpen(false)}>{t('Abbrechen')}</Button><Button variant="danger" onClick={ban} disabled={busy}>{busy?'...':t('Sperren')}</Button></>}>
        <div className="space-y-4">
          <Field label={t('IP oder CIDR')} hint={t('z.B. 203.0.113.42 oder 203.0.113.0/24')}><Input value={form.ip} onChange={e=>setForm({...form,ip:e.target.value})} placeholder="203.0.113.42" /></Field>
          <Field label={t('Dauer')}>
            <Select value={form.duration} onChange={e=>setForm({...form,duration:e.target.value})}>
              <option value="1h">{t('1 Stunde')}</option>
              <option value="4h">{t('4 Stunden')}</option>
              <option value="24h">{t('24 Stunden')}</option>
              <option value="7d">{t('7 Tage')}</option>
              <option value="30d">{t('30 Tage')}</option>
            </Select>
          </Field>
          <Field label={t('Grund')}><Input value={form.reason} onChange={e=>setForm({...form,reason:e.target.value})} /></Field>
        </div>
      </Modal>
    </div>
  )
}
