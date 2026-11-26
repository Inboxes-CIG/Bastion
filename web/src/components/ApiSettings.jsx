import { useEffect, useState } from 'react'
import { api, getUser } from '../api.js'
import { Card, Button, Input, Field, Modal, Table, Alert, Spinner, Badge } from '../components/UI.jsx'
import Icon from '../components/Icon.jsx'
import { t } from '../i18n.js'

function keyStatus(k) {
  if (k.revoked_at) return { label: t('widerrufen'), variant: 'slate' }
  if (k.expires_at && new Date(k.expires_at) < new Date()) return { label: t('abgelaufen'), variant: 'red' }
  return { label: t('aktiv'), variant: 'green' }
}

export default function ApiSettings() {
  const me = getUser()
  const isSuper = me?.role === 'superadmin'

  const [keys, setKeys] = useState(null)
  const [err, setErr] = useState('')
  const [msg, setMsg] = useState('')
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState({ name: '', expires: '', rpm: '' })
  const [created, setCreated] = useState(null)
  const [busy, setBusy] = useState(false)

  const [ipData, setIpData] = useState(null)
  const [rpmDefault, setRpmDefault] = useState(0)
  const [ipOpen, setIpOpen] = useState(false)
  const [ipEdit, setIpEdit] = useState(null)
  const [ipForm, setIpForm] = useState({ value: '', note: '' })

  const baseUrl = (typeof window !== 'undefined' ? window.location.origin : '') + '/api/v1'

  const loadKeys = () => api(isSuper ? '/admin/api-keys' : '/auth/api-keys')
    .then(d => setKeys(d.data || []))
    .catch(e => setErr(e.message))

  const loadAdmin = () => {
    if (!isSuper) return
    api('/admin/api-ip-rules').then(d => setIpData(d.data)).catch(() => {})
    api('/admin/api-settings').then(d => setRpmDefault(d.data.default_rate_limit_rpm || 0)).catch(() => {})
  }

  useEffect(() => { loadKeys(); loadAdmin() }, [])

  const createKey = async () => {
    setBusy(true); setErr('')
    try {
      const body = { name: form.name.trim() }
      if (form.expires) body.expires_at = new Date(form.expires + 'T23:59:59').toISOString()
      if (form.rpm !== '') body.rate_limit_rpm = Number(form.rpm)
      const r = await api('/auth/api-keys', { method: 'POST', body: JSON.stringify(body) })
      setCreated(r.data)
      setOpen(false)
      setForm({ name: '', expires: '', rpm: '' })
      loadKeys()
    } catch (e) { setErr(e.message) } finally { setBusy(false) }
  }

  const revoke = async (id) => {
    if (!confirm(t('Key widerrufen?'))) return
    try { await api('/auth/api-keys/' + id, { method: 'DELETE' }); loadKeys() } catch (e) { setErr(e.message) }
  }

  const openIpNew = () => { setIpEdit(null); setIpForm({ value: '', note: '' }); setIpOpen(true) }
  const openIpEdit = (r) => { setIpEdit(r.id); setIpForm({ value: r.value, note: r.note || '' }); setIpOpen(true) }
  const saveIp = async () => {
    setErr('')
    try {
      const body = { value: ipForm.value.trim(), note: ipForm.note }
      if (ipEdit) await api('/admin/api-ip-rules/' + ipEdit, { method: 'PUT', body: JSON.stringify(body) })
      else await api('/admin/api-ip-rules', { method: 'POST', body: JSON.stringify(body) })
      setIpOpen(false); loadAdmin()
    } catch (e) { setErr(e.message) }
  }
  const delIp = async (id) => {
    if (!confirm(t('Eintrag löschen?'))) return
    try { await api('/admin/api-ip-rules/' + id, { method: 'DELETE' }); loadAdmin() } catch (e) { setErr(e.message) }
  }
  const saveRpm = async () => {
    setMsg('')
    try {
      await api('/admin/api-settings', { method: 'PUT', body: JSON.stringify({ default_rate_limit_rpm: Number(rpmDefault) || 0 }) })
      setMsg(t('Gespeichert.'))
    } catch (e) { setMsg('Fehler: ' + e.message) }
  }

  if (err && !open && !ipOpen) return <Alert variant="error">{err}</Alert>
  if (!keys) return <Spinner />

  const rules = ipData?.rules || []
  const currentIp = ipData?.current_ip || '—'

  return (
    <div className="space-y-6">
      <Alert variant="info">
        {t('API-Basis-URL')}: <code className="font-mono text-xs bg-white/60 px-1.5 py-0.5 rounded">{baseUrl}</code>
        <div className="mt-1 text-xs text-slate-600">{t('Authentifizierung per Header:')} <code className="font-mono">Authorization: Bearer bsk_…</code></div>
      </Alert>

      <Card title={t('API-Keys')} subtitle={t('Für Skripte und Automatisierung – Rechte entsprechen deiner Rolle.')}>
        <div className="flex justify-end mb-2">
          <Button onClick={() => { setErr(''); setOpen(true) }}><Icon name="plus" className="w-4 h-4" stroke /> {t('Neuer Key')}</Button>
        </div>
        {keys.length === 0 ? (
          <div className="text-sm text-slate-400 py-6 text-center">{t('Keine Keys.')}</div>
        ) : (
          <Table
            columns={[t('Name'), isSuper ? t('Benutzer') : null, 'Prefix', t('Erstellt'), t('Zuletzt benutzt'), t('Ablauf'), t('Status'), ''].filter(Boolean)}
            rows={keys}
            renderRow={k => {
              const st = keyStatus(k)
              return (
                <tr key={k.id} className="border-b border-slate-100">
                  <td className="px-4 py-2 text-sm font-medium text-slate-800">{k.name}</td>
                  {isSuper && <td className="px-4 py-2 text-xs text-slate-600">{k.email || '—'}</td>}
                  <td className="px-4 py-2 text-xs font-mono text-slate-500">{k.prefix}…</td>
                  <td className="px-4 py-2 text-xs text-slate-500">{new Date(k.created_at).toLocaleDateString()}</td>
                  <td className="px-4 py-2 text-xs text-slate-500">{k.last_used_at ? new Date(k.last_used_at).toLocaleString() : '—'}</td>
                  <td className="px-4 py-2 text-xs text-slate-500">{k.expires_at ? new Date(k.expires_at).toLocaleDateString() : t('unbegrenzt')}</td>
                  <td className="px-4 py-2"><Badge variant={st.variant}>{st.label}</Badge></td>
                  <td className="px-4 py-2 text-right">
                    {!k.revoked_at && (
                      <button onClick={() => revoke(k.id)} className="p-1.5 text-slate-400 hover:text-red-600" title={t('Widerrufen')}>
                        <Icon name="trash" className="w-4 h-4" stroke />
                      </button>
                    )}
                  </td>
                </tr>
              )
            }}
          />
        )}
      </Card>

      {isSuper ? (
        <>
          <Card title={t('API-IP-Allowlist')} subtitle={t('Nur diese IPs/CIDRs dürfen API-Keys verwenden. Leer = nur Loopback (127.0.0.1).')}>
            {rules.length === 0 && <Alert variant="warn">{t('Leer = nur Loopback (127.0.0.1) erlaubt.')}</Alert>}
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 mb-2">
              <div className="text-xs text-slate-500">{t('Aktuelle IP:')} <code className="font-mono">{currentIp}</code></div>
              <div className="flex gap-2">
                <Button variant="secondary" onClick={() => { setIpEdit(null); setIpForm({ value: currentIp, note: t('Aktuelle Admin-IP') }); setIpOpen(true) }}>{t('Meine aktuelle IP übernehmen')}</Button>
                <Button onClick={openIpNew}><Icon name="plus" className="w-4 h-4" stroke /> {t('Hinzufügen')}</Button>
              </div>
            </div>
            {rules.length === 0 ? (
              <div className="text-sm text-slate-400 py-4 text-center">{t('Noch keine Einträge.')}</div>
            ) : (
              <div className="divide-y divide-slate-100">
                {rules.map(r => (
                  <div key={r.id} className="flex items-center justify-between py-2">
                    <div>
                      <div className="font-mono text-sm text-slate-800">{r.value}</div>
                      {r.note && <div className="text-xs text-slate-400">{r.note}</div>}
                    </div>
                    <div className="flex items-center">
                      <button onClick={() => openIpEdit(r)} className="p-1.5 text-slate-400 hover:text-indigo-600"><Icon name="edit" className="w-4 h-4" stroke /></button>
                      <button onClick={() => delIp(r.id)} className="p-1.5 text-slate-400 hover:text-red-600"><Icon name="trash" className="w-4 h-4" stroke /></button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </Card>

          <Card title={t('Standard-Rate-Limit')} subtitle={t('Gilt für Keys ohne eigenen Wert. 0 = deaktiviert.')}>
            <div className="flex flex-col sm:flex-row sm:items-end gap-3">
              <Field label={t('Anfragen pro Minute')} className="sm:w-56">
                <Input type="number" min="0" value={rpmDefault} onChange={e => setRpmDefault(e.target.value)} />
              </Field>
              <Button onClick={saveRpm}>{t('Speichern')}</Button>
              {msg && <span className={`text-sm ${msg.startsWith('Fehler') ? 'text-red-600' : 'text-emerald-600'}`}>{msg}</span>}
            </div>
          </Card>
        </>
      ) : (
        <Alert variant="info">{t('Die API-IP-Allowlist wird von einem Superadmin verwaltet.')}</Alert>
      )}

      <Modal open={open} onClose={() => setOpen(false)} title={t('Neuer Key')}
        footer={<><Button variant="secondary" onClick={() => setOpen(false)}>{t('Abbrechen')}</Button><Button onClick={createKey} disabled={busy || !form.name.trim()}>{busy ? t('Speichern...') : t('Erstellen')}</Button></>}>
        <div className="space-y-4">
          <Field label={t('Name')}><Input value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} placeholder="CI / Backup / Monitoring" autoFocus /></Field>
          <Field label={t('Ablauf (optional)')}><Input type="date" value={form.expires} onChange={e => setForm({ ...form, expires: e.target.value })} /></Field>
          <Field label={t('Rate-Limit (Anfragen/Minute, leer = Default)')}><Input type="number" min="0" value={form.rpm} onChange={e => setForm({ ...form, rpm: e.target.value })} placeholder={String(rpmDefault || 0)} /></Field>
        </div>
      </Modal>

      {created && (
        <Modal open onClose={() => setCreated(null)} title={t('Key erstellt')}
          footer={<Button onClick={() => setCreated(null)}>{t('Fertig')}</Button>}>
          <div className="space-y-3">
            <Alert variant="warn">{t('Der Key wird nur einmal angezeigt – jetzt kopieren.')}</Alert>
            <code className="block break-all rounded-lg bg-slate-100 px-3 py-2 font-mono text-xs text-slate-800">{created.key}</code>
            <div className="flex gap-2">
              <Button variant="secondary" onClick={() => { navigator.clipboard?.writeText(created.key) }}>{t('Kopieren')}</Button>
            </div>
            <div className="text-xs text-slate-500">{t('Beispiel')}:</div>
            <code className="block break-all rounded-lg bg-slate-900 px-3 py-2 font-mono text-[11px] text-slate-100">curl -fsS "{baseUrl}/sites" -H "Authorization: Bearer {created.key}"</code>
          </div>
        </Modal>
      )}

      <Modal open={ipOpen} onClose={() => setIpOpen(false)} title={ipEdit ? t('Eintrag bearbeiten') : t('Neuer Eintrag')}
        footer={<><Button variant="secondary" onClick={() => setIpOpen(false)}>{t('Abbrechen')}</Button><Button onClick={saveIp} disabled={!ipForm.value.trim()}>{t('Speichern')}</Button></>}>
        <div className="space-y-4">
          <Field label={t('IP oder CIDR')} hint={t('z. B. 203.0.113.5 oder 203.0.113.0/24')}>
            <Input value={ipForm.value} onChange={e => setIpForm({ ...ipForm, value: e.target.value })} placeholder="203.0.113.5" autoFocus />
          </Field>
          <Field label={t('Notiz')}><Input value={ipForm.note} onChange={e => setIpForm({ ...ipForm, note: e.target.value })} /></Field>
        </div>
      </Modal>
    </div>
  )
}
