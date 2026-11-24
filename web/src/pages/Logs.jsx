import { useEffect, useRef, useState } from 'react'
import { api } from '../api.js'
import { Card, Button, Input, Select, Badge, Spinner, Alert, EmptyState } from '../components/UI.jsx'
import Icon from '../components/Icon.jsx'
import { t } from '../i18n.js'

function statusVariant(code) {
  if (code >= 500) return 'red'
  if (code >= 400) return 'amber'
  if (code >= 300) return 'blue'
  return 'green'
}

export default function Logs() {
  const [sites, setSites] = useState([])
  const [files, setFiles] = useState([])
  const [entries, setEntries] = useState(null)
  const [err, setErr] = useState('')
  const [site, setSite] = useState('')
  const [file, setFile] = useState('')
  const [blocked, setBlocked] = useState('')
  const [q, setQ] = useState('')
  const [live, setLive] = useState(true)
  const [hideAdmin, setHideAdmin] = useState(false)
  const timer = useRef(null)

  useEffect(() => {
    api('/sites').then(d=>setSites(d.data||[])).catch(()=>{})
    api('/logs/files').then(d=>setFiles(d.data||[])).catch(()=>{})
  }, [])

  const load = () => {
    const p = new URLSearchParams()
    if (site) p.set('site', site)
    if (blocked) p.set('blocked', blocked)
    if (q) p.set('q', q)
    if (hideAdmin) p.set('hide_admin', 'true')
    if (file) p.set('file', file)
    p.set('limit', '300')
    api('/logs?'+p.toString()).then(d=>setEntries(d.data||[])).catch(e=>setErr(e.message))
  }

  useEffect(() => {
    load()
    if (timer.current) clearInterval(timer.current)
    if (live) timer.current = setInterval(load, 5000)
    return () => timer.current && clearInterval(timer.current)
  }, [site, blocked, q, live, file, hideAdmin])

  const refreshFiles = () => api('/logs/files').then(d=>setFiles(d.data||[])).catch(()=>{})
  const fmtTime = (value) => new Date(value).toLocaleString(undefined, { hour12: false })

  return (
    <div className="space-y-4 max-w-7xl">
      {err && <Alert variant="error">{err}</Alert>}

      <div className="flex flex-col xl:flex-row xl:items-end gap-3">
        <div className="flex-1 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
          <div>
            <label className="text-xs text-slate-500 mb-1 block">{t('Datei')}</label>
            <Select value={file} onChange={e=>{setFile(e.target.value); refreshFiles()}}>
              <option value="">{t('Live (Speicher)')}</option>
              {files.map(f=><option key={f.name} value={f.name}>{f.name.replace('access-','').replace('.jsonl','')}</option>)}
            </Select>
          </div>
          <div>
            <label className="text-xs text-slate-500 mb-1 block">{t('Site')}</label>
            <Select value={site} onChange={e=>setSite(e.target.value)}>
              <option value="">{t('Alle Sites')}</option>
              <option value="(admin)">{t('(admin – GUI)')}</option>
              <option value="(unknown)">{t('(unknown – fremde Hosts)')}</option>
              {sites.map(s=><option key={s.id} value={s.domain}>{s.domain}</option>)}
            </Select>
          </div>
          <div>
            <label className="text-xs text-slate-500 mb-1 block">{t('Typ')}</label>
            <Select value={blocked} onChange={e=>setBlocked(e.target.value)}>
              <option value="">{t('Alle')}</option>
              <option value="true">{t('Nur geblockt')}</option>
              <option value="false">{t('Nur erlaubt')}</option>
            </Select>
          </div>
          <div>
            <label className="text-xs text-slate-500 mb-1 block">{t('Suche')}</label>
            <Input value={q} onChange={e=>setQ(e.target.value)} placeholder={t('IP, Pfad, Host...')} />
          </div>
        </div>
        <div className="flex items-center gap-2">
          <label className="flex items-center gap-2 text-sm text-slate-600 mr-2">
            <input type="checkbox" checked={hideAdmin} onChange={e=>setHideAdmin(e.target.checked)} /> {t('Bastion ausblenden')}
          </label>
          <Button variant={live ? 'success' : 'secondary'} onClick={()=>setLive(!live)}>
            <span className={`w-2 h-2 rounded-full ${live?'bg-white':'bg-slate-400'}`}></span> {live ? 'Live' : t('Pausiert')}
          </Button>
          <Button variant="secondary" onClick={load}><Icon name="refresh" className="w-4 h-4" stroke /> {t('Refresh')}</Button>
        </div>
      </div>

      <Card className="p-0">
        {entries === null ? <Spinner /> : entries.length === 0 ? (
          <EmptyState icon="clock" title={t('Keine Log-Einträge')} description={t('Sobald Traffic über den Proxy läuft, erscheinen hier Einträge.')} />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-200 bg-slate-50/70 text-left text-slate-600">
                  <th className="px-3 py-2.5 font-semibold whitespace-nowrap">{t('Zeit')}</th>
                  <th className="px-3 py-2.5 font-semibold">Host / {t('Site')}</th>
                  <th className="px-3 py-2.5 font-semibold">Client IP</th>
                  <th className="px-3 py-2.5 font-semibold">{t('Methode')}</th>
                  <th className="px-3 py-2.5 font-semibold">{t('Pfad')}</th>
                  <th className="px-3 py-2.5 font-semibold">{t('Status')}</th>
                  <th className="px-3 py-2.5 font-semibold">Block</th>
                  <th className="px-3 py-2.5 font-semibold text-right">{t('Dauer')}</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((e, i) => (
                  <tr key={i} className={`border-b border-slate-100 ${e.blocked ? 'bg-rose-50/40' : 'hover:bg-slate-50'}`}>
                    <td className="px-3 py-2 text-slate-500 whitespace-nowrap text-xs">{fmtTime(e.time)}</td>
                    <td className="px-3 py-2">
                      <div className="font-medium text-slate-800">{e.domain || e.host || '—'}{e.admin && <span className="ml-1 text-[10px] uppercase text-indigo-500">admin</span>}</div>
                      {e.domain && e.host && e.domain !== e.host && <div className="text-xs text-slate-400">{e.host}</div>}
                    </td>
                    <td className="px-3 py-2 font-mono text-xs text-slate-600">{e.client_ip}</td>
                    <td className="px-3 py-2 text-xs text-slate-500">{e.method}</td>
                    <td className="px-3 py-2 text-slate-600 max-w-xs truncate" title={e.path}>{e.path}</td>
                    <td className="px-3 py-2"><Badge variant={statusVariant(e.status)}>{e.status}</Badge></td>
                    <td className="px-3 py-2">{e.blocked ? <Badge variant="red">{e.block_reason || t('blocked')}</Badge> : <span className="text-slate-300">—</span>}</td>
                    <td className="px-3 py-2 text-right text-xs text-slate-500">{e.duration_ms} ms</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
      <div className="text-xs text-slate-400">
        {t('Persistente Logs:')} <code className="font-mono">/var/lib/bastion/logs/access-YYYY-MM-DD.jsonl</code> ({t('tägliche Rotation, JSON Lines')}).
        {t('Live-Ansicht ist ein In-Memory-Puffer (letzte 10.000 Einträge).')}
      </div>
    </div>
  )
}
