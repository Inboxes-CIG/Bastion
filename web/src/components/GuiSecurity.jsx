import { useEffect, useState } from 'react'
import { api } from '../api.js'
import { Card, Alert, Spinner, Button, Input, Select, Field, Toggle, Modal, MultiSelect } from '../components/UI.jsx'
import Icon from '../components/Icon.jsx'
import { t } from '../i18n.js'

function RuleList({ title, kind, entries, onAdd, onEdit, onDelete }) {
  return (
    <div className="rounded-lg border border-slate-200 p-4">
      <div className="flex items-center justify-between mb-2">
        <div className="text-sm font-medium text-slate-700">{title}</div>
        <Button variant="secondary" className="!px-2.5 !py-1.5 text-xs" onClick={() => onAdd(kind)}>
          <Icon name="plus" className="w-3.5 h-3.5" stroke /> {t('Hinzufügen')}
        </Button>
      </div>
      {entries.length === 0 ? (
        <div className="text-sm text-slate-400 py-4 text-center">{t('Noch keine Einträge.')}</div>
      ) : (
        <div className="divide-y divide-slate-100">
          {entries.map(e => (
            <div key={e.id} className="flex items-center justify-between py-2">
              <div>
                <div className="font-mono text-sm text-slate-800">{e.value}</div>
                {e.note && <div className="text-xs text-slate-400">{e.note}</div>}
              </div>
              <div className="flex items-center">
                <button onClick={() => onEdit(e)} className="p-1.5 text-slate-400 hover:text-indigo-600"><Icon name="edit" className="w-4 h-4" stroke /></button>
                <button onClick={() => onDelete(e.id)} className="p-1.5 text-slate-400 hover:text-red-600"><Icon name="trash" className="w-4 h-4" stroke /></button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

export default function GuiSecurity() {
  const [geo, setGeo] = useState(null)
  const [countries, setCountries] = useState([])
  const [rules, setRules] = useState(null)
  const [err, setErr] = useState('')
  const [geoMsg, setGeoMsg] = useState('')
  const [busy, setBusy] = useState(false)
  const [modal, setModal] = useState(null)
  const [modalBusy, setModalBusy] = useState(false)

  const load = () => {
    api('/admin/geo').then(d => setGeo(d.data || {})).catch(e => setErr(e.message))
    api('/admin/ip-rules').then(d => setRules(d.data || [])).catch(e => setErr(e.message))
    api('/geoip/countries').then(d => setCountries(d.data || [])).catch(() => {})
  }
  useEffect(load, [])

  const saveGeo = async () => {
    setGeoMsg(''); setBusy(true)
    try {
      const r = await api('/admin/geo', { method: 'PUT', body: JSON.stringify({ enabled: !!geo.enabled, mode: geo.mode || 'block', countries: geo.countries || [] }) })
      setGeo(r.data); setGeoMsg(t('Gespeichert.'))
    } catch (e) { setGeoMsg('Fehler: ' + e.message) } finally { setBusy(false) }
  }

  const openAdd = (kind) => setModal({ kind, id: null, value: '', note: '', err: '' })
  const openEdit = (e) => setModal({ kind: e.kind, id: e.id, value: e.value, note: e.note || '', err: '' })
  const set = (patch) => setModal(m => ({ ...m, ...patch }))

  const saveRule = async () => {
    if (!(modal.value || '').trim()) { set({ err: t('IP oder CIDR erforderlich') }); return }
    setModalBusy(true)
    try {
      const body = { kind: modal.kind, value: modal.value.trim(), note: modal.note }
      await api(modal.id ? '/admin/ip-rules/' + modal.id : '/admin/ip-rules', { method: modal.id ? 'PUT' : 'POST', body: JSON.stringify(body) })
      setModal(null); load()
    } catch (e) { set({ err: e.message }) } finally { setModalBusy(false) }
  }

  const del = async (id) => {
    if (!confirm(t('Eintrag löschen?'))) return
    try { await api('/admin/ip-rules/' + id, { method: 'DELETE' }); load() } catch (e) { setErr(e.message) }
  }

  if (err) return <Alert variant="error">{err}</Alert>
  if (!geo || !rules) return <Spinner />

  const allow = rules.filter(r => r.kind === 'allow')
  const deny = rules.filter(r => r.kind === 'deny')

  return (
    <div className="space-y-6">
      <Alert variant="warn">{t('Diese Regeln gelten nur für die Admin-GUI (nicht für deine Sites). Loopback (127.0.0.1) ist immer erlaubt, damit du dich nicht aussperren kannst.')}</Alert>

      <Card title={t('GeoIP für die GUI')} subtitle={t('Länderfilter für die Admin-Oberfläche')}>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 items-start">
          <Field label={t('GeoIP aktiv')}>
            <div className="pt-2"><Toggle checked={!!geo.enabled} onChange={v => setGeo({ ...geo, enabled: v })} label={geo.enabled ? t('an') : t('aus')} /></div>
          </Field>
          <Field label={t('Modus')}>
            <Select value={geo.mode || 'block'} onChange={e => setGeo({ ...geo, mode: e.target.value })}>
              <option value="block">{t('Diese Länder sperren')}</option>
              <option value="allow">{t('Nur diese Länder erlauben')}</option>
            </Select>
          </Field>
        </div>
        <div className="mt-4">
          <Field label={t('Länder')} hint={t('Leere Liste = keine GeoIP-Einschränkung.')}>
            <MultiSelect options={countries} value={geo.countries || []} onChange={v => setGeo({ ...geo, countries: v })} placeholder={t('Länder auswählen...')} />
          </Field>
        </div>
        <div className="flex items-center gap-2 mt-4">
          <Button onClick={saveGeo} disabled={busy}>{busy ? t('Speichern...') : t('Speichern')}</Button>
          {geoMsg && <span className={`text-sm ${geoMsg.startsWith('Fehler') ? 'text-red-600' : 'text-emerald-600'}`}>{geoMsg}</span>}
        </div>
      </Card>

      <Card title={t('IP-Listen der GUI')} subtitle={t('Allowlist und Blocklist für den Zugriff auf die Admin-Oberfläche')}>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <RuleList title={t('Allowlist (GUI)')} kind="allow" entries={allow} onAdd={openAdd} onEdit={openEdit} onDelete={del} />
          <RuleList title={t('Blocklist (GUI)')} kind="deny" entries={deny} onAdd={openAdd} onEdit={openEdit} onDelete={del} />
        </div>
      </Card>

      {modal && (
        <Modal open onClose={() => setModal(null)}
          title={(modal.id ? t('Eintrag bearbeiten') : t('Neuer Eintrag')) + ' – ' + (modal.kind === 'allow' ? t('Allowlist (GUI)') : t('Blocklist (GUI)'))}
          footer={<>
            <Button variant="secondary" onClick={() => setModal(null)}>{t('Abbrechen')}</Button>
            <Button onClick={saveRule} disabled={modalBusy}>{modalBusy ? t('Speichern...') : t('Speichern')}</Button>
          </>}>
          <div className="space-y-4">
            {modal.err && <Alert variant="error">{modal.err}</Alert>}
            <Field label={t('IP oder CIDR')} hint={t('z. B. 203.0.113.5 oder 203.0.113.0/24')}>
              <Input value={modal.value} onChange={e => set({ value: e.target.value })} placeholder="203.0.113.5" autoFocus />
            </Field>
            <Field label={t('Notiz')}>
              <Input value={modal.note} onChange={e => set({ note: e.target.value })} placeholder={t('Grund, Kunde, System...')} />
            </Field>
          </div>
        </Modal>
      )}
    </div>
  )
}
