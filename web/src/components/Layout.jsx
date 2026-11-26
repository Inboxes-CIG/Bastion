import { useEffect, useState } from 'react'
import { Outlet, NavLink, useLocation, useNavigate } from 'react-router-dom'
import Icon from './Icon.jsx'
import ErrorBoundary from './ErrorBoundary.jsx'
import { me, logout, api, changePassword } from '../api.js'
import { t } from '../i18n.js'

const NAV = [
  { to: '/', label: 'Dashboard', icon: 'dashboard', end: true },
  { to: '/sites', label: 'Sites', icon: 'sites' },
  { to: '/sites/stats', label: 'Statistik', icon: 'stats' },
  { to: '/logs', label: 'Logs', icon: 'clock' },
  { to: '/templates', label: 'Templates', icon: 'templates' },
  { to: '/certificates', label: 'Certificates', icon: 'certs' },
  { to: '/crowdsec', label: 'CrowdSec', icon: 'shield', superOnly: true },
  { to: '/iplist', label: 'IP-Listen', icon: 'templates', superOnly: true },
  { to: '/audit', label: 'Audit', icon: 'templates' },
  { to: '/users', label: 'Users', icon: 'users' },
  { to: '/tenants', label: 'Tenants', icon: 'tenants', superOnly: true },
  { to: '/settings', label: 'Settings', icon: 'settings' },
]

function titleFor(path) {
  if (path === '/') return 'Dashboard'
  if (path === '/sites/stats') return 'Statistik'
  if (path.startsWith('/sites')) return 'Sites'
  if (path.startsWith('/logs')) return 'Logs'
  if (path.startsWith('/templates')) return 'Templates'
  if (path.startsWith('/certificates')) return 'Certificates'
  if (path.startsWith('/crowdsec')) return 'CrowdSec'
  if (path.startsWith('/iplist')) return 'IP-Listen'
  if (path.startsWith('/audit')) return 'Audit'
  if (path.startsWith('/users')) return 'Users'
  if (path.startsWith('/tenants')) return 'Tenants'
  if (path.startsWith('/settings')) return 'Settings'
  return 'Bastion'
}

function ForceChangePassword() {
  const [cur, setCur] = useState('')
  const [nw, setNw] = useState('')
  const [nw2, setNw2] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e) => {
    e.preventDefault()
    setErr('')
    if (nw.length < 8) { setErr(t('Das neue Passwort muss mindestens 8 Zeichen haben.')); return }
    if (nw !== nw2) { setErr(t('Die Passwörter stimmen nicht überein.')); return }
    setBusy(true)
    try {
      await changePassword(cur, nw)
      window.location.reload()
    } catch (e) { setErr(e.message) } finally { setBusy(false) }
  }

  return (
    <div className="fixed inset-0 z-[60] bg-slate-900/60 backdrop-blur-sm flex items-center justify-center p-4">
      <form onSubmit={submit} className="bg-white rounded-xl shadow-2xl w-full max-w-md p-6 space-y-4">
        <div>
          <h2 className="text-lg font-bold text-slate-800">{t('Passwort ändern erforderlich')}</h2>
          <p className="text-sm text-slate-500 mt-1">{t('Aus Sicherheitsgründen musst du dein Passwort vor der ersten Nutzung ändern.')}</p>
        </div>
        {err && <div className="rounded-lg bg-red-50 border border-red-200 text-red-700 text-sm px-3 py-2">{err}</div>}
        <label className="block text-sm font-medium text-slate-700">
          {t('Aktuelles Passwort')}
          <input type="password" className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" value={cur} onChange={e=>setCur(e.target.value)} required />
        </label>
        <label className="block text-sm font-medium text-slate-700">
          {t('Neues Passwort')}
          <input type="password" className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" value={nw} onChange={e=>setNw(e.target.value)} required />
        </label>
        <label className="block text-sm font-medium text-slate-700">
          {t('Neues Passwort wiederholen')}
          <input type="password" className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm" value={nw2} onChange={e=>setNw2(e.target.value)} required />
        </label>
        <div className="flex items-center justify-between">
          <button type="button" onClick={logout} className="text-sm text-slate-500 hover:text-slate-700">{t('Abmelden')}</button>
          <button disabled={busy} className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-60 text-white font-medium rounded-lg px-4 py-2 text-sm">{busy ? t('Speichern...') : t('Speichern')}</button>
        </div>
      </form>
    </div>
  )
}

export default function Layout() {
  const [user, setUser] = useState(null)
  const [ver, setVer] = useState(null)
  const [open, setOpen] = useState(false)
  const loc = useLocation()
  const nav = useNavigate()

  useEffect(() => {
    me().then(d => setUser(d.data)).catch(() => { window.location.href = '/login' })
    api('/version').then(d => setVer(d.data)).catch(() => {})
  }, [])

  // Superadmin-only sections: redirect if a tenant user deep-links them.
  useEffect(() => {
    if (!user || user.role === 'superadmin') return
    if (['/crowdsec', '/iplist', '/tenants'].some(p => loc.pathname.startsWith(p))) nav('/', { replace: true })
  }, [user, loc.pathname, nav])

  const isSuper = user?.role === 'superadmin'
  const visibleNav = NAV.filter(n => !n.superOnly || user == null || isSuper)

  const navLink = ({ isActive }) =>
    `flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
      isActive ? 'bg-indigo-600 text-white shadow-sm' : 'text-slate-300 hover:bg-slate-800 hover:text-white'
    }`

  const loading = !user

  return (
    <div className="min-h-screen flex bg-slate-100">
      {/* Sidebar */}
      <aside className={`fixed lg:static inset-y-0 left-0 z-40 w-64 bg-slate-900 flex flex-col transform transition-transform ${open ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}`}>
        <div className="h-16 flex items-center gap-3 px-5 border-b border-slate-800">
          <div className="w-9 h-9 rounded-lg bg-indigo-600 flex items-center justify-center text-white">
            <Icon name="shield" className="w-5 h-5" />
          </div>
          <div>
            <div className="text-white font-bold leading-tight">Bastion</div>
            <div className="text-[11px] text-slate-400 leading-tight">{t('Control Panel')}</div>
          </div>
        </div>
        <nav className="flex-1 p-3 space-y-1 overflow-y-auto">
          {visibleNav.map(n => (
            <NavLink key={n.to} to={n.to} end={n.end} className={navLink} onClick={() => setOpen(false)}>
              <Icon name={n.icon} className="w-5 h-5" />
              {t(n.label)}
            </NavLink>
          ))}
        </nav>
        <div className="p-3 border-t border-slate-800">
          <div className="px-3 py-2 mb-1">
            <div className="text-sm text-white truncate">{user?.email || '—'}</div>
            <div className="text-xs text-slate-400">{user?.role || ''}</div>
          </div>
          <button onClick={logout} className="w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm text-slate-300 hover:bg-slate-800 hover:text-white">
            <Icon name="logout" className="w-5 h-5" stroke /> {t('Logout')}
          </button>
          <div className="px-3 pt-3 mt-1 border-t border-slate-800 text-[11px] text-slate-500">
            <div>Bastion v{ver?.current || '…'}</div>
            {ver?.update_available && (
              <a href={ver.url} target="_blank" rel="noreferrer" className="text-amber-400 hover:underline">
                {t('Update verfügbar: v{version}', { version: ver.latest })}
              </a>
            )}
          </div>
        </div>
      </aside>

      {open && <div className="fixed inset-0 bg-slate-900/40 z-30 lg:hidden" onClick={() => setOpen(false)} />}

      {/* Main */}
      <div className="flex-1 flex flex-col min-w-0">
        <header className="h-16 bg-white border-b border-slate-200 flex items-center justify-between px-4 sm:px-6 sticky top-0 z-20">
          <div className="flex items-center gap-3">
            <button className="lg:hidden text-slate-500" onClick={() => setOpen(true)}>
              <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 6h16M4 12h16M4 18h16" /></svg>
            </button>
            <h1 className="text-lg font-semibold text-slate-800">{t(titleFor(loc.pathname))}</h1>
          </div>
          <div className="flex items-center gap-3">
            <span className="hidden sm:inline-flex items-center gap-1.5 text-xs text-emerald-600 bg-emerald-50 px-2.5 py-1 rounded-full font-medium">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-500"></span> {t('Running')}
            </span>
            <div className="w-9 h-9 rounded-full bg-slate-200 flex items-center justify-center text-slate-600 font-semibold text-sm">
              {(user?.email || '?').charAt(0).toUpperCase()}
            </div>
          </div>
        </header>
        <main className="flex-1 p-4 sm:p-6 overflow-auto">
          {loading ? <div className="text-sm text-slate-400">…</div> : <ErrorBoundary key={loc.pathname}><Outlet /></ErrorBoundary>}
        </main>
      </div>

      {user && user.must_change_password && <ForceChangePassword />}
    </div>
  )
}
