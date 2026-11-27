import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { t } from '../i18n.js'

// DomainList shows the primary domain and at most the first alias. If more
// aliases exist, a "…" button opens a floating panel with every domain on its
// own line. The panel is rendered in a portal and closes on outside click,
// Escape, scroll or resize.
export default function DomainList({ domain, aliases = [], className = '', showPrimary = true }) {
  const [open, setOpen] = useState(false)
  const [pos, setPos] = useState({ top: 0, left: 0, width: 360 })
  const buttonRef = useRef(null)
  const panelRef = useRef(null)

  const extra = Array.isArray(aliases) ? aliases.filter(Boolean) : []
  const list = [domain, ...extra].filter(Boolean)
  const firstAlias = extra[0]
  const hasMore = extra.length > 1

  const show = (e) => {
    e.stopPropagation()
    const rect = buttonRef.current.getBoundingClientRect()
    const width = Math.min(360, window.innerWidth - 16)
    let left = rect.left
    if (left + width > window.innerWidth - 8) {
      left = Math.max(8, window.innerWidth - width - 8)
    }
    const estimatedHeight = Math.min(320, 52 + list.length * 24)
    let top = rect.bottom + 6
    if (top + estimatedHeight > window.innerHeight - 8) {
      top = Math.max(8, rect.top - estimatedHeight - 6)
    }
    setPos({ top, left, width })
    setOpen(true)
  }

  useEffect(() => {
    if (!open) return
    const onPointer = (e) => {
      if (panelRef.current?.contains(e.target)) return
      if (buttonRef.current?.contains(e.target)) return
      setOpen(false)
    }
    const onKey = (e) => {
      if (e.key === 'Escape') setOpen(false)
    }
    const onViewport = () => setOpen(false)
    document.addEventListener('mousedown', onPointer)
    document.addEventListener('keydown', onKey)
    window.addEventListener('scroll', onViewport, true)
    window.addEventListener('resize', onViewport)
    return () => {
      document.removeEventListener('mousedown', onPointer)
      document.removeEventListener('keydown', onKey)
      window.removeEventListener('scroll', onViewport, true)
      window.removeEventListener('resize', onViewport)
    }
  }, [open])

  return (
    <span className={`block min-w-0 ${className}`}>
      {showPrimary && domain && (
        <span className="block truncate font-medium text-slate-800">{domain}</span>
      )}
      {firstAlias && (
        <span className="flex items-center gap-1 min-w-0 text-xs text-slate-400">
          <span className="truncate">{firstAlias}</span>
          {hasMore && (
            <button
              ref={buttonRef}
              type="button"
              onClick={show}
              aria-expanded={open}
              aria-haspopup="dialog"
              aria-label={t('Alle Domains anzeigen')}
              className="shrink-0 px-1 -my-0.5 rounded text-slate-400 hover:text-indigo-600 hover:bg-slate-100 font-semibold leading-none"
            >…</button>
          )}
        </span>
      )}
      {open && createPortal(
        <div
          ref={panelRef}
          role="dialog"
          aria-label={t('Domains')}
          onClick={(e) => e.stopPropagation()}
          className="fixed z-[60] rounded-xl border border-slate-200 bg-white shadow-2xl p-3 text-sm"
          style={{ top: pos.top, left: pos.left, width: pos.width }}
        >
          <div className="text-[11px] font-semibold uppercase tracking-wide text-slate-400 mb-2">{t('Domains')}</div>
          <div className="space-y-1 max-h-72 overflow-y-auto">
            {list.map((d, i) => (
              <div key={d + '|' + i} className={`break-all ${i === 0 ? 'font-medium text-slate-800' : 'text-slate-600'}`}>{d}</div>
            ))}
          </div>
        </div>,
        document.body
      )}
    </span>
  )
}
