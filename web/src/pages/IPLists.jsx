import { useEffect, useState } from 'react'
import { api } from '../api.js'
import { Card, Button, Input, Select, Field, Modal, Alert, Spinner } from '../components/UI.jsx'
import Icon from '../components/Icon.jsx'
import { t } from '../i18n.js'

const TTLS = [
  { v: '0', label: 'unbegrenzt' },
  { v: '600', label: '10 Minuten' },
  { v: '3600', label: '1 Stunde' },
  { v: '86400', label: '24 Stunden' },
  { v: '604800', label: '7 Tage' },
]

function ListCard({ title, subtitle, kind, entries, onAdd, onEdit, onDelete }) {
  return (
    <Card title={title} subtitle={subtitle}>
      <div className="flex justify-end mb-1">
        <Button variant="secondary" onClick={() => onAdd(kind)}>
          <Icon name="plus" className="w-4 h-4" stroke /> {t('Hinzufügen')}
        </Button>
      </div>
      {entries.length === 0 ? (
        <div className="text-sm text-slate-400 py-6 text-center">{t('Noch keine Einträge.')}</div>
      ) : (
        <div className="divide-y divide-slate-100">
          {entries.map(e => (
            <div key={e.id} className="flex items-center justify-between py-2">
              <div>
                <div className="font-mono text-sm text-slate-800">
                  {e.value}
                  {e.source === 'auto_ban' && <span className="ml-2 text-[10px] uppercase text-rose-500">{t('auto')}</span>}
                </div>
                {(e.note || e.expires_at) && (
                  <div className="text-xs text-slate-400">
                    {e.note}
                    {e.expires_at && <span className="ml-1">· {t('läuft ab')} {new Date(e.expires_at).toLocaleString()}</span>}
                  </div>
                )}
              </div>
              <div className="flex items-center">
                <button onClick={() => onEdit(e)} className="p-1.5 text-slate-400 hover:text-indigo-600"><Icon name="edit" className="w-4 h-4" stroke /></button>
                <button onClick={() => onDelete(e.id)} className="p-1.5 text-slate-400 hover:text-red-600"><Icon name="trash" className="w-4 h-4" stroke /></button>
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
  const [modal, setModal] = useState(null)
  const [busy, setBusy] = useState(false)

  const load = () => api('/iplist').then(d => setEntries(d.data || [])).catch(e => setErr(e.message))
  useEffect(() => { load() }, [])

  const openAdd = (kind) => setModal({ kind, id: null, value: '', note: '', ttl: '0', err: '' })
  const openEdit = (e) => setModal({ kind: e.kind, id: e.id, value: e.value, note: e.note || '', ttl: 'keep', err: '' })
  const set = (patch) => setModal(m => ({ ...m, ...patch }))

  const save = async () => {
    if (!(modal.value || '').trim()) { set({ err: t('IP oder CIDR erforderlich') }); return }
    setBusy(true)
    try {
      const body = { kind: modal.kind, value: modal.value.trim(), note: modal.note }
      if (modal.ttl !== 'keep') body.ttl_seconds = Number(modal.ttl) || 0
      await api(modal.id ? '/iplist/' + modal.id : '/iplist', { method: modal.id ? 'PUT' : 'POST', body: JSON.stringify(body) })
      setModal(null)
      load()
    } catch (e) {
      set({ err: e.message })
    } finally {
      setBusy(false)
    }
  }

  const del = async (id) => {
    if (!confirm(t('Eintrag löschen?'))) return
    try { await api('/iplist/' + id, { method: 'DELETE' }); load() } catch (e) { setErr(e.message) }
  }

  if (err) return <Alert variant="error">{err}</Alert>
  if (!entries) return <Spinner />

  const allow = entries.filter(e => e.kind === 'allow')
  const deny = entries.filter(e => e.kind === 'deny')
  const listName = (kind) => (kind === 'allow' ? t('Allowlist') : t('Blocklist'))

  return (
    <div className="space-y-4 max-w-6xl">
      <Alert variant="info">
        <b>{t('Allowlist')}</b> {t('überspringt CrowdSec- und GeoIP-Sperren (z.B. für dein Büro/VPN), aber NICHT den per-Site Login.')}
        <b> {t('Blocklist')}</b> {t('sperrt IPs sofort und ohne Rückmeldung – hat Vorrang vor der Allowlist.')}
        <div className="mt-1 text-xs text-slate-500">{t('Einträge können mit Ablauf hinterlegt werden; abgelaufene Sperren werden automatisch entfernt. Automatische 404-Sperren eskalieren: zuerst kurz, dann 24h, danach dauerhaft.')}</div>
      </Alert>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <ListCard title={t('Allowlist (Whitelist)')} subtitle={t('Immer erlaubte IPs / CIDRs')} kind="allow" entries={allow} onAdd={openAdd} onEdit={openEdit} onDelete={del} />
        <ListCard title={t('Blocklist (Blacklist)')} subtitle={t('Immer gesperrte IPs / CIDRs')} kind="deny" entries={deny} onAdd={openAdd} onEdit={openEdit} onDelete={del} />
      </div>

      {modal && (
        <Modal open onClose={() => setModal(null)}
          title={(modal.id ? t('Eintrag bearbeiten') : t('Neuer Eintrag') + ' – ' + listName(modal.kind))}
          footer={<>
            <Button variant="secondary" onClick={() => setModal(null)}>{t('Abbrechen')}</Button>
            <Button onClick={save} disabled={busy}>{busy ? t('Speichern...') : t('Speichern')}</Button>
          </>}>
          <div className="space-y-4">
            {modal.err && <Alert variant="error">{modal.err}</Alert>}
            <Field label={t('IP oder CIDR')} hint={t('z. B. 203.0.113.5 oder 203.0.113.0/24')}>
              <Input value={modal.value} onChange={e => set({ value: e.target.value })} placeholder="203.0.113.5" autoFocus />
            </Field>
            <Field label={t('Notiz')}>
              <Input value={modal.note} onChange={e => set({ note: e.target.value })} placeholder={t('Grund, Kunde, System...')} />
            </Field>
            <Field label={t('Ablauf')} hint={t('Abgelaufene Einträge werden automatisch entfernt.')}>
              <Select value={modal.ttl} onChange={e => set({ ttl: e.target.value })}>
                {modal.id && <option value="keep">{t('unverändert')}</option>}
                {TTLS.map(o => <option key={o.v} value={o.v}>{t(o.label)}</option>)}
              </Select>
            </Field>
          </div>
        </Modal>
      )}
    </div>
  )
}
