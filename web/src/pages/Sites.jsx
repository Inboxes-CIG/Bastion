import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api.js'
import { Card, Button, Badge, Table, Spinner, Alert, EmptyState } from '../components/UI.jsx'
import DomainList from '../components/DomainList.jsx'
import Icon from '../components/Icon.jsx'
import { t } from '../i18n.js'

function statusBadge(s) {
  if (!s.enabled) return <Badge variant="slate">{t('Deaktiviert')}</Badge>
  return <Badge variant="green"><span className="w-1.5 h-1.5 rounded-full bg-emerald-500"></span>{t('aktiv')}</Badge>
}

export default function Sites() {
  const [sites, setSites] = useState(null)
  const [templates, setTemplates] = useState([])
  const [q, setQ] = useState('')
  const [err, setErr] = useState('')
  const [cacheMsg, setCacheMsg] = useState('')
  const nav = useNavigate()

  const load = () => {
    api('/sites').then(d => setSites(d.data||[])).catch(e => setErr(e.message))
    api('/templates').then(d => setTemplates(d.data||[])).catch(()=>{})
  }
  useEffect(() => { load() }, [])

  const clearSite = async (e, id) => {
    e.stopPropagation()
    try {
      await api('/cache/clear', { method: 'POST', body: JSON.stringify({ site_id: id }) })
      setCacheMsg(t('Site-Cache geleert.'))
      setTimeout(() => setCacheMsg(''), 2500)
    } catch (err) { setCacheMsg('Fehler: ' + err.message) }
  }

  const clearCache = async () => {
    try {
      await api('/cache/clear', { method: 'POST' })
      setCacheMsg(t('Cache geleert.'))
      setTimeout(() => setCacheMsg(''), 2500)
    } catch (e) { setCacheMsg('Fehler: ' + e.message) }
  }

  const del = async (e, id) => {
    e.stopPropagation()
    if (!confirm(t('Site wirklich löschen?'))) return
    await api('/sites/'+id, { method: 'DELETE' })
    load()
  }

  const filtered = (sites||[]).filter(s =>
    !q || s.domain.toLowerCase().includes(q.toLowerCase()) ||
    (s.extra_domains||[]).some(d => d.toLowerCase().includes(q.toLowerCase()))
  )

  if (err) return <Alert variant="error">{err}</Alert>
  if (!sites) return <Spinner />

  return (
    <div className="space-y-4 max-w-7xl">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div className="relative">
          <Icon name="search" className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" stroke />
          <input value={q} onChange={e=>setQ(e.target.value)} placeholder={t('Sites durchsuchen...')}
            className="pl-9 pr-3 py-2 w-full sm:w-72 rounded-lg border border-slate-300 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500/30 focus:border-indigo-500" />
        </div>
        <div className="flex items-center gap-2">
          {cacheMsg && <span className="text-sm text-emerald-600">{cacheMsg}</span>}
          <Button variant="secondary" onClick={clearCache}><Icon name="trash" className="w-4 h-4" stroke /> {t('Cache leeren')}</Button>
          <Button onClick={() => nav('/sites/new')}><Icon name="plus" className="w-4 h-4" stroke /> {t('Neue Site')}</Button>
        </div>
      </div>

      <Card className="p-0">
        {filtered.length === 0 ? (
          <EmptyState icon="sites" title={t('Keine Sites')} description={t('Erstelle deine erste Site, um Traffic zu proxien.')}
            action={<Button onClick={()=>nav('/sites/new')}><Icon name="plus" className="w-4 h-4" stroke /> {t('Neue Site')}</Button>} />
        ) : (
          <Table
            columns={[t('Domain'), t('Template'), t('Upstream'), t('Status'), 'HTTP/2', 'HTTP/3', 'GeoIP', '']}
            rows={filtered}
            renderRow={s => (
              <tr key={s.id} onClick={() => nav('/sites/'+s.id)} className="border-b border-slate-100 hover:bg-slate-50 cursor-pointer">
                <td className="px-4 py-3 max-w-[280px]">
                  <DomainList domain={s.domain} aliases={s.extra_domains||[]} />
                </td>
                <td className="px-4 py-3 text-slate-600">{s.template_id ? (templates.find(t=>t.id===s.template_id)?.name || '?') : <span className="text-slate-400">—</span>}</td>
                <td className="px-4 py-3 text-slate-600 text-xs">{s.upstream_url || <span className="text-slate-400">{t('inherited')}</span>}</td>
                <td className="px-4 py-3">{statusBadge(s)}</td>
                <td className="px-4 py-3 text-xs text-slate-500">{s.http2_enabled===null?t('inherit'):s.http2_enabled?t('an'):t('aus')}</td>
                <td className="px-4 py-3 text-xs text-slate-500">{s.http3_enabled===null?t('inherit'):s.http3_enabled?t('an'):t('aus')}</td>
                <td className="px-4 py-3 text-xs text-slate-500">{s.geoip_enabled===null?t('inherit'):s.geoip_enabled?t('an'):t('aus')}</td>
                <td className="px-4 py-3 text-right">
                  <button onClick={(e)=>clearSite(e, s.id)} title={t('Cache dieser Site leeren')} className="p-1.5 text-slate-400 hover:text-amber-600"><Icon name="refresh" className="w-4 h-4" stroke /></button>
                  <button onClick={(e)=>{e.stopPropagation(); nav('/sites/'+s.id)}} className="p-1.5 text-slate-400 hover:text-indigo-600"><Icon name="edit" className="w-4 h-4" stroke /></button>
                  <button onClick={(e)=>del(e, s.id)} className="p-1.5 text-slate-400 hover:text-red-600"><Icon name="trash" className="w-4 h-4" stroke /></button>
                </td>
              </tr>
            )}
          />
        )}
      </Card>
    </div>
  )
}
