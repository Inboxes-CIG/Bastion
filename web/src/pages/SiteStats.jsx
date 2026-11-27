import { useEffect, useState } from 'react'
import { api } from '../api.js'
import { Card, Select, Field, Input, Alert, Spinner } from '../components/UI.jsx'
import DomainList from '../components/DomainList.jsx'
import { t } from '../i18n.js'

const RANGES = [
  { v: '24h', label: 'Letzte 24 Stunden' },
  { v: '7d', label: 'Letzte 7 Tage' },
  { v: '30d', label: 'Letzte 30 Tage' },
  { v: 'all', label: 'Gesamt' },
]

const STATUS_ORDER = ['2xx', '3xx', '4xx', '5xx', 'other']

function Rows({ title, rows }) {
  if (!rows || rows.length === 0) return null
  return (
    <div className="mt-3">
      <div className="text-xs text-slate-500 mb-1">{title}</div>
      <div className="rounded-lg border border-slate-100 overflow-hidden">
        {rows.map(([label, value], i) => (
          <div key={label} className={`flex items-center justify-between px-2.5 py-1.5 text-xs ${i % 2 === 0 ? 'bg-white' : 'bg-slate-50'}`}>
            <span className="text-slate-600">{label}</span>
            <span className="font-mono text-slate-700">{value}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

export default function SiteStats() {
  const [range, setRange] = useState('24h')
  const [q, setQ] = useState('')
  const [data, setData] = useState(null)
  const [err, setErr] = useState('')

  useEffect(() => {
    let alive = true
    setData(null)
    setErr('')
    api('/stats/sites?range=' + range)
      .then(d => { if (alive) setData(d.data) })
      .catch(e => { if (alive) setErr(e.message) })
    return () => { alive = false }
  }, [range])

  if (err) return <Alert variant="error">{err}</Alert>

  const needle = q.trim().toLowerCase()
  const sites = (data?.sites || []).filter(s => {
    if (!needle) return true
    if (s.domain.toLowerCase().includes(needle)) return true
    return (s.extra_domains || []).some(d => d.toLowerCase().includes(needle))
  })

  return (
    <div className="space-y-4 max-w-7xl">
      <div className="flex flex-col sm:flex-row sm:items-end gap-3">
        <Field label={t('Zeitraum')} className="sm:w-64">
          <Select value={range} onChange={e => setRange(e.target.value)}>
            {RANGES.map(r => <option key={r.v} value={r.v}>{t(r.label)}</option>)}
          </Select>
        </Field>
        <div className="flex-1" />
        {data && <div className="text-xs text-slate-400 pb-2.5">{sites.length} / {data.sites.length} {t('Sites')}</div>}
        <Field label={t('Suche')} className="sm:w-80">
          <Input value={q} onChange={e => setQ(e.target.value)} placeholder={t('Site oder Alias-Domain suchen...')} />
        </Field>
      </div>
      <div className="text-xs text-slate-400 -mt-2">{t('Gesamt = gesamter Aufbewahrungszeitraum der Logs.')}</div>

      {!data ? (
        <Spinner />
      ) : sites.length === 0 ? (
        <Card><div className="text-sm text-slate-400 py-8 text-center">{data.sites.length === 0 ? t('Keine Sites vorhanden.') : t('Keine Treffer für die Suche.')}</div></Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {sites.map(s => {
            const statusRows = STATUS_ORDER.filter(k => s.by_status && s.by_status[k]).map(k => [k, s.by_status[k]])
            const reasonRows = Object.entries(s.by_reason || {}).sort((a, b) => b[1] - a[1])
            return (
              <Card key={s.id}
                title={<DomainList domain={s.domain} aliases={s.extra_domains||[]} />}
                subtitle={(!s.extra_domains || s.extra_domains.length === 0) && s.total === 0 ? t('Keine Daten im Zeitraum.') : undefined}>
                <div className="grid grid-cols-3 gap-2 mb-2">
                  <div className="rounded-lg bg-slate-50 p-2 text-center">
                    <div className="text-lg font-bold text-slate-800">{s.total}</div>
                    <div className="text-[11px] text-slate-500">{t('Zugriffe')}</div>
                  </div>
                  <div className="rounded-lg bg-rose-50 p-2 text-center">
                    <div className="text-lg font-bold text-rose-700">{s.blocked}</div>
                    <div className="text-[11px] text-slate-500">{t('Blockiert')}</div>
                  </div>
                  <div className="rounded-lg bg-amber-50 p-2 text-center">
                    <div className="text-lg font-bold text-amber-700">{s.block_rate}%</div>
                    <div className="text-[11px] text-slate-500">{t('Blockrate')}</div>
                  </div>
                </div>

                <Rows title={t('Status')} rows={statusRows} />
                <Rows title={t('Blockgründe')} rows={reasonRows} />
              </Card>
            )
          })}
        </div>
      )}
    </div>
  )
}
