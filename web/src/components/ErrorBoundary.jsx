import React from 'react'
import { t } from '../i18n.js'

export default class ErrorBoundary extends React.Component {
  constructor(props) {
    super(props)
    this.state = { error: null }
  }
  static getDerivedStateFromError(error) {
    return { error }
  }
  componentDidCatch(error, info) {
    console.error('UI error:', error, info)
  }
  render() {
    if (!this.state.error) return this.props.children

    if (this.props.fullPage) {
      return (
        <div className="min-h-screen flex items-center justify-center bg-slate-50 p-6">
          <div className="max-w-md text-center">
            <h1 className="text-xl font-bold text-slate-800 mb-2">{t('Etwas ist schiefgelaufen')}</h1>
            <p className="text-sm text-slate-500 mb-4">{t('Die Oberfläche konnte nicht geladen werden. Bitte lade die Seite neu.')}</p>
            <button onClick={() => window.location.reload()} className="bg-indigo-600 hover:bg-indigo-700 text-white rounded-lg px-4 py-2 text-sm font-medium">{t('Neu laden')}</button>
          </div>
        </div>
      )
    }

    return (
      <div className="bg-white rounded-xl border border-red-200 p-6 max-w-2xl mx-auto mt-10 text-center shadow-sm">
        <h2 className="text-base font-semibold text-slate-800 mb-1">{t('Diese Seite konnte nicht geladen werden.')}</h2>
        <p className="text-sm text-slate-500 mb-3">{t('Wechsle die Seite oder versuche es erneut – andere Bereiche funktionieren weiter.')}</p>
        <details className="text-left mb-4">
          <summary className="text-xs text-slate-400 cursor-pointer select-none">{t('Details')}</summary>
          <pre className="mt-2 text-[11px] leading-relaxed text-red-600 bg-red-50 rounded-lg p-3 whitespace-pre-wrap break-all">{String((this.state.error && this.state.error.message) || this.state.error)}</pre>
        </details>
        <button onClick={() => this.setState({ error: null })} className="bg-indigo-600 hover:bg-indigo-700 text-white rounded-lg px-4 py-2 text-sm font-medium">{t('Erneut versuchen')}</button>
      </div>
    )
  }
}
