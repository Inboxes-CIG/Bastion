import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { login, loginMFA } from '../api.js'
import Icon from '../components/Icon.jsx'
import { t, currentLang, setLang, LANGS } from '../i18n.js'

export default function Login() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [mfaToken, setMfaToken] = useState('')
  const [code, setCode] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const nav = useNavigate()

  const submit = async (e) => {
    e.preventDefault()
    setErr(''); setBusy(true)
    try {
      if (mfaToken) {
        const d = await loginMFA(mfaToken, code)
        localStorage.setItem('token', d.data.tokens.access_token)
        nav('/')
        return
      }
      const d = await login(email, password)
      if (d.data.mfa_required) {
        setMfaToken(d.data.mfa_token)
        setCode('')
        return
      }
      localStorage.setItem('token', d.data.tokens.access_token)
      nav('/')
    } catch (e) { setErr(e.message) } finally { setBusy(false) }
  }

  const back = () => { setMfaToken(''); setCode(''); setErr('') }

  return (
    <div className="min-h-screen flex">
      <div className="hidden lg:flex w-1/2 bg-slate-900 text-white flex-col justify-between p-12 relative overflow-hidden">
        <div className="absolute -top-24 -right-24 w-96 h-96 rounded-full bg-indigo-600/20 blur-3xl" />
        <div className="absolute bottom-0 -left-24 w-96 h-96 rounded-full bg-sky-500/10 blur-3xl" />
        <div className="flex items-center gap-3 relative">
          <div className="w-10 h-10 rounded-xl bg-indigo-600 flex items-center justify-center"><Icon name="shield" className="w-6 h-6" /></div>
          <span className="text-xl font-bold">Bastion</span>
        </div>
        <div className="relative">
          <h2 className="text-3xl font-bold leading-tight mb-4">{t('Schutz für deine')}<br/>{t('Web-Anwendungen.')}</h2>
          <p className="text-slate-400 max-w-md">{t('Reverse-Proxy, TLS-Zertifikate, GeoIP-Filter.')}</p>
        </div>
        <div className="text-sm text-slate-500 relative flex items-center gap-3">
          <span>© 2026 Inboxes</span>
          <select value={currentLang()} onChange={e=>setLang(e.target.value)} className="bg-transparent border border-slate-700 rounded px-2 py-1 text-xs text-slate-300">
            {LANGS.map(l => <option key={l.value} value={l.value} className="text-slate-900">{l.label}</option>)}
          </select>
        </div>
      </div>

      <div className="flex-1 flex items-center justify-center p-6 bg-slate-50">
        <form onSubmit={submit} className="w-full max-w-sm">
          <div className="lg:hidden flex items-center gap-3 mb-8">
            <div className="w-10 h-10 rounded-xl bg-indigo-600 flex items-center justify-center text-white"><Icon name="shield" className="w-6 h-6" /></div>
            <span className="text-xl font-bold text-slate-800">Bastion</span>
          </div>

          {mfaToken ? (
            <>
              <h1 className="text-2xl font-bold text-slate-800 mb-1">{t('Zwei-Faktor-Authentifizierung')}</h1>
              <p className="text-sm text-slate-500 mb-6">{t('Gib den 6-stelligen Code aus deiner Authenticator-App ein.')}</p>

              {err && <div className="mb-4 rounded-lg bg-red-50 border border-red-200 text-red-700 text-sm px-4 py-2.5">{err}</div>}

              <label className="block text-sm font-medium text-slate-700 mb-1">{t('Code')}</label>
              <input
                className="w-full rounded-lg border border-slate-300 px-3 py-2.5 text-sm mb-6 tracking-[0.4em] text-center focus:outline-none focus:ring-2 focus:ring-indigo-500/30 focus:border-indigo-500"
                value={code}
                onChange={e=>setCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                placeholder="000000"
                inputMode="numeric"
                autoComplete="one-time-code"
                autoFocus
                required
              />

              <button disabled={busy || code.length < 6} className="w-full bg-indigo-600 hover:bg-indigo-700 disabled:opacity-60 text-white font-medium rounded-lg py-2.5 transition-colors">
                {busy ? t('Prüfe...') : t('Bestätigen')}
              </button>
              <button type="button" onClick={back} className="w-full mt-3 text-sm text-slate-500 hover:text-slate-700">
                {t('Zurück zur Anmeldung')}
              </button>
            </>
          ) : (
            <>
              <h1 className="text-2xl font-bold text-slate-800 mb-1">{t('Willkommen zurück')}</h1>
              <p className="text-sm text-slate-500 mb-6">{t('Melde dich an, um fortzufahren.')}</p>

              {err && <div className="mb-4 rounded-lg bg-red-50 border border-red-200 text-red-700 text-sm px-4 py-2.5">{err}</div>}

              <label className="block text-sm font-medium text-slate-700 mb-1">{t('E-Mail')}</label>
              <input className="w-full rounded-lg border border-slate-300 px-3 py-2.5 text-sm mb-4 focus:outline-none focus:ring-2 focus:ring-indigo-500/30 focus:border-indigo-500" value={email} onChange={e=>setEmail(e.target.value)} placeholder={t('E-Mail')} autoComplete="username" required />

              <label className="block text-sm font-medium text-slate-700 mb-1">{t('Passwort')}</label>
              <input type="password" autoComplete="current-password" className="w-full rounded-lg border border-slate-300 px-3 py-2.5 text-sm mb-6 focus:outline-none focus:ring-2 focus:ring-indigo-500/30 focus:border-indigo-500" value={password} onChange={e=>setPassword(e.target.value)} required />

              <button disabled={busy} className="w-full bg-indigo-600 hover:bg-indigo-700 disabled:opacity-60 text-white font-medium rounded-lg py-2.5 transition-colors">
                {busy ? t('Anmelden...') : t('Anmelden')}
              </button>
            </>
          )}
        </form>
      </div>
    </div>
  )
}
