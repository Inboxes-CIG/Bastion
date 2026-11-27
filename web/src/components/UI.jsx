import React, { useState } from 'react'
import Icon from './Icon.jsx'

export function Button({ children, variant = 'primary', size = 'md', className = '', ...props }) {
  const base = 'inline-flex items-center justify-center gap-2 font-medium rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed'
  const sizes = { sm: 'px-3 py-1.5 text-sm', md: 'px-4 py-2 text-sm', lg: 'px-5 py-2.5 text-base' }
  const variants = {
    primary: 'bg-indigo-600 text-white hover:bg-indigo-700 shadow-sm',
    secondary: 'bg-white text-slate-700 border border-slate-300 hover:bg-slate-50',
    danger: 'bg-red-600 text-white hover:bg-red-700',
    ghost: 'text-slate-600 hover:bg-slate-100',
    success: 'bg-emerald-600 text-white hover:bg-emerald-700',
  }
  return (
    <button className={`${base} ${sizes[size]} ${variants[variant]} ${className}`} {...props}>
      {children}
    </button>
  )
}

export function Badge({ children, variant = 'slate' }) {
  const variants = {
    slate: 'bg-slate-100 text-slate-700',
    green: 'bg-emerald-100 text-emerald-700',
    red: 'bg-red-100 text-red-700',
    amber: 'bg-amber-100 text-amber-700',
    indigo: 'bg-indigo-100 text-indigo-700',
    blue: 'bg-blue-100 text-blue-700',
  }
  return <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${variants[variant]}`}>{children}</span>
}

export function Card({ children, className = '', title, subtitle, actions }) {
  return (
    <div className={`bg-white rounded-xl border border-slate-200 shadow-sm ${className}`}>
      {(title || actions) && (
        <div className="flex items-center justify-between px-5 py-4 border-b border-slate-100">
          <div className="min-w-0">
            {title && <h3 className="font-semibold text-slate-800">{title}</h3>}
            {subtitle && <p className="text-sm text-slate-500 mt-0.5">{subtitle}</p>}
          </div>
          {actions && <div className="flex gap-2">{actions}</div>}
        </div>
      )}
      <div className="p-5">{children}</div>
    </div>
  )
}

export function StatCard({ label, value, icon, accent = 'indigo', hint }) {
  const accents = {
    indigo: 'bg-indigo-50 text-indigo-600',
    emerald: 'bg-emerald-50 text-emerald-600',
    amber: 'bg-amber-50 text-amber-600',
    rose: 'bg-rose-50 text-rose-600',
    sky: 'bg-sky-50 text-sky-600',
  }
  return (
    <div className="bg-white rounded-xl border border-slate-200 shadow-sm p-5 flex items-center gap-4">
      <div className={`w-12 h-12 rounded-lg flex items-center justify-center ${accents[accent]}`}>
        <Icon name={icon} className="w-6 h-6" />
      </div>
      <div>
        <div className="text-2xl font-bold text-slate-800">{value}</div>
        <div className="text-sm text-slate-500">{label}</div>
        {hint && <div className="text-xs text-slate-400 mt-0.5">{hint}</div>}
      </div>
    </div>
  )
}

export function Modal({ open, onClose, title, children, footer, size = 'md' }) {
  if (!open) return null
  const sizes = { sm: 'max-w-md', md: 'max-w-xl', lg: 'max-w-3xl', xl: 'max-w-5xl' }
  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center p-4 sm:p-8 overflow-y-auto bg-slate-900/50 backdrop-blur-sm" onClick={onClose}>
      <div className={`bg-white rounded-xl shadow-2xl w-full ${sizes[size]} my-8`} onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-100">
          <h3 className="font-semibold text-lg text-slate-800">{title}</h3>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-700"><Icon name="x" className="w-5 h-5" /></button>
        </div>
        <div className="px-6 py-5">{children}</div>
        {footer && <div className="px-6 py-4 border-t border-slate-100 flex justify-end gap-2">{footer}</div>}
      </div>
    </div>
  )
}

export function Field({ label, hint, children, className = '' }) {
  return (
    <div className={`flex flex-col gap-1 ${className}`}>
      {label && <label className="text-sm font-medium text-slate-700">{label}</label>}
      {children}
      {hint && <span className="text-xs text-slate-400">{hint}</span>}
    </div>
  )
}

const inputCls = 'w-full rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-800 placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-indigo-500/30 focus:border-indigo-500 bg-white'

export function Input(props) { return <input className={inputCls} {...props} /> }
export function Textarea(props) { return <textarea className={inputCls} rows={4} {...props} /> }
export function Select({ children, ...props }) { return <select className={inputCls} {...props}>{children}</select> }

export function Toggle({ checked, onChange, label, disabled }) {
  return (
    <label className={`inline-flex items-center gap-2 ${disabled ? 'opacity-60' : 'cursor-pointer'}`}>
      <span className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${checked ? 'bg-indigo-600' : 'bg-slate-300'}`}>
        <span className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${checked ? 'translate-x-4' : 'translate-x-0.5'}`} />
      </span>
      {label && <span className="text-sm text-slate-700">{label}</span>}
      <input type="checkbox" className="sr-only" checked={checked} onChange={(e) => onChange(e.target.checked)} disabled={disabled} />
    </label>
  )
}

export function Table({ columns, rows, empty = 'No data', renderRow }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-slate-200 bg-slate-50/70">
            {columns.map((c, i) => (
              <th key={i} className="px-4 py-3 text-left font-semibold text-slate-600 whitespace-nowrap">{c}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 && (
            <tr><td colSpan={columns.length} className="px-4 py-10 text-center text-slate-400">{empty}</td></tr>
          )}
          {rows.map(renderRow)}
        </tbody>
      </table>
    </div>
  )
}

export function EmptyState({ icon = 'info', title, description, action }) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <div className="w-14 h-14 rounded-full bg-slate-100 flex items-center justify-center text-slate-400 mb-4">
        <Icon name={icon} className="w-7 h-7" />
      </div>
      <h3 className="font-semibold text-slate-700">{title}</h3>
      {description && <p className="text-sm text-slate-500 mt-1 max-w-sm">{description}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  )
}

export function Spinner({ label = 'Loading...' }) {
  return (
    <div className="flex items-center justify-center py-16 text-slate-400 gap-3">
      <svg className="animate-spin w-5 h-5" viewBox="0 0 24 24" fill="none">
        <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" className="opacity-25" />
        <path d="M4 12a8 8 0 018-8" stroke="currentColor" strokeWidth="4" strokeLinecap="round" className="opacity-75" />
      </svg>
      <span className="text-sm">{label}</span>
    </div>
  )
}

export function Alert({ variant = 'info', children }) {
  const variants = {
    info: 'bg-blue-50 text-blue-800 border-blue-200',
    success: 'bg-emerald-50 text-emerald-800 border-emerald-200',
    error: 'bg-red-50 text-red-800 border-red-200',
    warn: 'bg-amber-50 text-amber-800 border-amber-200',
  }
  const icons = { info: 'info', success: 'check', error: 'alert', warn: 'alert' }
  return (
    <div className={`flex items-start gap-2 rounded-lg border px-4 py-3 text-sm ${variants[variant]}`}>
      <Icon name={icons[variant]} className="w-4 h-4 mt-0.5 shrink-0" />
      <div>{children}</div>
    </div>
  )
}

export function MultiSelect({ options, value = [], onChange, placeholder = 'Auswaehlen...' }) {
  const [open, setOpen] = useState(false)
  const [q, setQ] = useState('')
  const selected = new Set(value)
  const toggle = (code) => {
    const next = new Set(selected)
    if (next.has(code)) next.delete(code); else next.add(code)
    onChange([...next])
  }
  const filtered = options.filter(o =>
    o.code.toLowerCase().includes(q.toLowerCase()) || o.name.toLowerCase().includes(q.toLowerCase()))
  return (
    <div className="relative">
      <button type="button" onClick={() => setOpen(!open)}
        className="w-full min-h-[42px] rounded-lg border border-slate-300 bg-white px-3 py-2 text-left text-sm flex flex-wrap items-center gap-1 focus:outline-none focus:ring-2 focus:ring-indigo-500/30">
        {value.length === 0 && <span className="text-slate-400">{placeholder}</span>}
        {value.map(code => (
          <span key={code} className="inline-flex items-center gap-1 bg-indigo-50 text-indigo-700 rounded px-1.5 py-0.5 text-xs font-medium">
            {code}
            <span onClick={(e)=>{e.stopPropagation(); toggle(code)}} className="cursor-pointer text-indigo-400 hover:text-indigo-700">x</span>
          </span>
        ))}
        <span className="ml-auto text-slate-400">v</span>
      </button>
      {open && (
        <div className="absolute z-40 mt-1 w-full bg-white border border-slate-200 rounded-lg shadow-lg">
          <div className="p-2 border-b border-slate-100">
            <input autoFocus value={q} onChange={e=>setQ(e.target.value)} placeholder="Land suchen..."
              className="w-full rounded border border-slate-200 px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500/30" />
          </div>
          <div className="max-h-56 overflow-y-auto">
            {filtered.map(o => (
              <label key={o.code} className="flex items-center gap-2 px-3 py-1.5 hover:bg-slate-50 cursor-pointer text-sm">
                <input type="checkbox" checked={selected.has(o.code)} onChange={()=>toggle(o.code)} />
                <span className="font-mono text-xs text-slate-400 w-7">{o.code}</span>
                <span className="text-slate-700">{o.name}</span>
              </label>
            ))}
            {filtered.length===0 && <div className="px-3 py-4 text-sm text-slate-400 text-center">Keine Treffer</div>}
          </div>
          <div className="flex items-center justify-between px-3 py-2 border-t border-slate-100">
            <button type="button" onClick={()=>onChange([])} className="text-xs text-slate-500 hover:text-red-600">Alle entfernen</button>
            <button type="button" onClick={()=>setOpen(false)} className="text-xs font-medium text-indigo-600">Fertig</button>
          </div>
        </div>
      )}
    </div>
  )
}
