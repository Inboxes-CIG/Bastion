import { useEffect, useState } from 'react'
import { api } from '../api.js'
import { Card, Select, Field, Alert, Spinner } from '../components/UI.jsx'
import Icon from '../components/Icon.jsx'
import { t } from '../i18n.js'

const RANGES = [
  { v: '24h', label: 'Letzte 24 Stunden' },
  { v: '7d', label: 'Letzte 7 Tage' },
  { v: '30d', label: 'Letzte 30 Tage' },
  { v: 'all', label: 'Gesamt' },
]

function StatusChips({ byStatus }) {
  const order = ['2xx', '3xx', '4xx', '5xx', 'other']
  const entries = order.filter(k => byStatus && byStatus[k])
  if (entries.length === 0) return null
  return (
    <div className="flex flex-wrap gap-1 mt-1">
      {entries.map(k => (
        <span key={k} className="text-[11px] rounded bg-slate-100 text-slate-600 px-2 py-0.5">{k}: {byStatus[k]}</span>
      ))}
    </div>
  )
}

export default function SiteStats() {
  const [range, setRange] = useState('24h')
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

  return (
    <div className="space-y-4 max-w-7xl">
      <div className="flex flex-col sm:flex-row sm:items-end sm:justify-between gap-3">
        <Field label={t('Zeitraum')} className="sm:w-72" hint={t('Gesamt = gesamter Aufbewahrungszeitraum der Logs.')}>
          <Select value={range} onChange={e => setRange(e.target.value)}>
            {RANGES.map(r => <option key={r.v} value={r.v}>{t(r.label)}</option>)}
          </Select>
        </Field>
        {data && <div className="text-xs text-slate-400">{data.sites.length} {t('Sites')}</div>}
      </div>

      {!data ? (
        <Spinner />
      ) : data.sites.length === 0 ? (
        <Card><div className="text-sm text-slate-400 py-8 text-center">{t('Keine Sites vorhanden.')}</div></Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {data.sites.map(s => {
            const reasons = Object.entries(s.by_reason || {}).sort((a, b) => b[1] - a[1]).slice(0, 5)
            return (
              <Card key={s.id} title={s.domain} subtitle={s.total === 0 ? t('Keine Daten im Zeitraum.') : undefined}>
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

                <div className="text-xs text-slate-500">{t('Status')}</div>
                <StatusChips byStatus={s.by_status} />

                {reasons.length > 0 && (
                  <div className="mt-3">
                    <div className="text-xs text-slate-500 mb-1">{t('Blockgründe')}</div>
                    <div className="space-y-0.5">
                      {reasons.map(([k, v]) => (
                        <div key={k} className="flex items-center justify-between text-xs">
                          <span className="text-slate-600">{k}</span>
                          <span className="font-mono text-slate-700">{v}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </Card>
            )
          })}
        </div>
      )}
    </div>
  )
}
