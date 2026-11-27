const BASE = '/api/v1'

export function getToken() { return localStorage.getItem('token') }
export function getRefresh() { return localStorage.getItem('refresh_token') }
export function getUser() { try { return JSON.parse(localStorage.getItem('bastion_user')) } catch { return null } }
export function setUser(u) { try { localStorage.setItem('bastion_user', JSON.stringify(u)) } catch {} }
export function setTokens(data) {
  if (data && data.tokens) {
    localStorage.setItem('token', data.tokens.access_token)
    if (data.tokens.refresh_token) localStorage.setItem('refresh_token', data.tokens.refresh_token)
  }
}

let refreshing = null
async function doRefresh() {
  const rt = getRefresh()
  if (!rt) return null
  try {
    const res = await fetch(BASE + '/auth/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: rt })
    })
    if (!res.ok) return null
    const data = await res.json()
    if (data && data.data) {
      setTokens(data.data)
      if (data.data.user) setUser(data.data.user)
      return data
    }
  } catch { /* ignore */ }
  return null
}

export async function api(path, opts = {}) {
  const { meta, ...init } = opts
  const headers = { 'Content-Type': 'application/json', ...(init.headers || {}) }
  const t = getToken()
  if (t) headers['Authorization'] = 'Bearer ' + t
  let res = await fetch(BASE + path, { ...init, headers })
  if (res.status === 401 && getRefresh()) {
    if (!refreshing) refreshing = doRefresh().finally(() => { refreshing = null })
    const d = await refreshing
    if (d) {
      const headers2 = { 'Content-Type': 'application/json', ...(init.headers || {}) }
      const t2 = getToken()
      if (t2) headers2['Authorization'] = 'Bearer ' + t2
      res = await fetch(BASE + path, { ...init, headers: headers2 })
    }
  }
  if (res.status === 401) {
    localStorage.removeItem('token')
    localStorage.removeItem('refresh_token')
    localStorage.removeItem('bastion_user')
    if (window.location.pathname !== '/login') window.location.href = '/login'
    throw new Error('unauthorized')
  }
  let data = {}
  try { data = await res.json() } catch (e) { data = {} }
  if (!res.ok) {
    const err = new Error(data.error || res.statusText || 'request failed')
    err.code = data.code
    err.status = res.status
    throw err
  }
  if (meta) {
    return { data: data.data, total: Number(res.headers.get('X-Total-Count') || 0) }
  }
  return data
}

export const login = (email, password) =>
  api('/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) })
export const loginMFA = (mfa_token, code) =>
  api('/auth/login/mfa', { method: 'POST', body: JSON.stringify({ mfa_token, code }) })
export const mfaSetup = () => api('/auth/mfa/setup', { method: 'POST' })
export const mfaEnable = (code) =>
  api('/auth/mfa/enable', { method: 'POST', body: JSON.stringify({ code }) })
export const mfaDisable = (code, password) =>
  api('/auth/mfa/disable', { method: 'POST', body: JSON.stringify({ code, password }) })
export const changePassword = (current_password, new_password) =>
  api('/auth/change-password', { method: 'POST', body: JSON.stringify({ current_password, new_password }) })
export const me = () => api('/auth/me')
export const stats = () => api('/stats')

export function logout() {
  const rt = getRefresh()
  localStorage.removeItem('token')
  localStorage.removeItem('refresh_token')
  localStorage.removeItem('bastion_user')
  if (rt) {
    fetch(BASE + '/auth/logout', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: rt })
    }).catch(() => {})
  }
  window.location.href = '/login'
}
